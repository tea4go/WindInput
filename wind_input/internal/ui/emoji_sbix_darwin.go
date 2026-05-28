//go:build darwin

package ui

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"os"
	"sync"

	ggtext "github.com/gogpu/gg/text"
	"github.com/gogpu/gg/text/emoji"
	xdraw "golang.org/x/image/draw"

	"github.com/huanfeng/wind_input/pkg/systemfont"
)

// emoji_sbix_darwin.go — darwin 上的彩色 emoji 渲染。
//
// 背景: gg/text v0.48.x 的彩色字形基础设施 (ColorFont 接口 / DrawWithEmoji) 是
// 休眠状态 —— 默认 ximage 解析器不实现 ColorFont, 所以 ggtext.DrawWithEmoji 永远
// 回退到单色 Draw(), 而 Apple Color Emoji 是纯 sbix 位图 (无轮廓), 单色路径渲染
// 为空白。这里直接用 gg 的 emoji.SBIXParser 从 Apple Color Emoji 提取 sbix 位图
// 自行合成, 绕过缺失的解析器接线。
//
// 仅处理 sbix (Apple 格式); CBDT/COLR 暂不涉及 (macOS 系统 emoji 就是 sbix)。

type sbixRenderer struct {
	parser   *emoji.SBIXParser
	numGlyph uint16
	parsed   ggtext.ParsedFont // emoji 字体自身的解析结果, 用于 rune→glyphID (不依赖调用方 face)
}

var (
	sbixOnce sync.Once
	sbixInst *sbixRenderer // nil 表示不可用 (无字体 / 提取失败)
)

func sbixRendererInstance() *sbixRenderer {
	sbixOnce.Do(func() {
		path := systemfont.ResolveFile("Apple Color Emoji", false)
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		sbixData, numGlyphs, ok := extractSbixTable(data)
		if !ok {
			return
		}
		p, err := emoji.NewSBIXParser(sbixData, numGlyphs)
		if err != nil {
			return
		}
		// 加载 emoji 字体自身的解析结果, 用它做 rune→glyphID, 避免依赖调用方
		// 传入的 face (segmentByFont 可能把 emoji 路由到别的 fallback face)。
		src, err := ggtext.NewFontSourceFromFile(path)
		if err != nil {
			return
		}
		sbixInst = &sbixRenderer{parser: p, numGlyph: numGlyphs, parsed: src.Parsed()}
	})
	return sbixInst
}

// extractSbixTable 从 sfnt/TTC 字节流里抽出 sbix 表 (字节切片) 与字形总数 (maxp)。
// TTC 取第 0 个子字体。表记录里的 offset 都是从文件开头算起。
func extractSbixTable(data []byte) (sbix []byte, numGlyphs uint16, ok bool) {
	if len(data) < 12 {
		return nil, 0, false
	}
	sfntOffset := uint32(0)
	if string(data[0:4]) == "ttcf" {
		// TTC header: tag(4) version(4) numFonts(4) offsetTable[0](4)...
		if len(data) < 16 {
			return nil, 0, false
		}
		sfntOffset = binary.BigEndian.Uint32(data[12:16])
	}
	if int(sfntOffset)+12 > len(data) {
		return nil, 0, false
	}
	numTables := binary.BigEndian.Uint16(data[sfntOffset+4 : sfntOffset+6])
	rec := sfntOffset + 12 // 表记录区起点
	var sbixOff, sbixLen, maxpOff uint32
	for i := uint16(0); i < numTables; i++ {
		base := rec + uint32(i)*16
		if int(base)+16 > len(data) {
			return nil, 0, false
		}
		tag := string(data[base : base+4])
		off := binary.BigEndian.Uint32(data[base+8 : base+12])
		length := binary.BigEndian.Uint32(data[base+12 : base+16])
		switch tag {
		case "sbix":
			sbixOff, sbixLen = off, length
		case "maxp":
			maxpOff = off
		}
	}
	if sbixOff == 0 || sbixLen == 0 || maxpOff == 0 {
		return nil, 0, false
	}
	if int(sbixOff+sbixLen) > len(data) || int(maxpOff)+6 > len(data) {
		return nil, 0, false
	}
	numGlyphs = binary.BigEndian.Uint16(data[maxpOff+4 : maxpOff+6])
	return data[sbixOff : sbixOff+sbixLen], numGlyphs, true
}

// drawColorEmoji 把 text 中的字形按 sbix 彩色位图合成到 dst。
// x,y 为基线原点; 返回 (本段总步进宽度, 是否处理)。handled=false 时调用方回退,
// 且应忽略返回的 advance。每个字形按 emoji 字体的 hmtx advance (= 整 em) 步进,
// 与布局测量 (colorEmojiAdvance) 保持一致, 避免候选显得拥挤。
func drawColorEmoji(dst draw.Image, face ggtext.Face, text string, x, y float64, _ color.Color) (float64, bool) {
	if text == "" || !allEmoji(text) {
		// 只处理纯 emoji 段; 普通文字交回单色路径, 避免拿非 emoji 字形 ID
		// 误查 emoji sbix 表造成错乱。
		return 0, false
	}
	r := sbixRendererInstance()
	if r == nil || r.parsed == nil {
		return 0, false
	}
	parsed := r.parsed // 用 emoji 字体自身的 GlyphIndex, 与 sbix 表一致
	size := face.Size()
	ppem := uint16(size)
	if ppem == 0 {
		ppem = 1
	}
	strike := r.parser.BestStrikeForPPEM(ppem)
	if strike < 0 {
		return 0, false
	}
	cx := x
	for _, ru := range text {
		gid := parsed.GlyphIndex(ru)
		adv := parsed.GlyphAdvance(gid, size)
		if r.parser.HasGlyph(int(gid), strike) {
			if bm, err := r.parser.GetGlyph(int(gid), strike); err == nil {
				if img, err := bm.Decode(); err == nil {
					compositeBitmapGlyph(dst, img, bm, cx, y, ppem)
				}
			}
		}
		cx += adv
	}
	return cx - x, true
}

// colorEmojiAdvance 返回纯 emoji 段的步进总宽 (用 emoji 字体 hmtx advance, = 整 em)。
// 供 MeasureString 与 DrawString 取一致的宽度, 不依赖 gg 对位图字体不准的测量。
func colorEmojiAdvance(text string, size float64) (float64, bool) {
	if text == "" || !allEmoji(text) {
		return 0, false
	}
	r := sbixRendererInstance()
	if r == nil || r.parsed == nil {
		return 0, false
	}
	total := 0.0
	for _, ru := range text {
		total += r.parsed.GlyphAdvance(r.parsed.GlyphIndex(ru), size)
	}
	return total, true
}

// allEmoji 报告 text 是否全部为 emoji 相关码点 (emoji 本体 / 变体选择符 / ZWJ /
// 肤色修饰符)。用于把候选里的 emoji 段与普通文字段区分开。
func allEmoji(text string) bool {
	for _, ru := range text {
		switch {
		case emoji.IsEmoji(ru), emoji.IsVariationSelector(ru), emoji.IsZWJ(ru),
			emoji.IsEmojiModifier(ru), emoji.IsRegionalIndicator(ru):
			// ok
		default:
			return false
		}
	}
	return true
}

func compositeBitmapGlyph(dst draw.Image, img image.Image, bm *emoji.BitmapGlyph, x, y float64, ppem uint16) {
	if bm.PPEM == 0 {
		return
	}
	scale := float64(ppem) / float64(bm.PPEM)
	if scale <= 0 {
		scale = 1
	}
	w := int(float64(bm.Width) * scale)
	h := int(float64(bm.Height) * scale)
	if w <= 0 || h <= 0 {
		return
	}
	// sbix originOffsetY 多为 0 (位图左下角对齐基线)。上移整字高会让底边正好贴基线,
	// 但文字本身有 descent (基线下沉), 那样 emoji 会偏高一点。上移 0.85 字高, 让底边
	// 落在基线略下方 (≈descent), 与 CJK/拉丁字视觉底部对齐。
	destX := int(x + float64(bm.OriginX)*scale)
	destY := int(y - float64(bm.OriginY)*scale - float64(h)*0.85)
	rect := image.Rect(destX, destY, destX+w, destY+h)
	xdraw.CatmullRom.Scale(dst, rect, img, img.Bounds(), xdraw.Over, nil)
}
