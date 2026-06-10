// manager_evict_test.go — evictStaleEnginesLocked 的保留集与资源释放行为
package engine

import (
	"log/slog"
	"testing"

	"github.com/huanfeng/wind_input/internal/candidate"
)

// fakeEvictEngine 实现 Engine + Close，记录是否被关闭。
type fakeEvictEngine struct {
	closed bool
}

func (f *fakeEvictEngine) Convert(string, int) ([]candidate.Candidate, error) { return nil, nil }
func (f *fakeEvictEngine) Reset()                                             {}
func (f *fakeEvictEngine) Type() string                                       { return "fake" }
func (f *fakeEvictEngine) Close() error                                       { f.closed = true; return nil }

func newEvictTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(slog.Default())
}

func TestEvictStaleEngines_KeepSetAndClose(t *testing.T) {
	m := newEvictTestManager(t)

	engines := map[string]*fakeEvictEngine{
		"current": {}, "prev": {}, "pinyin": {}, "stale1": {}, "stale2": {},
	}
	m.mu.Lock()
	for id, e := range engines {
		m.engines[id] = e
		m.warmedSchemas.Store(id, struct{}{})
	}
	m.currentID = "current"
	m.currentEngine = engines["current"]
	m.primaryPinyinID = "pinyin"

	m.evictStaleEnginesLocked("prev")
	m.mu.Unlock()

	for _, keep := range []string{"current", "prev", "pinyin"} {
		if _, ok := m.engines[keep]; !ok {
			t.Errorf("保留集方案 %q 不应被驱逐", keep)
		}
		if engines[keep].closed {
			t.Errorf("保留集方案 %q 的引擎不应被 Close", keep)
		}
	}
	for _, stale := range []string{"stale1", "stale2"} {
		if _, ok := m.engines[stale]; ok {
			t.Errorf("闲置方案 %q 应被驱逐", stale)
		}
		if !engines[stale].closed {
			t.Errorf("闲置方案 %q 的引擎应被 Close", stale)
		}
		if _, ok := m.warmedSchemas.Load(stale); ok {
			t.Errorf("闲置方案 %q 的预热标记应被清除", stale)
		}
	}
}

func TestEvictStaleEngines_SkipWarming(t *testing.T) {
	m := newEvictTestManager(t)

	warmingEng := &fakeEvictEngine{}
	m.mu.Lock()
	m.engines["warming-schema"] = warmingEng
	m.currentID = "current"
	m.warming.Store("warming-schema", struct{}{})

	m.evictStaleEnginesLocked("prev")
	m.mu.Unlock()

	if _, ok := m.engines["warming-schema"]; !ok {
		t.Fatal("预热中的方案不应被驱逐")
	}
	if warmingEng.closed {
		t.Fatal("预热中的引擎不应被 Close")
	}

	// 预热结束后再次切换应可驱逐
	m.mu.Lock()
	m.warming.Delete("warming-schema")
	m.evictStaleEnginesLocked("prev")
	m.mu.Unlock()

	if _, ok := m.engines["warming-schema"]; ok {
		t.Fatal("预热结束后闲置方案应被驱逐")
	}
	if !warmingEng.closed {
		t.Fatal("预热结束后驱逐应 Close 引擎")
	}
}
