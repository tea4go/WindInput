package ui

// viewbox_status.go — 状态泡（mode indicator bubble）的 View 树构建与颜色解析（P4-A）。
// 状态泡复用包级 Layout/PaintTree 引擎核心；颜色经 token 解析自 views.status，
// 默认映射 ResolvedTheme.ModeIndicator（零回归），运行时 StatusWindowConfig 自定义色优先。

import (
	"image/color"
	"strings"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// resolveTokenColor 通用颜色 token 解析：hex(#RRGGBB[AA]) 直解；${name} 交给 resolver；
// 空 / 未知 token / 解析失败返回 nil（调用方据此回退）。各窗口注入自己的 resolver。
func resolveTokenColor(s string, resolver func(name string) color.Color) color.Color {
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		return resolver(s[2 : len(s)-1])
	}
	if c, err := theme.ParseHexColor(s); err == nil {
		return c
	}
	return nil
}

// resolveStatusColors 计算状态泡最终颜色，优先级：自定义 cfg > views.status token > ModeIndicator 默认。
func (r *StatusRenderer) resolveStatusColors(cfg StatusWindowConfig) theme.ResolvedStatusViews {
	// base：ModeIndicator 主题色（无主题时用内置默认）
	bg := color.Color(color.RGBA{60, 60, 60, 240})
	text := color.Color(color.RGBA{255, 255, 255, 255})
	if r.resolvedTheme != nil {
		bg = r.resolvedTheme.ModeIndicator.BackgroundColor
		text = r.resolvedTheme.ModeIndicator.TextColor
	}

	// views.status token 覆盖（resolver 映射到 ModeIndicator 同源色）
	if r.themeViews != nil && r.themeViews.Status != nil {
		res := func(name string) color.Color {
			if r.resolvedTheme == nil {
				return nil
			}
			switch name {
			case "background":
				return r.resolvedTheme.ModeIndicator.BackgroundColor
			case "text":
				return r.resolvedTheme.ModeIndicator.TextColor
			}
			return nil
		}
		if c := resolveTokenColor(r.themeViews.Status.Background.Color, res); c != nil {
			bg = c
		}
		if c := resolveTokenColor(r.themeViews.Status.Color, res); c != nil {
			text = c
		}
	}

	// 自定义 cfg 优先级最高
	if cfg.BackgroundColor != "" {
		if c, ok := parseHexColor(cfg.BackgroundColor); ok {
			bg = c
		}
	}
	if cfg.TextColor != "" {
		if c, ok := parseHexColor(cfg.TextColor); ok {
			text = c
		}
	}

	return theme.ResolvedStatusViews{BgColor: bg, TextColor: text}
}

// buildStatusTree 构建状态泡的 View 树（单文本节点：bg 圆角 + padding + 居中文本）。
// 所有尺寸入参为逻辑像素，内部乘 scale 得最终像素（与现状 Render 一致）。
// minWidth=32（逻辑）经 FixedW 钳制；高 = fontSize + padding*2（由 measure 算出）。
func buildStatusTree(text string, rsv theme.ResolvedStatusViews, fontSize, padding, borderRadius, scale float64, m TextMeasurer) *View {
	fs := fontSize * scale
	pad := int(padding * scale)
	minW := int(32.0 * scale)

	tw := m.MeasureString(text, fs)
	w := max(int(tw)+pad*2, minW)

	return &View{
		Text:       text,
		TextStyle:  TextStyle{FontSize: fs, Color: rsv.TextColor, Align: AlignCenter},
		Padding:    Edges{Top: pad, Right: pad, Bottom: pad, Left: pad},
		Background: Fill{Color: rsv.BgColor},
		Border:     Border{Radius: int(borderRadius * scale)},
		FixedW:     w,
	}
}
