//go:build darwin

package ui

// text_backend_darwin.go — darwin 版 TextBackendManager: 只走 freetype (gg/text)。
// 公开 API 与 text_backend_windows.go 等价, 让 Renderer 等跨平台调用方无需感知差异。
// macOS 端不需要 GDI / DirectWrite, freetype 已能处理 PingFang/Helvetica/Apple Color Emoji
// 等 (字体定位走 systemfont darwin catalog + AssetsV2)。
//
// 公开方法集 (与 win 版同名):
//   - FontConfig() / TextDrawer() / FontFamily() / FontReady()
//   - SetTextRenderMode (忽略入参 mode, 始终 freetype)
//   - SetFontFamily / ResolvePrimaryFontPath / Close
//   - SetGDIFontParams / SetDWriteFontFallbackForPUA (darwin no-op, 保持 API 兼容)

type TextBackendManager struct {
	fontCache  *fontCache
	textDrawer TextDrawer
	fontConfig *FontConfig
	fontSpec   string
	fontReady  bool
	label      string
}

// NewTextBackendManager creates a darwin TextBackendManager.
// label 当前 darwin 端未用 (Win 端 DWrite renderer 用其区分 candidate/toolbar 等),
// 保留参数以保持跨平台 API 一致。
func NewTextBackendManager(label string) TextBackendManager {
	return TextBackendManager{
		fontConfig: NewFontConfig(),
		label:      label,
	}
}

func (m *TextBackendManager) FontConfig() *FontConfig { return m.fontConfig }
func (m *TextBackendManager) TextDrawer() TextDrawer  { return m.textDrawer }
func (m *TextBackendManager) FontFamily() string      { return m.fontSpec }
func (m *TextBackendManager) FontReady() bool         { return m.fontReady }

// ResolvePrimaryFontPath 解析当前 fontSpec 到具体路径, 与 win 版语义对齐。
func (m *TextBackendManager) ResolvePrimaryFontPath() string {
	m.fontConfig.SetPrimaryFont(m.fontSpec)
	resolved := m.fontConfig.ResolvePrimaryFont()
	if resolved != "" {
		m.fontSpec = resolved
	}
	return resolved
}

// EnsureFontCache lazily creates the gg/text font cache.
func (m *TextBackendManager) EnsureFontCache() *fontCache {
	if m.fontCache == nil {
		m.fontCache = newFontCache()
	}
	m.fontConfig.SetPrimaryFont(m.fontSpec)
	resolved := m.fontConfig.ResolveTextPrimaryFont()
	if resolved == "" {
		return m.fontCache
	}
	m.fontCache.mu.Lock()
	_ = m.fontCache.loadFont(resolved)
	m.fontCache.mu.Unlock()
	m.fontReady = true
	return m.fontCache
}

// SetTextRenderMode darwin 上始终落到 freetype (gg/text), 忽略入参 mode。
// 保持 win 版同名 API, 让 Renderer 跨平台代码统一调用。
func (m *TextBackendManager) SetTextRenderMode(mode TextRenderMode) {
	_ = mode
	fc := m.EnsureFontCache()
	m.textDrawer = newFreeTypeDrawer(fc, m.fontConfig)
}

// SetFontFamily updates the primary font for the freetype backend.
func (m *TextBackendManager) SetFontFamily(fontSpec string) {
	m.fontSpec = fontSpec
	m.fontConfig.SetPrimaryFont(m.fontSpec)
	textResolved := m.fontConfig.ResolveTextPrimaryFont()
	if m.fontCache != nil && textResolved != "" {
		m.fontCache.mu.Lock()
		_ = m.fontCache.loadFont(textResolved)
		m.fontCache.mu.Unlock()
		m.fontReady = true
	}
}

// SetGDIFontParams darwin 上 no-op (无 GDI), 保留 API 让 Renderer 调用统一。
func (m *TextBackendManager) SetGDIFontParams(weight int, scale float64) {
	m.fontConfig.SetGDIFontWeight(weight)
	m.fontConfig.SetGDIFontScale(scale)
}

// SetDWriteFontFallbackForPUA darwin 上 no-op (无 DWrite)。
// PUA fallback 在 darwin 上将由 freetype font fallback chain (FontConfig 内) 接管。
func (m *TextBackendManager) SetDWriteFontFallbackForPUA(familyName string) {}

// ReleaseFreeTypeBackend closes and clears the freetype font cache.
func (m *TextBackendManager) ReleaseFreeTypeBackend() {
	if m.fontCache != nil {
		m.fontCache.Close()
		m.fontCache = nil
	}
	m.fontReady = false
}

// Close releases all backends (darwin 仅 freetype)。
func (m *TextBackendManager) Close() {
	m.ReleaseFreeTypeBackend()
}
