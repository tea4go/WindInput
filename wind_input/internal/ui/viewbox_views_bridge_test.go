package ui

import "testing"

// TestRenderConfigToViews_Bridge 验证合成桥从 parityConfig 产出预期几何值（逻辑像素）。
func TestRenderConfigToViews_Bridge(t *testing.T) {
	cfg := parityConfig() // Padding=8, CornerRadius=8, ItemHeight=32, FontSize=18, IndexFontSize=14, IndexStyle="circle"
	rv := NewRenderer(cfg).buildResolvedViews()

	if rv.Window.PadLeft != 8 || rv.Window.PadTop != 8 {
		t.Errorf("window padding 应为 8, got L=%d T=%d", rv.Window.PadLeft, rv.Window.PadTop)
	}
	if rv.Window.BorderRadius != 8 {
		t.Errorf("window border radius 应为 8, got %d", rv.Window.BorderRadius)
	}
	if rv.Item.PadLeft != 8 || rv.Item.PadRight != 8 {
		t.Errorf("item padding 应为 8, got L=%d R=%d", rv.Item.PadLeft, rv.Item.PadRight)
	}
	if rv.Item.BorderRadius != 4 {
		t.Errorf("item border radius 应为 4, got %d", rv.Item.BorderRadius)
	}
	if rv.Text.MarginLeft != 4 {
		t.Errorf("text margin left（序号间距）应为 4, got %d", rv.Text.MarginLeft)
	}
	if rv.Comment.MarginLeft != 8 {
		t.Errorf("comment margin left 应为 8, got %d", rv.Comment.MarginLeft)
	}
	if rv.Text.FontSize != 18 {
		t.Errorf("text font size 应为 18, got %v", rv.Text.FontSize)
	}
	if rv.Index.FontSize != 14 {
		t.Errorf("index font size 应为 14, got %v", rv.Index.FontSize)
	}
	if rv.ItemHeight != 32 {
		t.Errorf("item height 应为 32, got %v", rv.ItemHeight)
	}
	if rv.ItemSpacing != 12 {
		t.Errorf("circle index 的 item spacing 应为 12, got %d", rv.ItemSpacing)
	}
	if rv.VerticalMaxWidth != 600 {
		t.Errorf("vertical max width 应回退 600, got %v", rv.VerticalMaxWidth)
	}
}

// TestRenderConfigToViews_TextIndexSpacing 文本序号时 spacing=16。
func TestRenderConfigToViews_TextIndexSpacing(t *testing.T) {
	cfg := parityConfig()
	cfg.IndexStyle = "text"
	rv := NewRenderer(cfg).buildResolvedViews()
	if rv.ItemSpacing != 16 {
		t.Errorf("text index 的 item spacing 应为 16, got %d", rv.ItemSpacing)
	}
}

// TestBridge_ItemRadius 验证 item_radius 经 cfg 生效（修复 hardcode 4 死配置）。
func TestBridge_ItemRadius(t *testing.T) {
	cfg := parityConfig()
	cfg.ItemRadius = 6
	if rv := (NewRenderer(cfg)).buildResolvedViews(); rv.Item.BorderRadius != 6 {
		t.Errorf("item radius 应=cfg.ItemRadius 6, got %d", rv.Item.BorderRadius)
	}
	cfg.ItemRadius = 0
	if rv := (NewRenderer(cfg)).buildResolvedViews(); rv.Item.BorderRadius != 4 {
		t.Errorf("item radius 0 应回退默认 4, got %d", rv.Item.BorderRadius)
	}
}

// TestBridgeColors 验证合成桥把 cfg 颜色填入对应 RVNode 颜色字段。
func TestBridgeColors(t *testing.T) {
	cfg := parityConfig()
	rv := NewRenderer(cfg).buildResolvedViews()
	if rv.Window.BgColor != cfg.BackgroundColor {
		t.Error("window bg 应=cfg.BackgroundColor")
	}
	if rv.Item.SelectedBg != cfg.SelectedBgColor {
		t.Error("item selected bg 应=cfg.SelectedBgColor")
	}
	if rv.Index.BgColor != cfg.IndexBgColor {
		t.Error("index bg 应=cfg.IndexBgColor")
	}
	if rv.Text.TextColor != cfg.TextColor {
		t.Error("text color 应=cfg.TextColor")
	}
}
