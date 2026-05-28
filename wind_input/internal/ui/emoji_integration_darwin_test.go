//go:build darwin

package ui

import (
	"image"
	"image/color"
	"testing"
)

// 走完整 freeTypeDrawer 路径 (主字体 + fallback 链 + segmentByFont + drawColorEmoji),
// 验证候选里的 emoji 能真正画出 (复现真实渲染管线, 而非直接喂 emoji face)。
func TestEmojiThroughDrawerPipeline(t *testing.T) {
	m := NewTextBackendManager("test")
	// 主字体用中文字体 (与 forwarder 一致); emoji 应经 fallback 链命中 Apple Color Emoji。
	for _, fam := range []string{"PingFang SC", "Hiragino Sans GB", "STHeiti"} {
		m.SetFontFamily(fam)
		if m.FontReady() {
			t.Logf("primary=%s", fam)
			break
		}
	}
	m.SetTextRenderMode(TextRenderModeFreetype)
	td := m.TextDrawer()
	if td == nil {
		t.Fatal("textDrawer nil")
	}

	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	td.BeginDraw(img)
	// 混合: 中文 + emoji, 验证两者都画出且互不干扰
	td.DrawString("好\U0001F600", 4, 56, 32, color.Black)
	td.EndDraw()

	nonTransparent := 0
	colored := 0 // 非黑非透明 = 彩色 emoji 像素
	for i := 0; i < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		if a == 0 {
			continue
		}
		nonTransparent++
		r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		if !(r == g && g == b) { // 非灰阶 → 彩色
			colored++
		}
	}
	t.Logf("非透明=%d 彩色=%d", nonTransparent, colored)
	if nonTransparent < 50 {
		t.Fatalf("整体近空白 (非透明=%d)", nonTransparent)
	}
	if colored < 20 {
		t.Fatalf("emoji 未彩色渲染 (彩色像素=%d)", colored)
	}
}
