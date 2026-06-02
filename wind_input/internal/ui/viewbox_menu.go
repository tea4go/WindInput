package ui

// viewbox_menu.go — 弹出菜单 View 树构建与颜色解析（P4-D）。
// root LayoutColumn + 每项 LayoutRow（check/text/arrow），勾选✓/箭头▸/文本走 View 文本叶子；
// 分隔线后处理（矢量，定位用分隔项 Rect()）。复用 resolveTokenColor + newSharedDrawContext。

import (
	"image/color"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// resolveMenuColors 解析菜单 7 色：默认从 ResolvedTheme.PopupMenu，views.menu token 覆盖。
func (m *PopupMenu) resolveMenuColors() theme.ResolvedMenuViews {
	pm := m.getPopupMenuColors()
	rmv := theme.ResolvedMenuViews{
		BgColor: pm.BackgroundColor, BorderColor: pm.BorderColor, TextColor: pm.TextColor,
		DisabledColor: pm.DisabledColor, HoverBgColor: pm.HoverBgColor,
		HoverTextColor: pm.HoverTextColor, SeparatorColor: pm.SeparatorColor,
	}
	if m.themeViews == nil || m.themeViews.Menu == nil {
		return rmv
	}
	mv := m.themeViews.Menu
	res := func(name string) color.Color {
		switch name {
		case "background":
			return pm.BackgroundColor
		case "border":
			return pm.BorderColor
		case "text":
			return pm.TextColor
		case "disabled":
			return pm.DisabledColor
		case "hover_bg":
			return pm.HoverBgColor
		case "hover_text":
			return pm.HoverTextColor
		case "separator":
			return pm.SeparatorColor
		}
		return nil
	}
	set := func(dst *color.Color, s string) {
		if c := resolveTokenColor(s, res); c != nil {
			*dst = c
		}
	}
	set(&rmv.BgColor, mv.Background.Color)
	set(&rmv.BorderColor, mv.Border.Color)
	set(&rmv.TextColor, mv.Color)
	set(&rmv.SeparatorColor, mv.Separator.Color)
	set(&rmv.DisabledColor, mv.Disabled)
	set(&rmv.HoverBgColor, mv.Hover.Background.Color)
	set(&rmv.HoverTextColor, mv.Hover.Color)
	return rmv
}

// menuTree 持有 root + 分隔项 View 引用（分隔线后处理定位用其 Rect()）。
type menuTree struct {
	root       *View
	separators []*View
}

// buildMenuTree 构建菜单 View 树（root LayoutColumn + 每项 LayoutRow）。
// width/height 用预算值（与命中测试一致）。hoverIdx/submenuIdx 决定 hover 态。
// 勾选✓/箭头▸/文本走 View 文本叶子；分隔项收集到 separators 供后处理画线。
func buildMenuTree(items []MenuItem, hoverIdx, submenuIdx int, hasChecked, hasChildren bool, rmv theme.ResolvedMenuViews, width, height int, baseFontSize float64, itemHeightLogical int, scale float64) *menuTree {
	fontSize := baseFontSize * scale
	itemH := int(float64(itemHeightLogical) * scale)
	sepH := int(float64(menuSeparatorHeight) * scale)
	padY := int(float64(menuPaddingY) * scale)
	padXHalf := int(float64(menuPaddingX) * scale / 2)
	checkW := 0
	if hasChecked {
		checkW = int(float64(menuCheckMarkWidth) * scale)
	}
	arrowW := 0
	if hasChildren {
		arrowW = int(float64(menuArrowWidth) * scale)
	}
	radius := int(float64(menuCornerRadius) * scale)

	root := &View{
		FixedW:     width,
		FixedH:     height,
		Layout:     LayoutColumn,
		Padding:    Edges{Top: padY, Bottom: padY},
		Background: Fill{Color: rmv.BgColor},
		Border:     Border{Radius: radius, Color: rmv.BorderColor, Width: 1},
	}
	mt := &menuTree{root: root}

	for i := range items {
		item := items[i]
		if item.Separator {
			sep := &View{FixedH: sepH, Stretch: true}
			root.Children = append(root.Children, sep)
			mt.separators = append(mt.separators, sep)
			continue
		}
		isHovered := (i == hoverIdx && !item.Disabled) || (i == submenuIdx)
		textColor := rmv.TextColor
		switch {
		case item.Disabled:
			textColor = rmv.DisabledColor
		case isHovered:
			textColor = rmv.HoverTextColor
		}

		row := &View{
			FixedH:     itemH,
			Layout:     LayoutRow,
			Stretch:    true,        // 撑满 root 宽（hover 满宽）
			CrossAlign: AlignCenter, // check/text/arrow 在 itemH 内垂直居中
			Padding:    Edges{Left: padXHalf, Right: padXHalf},
		}
		if isHovered {
			row.Background = Fill{Color: rmv.HoverBgColor}
		}

		if hasChecked {
			check := &View{FixedW: checkW}
			if item.Checked {
				check.Text = "✓"
				check.TextStyle = TextStyle{FontSize: fontSize, Color: textColor, Align: AlignCenter}
			}
			row.Children = append(row.Children, check)
		}
		text := &View{
			Text:      item.Text,
			TextStyle: TextStyle{FontSize: fontSize, Color: textColor},
			Margin:    Edges{Left: padXHalf},
			Grow:      true,
		}
		row.Children = append(row.Children, text)
		if hasChildren {
			arrow := &View{FixedW: arrowW}
			if len(item.Children) > 0 {
				arrow.Text = "▸"
				arrow.TextStyle = TextStyle{FontSize: fontSize, Color: textColor, Align: AlignCenter}
			}
			row.Children = append(row.Children, arrow)
		}
		root.Children = append(root.Children, row)
	}
	return mt
}
