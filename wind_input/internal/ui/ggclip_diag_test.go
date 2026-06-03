//go:build windows

package ui

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestGGClipOnPixmap 诊断 gg.Clip() 在 newSharedDrawContext（gg.NewContextForPixmap + pm.ImageView
// 共享缓冲）上是否真正生效——这决定圆角裁剪能否靠 dc.Clip 实现，还是必须像素层 mask。
//
// 做法：设一个圆角矩形 clip，再用满矩形填充整张画布。
//   - 若 Clip 生效：圆角外的角落像素不被填充（alpha=0）。
//   - 若 Clip 是 no-op：角落被填满（alpha!=0）。
//
// 同时输出 PNG 供肉眼确认（路径打印到测试日志）。
func TestGGClipOnPixmap(t *testing.T) {
	const w, h, rad = 60, 40, 12

	dc, img := newSharedDrawContext(w, h)
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), float64(rad))
	dc.Clip()
	dc.SetColor(color.RGBA{255, 0, 0, 255})
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	dc.Fill()

	out := filepath.Join(os.TempDir(), "wind_ggclip_diag.png")
	if f, err := os.Create(out); err == nil {
		_ = png.Encode(f, img)
		_ = f.Close()
		t.Logf("诊断 PNG（肉眼确认四角是否被裁圆）: %s", out)
	}

	corner := alphaAt(img, 0, 0)        // 圆角外：(0.5,0.5) 距圆心(12,12)≈16.3 > 12
	center := alphaAt(img, w/2, h/2)    // clip 内
	edgeMid := alphaAt(img, w/2, 0)     // 顶边中点：在圆角矩形内（非角落），应被填充
	t.Logf("alpha — 角落(0,0)=%d  顶边中点(%d,0)=%d  中心=%d", corner, w/2, edgeMid, center)

	if center == 0 {
		t.Fatal("中心 alpha=0 → 连 clip 内都没画上，绘制本身失败（非 clip 问题）")
	}
	if edgeMid == 0 {
		t.Error("顶边中点 alpha=0 → clip 把非角落区域也裁了，clip 形状异常")
	}
	if corner != 0 {
		t.Logf("结论：gg.Clip() 在 NewContextForPixmap 上【未生效】(no-op)，角落 alpha=%d。"+
			"圆角裁剪必须走像素层 mask（menu 已采用 maskRoundedCorners）。", corner)
	} else {
		t.Log("结论：gg.Clip() 在 NewContextForPixmap 上【生效】，角落被正确裁透明。")
	}
}

func alphaAt(img interface{ At(x, y int) color.Color }, x, y int) uint32 {
	_, _, _, a := img.At(x, y).RGBA()
	return a >> 8
}
