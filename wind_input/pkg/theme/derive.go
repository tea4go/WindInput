package theme

import (
	"image/color"
	"math"
)

// derivedSemantics 表示从 primary 派生出的语义色集合。
// 仅在用户未显式填写时作为缺省值；用户显式值始终优先。
type derivedSemantics struct {
	Bg       string
	Surface  string
	Border   string
	Text     string
	TextDim  string
	TextHint string
	Accent   string
	OnAccent string
	Shadow   string
}

// derivePalette 根据 algorithm 与 isDark 派生一整套语义色。
// algorithm: hct | hsl-shift | none
// 本期 hct 暂以 hsl-shift 落地（spec §十二风险表已注明），后续 PR 再独立实现真正的 HCT。
func derivePalette(primaryHex string, algorithm string, isDark bool) derivedSemantics {
	if algorithm == "none" {
		return derivedSemantics{} // 全空，由调用方再用 fallback 兜底
	}
	return deriveHSLShift(primaryHex, isDark)
}

// deriveHSLShift 用 HSL 偏移派生语义色。
// 思路：以 primary 的 H 为锚点；按 isDark 选不同 L 阈值生成 surface/border/hover 等。
func deriveHSLShift(primaryHex string, isDark bool) derivedSemantics {
	p, err := ParseHexColor(primaryHex)
	if err != nil || p == nil {
		return derivedSemantics{}
	}
	r, g, b, _ := p.RGBA()
	h, _, _ := rgbToHSL(uint8(r>>8), uint8(g>>8), uint8(b>>8))

	if isDark {
		return derivedSemantics{
			Bg:       hslHex(0, 0, 0.18), // 深灰底
			Surface:  hslHex(0, 0, 0.23), // 略浅
			Border:   hslHex(0, 0, 0.30), // 边框
			Text:     hslHex(0, 0, 0.88), // 浅灰文字
			TextDim:  hslHex(0, 0, 0.69),
			TextHint: hslHex(0, 0, 0.50),
			Accent:   primaryHex,
			OnAccent: pickOnColor(p),
			Shadow:   "#0000001A",
		}
	}
	// light
	return derivedSemantics{
		Bg:       "#FFFFFF",
		Surface:  hslHex(0, 0, 0.94),
		Border:   hslHex(h, 0.08, 0.78), // 略带主色调的浅边框
		Text:     hslHex(0, 0, 0.12),
		TextDim:  hslHex(0, 0, 0.39),
		TextHint: hslHex(0, 0, 0.59),
		Accent:   primaryHex,
		OnAccent: pickOnColor(p),
		Shadow:   "#0000000F",
	}
}

// pickOnColor 按对比度选黑或白
func pickOnColor(c color.Color) string {
	r, g, b, _ := c.RGBA()
	// 相对亮度（WCAG）
	l := 0.2126*float64(r>>8)/255 + 0.7152*float64(g>>8)/255 + 0.0722*float64(b>>8)/255
	if l > 0.5 {
		return "#000000"
	}
	return "#FFFFFF"
}

// applyDerivedToVariant 用派生值填充 PaletteVariant 中未显式给出的语义色字段。
// 用户显式给值的字段不变。
func applyDerivedToVariant(v *PaletteVariant, d derivedSemantics) {
	if v.Bg == "" {
		v.Bg = d.Bg
	}
	if v.Surface == "" {
		v.Surface = d.Surface
	}
	if v.Border == "" {
		v.Border = d.Border
	}
	if v.Text == "" {
		v.Text = d.Text
	}
	if v.TextDim == "" {
		v.TextDim = d.TextDim
	}
	if v.TextHint == "" {
		v.TextHint = d.TextHint
	}
	if v.Accent == "" {
		v.Accent = d.Accent
	}
	if v.OnAccent == "" {
		v.OnAccent = d.OnAccent
	}
	if v.Shadow == "" {
		v.Shadow = d.Shadow
	}
}

// rgbToHSL converts 0..255 RGB to 0..1 HSL
func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l = (max + min) / 2
	if max == min {
		return 0, 0, l
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h /= 6
	return
}

// hslToRGB converts 0..1 HSL to 0..255 RGB
func hslToRGB(h, s, l float64) (r, g, b uint8) {
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r = uint8(math.Round(hueToRGB(p, q, h+1.0/3) * 255))
	g = uint8(math.Round(hueToRGB(p, q, h) * 255))
	b = uint8(math.Round(hueToRGB(p, q, h-1.0/3) * 255))
	return
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2 {
		return q
	}
	if t < 2.0/3 {
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

// hslHex 把 HSL 转为 #RRGGBB
func hslHex(h, s, l float64) string {
	r, g, b := hslToRGB(h, s, l)
	hex := []byte("#000000")
	const digits = "0123456789ABCDEF"
	hex[1] = digits[r>>4]
	hex[2] = digits[r&0xF]
	hex[3] = digits[g>>4]
	hex[4] = digits[g&0xF]
	hex[5] = digits[b>>4]
	hex[6] = digits[b&0xF]
	return string(hex)
}
