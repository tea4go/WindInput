package ui

import (
	"image"
	"image/color"
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// recordingDrawer 是记录型 TextDrawer，捕获最近一次 family-aware 调用，用于守护 P7-B 字体路由。
type recordingDrawer struct {
	lastDrawFamily    string
	lastMeasureFamily string
}

func (d *recordingDrawer) SetFont(string)                                            {}
func (d *recordingDrawer) MeasureString(string, float64) float64                     { return 10 }
func (d *recordingDrawer) BeginDraw(*image.RGBA)                                     {}
func (d *recordingDrawer) DrawString(string, float64, float64, float64, color.Color) {}
func (d *recordingDrawer) DrawStringWithWeight(string, float64, float64, float64, color.Color, int) {
}
func (d *recordingDrawer) MeasureStringFont(_ string, _ float64, family string) float64 {
	d.lastMeasureFamily = family
	return 10
}
func (d *recordingDrawer) DrawStringFull(_ string, _, _, _ float64, _ color.Color, _ int, family string) {
	d.lastDrawFamily = family
}
func (d *recordingDrawer) EndDraw() {}
func (d *recordingDrawer) Close()   {}

// TestPaintText_FamilyThreaded 验证 P7-B：TextStyle.Family 经 paintText 透传到 DrawStringFull；
// AlignCenter 测量走 family-aware MeasureStringFont。
func TestPaintText_FamilyThreaded(t *testing.T) {
	td := &recordingDrawer{}
	v := &View{Text: "甲", TextStyle: TextStyle{FontSize: 16, Family: "KaiTi", Color: color.Black, Align: AlignCenter}}
	Layout(v, 0, 0, td)
	v.paintText(td)
	if td.lastDrawFamily != "KaiTi" {
		t.Errorf("DrawStringFull 应收到 family=KaiTi, got %q", td.lastDrawFamily)
	}
	if td.lastMeasureFamily != "KaiTi" {
		t.Errorf("AlignCenter 测量应走 MeasureStringFont(family=KaiTi), got %q", td.lastMeasureFamily)
	}
}

// TestMeasureText_FamilyRouting 验证 measureText：family 非空且测量器支持时走 MeasureStringFont，否则 MeasureString。
func TestMeasureText_FamilyRouting(t *testing.T) {
	td := &recordingDrawer{}
	_ = measureText(td, "x", 16, "Arial")
	if td.lastMeasureFamily != "Arial" {
		t.Errorf("family 非空应路由 MeasureStringFont, got %q", td.lastMeasureFamily)
	}
	td.lastMeasureFamily = "sentinel"
	_ = measureText(td, "x", 16, "") // 空 family 不应调用 MeasureStringFont
	if td.lastMeasureFamily != "sentinel" {
		t.Errorf("空 family 应走 MeasureString，不触碰 MeasureStringFont")
	}
}

// TestRefreshResolvedViews_FontOverride 验证 P7-B 字号回填语义：
//   - views 显式 font_size（逻辑像素）经 refreshResolvedViews ×DPI scale 生效（绝对覆盖）；
//   - font_weight 原样透传进 RVNode；
//   - 未写 font_size 的元素回退运行时派生字号（config.FontSize，已含 scale）。
func TestRefreshResolvedViews_FontOverride(t *testing.T) {
	cfg := parityConfig()
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	views := themePathViews(6, 8)
	views.Text.FontSize = ip(20)
	views.Text.FontWeight = ip(700)
	r.resolvedV25 = &theme.ResolvedV25{Palette: themePathPalette(), Behavior: theme.ResolvedBehavior{FontSize: 18, ShowPageNumber: true, VerticalMaxWidth: 600}}
	r.themeViews = &views
	r.refreshResolvedViews()

	scale := GetDPIScale()
	if got, want := r.resolvedViews.Text.FontSize, 20*scale; got != want {
		t.Errorf("Text.FontSize 显式 20(逻辑px) 应 ×scale=%v, got %v", want, got)
	}
	if r.resolvedViews.Text.FontWeight != 700 {
		t.Errorf("Text.FontWeight 应透传 700, got %d", r.resolvedViews.Text.FontWeight)
	}
	// PreeditBar 未写 font_size → 回退运行时派生（config.FontSize，已 ×scale）
	if got, want := r.resolvedViews.PreeditBar.FontSize, r.config.FontSize; got != want {
		t.Errorf("PreeditBar.FontSize 未写应回退 config.FontSize=%v, got %v", want, got)
	}
}
