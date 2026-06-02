package theme

import "testing"

func TestEdgeOr(t *testing.T) {
	if got := edgeOr(nil, 8); got != 8 {
		t.Errorf("nil 应回退默认 8, got %d", got)
	}
	if got := edgeOr(intp(0), 8); got != 0 {
		t.Errorf("显式 0 应保留, got %d", got)
	}
	if got := edgeOr(intp(5), 8); got != 5 {
		t.Errorf("显式值应生效, got %d", got)
	}
}

func TestMergeViews_PointerOverride(t *testing.T) {
	base := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(4)}}}
	ov := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(8)}}}
	got := mergeViews(base, ov)
	if got.Item.Border.Radius == nil || *got.Item.Border.Radius != 8 {
		t.Errorf("覆盖失败: %v", got.Item.Border.Radius)
	}
}

func TestMergeViews_NilKeepsBase(t *testing.T) {
	base := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(4)}, Padding: ViewEdges{Left: intp(8)}}}
	got := mergeViews(base, Views{})
	if got.Item.Border.Radius == nil || *got.Item.Border.Radius != 4 {
		t.Error("nil 覆盖应保留基线 radius")
	}
	if got.Item.Padding.Left == nil || *got.Item.Padding.Left != 8 {
		t.Error("nil 覆盖应保留基线 padding")
	}
}

func TestMergeViews_ExplicitZero(t *testing.T) {
	base := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(4)}}}
	ov := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(0)}}}
	got := mergeViews(base, ov)
	if got.Item.Border.Radius == nil || *got.Item.Border.Radius != 0 {
		t.Errorf("显式 0 应覆盖基线, got %v", got.Item.Border.Radius)
	}
}

func TestMergeViews_StatesRecursive(t *testing.T) {
	base := Views{Item: ViewNode{Selected: &ViewNode{Background: ViewFill{Color: "#base"}}}}
	ov := Views{Item: ViewNode{Selected: &ViewNode{Border: ViewBorder{Radius: intp(6)}}}}
	got := mergeViews(base, ov)
	if got.Item.Selected == nil {
		t.Fatal("Selected 不应为 nil")
	}
	if got.Item.Selected.Background.Color != "#base" {
		t.Errorf("Selected 应保留基线 bg, got %q", got.Item.Selected.Background.Color)
	}
	if got.Item.Selected.Border.Radius == nil || *got.Item.Selected.Border.Radius != 6 {
		t.Error("Selected 应合并覆盖 radius")
	}
}

func TestDefaultViews_Baseline(t *testing.T) {
	v := defaultViews()
	if got := edgeOr(v.Window.Padding.Left, -1); got != 8 {
		t.Errorf("window padding left 基线应为 8, got %d", got)
	}
	if got := edgeOr(v.Window.Border.Radius, -1); got != 8 {
		t.Errorf("window border radius 基线应为 8, got %d", got)
	}
	if got := edgeOr(v.Item.Border.Radius, -1); got != 4 {
		t.Errorf("item border radius 基线应为 4, got %d", got)
	}
	if got := edgeOr(v.Text.Margin.Left, -1); got != 4 {
		t.Errorf("text margin left 基线应为 4, got %d", got)
	}
	if got := edgeOr(v.Comment.Margin.Left, -1); got != 8 {
		t.Errorf("comment margin left 基线应为 8, got %d", got)
	}
}
