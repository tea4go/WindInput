//go:build windows

package ui

import (
	"image/color"
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// TestEffectiveNode 守护架构统一核心：基态 ⊕ 当前激活状态扁平——状态非零字段覆盖基态、
// 未配状态=基态；字号不随状态变（决策）。
func TestEffectiveNode(t *testing.T) {
	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	blue := color.RGBA{0, 0, 255, 255}
	red := color.RGBA{255, 0, 0, 255}

	base := theme.RVNode{TextColor: black, FontWeight: 400, FontSize: 18}
	base.Selected = &theme.RVNode{
		BgColor: blue, BorderColor: red, BorderWidth: theme.Dp(2),
		TextColor: white, FontWeight: 700, FontFamily: "X", FontSize: 99,
	}

	// 无状态 → 基态原样。
	if eff := effectiveNode(base, false, false); eff.BgColor != nil || eff.TextColor != color.Color(black) || eff.FontWeight != 400 {
		t.Errorf("无状态应=基态, got bg=%v text=%v w=%d", eff.BgColor, eff.TextColor, eff.FontWeight)
	}

	// 选中态：bg/border/text/weight/family 被覆盖；字号保持基态（不随状态变）。
	eff := effectiveNode(base, true, false)
	if eff.BgColor != color.Color(blue) || eff.BorderColor != color.Color(red) || eff.BorderWidth != theme.Dp(2) {
		t.Errorf("选中态 bg/border 未覆盖: bg=%v border=%v bw=%v", eff.BgColor, eff.BorderColor, eff.BorderWidth)
	}
	if eff.TextColor != color.Color(white) || eff.FontWeight != 700 || eff.FontFamily != "X" {
		t.Errorf("选中态 文字色/字重/字体族 未覆盖: text=%v w=%d f=%q", eff.TextColor, eff.FontWeight, eff.FontFamily)
	}
	if eff.FontSize != 18 {
		t.Errorf("字号不应随状态变, 期望 18, got %v", eff.FontSize)
	}

	// selected 优先于 hover。
	base.Hover = &theme.RVNode{BgColor: red}
	if eff := effectiveNode(base, true, true); eff.BgColor != color.Color(blue) {
		t.Errorf("selected 应优先 hover, got bg=%v", eff.BgColor)
	}
	if eff := effectiveNode(base, false, true); eff.BgColor != color.Color(red) {
		t.Errorf("hover 态 bg 应=red, got %v", eff.BgColor)
	}
}
