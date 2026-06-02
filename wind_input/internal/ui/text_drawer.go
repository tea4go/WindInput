package ui

import (
	"image"
	"image/color"
	"image/draw"
	"unicode/utf8"

	"github.com/gogpu/gg"
	ggtext "github.com/gogpu/gg/text"
)

// TextRenderMode defines the text rendering backend.
// 历史上配置值已经对外暴露，所以这里保留了 "freetype" 这个模式名。
type TextRenderMode string

const (
	TextRenderModeGDI         TextRenderMode = "gdi"         // Windows GDI native rendering
	TextRenderModeFreetype    TextRenderMode = "freetype"    // gogpu/gg text rendering
	TextRenderModeDirectWrite TextRenderMode = "directwrite" // DirectWrite + Direct2D rendering
)

// TextDrawer provides a unified interface for text measurement and drawing.
// Both FreeType and GDI backends implement this interface.
// The interface is designed to be engine-agnostic so that rendering backends
// (FreeType, GDI, DirectWrite, etc.) can be swapped transparently.
type TextDrawer interface {
	// SetFont sets the font by file path. The backend resolves it to the
	// appropriate internal representation (e.g., GDI family name, gg/text source).
	SetFont(fontPath string)
	// MeasureString measures text width in pixels for the given font size.
	MeasureString(text string, fontSize float64) float64
	// BeginDraw prepares for text drawing on the given image.
	// Must be called before DrawString, and EndDraw must be called after.
	BeginDraw(img *image.RGBA)
	// DrawString draws text at baseline position (x, y), matching gg.DrawString coordinates.
	DrawString(text string, x, y float64, fontSize float64, clr color.Color)
	// DrawStringWithWeight draws text with a specific font weight (100-900).
	// If weight <= 0, behaves like DrawString with global weight.
	DrawStringWithWeight(text string, x, y float64, fontSize float64, clr color.Color, weight int)
	// MeasureStringFont measures text width using a specific platform font family (P7-B 逐元素字体)。
	// 空 family 回退全局字体；未知 family 由平台文本引擎自行替换。
	MeasureStringFont(text string, fontSize float64, family string) float64
	// DrawStringFull draws text with explicit weight + platform font family (P7-B)。
	// 空 family 回退全局字体；weight<=0 用全局字重；未知 family 由平台引擎替换。
	DrawStringFull(text string, x, y float64, fontSize float64, clr color.Color, weight int, family string)
	// EndDraw finalizes text drawing and flushes results to the image.
	EndDraw()
	// Close releases all resources held by this drawer.
	Close()
}

// --- FreeType (original) implementation with font fallback ---

// freeTypeDrawer keeps the existing TextRenderMode name for config compatibility,
// but the implementation now uses gogpu/gg text faces instead of fogleman/gg.
// Fallback still happens at glyph granularity so symbols like ✓ and ▸ can be
// resolved independently of the primary font.
type freeTypeDrawer struct {
	cache        *fontCache
	dc           *gg.Context
	target       *image.RGBA
	emojiOverlay *image.RGBA // 彩色 emoji 专用图层: gg.Context.Image() 返回的是拷贝,
	// 在它上面合成会丢失, 故 emoji 单独画到这里, EndDraw 一并叠回。
	fontConfig     *FontConfig
	fallbackCaches []*fontCache        // Font face caches (one per fallback font path)
	fallbackFonts  []fallbackFontEntry // References to fallback font entries
	fallbackInited bool                // Whether fallback has been initialized
}

func newFreeTypeDrawer(cache *fontCache, fontConfig *FontConfig) *freeTypeDrawer {
	return &freeTypeDrawer{
		cache:      cache,
		fontConfig: fontConfig,
	}
}

func (d *freeTypeDrawer) SetFont(fontPath string) {
	d.cache.mu.Lock()
	defer d.cache.mu.Unlock()
	_ = d.cache.loadFont(fontPath)
	// 主字体变化后，fallback 选择顺序也可能变化，所以一并重建。
	d.fallbackInited = false
	d.fallbackCaches = nil
	d.fallbackFonts = nil
}

// initFallbacks lazily initializes fallback metadata.
// The actual font files are not parsed here. Each drawer only creates its own
// lightweight fontCache descriptors, and the underlying FontSource is loaded on
// first glyph lookup through fontCache.getFace().
func (d *freeTypeDrawer) initFallbacks() {
	if d.fallbackInited {
		return
	}
	d.fallbackInited = true

	if d.fontConfig == nil {
		return
	}

	fallbackPaths := d.fontConfig.GetTextFallbackFonts()
	shared := GetSharedFallbackFonts(fallbackPaths)

	for _, entry := range shared {
		// Create a lightweight fontCache descriptor for each fallback path.
		// The font file itself is still loaded lazily in getFace().
		fc := newFontCache()
		fc.fontPath = entry.path
		d.fallbackCaches = append(d.fallbackCaches, fc)
		d.fallbackFonts = append(d.fallbackFonts, entry)
	}
}

// fontSegment represents a contiguous run of text that uses the same font.
type fontSegment struct {
	text string
	face ggtext.Face
}

// segmentByFont splits text into segments, each using the best available font.
// The primary font is preferred; fallback fonts are tried for missing glyphs.
func (d *freeTypeDrawer) segmentByFont(text string, fontSize float64) []fontSegment {
	d.initFallbacks()

	primaryFace := d.cache.getFace(fontSize)
	if primaryFace == nil {
		return nil
	}

	if len(d.fallbackCaches) == 0 {
		return []fontSegment{{text: text, face: primaryFace}}
	}

	var segments []fontSegment
	var currentText []byte
	var currentFace ggtext.Face

	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size <= 1 {
			i++
			continue
		}

		// Determine which face to use for this rune.
		bestFace := primaryFace
		if !primaryFace.HasGlyph(r) {
			// Try fallback fonts in priority order. These faces are loaded lazily,
			// so common strings never pay the cost of parsing the whole chain.
			for j := range d.fallbackCaches {
				face := d.fallbackCaches[j].getFace(fontSize)
				if face != nil && face.HasGlyph(r) {
					bestFace = face
					break
				}
			}
		}

		if bestFace != currentFace {
			// Flush current segment. gg/text still works one face at a time, so
			// segmented runs preserve the old glyph-level fallback behavior.
			if len(currentText) > 0 {
				segments = append(segments, fontSegment{text: string(currentText), face: currentFace})
				currentText = currentText[:0]
			}
			currentFace = bestFace
		}
		currentText = append(currentText, text[i:i+size]...)
		i += size
	}

	if len(currentText) > 0 {
		segments = append(segments, fontSegment{text: string(currentText), face: currentFace})
	}

	return segments
}

func (d *freeTypeDrawer) MeasureString(text string, fontSize float64) float64 {
	if text == "" {
		return 0
	}

	segments := d.segmentByFont(text, fontSize)
	if len(segments) == 0 {
		return 0
	}

	dc := d.dc
	if dc == nil {
		dc = gg.NewContext(1, 1)
	}

	// gg.MeasureString works per current font, so segmented fallback text needs
	// to be measured run by run and summed manually.
	var totalW float64
	for _, seg := range segments {
		// emoji 段用专属 advance (整 em), 与 DrawString 步进一致, 避免布局拥挤。
		if adv, ok := colorEmojiAdvance(seg.text, fontSize); ok {
			totalW += adv
			continue
		}
		dc.SetFont(seg.face)
		w, _ := dc.MeasureString(seg.text)
		totalW += w
	}
	return totalW
}

func (d *freeTypeDrawer) BeginDraw(img *image.RGBA) {
	// TODO(gogpu/gg): NewContextForImage 的文本路径有 bug，DrawString 不会稳定落到目标位图上。
	// 等上游修复后可改为 NewContextForImage + overlay 复用，避免每帧分配新 RGBA 缓冲。
	// 当前 workaround：在独立 overlay 上绘字，EndDraw 阶段合成回原图。
	bounds := img.Bounds()
	d.target = img
	d.dc = gg.NewContext(bounds.Dx(), bounds.Dy())
	d.emojiOverlay = nil // 懒分配: 仅在真有 emoji 段时创建 (见 DrawString)
}

func (d *freeTypeDrawer) DrawString(text string, x, y float64, fontSize float64, clr color.Color) {
	if d.dc == nil || text == "" {
		return
	}

	segments := d.segmentByFont(text, fontSize)
	if len(segments) == 0 {
		return
	}

	d.dc.SetColor(clr)
	drawX := x
	for _, seg := range segments {
		d.dc.SetFont(seg.face)
		// 彩色字体 (Apple Color Emoji 等): gg.Context.DrawString 只走单色轮廓,
		// 对纯位图 emoji 渲不出; 且 gg.Context.Image() 返回的是拷贝, 直接往上画会
		// 丢失。故把彩色 emoji 画到独立 overlay (与 target 同尺寸), EndDraw 再叠回。
		drawn := false
		var segAdvance float64
		hasAdvance := false
		if d.target != nil {
			if d.emojiOverlay == nil {
				d.emojiOverlay = image.NewRGBA(d.target.Bounds())
			}
			if adv, ok := drawColorEmoji(d.emojiOverlay, seg.face, seg.text, drawX, y, clr); ok {
				drawn, segAdvance, hasAdvance = true, adv, true
			} else if faceHasColorGlyphs(seg.face) {
				ggtext.DrawWithEmoji(d.emojiOverlay, seg.text, seg.face, drawX, y, clr)
				drawn = true
			}
		}
		if !drawn {
			d.dc.DrawString(seg.text, drawX, y)
		}
		// 步进: emoji 段用其专属 advance (整 em, 与 MeasureString 一致); 其余用 gg 测量。
		if !hasAdvance {
			segAdvance, _ = d.dc.MeasureString(seg.text)
		}
		drawX += segAdvance
	}
}

// faceHasColorGlyphs 报告该 face 是否带彩色字形表 (sbix/CBDT/COLR)。
// 与 ggtext.DrawWithEmoji 内部的检测一致, 提前判断以决定走彩色还是单色路径。
func faceHasColorGlyphs(face ggtext.Face) bool {
	if face == nil {
		return false
	}
	src := face.Source()
	if src == nil {
		return false
	}
	cf, ok := src.Parsed().(ggtext.ColorFont)
	return ok && cf.HasColorTables()
}

func (d *freeTypeDrawer) DrawStringWithWeight(text string, x, y float64, fontSize float64, clr color.Color, weight int) {
	// FreeType doesn't support per-draw weight, fall back to regular DrawString
	d.DrawString(text, x, y, fontSize, clr)
}

// MeasureStringFont：freeType 按文件路径加载字体，不支持按族名切换；忽略 family，用全局/fallback 度量。
func (d *freeTypeDrawer) MeasureStringFont(text string, fontSize float64, family string) float64 {
	return d.MeasureString(text, fontSize)
}

// DrawStringFull：freeType 无 per-draw 字重/族名能力，回退全局字体绘制。
func (d *freeTypeDrawer) DrawStringFull(text string, x, y float64, fontSize float64, clr color.Color, weight int, family string) {
	d.DrawString(text, x, y, fontSize, clr)
}

func (d *freeTypeDrawer) EndDraw() {
	if d.target != nil && d.dc != nil {
		if overlay, ok := d.dc.Image().(*image.RGBA); ok {
			draw.Draw(d.target, d.target.Bounds(), overlay, overlay.Bounds().Min, draw.Over)
		}
		// 彩色 emoji 图层叠在文字之上 (二者 x 不重叠, 顺序不影响, 但保证 emoji 可见)。
		if d.emojiOverlay != nil {
			draw.Draw(d.target, d.target.Bounds(), d.emojiOverlay, d.emojiOverlay.Bounds().Min, draw.Over)
		}
	}
	d.dc = nil
	d.target = nil
	d.emojiOverlay = nil
}

func (d *freeTypeDrawer) Close() {
	// d.cache 由外层 renderer 持有和复用，这里只释放当前 drawer 私有状态。
	d.dc = nil
	d.target = nil
	d.emojiOverlay = nil
	d.fallbackCaches = nil
	d.fallbackFonts = nil
}

// GDI / DirectWrite 实现移至 text_drawer_windows.go (Win 专用)。
