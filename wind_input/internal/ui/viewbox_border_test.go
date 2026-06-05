//go:build windows

package ui

import (
	"image/color"
	"testing"
)

// TestPaintBorderUniform 守护边框为均匀粗细的实心环（even-odd 填充「外圆角矩形−内圆角矩形」，
// 非 gg 中心描边）。gg@v0.48.3 的 Stroke 存在 AA 渗色/对圆角路径偏移不均，致边框粗细变化；
// 填充环保证四边各处恒为 Width 宽、带内每像素都是实心边框色。
func TestPaintBorderUniform(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	const W, H, BW = 24, 24, 2
	v := &View{
		FixedW: W, FixedH: H,
		Background: Fill{Color: white},
		Border:     Border{Color: red, Width: BW}, // rad=0 直角，便于逐像素核验
	}
	Layout(v, 0, 0, fixedMeasurer{charW: 1})
	dc, img := newSharedDrawContext(W, H)
	v.paintShapes(dc, img)

	// band：从扫描线外缘起，连续「精确等于边框色」的像素数（AA 渗色会致非精确像素、提前中断）。
	band := func(pixels []color.RGBA) int {
		n := 0
		for _, c := range pixels {
			if c != red {
				break
			}
			n++
		}
		return n
	}
	col := func(x int) []color.RGBA {
		out := make([]color.RGBA, H)
		for y := 0; y < H; y++ {
			out[y] = img.RGBAAt(x, y)
		}
		return out
	}
	row := func(y int) []color.RGBA {
		out := make([]color.RGBA, W)
		for x := 0; x < W; x++ {
			out[x] = img.RGBAAt(x, y)
		}
		return out
	}
	reverse := func(s []color.RGBA) []color.RGBA {
		out := make([]color.RGBA, len(s))
		for i := range s {
			out[len(s)-1-i] = s[i]
		}
		return out
	}

	// 远离圆角的直边段（中部多点）四边厚度恒=BW。
	for _, p := range []int{3, 8, 12, 16, 20} {
		if n := band(col(p)); n != BW {
			t.Errorf("顶边 x=%d 厚度应=%d, got %d", p, BW, n)
		}
		if n := band(reverse(col(p))); n != BW {
			t.Errorf("底边 x=%d 厚度应=%d, got %d", p, BW, n)
		}
		if n := band(row(p)); n != BW {
			t.Errorf("左边 y=%d 厚度应=%d, got %d", p, BW, n)
		}
		if n := band(reverse(row(p))); n != BW {
			t.Errorf("右边 y=%d 厚度应=%d, got %d", p, BW, n)
		}
	}
	if img.RGBAAt(12, 12) != white {
		t.Errorf("盒内部应为底色白（边框厚度恰=2，不溢入内部）, got %+v", img.RGBAAt(12, 12))
	}
}
