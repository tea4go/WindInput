package store

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

// ErrPaused 表示存储处于暂停状态（Pause 后、Resume 前），读写被拒绝。
// 调用方可据此区分「暂停中」与真实 I/O 错误（如 flushFreqDeltas 的增量回放）。
var ErrPaused = errors.New("store: database is paused")

// boltOptions 统一的 bbolt 打开参数。
// Timeout：bolt.Open 在文件锁被其它进程持有时默认无限阻塞，设超时让
// Open/Resume/Restore 快速失败并向调用方报错。
// 注意：不要设置 NoSync——默认每次提交 fsync 是 user_data.db 崩溃安全的前提。
var boltOptions = &bolt.Options{Timeout: 2 * time.Second}

var (
	bucketMeta    = []byte("Meta")
	bucketSchemas = []byte("Schemas")
	bucketPhrases = []byte("Phrases")
)

// freqDeltaKey 词频增量的 map 键
type freqDeltaKey struct {
	schemaID, code, text string
}

// Store wraps a bbolt database with helpers for the wind_input schema.
//
// 并发模型：db 字段受 dbMu 保护。所有读写事务经 view/update 辅助方法在 RLock 下
// 执行（覆盖整个事务生命周期）；Pause/Resume 持写锁热替换 db，因此会自然等待
// 在途事务排空，且替换期间新请求返回 ErrPaused 而非对 nil db 解引用。
type Store struct {
	dbMu sync.RWMutex
	db   *bolt.DB // guarded by dbMu；nil 表示已暂停
	path string   // guarded by dbMu（仅 Resume 修改）

	freqDeltas  map[freqDeltaKey]int
	freqMu      sync.Mutex
	freqFlushCh chan struct{}
	freqDone    chan struct{}
	freqWg      sync.WaitGroup
}

// Open opens (or creates) the bbolt database at path and initialises top-level
// buckets and default Meta values.
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, boltOptions)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}
	s := &Store{
		db:          db,
		path:        path,
		freqDeltas:  make(map[freqDeltaKey]int),
		freqFlushCh: make(chan struct{}, 1),
		freqDone:    make(chan struct{}),
	}
	if err := initBuckets(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.freqWg.Add(1)
	go s.freqFlushLoop()
	return s, nil
}

// view 在 dbMu 读锁下执行只读事务；暂停状态返回 ErrPaused。
// 读锁覆盖整个事务，保证 Pause/Resume 的写锁等到在途事务结束才替换 db。
func (s *Store) view(fn func(*bolt.Tx) error) error {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	if s.db == nil {
		return ErrPaused
	}
	return s.db.View(fn)
}

// update 在 dbMu 读锁下执行读写事务；暂停状态返回 ErrPaused。
// 注：bbolt 自身串行化写事务，dbMu 读锁只保护 db 指针的生命周期，不引入额外写竞争。
func (s *Store) update(fn func(*bolt.Tx) error) error {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	if s.db == nil {
		return ErrPaused
	}
	return s.db.Update(fn)
}

// initBuckets creates required buckets and seeds Meta defaults on first open.
// 直接操作传入的 db（不经 view/update），供 Open 与持有 dbMu 写锁的 Resume 调用。
func initBuckets(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		meta, err := tx.CreateBucketIfNotExists(bucketMeta)
		if err != nil {
			return fmt.Errorf("create Meta bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(bucketSchemas); err != nil {
			return fmt.Errorf("create Schemas bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(bucketPhrases); err != nil {
			return fmt.Errorf("create Phrases bucket: %w", err)
		}

		// Stats bucket (with sub-buckets)
		statsBucket, err := tx.CreateBucketIfNotExists(bucketStats)
		if err != nil {
			return fmt.Errorf("create Stats bucket: %w", err)
		}
		if _, err := statsBucket.CreateBucketIfNotExists(bucketStatsDay); err != nil {
			return fmt.Errorf("create Stats/Daily bucket: %w", err)
		}
		if _, err := statsBucket.CreateBucketIfNotExists(bucketStatsMeta); err != nil {
			return fmt.Errorf("create Stats/Meta bucket: %w", err)
		}

		// Seed version if not yet set.
		if meta.Get([]byte("version")) == nil {
			if err := meta.Put([]byte("version"), []byte("1")); err != nil {
				return fmt.Errorf("set version: %w", err)
			}
		}

		// Seed device_id if not yet set.
		if meta.Get([]byte("device_id")) == nil {
			id := uuid.New().String()
			if err := meta.Put([]byte("device_id"), []byte(id)); err != nil {
				return fmt.Errorf("set device_id: %w", err)
			}
		}

		return nil
	})
}

// Close flushes pending freq deltas, stops background goroutines, and closes the database.
func (s *Store) Close() error {
	// 先停 freq loop：其最终 flush 经 update（RLock），必须发生在下面取写锁关库之前。
	if s.freqDone != nil {
		close(s.freqDone)
		s.freqWg.Wait()
	}
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Pause 暂停存储：关闭底层数据库以释放文件锁，但保留 Store 实例。
// 写锁会等待所有在途事务（view/update 的读锁）排空后才关库；
// 暂停期间后续读写方法返回 ErrPaused。
func (s *Store) Pause() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Resume 恢复存储：重新打开数据库文件。
// 使用创建时的路径，如果传入 newPath 非空则使用新路径。
func (s *Store) Resume(newPath string) error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db != nil {
		return fmt.Errorf("store is not paused")
	}
	path := s.path
	if newPath != "" {
		path = newPath
	}
	db, err := bolt.Open(path, 0600, boltOptions)
	if err != nil {
		return fmt.Errorf("store.Resume: %w", err)
	}
	// 持写锁期间直接初始化（不可经 update，避免自死锁），成功后才发布到 s.db。
	if err := initBuckets(db); err != nil {
		_ = db.Close()
		return fmt.Errorf("store.Resume init: %w", err)
	}
	s.db = db
	s.path = path
	return nil
}

// IsPaused 返回存储是否处于暂停状态
func (s *Store) IsPaused() bool {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	return s.db == nil
}

// DB returns the underlying *bolt.DB.
// 注意：返回的指针不受 dbMu 保护，仅限测试等无并发 Pause 的场景使用。
func (s *Store) DB() *bolt.DB {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	return s.db
}

// ClearSchema removes all data (UserWords, TempWords, Shadow, Freq) for a
// specific schema by deleting and recreating its bucket under Schemas.
func (s *Store) ClearSchema(schemaID string) error {
	return s.update(func(tx *bolt.Tx) error {
		schemas := tx.Bucket(bucketSchemas)
		if schemas == nil {
			return nil
		}
		key := []byte(schemaID)
		if schemas.Bucket(key) != nil {
			if err := schemas.DeleteBucket(key); err != nil {
				return fmt.Errorf("delete schema bucket %q: %w", schemaID, err)
			}
		}
		// 重新创建空 bucket，保持结构一致
		_, err := schemas.CreateBucket(key)
		return err
	})
}

// DeleteSchema completely removes a schema bucket from the Store.
// Unlike ClearSchema, this does not recreate an empty bucket.
func (s *Store) DeleteSchema(schemaID string) error {
	return s.update(func(tx *bolt.Tx) error {
		schemas := tx.Bucket(bucketSchemas)
		if schemas == nil {
			return nil
		}
		key := []byte(schemaID)
		if schemas.Bucket(key) != nil {
			return schemas.DeleteBucket(key)
		}
		return nil
	})
}

// ClearAllSchemas removes all schema data by deleting and recreating the
// top-level Schemas bucket. Meta (version, device_id) is preserved.
func (s *Store) ClearAllSchemas() error {
	return s.update(func(tx *bolt.Tx) error {
		if tx.Bucket(bucketSchemas) != nil {
			if err := tx.DeleteBucket(bucketSchemas); err != nil {
				return fmt.Errorf("delete Schemas bucket: %w", err)
			}
		}
		_, err := tx.CreateBucket(bucketSchemas)
		return err
	})
}

// Path returns the filesystem path of the database file.
func (s *Store) Path() string {
	return s.path
}

// GetMeta reads a value from the Meta bucket.
func (s *Store) GetMeta(key string) (string, error) {
	var value string
	err := s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		if b == nil {
			return fmt.Errorf("Meta bucket not found")
		}
		v := b.Get([]byte(key))
		if v != nil {
			value = string(v)
		}
		return nil
	})
	return value, err
}

// SetMeta writes a value to the Meta bucket.
func (s *Store) SetMeta(key, value string) error {
	return s.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		if b == nil {
			return fmt.Errorf("Meta bucket not found")
		}
		return b.Put([]byte(key), []byte(value))
	})
}

// schemaBucket navigates to Schemas -> {schemaID}.
// If create is true the bucket is created if absent; otherwise an error is
// returned when it does not exist.
func schemaBucket(tx *bolt.Tx, schemaID string, create bool) (*bolt.Bucket, error) {
	schemas := tx.Bucket(bucketSchemas)
	if schemas == nil {
		return nil, fmt.Errorf("Schemas bucket not found")
	}
	key := []byte(schemaID)
	if create {
		b, err := schemas.CreateBucketIfNotExists(key)
		if err != nil {
			return nil, fmt.Errorf("create schema bucket %q: %w", schemaID, err)
		}
		return b, nil
	}
	b := schemas.Bucket(key)
	if b == nil {
		return nil, fmt.Errorf("schema bucket %q not found", schemaID)
	}
	return b, nil
}

// schemaSubBucket navigates to Schemas -> {schemaID} -> {sub}.
func schemaSubBucket(tx *bolt.Tx, schemaID, sub string, create bool) (*bolt.Bucket, error) {
	parent, err := schemaBucket(tx, schemaID, create)
	if err != nil {
		return nil, err
	}
	key := []byte(sub)
	if create {
		b, err := parent.CreateBucketIfNotExists(key)
		if err != nil {
			return nil, fmt.Errorf("create sub-bucket %q/%q: %w", schemaID, sub, err)
		}
		return b, nil
	}
	b := parent.Bucket(key)
	if b == nil {
		return nil, fmt.Errorf("sub-bucket %q/%q not found", schemaID, sub)
	}
	return b, nil
}

// ListSchemaIDs returns a sorted list of all schema IDs that have data stored
// under the Schemas bucket.
func (s *Store) ListSchemaIDs() ([]string, error) {
	var ids []string
	err := s.view(func(tx *bolt.Tx) error {
		schemas := tx.Bucket(bucketSchemas)
		if schemas == nil {
			return nil
		}
		return schemas.ForEach(func(k, v []byte) error {
			// Only include sub-buckets (v is nil for buckets).
			if v == nil && schemas.Bucket(k) != nil {
				ids = append(ids, string(k))
			}
			return nil
		})
	})
	sort.Strings(ids)
	return ids, err
}
