package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
	"github.com/huanfeng/wind_input/pkg/theme"
)

// applyParityThemePath 给未调 SetTheme 的渲染测试装上 parity 等价 theme 路径，并立即
// 填充 r.resolvedViews（合成桥退役后已无 RenderConfig 兜底；直接 build 的测试也需此值）。
func applyParityThemePath(r *Renderer) {
	views := themePathViews(8, 8)
	r.resolvedV25 = &theme.ResolvedV25{Palette: themePathPalette(), Behavior: theme.ResolvedBehavior{FontSize: 18, ShowPageNumber: true, VerticalMaxWidth: 600}}
	r.themeViews = &views
	r.refreshResolvedViews()
}

func parityConfig() RenderConfig {
	return RenderConfig{
		TextRenderMode: TextRenderModeFreetype,
		Layout:         config.LayoutHorizontal,
		FontSize:       18,
		ItemHeight:     32,
		IndexStyle:     "circle",
		HasAccentBar:   true,
		ShowPageNumber: true,
	}
}

func writePNG(t *testing.T, name string, img *image.RGBA) string {
	t.Helper()
	p := filepath.Join(os.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode %s: %v", name, err)
	}
	f.Close()
	return p
}

// TestViewEngine_ModeLabelGlow_DumpPNG 验证 ModeLabel(右对齐) + accent-glow(边框/input 叠加)。
func TestViewEngine_ModeLabelGlow_DumpPNG(t *testing.T) {
	cfg := parityConfig()
	cfg.ModeLabel = "临时拼音"
	cfg.ModeAccentColor = color.RGBA{0, 150, 136, 255} // teal glow
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	applyParityThemePath(r)
	cands := []Candidate{{Text: "中文", Index: 1}, {Text: "中", Index: 2}, {Text: "众", Index: 3}}
	img, _ := r.renderHorizontalV2(cands, "zhong", 5, 1, 1, 1, "", 0)
	p := writePNG(t, "wind_modelabel.png", img)
	t.Logf("ModeLabel+glow: %s (%dx%d)", p, img.Bounds().Dx(), img.Bounds().Dy())
}

// TestViewEngine_Embedded_DumpPNG 验证内嵌预编辑（编码内嵌候选行首）。
func TestViewEngine_Embedded_DumpPNG(t *testing.T) {
	cfg := parityConfig()
	cfg.PreeditMode = config.PreeditEmbedded
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	applyParityThemePath(r)
	cands := []Candidate{{Text: "中文", Index: 1}, {Text: "中", Index: 2}, {Text: "众", Index: 3}}
	img, _ := r.renderHorizontalV2(cands, "zhong", 5, 1, 1, 1, "", 0)
	p := writePNG(t, "wind_embedded.png", img)
	t.Logf("embedded: %s (%dx%d)", p, img.Bounds().Dx(), img.Bounds().Dy())
}

// TestViewEngine_VerticalTruncation_DumpPNG 竖排长候选省略号截断验证：
// 两个超长候选应被截断至 VerticalMaxWidth 以内，短候选不截断。
func TestViewEngine_VerticalTruncation_DumpPNG(t *testing.T) {
	cfg := parityConfig()
	cfg.Layout = config.LayoutVertical
	cfg.VerticalMaxWidth = 200 // 明确设置较小上限，迫使长候选截断
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	applyParityThemePath(r)
	cands := []Candidate{
		{Text: "短候选", Index: 1},
		{Text: "这是一个非常非常非常非常非常非常非常长的候选词条", Index: 2},
		{Text: "另一个超出宽度限制的候选词语测试字符串", Index: 3, Comment: "注释"},
		{Text: "中文", Index: 4},
	}
	img, _ := r.renderVerticalV2(cands, "test", 4, 1, 1, 0, "", 0)
	p := writePNG(t, "wind_vtrunc.png", img)
	t.Logf("V-Truncation: %s (%dx%d)", p, img.Bounds().Dx(), img.Bounds().Dy())
	// 窗口宽（减去左右 padding、shadow）应不超过 VerticalMaxWidth + padding*2 + shadow
	maxExpected := int(cfg.VerticalMaxWidth) + 2*8 + 4
	if img.Bounds().Dx() > maxExpected {
		t.Errorf("窗口宽 %d 超出预期上限 %d（VerticalMaxWidth=%v）",
			img.Bounds().Dx(), maxExpected, cfg.VerticalMaxWidth)
	}
}
