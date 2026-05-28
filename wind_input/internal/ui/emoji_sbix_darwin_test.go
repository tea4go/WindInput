//go:build darwin

package ui

import (
	"image"
	"testing"

	ggtext "github.com/gogpu/gg/text"

	"github.com/huanfeng/wind_input/pkg/systemfont"
)

func TestSbixEmojiRendersNonBlank(t *testing.T) {
	path := systemfont.ResolveFile("Apple Color Emoji", false)
	if path == "" {
		t.Skip("Apple Color Emoji 未找到, 跳过")
	}
	src, err := ggtext.NewFontSourceFromFile(path)
	if err != nil {
		t.Fatalf("加载 emoji 字体失败: %v", err)
	}
	face := src.Face(32)

	r := sbixRendererInstance()
	if r == nil {
		t.Fatalf("sbix renderer 初始化失败 (extractSbixTable?)")
	}
	t.Logf("sbix numGlyphs=%d strikes=%d", r.numGlyph, r.parser.NumStrikes())

	dst := image.NewRGBA(image.Rect(0, 0, 64, 64))
	// 😀 U+1F600 基线放在 y=48
	if _, ok := drawColorEmoji(dst, face, "\U0001F600", 4, 48, nil); !ok {
		t.Fatalf("drawColorEmoji 返回 false (未画出)")
	}

	// 统计非透明像素
	nonTransparent := 0
	for i := 3; i < len(dst.Pix); i += 4 {
		if dst.Pix[i] != 0 {
			nonTransparent++
		}
	}
	t.Logf("非透明像素 = %d", nonTransparent)
	if nonTransparent < 50 {
		t.Fatalf("渲染近乎空白, 非透明像素仅 %d", nonTransparent)
	}
}
