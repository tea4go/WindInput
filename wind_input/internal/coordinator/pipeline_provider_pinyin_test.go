package coordinator

import "testing"

func TestPinyinProviderMeta(t *testing.T) {
	p := pinyinProvider{c: &Coordinator{}}
	if p.ID() != ProviderPinyin {
		t.Errorf("ID: got %q, want %q", p.ID(), ProviderPinyin)
	}
	if p.Rank() != 40 {
		t.Errorf("Rank: got %d, want 40", p.Rank())
	}
	// Rank 数值越大段位越靠后。拼音（语言类）须排在全部结构化基础候选（date/calc/number）之后，
	// 即其 Rank 数值须大于它们。
	for _, bp := range (&Coordinator{}).quickInputBaseProviders() {
		if p.Rank() <= bp.Rank() {
			t.Errorf("拼音 Rank(%d) 数值应大于 %s(%d)（段位靠后）", p.Rank(), bp.ID(), bp.Rank())
		}
	}
}

// TestPinyinProviderNilSafe 验证引擎缺失 / 空 buffer 时安全返回空（Query 与 query 一致）。
func TestPinyinProviderNilSafe(t *testing.T) {
	p := pinyinProvider{c: &Coordinator{}} // engineMgr == nil

	if got := p.Query("ni"); got != nil {
		t.Errorf("nil 引擎应返回 nil 候选，got %v", got)
	}
	if cands, pre := p.query("ni"); cands != nil || pre != "" {
		t.Errorf("nil 引擎应返回 nil/空，got cands=%v pre=%q", cands, pre)
	}
	if got := p.Query(""); got != nil {
		t.Errorf("空 buffer 应返回 nil 候选，got %v", got)
	}
	// 注：「空 buffer + 非 nil engineMgr」路径（query 内 || 第二条件短路）依赖真实引擎，
	// engineMgr 是具体 *engine.Manager 难 mock，留 S4 集成 + 真机测试覆盖。
}
