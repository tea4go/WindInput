package theme

import (
	"image/color"
	"testing"
)

func testPalette() ResolvedPalette {
	return ResolvedPalette{
		Shadow: color.RGBA{1, 1, 1, 15},
		CandidateWindow: ResolvedCandidateWindowPalette{
			Background:  color.RGBA{255, 255, 255, 255},
			Border:      color.RGBA{200, 200, 200, 255},
			Text:        color.RGBA{30, 30, 30, 255},
			Comment:     color.RGBA{150, 150, 150, 255},
			IndexBg:     color.RGBA{66, 133, 244, 255},
			IndexText:   color.RGBA{255, 255, 255, 255},
			HoverBg:     color.RGBA{230, 240, 255, 255},
			SelectedBg:  color.RGBA{210, 228, 255, 255},
			PreeditBg:   color.RGBA{240, 240, 240, 255},
			PreeditText: color.RGBA{100, 100, 100, 255},
			AccentBar:   color.RGBA{0, 120, 212, 255},
		},
	}
}

func TestResolveCandidateViewColor(t *testing.T) {
	pal := testPalette()
	cases := map[string]color.Color{
		"${background}":  pal.CandidateWindow.Background,
		"${index_bg}":    pal.CandidateWindow.IndexBg,
		"${selected_bg}": pal.CandidateWindow.SelectedBg,
		"${comment}":     pal.CandidateWindow.Comment,
		"${accent}":      pal.CandidateWindow.AccentBar,
		"${shadow}":      pal.Shadow,
	}
	for tok, want := range cases {
		if got := resolveCandidateViewColor(tok, pal); got != want {
			t.Errorf("%s → got %v, want %v", tok, got, want)
		}
	}
	if got := resolveCandidateViewColor("#FF0000", pal); got == nil {
		t.Error("hex 应解析为非 nil")
	}
	if resolveCandidateViewColor("", pal) != nil {
		t.Error("空串应为 nil")
	}
	if resolveCandidateViewColor("${unknown}", pal) != nil {
		t.Error("未知 token 应为 nil")
	}
}

func TestResolveCandidateViews_GeometryAndColor(t *testing.T) {
	pal := testPalette()
	v := defaultViews()
	v.Window.Background = ViewFill{Color: "${background}"}
	rv := ResolveCandidateViews(v, pal)

	if rv.Window.PadLeft != 8 || rv.Window.PadTop != 8 {
		t.Errorf("Window padding 应为 8, got L=%d T=%d", rv.Window.PadLeft, rv.Window.PadTop)
	}
	if rv.Item.BorderRadius != 4 {
		t.Errorf("Item radius 应为 4, got %d", rv.Item.BorderRadius)
	}
	if rv.ItemSpacing != 12 || rv.WindowGap != 4 || rv.ShadowOffset != 2 {
		t.Errorf("metrics 顶层错: spacing=%d gap=%d shadow=%d", rv.ItemSpacing, rv.WindowGap, rv.ShadowOffset)
	}
	if rv.AccentBarWidth != 3 || rv.AccentBarOffset != 1 || rv.AccentBarHRatio != 0.6 {
		t.Errorf("accent metrics 错: w=%d off=%d hr=%v", rv.AccentBarWidth, rv.AccentBarOffset, rv.AccentBarHRatio)
	}
	if rv.Index.BgColor != pal.CandidateWindow.IndexBg {
		t.Errorf("Index.BgColor 默认应=palette.IndexBg, got %v", rv.Index.BgColor)
	}
	if rv.Text.TextColor != pal.CandidateWindow.Text {
		t.Errorf("Text.TextColor 默认应=palette.Text, got %v", rv.Text.TextColor)
	}
	if rv.Item.SelectedBg != pal.CandidateWindow.SelectedBg || rv.Item.HoverBg != pal.CandidateWindow.HoverBg {
		t.Errorf("Item selected/hover 默认错")
	}
	if rv.Window.BgColor != pal.CandidateWindow.Background {
		t.Errorf("Window.BgColor token 覆盖错, got %v", rv.Window.BgColor)
	}
	if rv.ShadowColor != pal.Shadow {
		t.Errorf("ShadowColor 应=palette.Shadow, got %v", rv.ShadowColor)
	}
	if rv.Text.FontSize != 0 || rv.ItemHeight != 0 || rv.VerticalMaxWidth != 0 {
		t.Errorf("字号/行高/竖排max 本层应为零值（ui 回填）, got fs=%v ih=%v vmax=%v", rv.Text.FontSize, rv.ItemHeight, rv.VerticalMaxWidth)
	}
}
