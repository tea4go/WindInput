package theme

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

// LoadBackgroundImage 从文件路径或 data: URI 加载图片为 *image.RGBA。
func LoadBackgroundImage(pathOrDataURI string) (*image.RGBA, error) {
	if pathOrDataURI == "" {
		return nil, fmt.Errorf("empty image path")
	}
	if strings.HasPrefix(pathOrDataURI, "data:") {
		return decodeDataURI(pathOrDataURI)
	}
	f, err := os.Open(pathOrDataURI)
	if err != nil {
		return nil, fmt.Errorf("open background image: %w", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode background image: %w", err)
	}
	return toRGBA(img), nil
}

func decodeDataURI(uri string) (*image.RGBA, error) {
	// data:image/png;base64,xxxxx
	commaIdx := strings.IndexByte(uri, ',')
	if commaIdx < 0 {
		return nil, fmt.Errorf("invalid data URI: missing comma")
	}
	header := uri[:commaIdx]
	body := uri[commaIdx+1:]
	if !strings.Contains(header, ";base64") {
		return nil, fmt.Errorf("only base64-encoded data URIs supported")
	}
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode data URI image: %w", err)
	}
	return toRGBA(img), nil
}

func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba
}

// DrawBackground 把 src 按 mode 绘制到 dst 的 rect 区域。
// mode: nine_slice | stretch | tile | center
// slice: 仅 nine_slice 用，描述源图四边边距像素值
// opacity: 0..1，作为整体 alpha 倍率应用到 src
func DrawBackground(dst *image.RGBA, rect image.Rectangle, src *image.RGBA, mode string, slice Padding, opacity float64) {
	if src == nil || rect.Empty() {
		return
	}
	if opacity <= 0 {
		return
	}
	if opacity > 1 {
		opacity = 1
	}

	switch mode {
	case "nine_slice":
		drawNineSlice(dst, rect, src, slice, opacity)
	case "tile":
		drawTile(dst, rect, src, opacity)
	case "center":
		drawCenter(dst, rect, src, opacity)
	case "stretch":
		fallthrough
	default:
		drawStretch(dst, rect, src, opacity)
	}
}

func drawStretch(dst *image.RGBA, rect image.Rectangle, src *image.RGBA, opacity float64) {
	scaled := scaleImage(src, rect.Dx(), rect.Dy())
	blendOver(dst, rect.Min, scaled, scaled.Bounds(), opacity)
}

func drawCenter(dst *image.RGBA, rect image.Rectangle, src *image.RGBA, opacity float64) {
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	x := rect.Min.X + (rect.Dx()-w)/2
	y := rect.Min.Y + (rect.Dy()-h)/2
	// 裁剪到 rect
	dstClip := image.Rect(x, y, x+w, y+h).Intersect(rect)
	if dstClip.Empty() {
		return
	}
	// 计算 src 中对应裁剪
	srcClip := image.Rect(
		sb.Min.X+(dstClip.Min.X-x),
		sb.Min.Y+(dstClip.Min.Y-y),
		sb.Min.X+(dstClip.Max.X-x),
		sb.Min.Y+(dstClip.Max.Y-y),
	)
	blendOver(dst, dstClip.Min, src, srcClip, opacity)
}

func drawTile(dst *image.RGBA, rect image.Rectangle, src *image.RGBA, opacity float64) {
	sb := src.Bounds()
	tw, th := sb.Dx(), sb.Dy()
	if tw == 0 || th == 0 {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y += th {
		for x := rect.Min.X; x < rect.Max.X; x += tw {
			tile := image.Rect(x, y, x+tw, y+th).Intersect(rect)
			if tile.Empty() {
				continue
			}
			srcClip := image.Rect(
				sb.Min.X,
				sb.Min.Y,
				sb.Min.X+tile.Dx(),
				sb.Min.Y+tile.Dy(),
			)
			blendOver(dst, tile.Min, src, srcClip, opacity)
		}
	}
}

// drawNineSlice 把 src 按 slice 划分为 9 块：四角原样、四边在主轴方向拉伸、中心双轴拉伸
func drawNineSlice(dst *image.RGBA, rect image.Rectangle, src *image.RGBA, slice Padding, opacity float64) {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()

	// 边距夹紧避免越界
	clamp := func(v, max int) int {
		if v < 0 {
			return 0
		}
		if v > max {
			return max
		}
		return v
	}
	st := clamp(slice.Top, sh)
	sbt := clamp(slice.Bottom, sh)
	sl := clamp(slice.Left, sw)
	sr := clamp(slice.Right, sw)
	if st+sbt >= sh {
		st, sbt = sh/3, sh/3
	}
	if sl+sr >= sw {
		sl, sr = sw/3, sw/3
	}

	dw, dh := rect.Dx(), rect.Dy()
	dt := st
	dbt := sbt
	dl := sl
	dr := sr
	if dt+dbt > dh {
		ratio := float64(dh) / float64(dt+dbt)
		dt = int(float64(dt) * ratio)
		dbt = dh - dt
	}
	if dl+dr > dw {
		ratio := float64(dw) / float64(dl+dr)
		dl = int(float64(dl) * ratio)
		dr = dw - dl
	}

	// 9 区源/目标矩形
	type slice9 struct{ src, dst image.Rectangle }
	x0, y0 := sb.Min.X, sb.Min.Y
	X0, Y0 := rect.Min.X, rect.Min.Y
	slices := []slice9{
		// 上排：左上 / 上中 / 右上
		{image.Rect(x0, y0, x0+sl, y0+st), image.Rect(X0, Y0, X0+dl, Y0+dt)},
		{image.Rect(x0+sl, y0, x0+sw-sr, y0+st), image.Rect(X0+dl, Y0, X0+dw-dr, Y0+dt)},
		{image.Rect(x0+sw-sr, y0, x0+sw, y0+st), image.Rect(X0+dw-dr, Y0, X0+dw, Y0+dt)},
		// 中排：左中 / 中心 / 右中
		{image.Rect(x0, y0+st, x0+sl, y0+sh-sbt), image.Rect(X0, Y0+dt, X0+dl, Y0+dh-dbt)},
		{image.Rect(x0+sl, y0+st, x0+sw-sr, y0+sh-sbt), image.Rect(X0+dl, Y0+dt, X0+dw-dr, Y0+dh-dbt)},
		{image.Rect(x0+sw-sr, y0+st, x0+sw, y0+sh-sbt), image.Rect(X0+dw-dr, Y0+dt, X0+dw, Y0+dh-dbt)},
		// 下排：左下 / 下中 / 右下
		{image.Rect(x0, y0+sh-sbt, x0+sl, y0+sh), image.Rect(X0, Y0+dh-dbt, X0+dl, Y0+dh)},
		{image.Rect(x0+sl, y0+sh-sbt, x0+sw-sr, y0+sh), image.Rect(X0+dl, Y0+dh-dbt, X0+dw-dr, Y0+dh)},
		{image.Rect(x0+sw-sr, y0+sh-sbt, x0+sw, y0+sh), image.Rect(X0+dw-dr, Y0+dh-dbt, X0+dw, Y0+dh)},
	}
	for _, s := range slices {
		if s.src.Empty() || s.dst.Empty() {
			continue
		}
		// 角块尺寸相同直接 copy；否则缩放
		if s.src.Dx() == s.dst.Dx() && s.src.Dy() == s.dst.Dy() {
			blendOver(dst, s.dst.Min, src, s.src, opacity)
		} else {
			scaled := scaleImageRect(src, s.src, s.dst.Dx(), s.dst.Dy())
			blendOver(dst, s.dst.Min, scaled, scaled.Bounds(), opacity)
		}
	}
}

// scaleImage 使用最近邻把 src 缩放到 (w, h)
func scaleImage(src *image.RGBA, w, h int) *image.RGBA {
	return scaleImageRect(src, src.Bounds(), w, h)
}

// scaleImageRect 把 src 中的 srcRect 缩放到 (w, h)，最近邻；直接读 src.Pix 避免 At() 开销
func scaleImageRect(src *image.RGBA, srcRect image.Rectangle, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sw, sh := srcRect.Dx(), srcRect.Dy()
	if sw == 0 || sh == 0 || w == 0 || h == 0 {
		return dst
	}
	sb := src.Bounds()
	for dy := 0; dy < h; dy++ {
		sy := srcRect.Min.Y + dy*sh/h
		if sy >= srcRect.Max.Y {
			sy = srcRect.Max.Y - 1
		}
		for dx := 0; dx < w; dx++ {
			sx := srcRect.Min.X + dx*sw/w
			if sx >= srcRect.Max.X {
				sx = srcRect.Max.X - 1
			}
			sOff := (sy-sb.Min.Y)*src.Stride + (sx-sb.Min.X)*4
			dOff := dy*dst.Stride + dx*4
			dst.Pix[dOff] = src.Pix[sOff]
			dst.Pix[dOff+1] = src.Pix[sOff+1]
			dst.Pix[dOff+2] = src.Pix[sOff+2]
			dst.Pix[dOff+3] = src.Pix[sOff+3]
		}
	}
	return dst
}

// blendOver 以 opacity 倍率把 src 的 srcRect 区域 over-blend 到 dst 的 dstMin 起点。
//
// 圆角保护：仅当 dst 当前像素 alpha > 0 时绘制（候选窗在画 bg 图前已 Fill 圆角背景色，
// 圆角外区域 alpha=0 或来自 shadow 的低 alpha，按 dst alpha 比例减弱避免污染）。
//
// over operator 近似：out_rgb = src_rgb*src_a + dst_rgb*(1-src_a)，
// 假定 src/dst 均为 straight (non-premultiplied) alpha；对单次叠绘视觉接近正确，
// 多次叠绘会有色偏（本场景每帧只画一次）。
func blendOver(dst *image.RGBA, dstMin image.Point, src *image.RGBA, srcRect image.Rectangle, opacity float64) {
	if opacity <= 0 {
		return
	}
	alphaMul := uint32(opacity*255 + 0.5)

	dx0, dy0 := dstMin.X, dstMin.Y
	sx0, sy0 := srcRect.Min.X, srcRect.Min.Y
	w, h := srcRect.Dx(), srcRect.Dy()

	dstBounds := dst.Bounds()
	srcBounds := src.Bounds()
	for y := 0; y < h; y++ {
		dy := dy0 + y
		if dy < dstBounds.Min.Y || dy >= dstBounds.Max.Y {
			continue
		}
		sy := sy0 + y
		if sy < srcBounds.Min.Y || sy >= srcBounds.Max.Y {
			continue
		}
		for x := 0; x < w; x++ {
			dx := dx0 + x
			if dx < dstBounds.Min.X || dx >= dstBounds.Max.X {
				continue
			}
			sx := sx0 + x
			if sx < srcBounds.Min.X || sx >= srcBounds.Max.X {
				continue
			}

			dOff := (dy-dstBounds.Min.Y)*dst.Stride + (dx-dstBounds.Min.X)*4
			dA := uint32(dst.Pix[dOff+3])
			// 圆角保护：dst 完全透明的像素不绘制
			if dA == 0 {
				continue
			}

			sOff := (sy-srcBounds.Min.Y)*src.Stride + (sx-srcBounds.Min.X)*4
			sR := uint32(src.Pix[sOff])
			sG := uint32(src.Pix[sOff+1])
			sB := uint32(src.Pix[sOff+2])
			sA := uint32(src.Pix[sOff+3]) * alphaMul / 255
			// 用 dst 当前 alpha 限制 src alpha，避免圆角抗锯齿边缘被覆盖为不透明
			if sA > dA {
				sA = dA
			}

			dR := uint32(dst.Pix[dOff])
			dG := uint32(dst.Pix[dOff+1])
			dB := uint32(dst.Pix[dOff+2])

			inv := 255 - sA
			outR := (sR*sA + dR*inv) / 255
			outG := (sG*sA + dG*inv) / 255
			outB := (sB*sA + dB*inv) / 255
			outA := sA + dA*inv/255
			dst.Pix[dOff] = uint8(outR)
			dst.Pix[dOff+1] = uint8(outG)
			dst.Pix[dOff+2] = uint8(outB)
			dst.Pix[dOff+3] = uint8(outA)
		}
	}
}
