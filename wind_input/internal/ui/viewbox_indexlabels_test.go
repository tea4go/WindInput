package ui

import "testing"

// TestEffectiveIndexLabels 验证序号标签四层优先级中的「全局覆盖 > 主题」两层
// （运行时 per-候选 IndexLabel 与默认回退由 indexLabel() 内部处理）。
func TestEffectiveIndexLabels(t *testing.T) {
	cfg := parityConfig()
	cfg.IndexLabels = "①②③④⑤⑥⑦⑧⑨⑩"
	r := NewRenderer(cfg)

	if got := r.effectiveIndexLabels(); got != "①②③④⑤⑥⑦⑧⑨⑩" {
		t.Errorf("全局空时应回退主题 labels, got %q", got)
	}

	r.config.GlobalIndexLabels = "1./2./3./4./5./6./7./8./9./0."
	if got := r.effectiveIndexLabels(); got != "1./2./3./4./5./6./7./8./9./0." {
		t.Errorf("全局非空时应覆盖主题, got %q", got)
	}
}
