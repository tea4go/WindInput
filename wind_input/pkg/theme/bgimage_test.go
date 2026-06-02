package theme

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// makeTestImage 生成 width x height 的纯色 RGBA 图
func makeTestImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestLoadBackgroundImage_File(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bgimg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	src := makeTestImage(8, 8, color.RGBA{255, 0, 0, 255})
	p := filepath.Join(tmp, "test.png")
	f, _ := os.Create(p)
	_ = png.Encode(f, src)
	f.Close()

	img, err := LoadBackgroundImage(p)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Errorf("size mismatch")
	}
	r, _, _, _ := img.At(0, 0).RGBA()
	if r>>8 != 255 {
		t.Errorf("expected red pixel")
	}
}

func TestLoadBackgroundImage_DataURI(t *testing.T) {
	src := makeTestImage(4, 4, color.RGBA{0, 255, 0, 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, src)
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	img, err := LoadBackgroundImage(uri)
	if err != nil {
		t.Fatalf("load data URI: %v", err)
	}
	_, g, _, _ := img.At(0, 0).RGBA()
	if g>>8 != 255 {
		t.Errorf("expected green pixel")
	}
}

func TestLoadBackgroundImage_InvalidPath(t *testing.T) {
	if _, err := LoadBackgroundImage("/nonexistent/x.png"); err == nil {
		t.Errorf("expected error for missing file")
	}
}

// fillAlpha 预填 dst 为不透明白色，模拟 renderer 先画背景色再叠 bg image 的流程
func fillAlpha(dst *image.RGBA) {
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
}

func TestDrawBackground_Stretch(t *testing.T) {
	src := makeTestImage(2, 2, color.RGBA{255, 0, 0, 255})
	dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fillAlpha(dst)
	DrawBackground(dst, dst.Bounds(), src, "stretch", Padding{}, 1.0, 0)
	r, _, _, a := dst.At(5, 5).RGBA()
	if r>>8 != 255 || a>>8 != 255 {
		t.Errorf("stretch center should be opaque red, got rgba=%d,%d", r>>8, a>>8)
	}
}

func TestDrawBackground_Tile(t *testing.T) {
	src := makeTestImage(3, 3, color.RGBA{0, 0, 255, 255})
	dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fillAlpha(dst)
	DrawBackground(dst, dst.Bounds(), src, "tile", Padding{}, 1.0, 0)
	_, _, b, _ := dst.At(9, 9).RGBA()
	if b>>8 != 255 {
		t.Errorf("tile bottom-right should be blue")
	}
}

func TestDrawBackground_Center(t *testing.T) {
	src := makeTestImage(2, 2, color.RGBA{128, 128, 128, 255})
	dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
	// 填充 dst 为白色，便于看出 center 区域
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			dst.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	DrawBackground(dst, dst.Bounds(), src, "center", Padding{}, 1.0, 0)
	// 中心 (4,4) 或 (5,5) 应是灰色
	r, _, _, _ := dst.At(4, 4).RGBA()
	if r>>8 != 128 {
		t.Errorf("center pixel should be gray 128, got %d", r>>8)
	}
	// 边角仍是白色
	r2, _, _, _ := dst.At(0, 0).RGBA()
	if r2>>8 != 255 {
		t.Errorf("corner should remain white, got %d", r2>>8)
	}
}

func TestDrawBackground_NineSlice(t *testing.T) {
	// 5x5 源图：中心一个像素与四角不同色，便于验证拉伸不污染四角
	src := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			if x == 2 && y == 2 {
				src.Set(x, y, color.RGBA{0, 255, 0, 255}) // 中心绿
			} else {
				src.Set(x, y, color.RGBA{255, 0, 0, 255}) // 其它红
			}
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 20, 20))
	fillAlpha(dst)
	DrawBackground(dst, dst.Bounds(), src, "nine_slice", Padding{Top: 2, Right: 2, Bottom: 2, Left: 2}, 1.0, 0)
	// 四角应为红色（原样复制）
	r, _, _, _ := dst.At(0, 0).RGBA()
	if r>>8 != 255 {
		t.Errorf("nine_slice corner should be red, got %d", r>>8)
	}
	// 中心应为绿色（来自中心 1x1 块拉伸）
	_, g, _, _ := dst.At(10, 10).RGBA()
	if g>>8 != 255 {
		t.Errorf("nine_slice center should be green, got %d", g>>8)
	}
}

// TestDrawBackground_SemiTransparentEdge 守护预乘 alpha 合成修复：
// 半透明像素叠到不透明底色上必须按预乘 over 正确羽化，而不是被 src_a 二次衰减发暗。
// 早期 blendOver 误把预乘 RGB 当 straight、再乘一次 alpha，使水印半透明边缘出现暗环
// （视觉上像多了一圈边框）。50% 橙 (242,140,40) 叠白底：正确结果 R≈248，旧误算 R≈187。
func TestDrawBackground_SemiTransparentEdge(t *testing.T) {
	// 直 alpha 橙色 @ 50%，经 toRGBA 预乘——复刻真实加载管线（NRGBA→RGBA 预乘）
	n := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(n.Pix); i += 4 {
		n.Pix[i], n.Pix[i+1], n.Pix[i+2], n.Pix[i+3] = 242, 140, 40, 128
	}
	src := toRGBA(n)

	dst := image.NewRGBA(image.Rect(0, 0, 2, 2))
	fillAlpha(dst) // 不透明白底

	DrawBackground(dst, dst.Bounds(), src, "stretch", Padding{}, 1.0, 0)

	r, _, _, a := dst.At(0, 0).RGBA()
	r8, a8 := r>>8, a>>8
	// 旧 straight 误算 R≈187（明显偏暗）；预乘正确 R≈248。断言 R>=230 区分二者。
	if r8 < 230 {
		t.Errorf("半透明边缘叠白后 R 应≈248（预乘合成），得 %d——疑似回退 straight 二次衰减发暗", r8)
	}
	if a8 != 255 {
		t.Errorf("叠到不透明底色后 alpha 应为 255，得 %d", a8)
	}
}

// TestDrawBackground_RoundedCornerClip 守护 P7-D 圆角裁剪：内部元素（四周已被不透明底色填满，
// alpha-gate 失效）传 radius>0 时，半径外的四角不被位图覆盖、露出底色；中心正常绘制。
func TestDrawBackground_RoundedCornerClip(t *testing.T) {
	// 不透明红底，模拟选中候选项四周的窗口底色
	dst := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = 255, 0, 0, 255
	}
	src := makeTestImage(4, 4, color.RGBA{0, 0, 255, 255}) // 纯蓝高亮
	DrawBackground(dst, dst.Bounds(), src, "stretch", Padding{}, 1.0, 6)

	// 角 (0,0)：在 radius=6 圆弧外 → 被裁，保留红底
	r, _, b, _ := dst.At(0, 0).RGBA()
	if b>>8 > 64 {
		t.Errorf("圆角外 (0,0) 不应被高亮图覆盖, got b=%d", b>>8)
	}
	if r>>8 < 200 {
		t.Errorf("圆角外 (0,0) 应保留底色红, got r=%d", r>>8)
	}
	// 中心 (10,10)：圆角内 → 蓝色高亮
	_, _, bc, _ := dst.At(10, 10).RGBA()
	if bc>>8 < 200 {
		t.Errorf("中心应被高亮图覆盖, got b=%d", bc>>8)
	}
}

func TestDrawBackground_OpacityZero(t *testing.T) {
	src := makeTestImage(4, 4, color.RGBA{255, 0, 0, 255})
	dst := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			dst.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	DrawBackground(dst, dst.Bounds(), src, "stretch", Padding{}, 0.0, 0)
	r, _, _, _ := dst.At(2, 2).RGBA()
	if r>>8 != 0 {
		t.Errorf("opacity=0 should not draw, dst remains black, got r=%d", r>>8)
	}
}
