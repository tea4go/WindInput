package coordinator

import "testing"

func TestEnglishProviderMeta(t *testing.T) {
	p := englishProvider{c: &Coordinator{}}
	if p.ID() != ProviderEnglish {
		t.Errorf("ID: got %q, want %q", p.ID(), ProviderEnglish)
	}
	if p.Rank() != 60 {
		t.Errorf("Rank: got %d, want 60", p.Rank())
	}
}

// TestEnglishProviderNilSafe：engineMgr 缺失 / buffer 空 均返回 nil
// （真实 SearchEnglish 查询依赖具体 *engine.Manager，留 F6 集成 + 真机测试覆盖）。
func TestEnglishProviderNilSafe(t *testing.T) {
	p := englishProvider{c: &Coordinator{}} // engineMgr == nil
	if got := p.Query("hello"); got != nil {
		t.Errorf("nil engineMgr 应返回 nil，got %v", got)
	}
	if got := p.Query(""); got != nil {
		t.Errorf("空 buffer 应返回 nil，got %v", got)
	}
}
