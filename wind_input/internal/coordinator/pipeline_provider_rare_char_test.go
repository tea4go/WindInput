package coordinator

import "testing"

func TestRareCharProviderMeta(t *testing.T) {
	p := rareCharProvider{c: &Coordinator{}, id: "rare"}
	if p.ID() != ProviderRareChar {
		t.Errorf("ID: got %q, want %q", p.ID(), ProviderRareChar)
	}
	if p.Rank() != 50 {
		t.Errorf("Rank: got %d, want 50", p.Rank())
	}
	// 段位序：拼音(40) < 生僻字(50) < 英文(60)
	if !(pinyinProvider{}.Rank() < p.Rank() && p.Rank() < englishProvider{}.Rank()) {
		t.Errorf("段位序应为 拼音<生僻字<英文，实际 %d/%d/%d",
			pinyinProvider{}.Rank(), p.Rank(), englishProvider{}.Rank())
	}
	// 语言类（生僻字）段位须高于全部结构化基础候选（date/calc/number）。
	for _, bp := range (&Coordinator{}).quickInputBaseProviders() {
		if p.Rank() <= bp.Rank() {
			t.Errorf("生僻字 Rank(%d) 应大于结构化 %s(%d)", p.Rank(), bp.ID(), bp.Rank())
		}
	}
}

// TestRareCharProviderNilSafe：reg 缺失 / id 空 / buffer 空 / 实例未加载 均返回 nil。
func TestRareCharProviderNilSafe(t *testing.T) {
	// specialModeReg == nil
	if got := (rareCharProvider{c: &Coordinator{}, id: "rare"}).Query("ni"); got != nil {
		t.Errorf("nil specialModeReg 应返回 nil，got %v", got)
	}
	// id 为空
	if got := (rareCharProvider{c: &Coordinator{}, id: ""}).Query("ni"); got != nil {
		t.Errorf("空 id 应返回 nil，got %v", got)
	}
	// buffer 为空
	if got := (rareCharProvider{c: &Coordinator{}, id: "rare"}).Query(""); got != nil {
		t.Errorf("空 buffer 应返回 nil，got %v", got)
	}
	// 注：「实例已注册但 inst.table==nil（eager 加载未完成）→ 返回 nil」路径需构造 specialModeReg
	// 桩，成本较高，留 F5 集成 + 真机测试覆盖。
}
