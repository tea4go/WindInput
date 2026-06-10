package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestPauseResume_ConcurrentAccess 回归测试：Pause/Resume 热替换 db 期间，
// 并发读写不得 panic（nil 解引用）也不得触发 data race（go test -race）。
// 暂停窗口内的访问应返回 ErrPaused 或正常结果，绝不崩溃。
func TestPauseResume_ConcurrentAccess(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "race.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 读写方：模拟按键热路径（查询 + 学词 + 异步词频）
	for i := range 4 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				word := fmt.Sprintf("w%d-%d", id, n)
				if err := s.AddUserWord("pinyin", "ce", word, 0); err != nil && !errors.Is(err, ErrPaused) {
					t.Errorf("AddUserWord: %v", err)
					return
				}
				if _, err := s.AllUserWords("pinyin"); err != nil && !errors.Is(err, ErrPaused) {
					t.Errorf("AllUserWords: %v", err)
					return
				}
				s.IncrementFreqAsync("pinyin", "ce", word)
			}
		}(i)
	}

	// 暂停/恢复方：模拟备份还原期间的热替换
	for range 20 {
		if err := s.Pause(); err != nil {
			t.Fatalf("Pause: %v", err)
		}
		time.Sleep(time.Millisecond)
		if err := s.Resume(""); err != nil {
			t.Fatalf("Resume: %v", err)
		}
		time.Sleep(time.Millisecond)
	}

	close(stop)
	wg.Wait()
}
