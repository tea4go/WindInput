package ui

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/gogpu/gg"
	ggtext "github.com/gogpu/gg/text"
	"github.com/huanfeng/wind_input/pkg/buildvariant"
	"github.com/huanfeng/wind_input/pkg/config"
	"github.com/huanfeng/wind_input/pkg/theme"
)

// RenderConfig contains rendering configuration.
// 这里只描述候选窗的运行时参数；候选窗外观（颜色/几何）已由 theme.ResolveCandidateViews
// 经 r.resolvedViews 承载（P6 阶段2e 删合成桥后，颜色/几何字段不再经 RenderConfig 中转）。
// 字体文件选择与 fallback 细节由 FontConfig 接管。
type RenderConfig struct {
	FontPath          string
	FontSize          float64                // 候选主文本字号（已乘 DPI scale）；回填进 resolvedViews
	IndexFontSize     float64                // 序号字号（= FontSize-4，已乘 scale）；回填进 resolvedViews
	ItemHeight        float64                // 行高（已乘 scale）；回填进 resolvedViews
	Layout            config.CandidateLayout // "horizontal" or "vertical"
	HidePreedit       bool                   // Hide preedit area when inline_preedit is enabled
	IndexStyle        string                 // "circle" (default) or "text" (plain text index)
	HasAccentBar      bool                   // Whether to draw accent bar
	VerticalMaxWidth  float64                // Vertical layout maximum width (scaled px), 0 = default 600
	AlwaysShowPager   bool                   // Always show page navigation (disable buttons when not navigable)
	ShowPageNumber    bool                   // Show page number text (e.g. "1/3")
	TextRenderMode    TextRenderMode         // "gdi" (Windows native) or "freetype" (original)
	ModeLabel         string                 // Temporary mode label (e.g. "临时拼音", "快捷输入"), empty = no label
	ModeAccentColor   color.Color            // Inner glow border color for special modes, nil = no glow
	PreeditMode       config.PreeditMode     // "top" (default) or "embedded" (inline before candidates); only effective when HidePreedit=false
	IndexLabels       string                 // 主题序号标签（来自 views.index.labels / 旧 layout）；空 = 默认 1-9,0
	GlobalIndexLabels string                 // 用户全局序号标签覆盖（config.UI.CandidateIndexLabels）；非空时覆盖 IndexLabels
	CmdbarPrefix      string                 // 副作用 cmdbar 候选 (Actions 含 ActionEffect) 的前缀符号; 空 = 不显示前缀

	// v2.5 候选窗背景图（nil = 仅纯色背景）
	BackgroundImage   *image.RGBA
	BackgroundMode    string // nine_slice | stretch | tile | center
	BackgroundSlice   theme.Padding
	BackgroundOpacity float64
}

// DefaultRenderConfig returns default rendering configuration with DPI scaling
func DefaultRenderConfig() RenderConfig {
	// Get DPI scale factor
	scale := GetDPIScale()

	return RenderConfig{
		FontPath:       "", // Will use system font
		FontSize:       18 * scale,
		IndexFontSize:  14 * scale,
		ItemHeight:     32 * scale,
		Layout:         config.LayoutHorizontal, // Default to horizontal layout
		HidePreedit:    false,
		ShowPageNumber: true,
		CmdbarPrefix:   DefaultCmdbarCandidatePrefix,
	}
}

// fontCache caches loaded gg/text FontSource instances and per-size faces.
// The shared FontSource is global; this struct only tracks the small face cache
// that varies by requested font size inside one renderer.
// maxFontFaces limits the number of cached ggtext.Face instances per fontCache.
// When exceeded, the least recently used face is closed and evicted.
const maxFontFaces = 16

type fontCache struct {
	mu        sync.RWMutex
	source    *ggtext.FontSource
	fontPath  string
	faces     map[float64]ggtext.Face // Cache font faces by size
	faceOrder []float64               // LRU order: most recently used at end
}

// newFontCache creates an empty per-renderer face cache.
func newFontCache() *fontCache {
	return &fontCache{
		faces: make(map[float64]ggtext.Face),
	}
}

// loadFont records the font path for lazy loading.
func (fc *fontCache) loadFont(path string) error {
	if fc.fontPath == path && fc.source != nil {
		return nil
	}
	// Switching fonts invalidates all per-size faces because gg/text Face objects
	// are derived from the FontSource and size together.
	fc.faces = make(map[float64]ggtext.Face)
	fc.faceOrder = nil
	fc.source = nil
	fc.fontPath = path
	return nil
}

// ensureFontSource loads the gg/text FontSource from the global registry on demand.
// Must be called with fc.mu held for writing.
func (fc *fontCache) ensureFontSource() error {
	if fc.source != nil {
		return nil
	}
	if fc.fontPath == "" {
		return nil
	}
	source, err := GetSharedFontSource(fc.fontPath)
	if err != nil {
		return err
	}
	fc.source = source
	return nil
}

// getFace returns a cached gg/text face for the given size, with LRU eviction.
func (fc *fontCache) getFace(size float64) ggtext.Face {
	fc.mu.RLock()
	if face, ok := fc.faces[size]; ok {
		fc.mu.RUnlock()
		fc.mu.Lock()
		fc.touchLRU(size)
		fc.mu.Unlock()
		return face
	}
	fc.mu.RUnlock()

	fc.mu.Lock()
	defer fc.mu.Unlock()

	if face, ok := fc.faces[size]; ok {
		fc.touchLRU(size)
		return face
	}

	if err := fc.ensureFontSource(); err != nil || fc.source == nil {
		return nil
	}

	// gg/text Face is lightweight, so creating it lazily and caching by size keeps
	// repeated measurements and draws cheap without duplicating font file data.
	face := fc.source.Face(size)

	if len(fc.faces) >= maxFontFaces && len(fc.faceOrder) > 0 {
		oldest := fc.faceOrder[0]
		fc.faceOrder = fc.faceOrder[1:]
		if _, ok := fc.faces[oldest]; ok {
			delete(fc.faces, oldest)
		}
	}

	fc.faces[size] = face
	fc.faceOrder = append(fc.faceOrder, size)
	return face
}

// touchLRU moves size to the end of the LRU order. Must be called with fc.mu held.
func (fc *fontCache) touchLRU(size float64) {
	for i, s := range fc.faceOrder {
		if s == size {
			fc.faceOrder = append(fc.faceOrder[:i], fc.faceOrder[i+1:]...)
			fc.faceOrder = append(fc.faceOrder, size)
			return
		}
	}
	fc.faceOrder = append(fc.faceOrder, size)
}

// Close releases per-instance face references. FontSource instances are shared
// globally and intentionally stay alive for the process lifetime.
func (fc *fontCache) Close() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.faces = make(map[float64]ggtext.Face)
	fc.faceOrder = nil
	fc.source = nil
}

// Renderer renders candidate window content
type Renderer struct {
	config        RenderConfig
	resolvedV25   *theme.ResolvedV25
	resolvedViews theme.ResolvedViews // 候选窗盒模型外观：每帧由 refreshResolvedViews 经 theme.ResolveCandidateViews 重建（几何+颜色）+ 运行时字号回填
	themeViews    *theme.Views        // 主题盒模型 views（已 merge defaultViews 基线）；来自 rv.Views
	TextBackendManager

	// Base (unscaled) values for DPI recalculation
	baseFontSize    float64 // 有效基准字号（= 跟随主题 ? themeFontSize : userFontSize），派生 FontSize/IndexFontSize/ItemHeight
	userFontSize    float64 // 用户全局字号（config.UI.FontSize），自定义模式下生效
	themeFontSize   float64 // 主题 behavior.font_size（来自 SetTheme），跟随模式下生效
	fontFollowTheme bool    // true=候选字号跟随主题 behavior.font_size；false=用 userFontSize
	themeRowHeight  float64 // unscaled row height from theme; 0 = auto-compute from font size
	lastDPI         int     // Last DPI used for scaling; 0 means not yet set

	// 候选框绘制缓冲. 跨帧复用以避免 gg.NewPixmap + dc.Image() 的双倍分配
	// (旧 pprof 中合计 ~2.3 GB 累计). RenderCandidates 在 UI 单线程调用,
	// UpdateLayeredWindow 同步消费 img, 之后下一帧才会写入 — 无并发竞争.
	scratchPix []byte
}

// acquireDrawContext 返回一个 gg.Context 与对应的 *image.RGBA, 二者共享
// Renderer.scratchPix 底层数组. 容量不足时一次性扩张, 内容预清零 (gg 期望
// 透明背景起步). 调用方拿到的 img 在下一次 acquireDrawContext 前都有效.
func (r *Renderer) acquireDrawContext(w, h int) (*gg.Context, *image.RGBA) {
	need := w * h * 4
	if cap(r.scratchPix) < need {
		r.scratchPix = make([]byte, need)
	} else {
		r.scratchPix = r.scratchPix[:need]
		clear(r.scratchPix)
	}
	pm := gg.NewPixmapFromBuffer(r.scratchPix, w, h)
	return gg.NewContextForPixmap(pm), pm.ImageView()
}

// NewRenderer creates a new renderer
func NewRenderer(config RenderConfig) *Renderer {
	r := &Renderer{
		config:             config,
		TextBackendManager: NewTextBackendManager("candidate"),
		baseFontSize:       18, // Default base font size (unscaled)
		userFontSize:       18,
	}
	r.SetTextRenderMode(config.TextRenderMode)
	return r
}

// SetTextRenderMode switches between GDI, gg/text, and DirectWrite rendering.
func (r *Renderer) SetTextRenderMode(mode TextRenderMode) {
	r.config.TextRenderMode = mode
	r.TextBackendManager.SetTextRenderMode(mode)
}

// GetTextRenderMode returns the current text rendering mode
func (r *Renderer) GetTextRenderMode() TextRenderMode {
	return r.config.TextRenderMode
}

// applyFontDerivation 用当前 baseFontSize + DPI scale 重算字号派生：
// 主文本=base、序号=base-4、行高=themeRowHeight 或 max(32, base*1.8)，均 ×scale。
func (r *Renderer) applyFontDerivation() {
	scale := GetDPIScale()
	base := r.baseFontSize
	if base <= 0 {
		base = 18
	}
	r.config.FontSize = base * scale
	r.config.IndexFontSize = (base - 4) * scale
	if r.themeRowHeight > 0 {
		r.config.ItemHeight = r.themeRowHeight * scale
	} else {
		r.config.ItemHeight = math.Max(32, base*1.8) * scale
	}
}

// recomputeBaseFont 依「跟随主题/自定义」选定有效基准字号，再重算派生。
// 跟随且主题字号有效→主题 behavior.font_size；否则→用户全局字号。
func (r *Renderer) recomputeBaseFont() {
	if r.fontFollowTheme && r.themeFontSize > 0 {
		r.baseFontSize = r.themeFontSize
	} else if r.userFontSize > 0 {
		r.baseFontSize = r.userFontSize
	}
	r.applyFontDerivation()
}

// SetFontFollowTheme 设置候选字号是否跟随主题 behavior.font_size（来自 config.UI.FontSizeFollowTheme）。
func (r *Renderer) SetFontFollowTheme(follow bool) {
	r.fontFollowTheme = follow
	r.recomputeBaseFont()
}

// UpdateFont updates font settings
func (r *Renderer) UpdateFont(fontSize float64, fontFamily string) {
	if fontSize > 0 {
		r.userFontSize = fontSize
		r.recomputeBaseFont() // 跟随模式下忽略用户字号，用主题字号
	}

	if fontFamily != r.FontFamily() {
		r.SetFontFamily(fontFamily)
	}
}

// refreshDPIIfNeeded checks if DPI has changed since last render and recalculates if needed.
func (r *Renderer) refreshDPIIfNeeded() {
	// 用跨平台 GetDPIScale * 96 还原成 int dpi, 保持原 lastDPI int 字段语义不变。
	// Win 端 GetDPIScale() = GetEffectiveDPI()/96, 反推回 currentDPI = GetEffectiveDPI()。
	// darwin 端 GetDPIScale() 默认 1.0, currentDPI = 96 恒定 (无 per-monitor 切换)。
	currentDPI := int(GetDPIScale() * 96.0)
	if r.lastDPI != currentDPI {
		r.lastDPI = currentDPI
		r.RefreshDPIScale()
	}
}

// RefreshDPIScale recalculates all DPI-dependent config values.
// Called when the effective DPI changes (e.g., monitor switch). baseFontSize 已是有效值，
// 仅按新 scale 重算派生（窗口 padding/圆角等已于 P6 阶段2e 退役，不再 RenderConfig 中转）。
func (r *Renderer) RefreshDPIScale() {
	r.applyFontDerivation()
}

// SetLayout sets the candidate layout mode
func (r *Renderer) SetLayout(layout config.CandidateLayout) {
	if layout.Valid() {
		r.config.Layout = layout
	}
}

// SetHidePreedit sets whether to hide the preedit area
func (r *Renderer) SetHidePreedit(hide bool) {
	r.config.HidePreedit = hide
}

// SetPreeditMode sets the preedit display mode ("top" or "embedded")
func (r *Renderer) SetPreeditMode(mode config.PreeditMode) {
	r.config.PreeditMode = mode
}

// SetCmdbarPrefix 设置副作用 cmdbar 候选的渲染前缀符号。空字符串=不显示前缀。
func (r *Renderer) SetCmdbarPrefix(prefix string) {
	r.config.CmdbarPrefix = prefix
}

// SetAlwaysShowPager 设置是否在单页时也显示翻页区域
func (r *Renderer) SetAlwaysShowPager(v bool) {
	r.config.AlwaysShowPager = v
}

// SetShowPageNumber 设置是否在翻页区域显示页码文字（如 "1/3"）
func (r *Renderer) SetShowPageNumber(v bool) {
	r.config.ShowPageNumber = v
}

// SetModeLabel sets the temporary mode label for display
func (r *Renderer) SetModeLabel(label string) {
	r.config.ModeLabel = label
}

// SetModeAccentColor sets the inner glow border color for the current mode, nil = no glow
func (r *Renderer) SetModeAccentColor(c color.Color) {
	r.config.ModeAccentColor = c
}

// SetTheme sets the theme for the renderer and updates colors
func (r *Renderer) SetTheme(rv *theme.ResolvedV25) {
	if rv == nil {
		return
	}
	r.resolvedV25 = rv
	r.themeViews = rv.Views // 主题盒模型 views（已 merge defaultViews 基线）
	// 候选窗颜色/几何已由 theme.ResolveCandidateViews 经 r.resolvedViews 承载（P6 阶段2e 删合成桥）；
	// 此处只搬运渲染运行时仍需的 RenderConfig 字段：IndexStyle / HasAccentBar / page 策略 /
	// 行高（字号派生）/ 序号标签。颜色与几何 padding 不再经 RenderConfig 中转。
	lay := rv.Layout.CandidateWindow
	idx := lay.CandidateList.Index
	if idx.Circle {
		r.config.IndexStyle = "circle"
	} else {
		r.config.IndexStyle = "text"
	}
	r.config.HasAccentBar = lay.CandidateList.AccentBar.Enabled
	// page 策略默认来自主题 behavior（P6 阶段2d）；用户 PagerDisplayMode 覆盖在
	// applyPagerOverride 注入（Default=跟随主题，即保留此处写入的 behavior 值）。
	r.config.AlwaysShowPager = rv.Behavior.AlwaysShowPager
	r.config.ShowPageNumber = rv.Behavior.ShowPageNumber
	// 行高来源：主题 layout 指定则用之，否则由有效字号派生（recomputeBaseFont→applyFontDerivation 内处理）。
	if rh := float64(lay.CandidateList.ItemHeight); rh > 0 {
		r.themeRowHeight = rh
	} else {
		r.themeRowHeight = 0
	}
	// 字号跟随：记录主题 behavior.font_size，按「跟随/自定义」重算有效基准字号 + 派生（含行高）。
	r.themeFontSize = float64(rv.Behavior.FontSize)
	r.recomputeBaseFont()
	r.config.IndexLabels = theme.BuildIndexLabelsFromSlots(idx.Labels)

	// 候选窗背景图：ResolvedV25 不支持解码后的背景图（种子主题无图），直接清空（零回归）
	r.config.BackgroundImage = nil
	r.config.BackgroundMode = ""
	r.config.BackgroundOpacity = 0
}

// getCommentColor returns the comment color from theme or default
func (r *Renderer) getCommentColor() color.Color {
	if r.resolvedV25 != nil {
		return r.resolvedV25.Palette.CandidateWindow.Comment
	}
	return color.RGBA{150, 150, 150, 255}
}

// getModeIndicatorColors returns mode indicator colors from theme or defaults
func (r *Renderer) getModeIndicatorColors() (bgColor, textColor color.Color) {
	if r.resolvedV25 != nil {
		return r.resolvedV25.Palette.Toast.Background, r.resolvedV25.Palette.Toast.Text
	}
	return color.RGBA{50, 50, 50, 230}, color.RGBA{255, 255, 255, 255}
}

// GetLayout returns the current layout mode
func (r *Renderer) GetLayout() config.CandidateLayout {
	return r.config.Layout
}

func radians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func (r *Renderer) drawRoundedRect(dc *gg.Context, x, y, w, h, radius float64) {
	dc.DrawRoundedRectangle(x, y, w, h, radius)
}

// RenderModeIndicator renders a mode indicator with adaptive width
func (r *Renderer) RenderModeIndicator(mode string) *image.RGBA {
	scale := GetDPIScale()
	td := r.TextDrawer()

	minWidth := 50.0 * scale
	height := 36.0 * scale
	fontSize := 20.0 * scale
	padding := 12.0 * scale

	// Measure text width
	textWidth := td.MeasureString(mode, fontSize)

	// Adaptive width: max(minWidth, textWidth + padding*2)
	width := textWidth + padding*2
	if width < minWidth {
		width = minWidth
	}

	dc := gg.NewContext(int(width), int(height))

	// Get colors from theme
	bgColor, textColor := r.getModeIndicatorColors()

	// Draw background shape
	dc.SetColor(bgColor)
	r.drawRoundedRect(dc, 2*scale, 2*scale, width-4*scale, height-4*scale, 6*scale)
	dc.Fill()

	// Draw mode text
	img := dc.Image().(*image.RGBA)
	td.BeginDraw(img)
	tw := td.MeasureString(mode, fontSize)
	td.DrawString(mode, width/2-tw/2, height/2+7*scale, fontSize, textColor)
	td.EndDraw()

	return img
}

// SetGlobalIndexLabels 设置用户全局序号标签覆盖（config.UI.CandidateIndexLabels）。
// 非空时在 build 中优先于主题 IndexLabels（见 effectiveIndexLabels）。
func (r *Renderer) SetGlobalIndexLabels(labels string) {
	r.config.GlobalIndexLabels = labels
}

// DrawDebugBanner draws a small anti-aliased green dot in the top-right area (debug variant only)
// Inset from the corner to stay within rounded rectangle bounds.
func DrawDebugBanner(img *image.RGBA) {
	if !buildvariant.IsDebug() {
		return
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 20 || h < 20 {
		return
	}

	scale := GetDPIScale()
	radius := 4.0 * scale // 圆点半径
	inset := 8.0 * scale  // 从边缘内缩，避开圆角

	// 调试圆点统一为绿色（盒模型 View 引擎是唯一渲染路径）。
	dotR, dotG, dotB := uint16(40), uint16(180), uint16(70)

	// 圆点中心（右上角内缩）
	cxf := float64(w) - inset
	cyf := inset

	// 扫描范围：圆心 ± (radius+1)，+1 为抗锯齿过渡带
	ri := int(math.Ceil(radius)) + 1
	for dy := -ri; dy <= ri; dy++ {
		for dx := -ri; dx <= ri; dx++ {
			dist := math.Sqrt(float64(dx)*float64(dx) + float64(dy)*float64(dy))
			if dist > radius+1 {
				continue
			}

			// 抗锯齿：边缘 1px 过渡带内线性插值 alpha
			var alpha float64
			if dist <= radius-0.5 {
				alpha = 1.0
			} else if dist >= radius+0.5 {
				alpha = 0.0
			} else {
				alpha = 1.0 - (dist - (radius - 0.5))
			}
			if alpha <= 0 {
				continue
			}

			px := bounds.Min.X + int(cxf) + dx
			py := bounds.Min.Y + int(cyf) + dy
			if px < bounds.Min.X || px >= bounds.Max.X || py < bounds.Min.Y || py >= bounds.Max.Y {
				continue
			}

			// Alpha 混合：将红点与背景像素混合
			a := uint8(alpha * 255)
			bg := img.RGBAAt(px, py)
			invA := 255 - uint16(a)
			nr := uint8((dotR*uint16(a) + uint16(bg.R)*invA) / 255)
			ng := uint8((dotG*uint16(a) + uint16(bg.G)*invA) / 255)
			nb := uint8((dotB*uint16(a) + uint16(bg.B)*invA) / 255)
			na := uint8(math.Min(float64(uint16(a)+uint16(bg.A)), 255))
			img.SetRGBA(px, py, color.RGBA{nr, ng, nb, na})
		}
	}
}
