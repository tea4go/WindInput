package theme

import (
	"image/color"
	"testing"
)

// views_spacing_test.go — 守护间距字段（line_spacing/col_gap/title_gap）的 merge + resolve。
// 消费层（tooltip/toast 行列距、footer 箭头 padding）兜底零回归由 internal/ui 既有测试覆盖；
// 本测试守护「新值正确解析进 RVNode」+「merge 覆盖语义」（golden 不含这些字段，故单独守护）。

func TestViewNode_SpacingResolve(t *testing.T) {
	pal := ResolvedPalette{Tokens: map[string]color.Color{}}
	resolve := func(c ColorRef) color.Color { return resolveColorToken(c.Select(false), pal) }

	rv := resolveViewNode(ViewNode{
		LineSpacing: dimp(3),
		ColGap:      dimp(20),
		TitleGap:    dimp(10),
	}, resolve, nil, nil, nil)
	if rv.LineSpacing.Value != 3 {
		t.Errorf("LineSpacing=%d want 3", rv.LineSpacing.Value)
	}
	if rv.ColGap.Value != 20 {
		t.Errorf("ColGap=%d want 20", rv.ColGap.Value)
	}
	if rv.TitleGap.Value != 10 {
		t.Errorf("TitleGap=%d want 10", rv.TitleGap.Value)
	}

	// 未配 → 零值（消费层据此兜底现状，零回归）。
	empty := resolveViewNode(ViewNode{}, resolve, nil, nil, nil)
	if empty.LineSpacing != (Dimension{}) || empty.ColGap != (Dimension{}) || empty.TitleGap != (Dimension{}) {
		t.Errorf("未配间距应为零值，got line=%v col=%v title=%v", empty.LineSpacing, empty.ColGap, empty.TitleGap)
	}
}

func TestViewNode_SpacingMerge(t *testing.T) {
	base := ViewNode{LineSpacing: dimp(2), ColGap: dimp(16)}
	ov := ViewNode{ColGap: dimp(24), TitleGap: dimp(8)}
	out := mergeViewNode(base, ov)
	if out.LineSpacing == nil || out.LineSpacing.Value != 2 {
		t.Errorf("LineSpacing 应保留 base=2，got %v", out.LineSpacing)
	}
	if out.ColGap == nil || out.ColGap.Value != 24 {
		t.Errorf("ColGap 应被 ov 覆盖为 24，got %v", out.ColGap)
	}
	if out.TitleGap == nil || out.TitleGap.Value != 8 {
		t.Errorf("TitleGap 应取 ov=8，got %v", out.TitleGap)
	}
}
