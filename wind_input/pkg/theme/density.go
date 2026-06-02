package theme

// density 基线表 — 用户在 yaml 中写 `density: compact` 等同于先把所有 layout 字段填成该档基线，
// 再用 yaml 中显式写出的字段覆盖。基线与代码同源，不暴露为用户可配置文件。

// densityBaseline 返回指定 density 档位的完整 ResolvedLayout 基线。
// 未识别的档位回退到 compact。
func densityBaseline(d string) ResolvedLayout {
	switch d {
	case "cozy":
		return cozyBaseline()
	case "comfortable":
		return comfortableBaseline()
	default:
		return compactBaseline()
	}
}

func compactBaseline() ResolvedLayout {
	return ResolvedLayout{
		Density: "compact",
		Scale:   1.0,
		Toolbar: ToolbarLayout{
			Height:  28,
			Padding: Padding{Top: 4, Right: 6, Bottom: 4, Left: 6},
			ItemGap: 4,
		},
		Status: StatusLayout{
			BorderWidth: 2,
			Padding:     Padding{Top: 4, Right: 8, Bottom: 4, Left: 8},
		},
		Tooltip: TooltipLayout{
			Padding:      Padding{Top: 4, Right: 8, Bottom: 4, Left: 8},
			FontSize:     12,
			BorderRadius: 4,
			MaxWidth:     320,
		},
		PopupMenu: PopupMenuLayout{
			ItemPadding:     Padding{Top: 4, Right: 12, Bottom: 4, Left: 12},
			ItemHeight:      24,
			SeparatorHeight: 1,
		},
		Toast: ToastLayout{
			Padding:      Padding{Top: 8, Right: 12, Bottom: 8, Left: 12},
			FontSize:     13,
			BorderRadius: 8,
			MaxWidth:     280,
		},
	}
}

func cozyBaseline() ResolvedLayout {
	// cozy = compact 各内边距 +2、字号 +1
	b := compactBaseline()
	b.Density = "cozy"
	bumpPadding(&b.Toolbar.Padding, 2)
	bumpPadding(&b.Status.Padding, 2)
	bumpPadding(&b.Tooltip.Padding, 2)
	bumpPadding(&b.PopupMenu.ItemPadding, 2)
	bumpPadding(&b.Toast.Padding, 2)
	b.Tooltip.FontSize++
	b.Toast.FontSize++
	b.Toolbar.Height += 2
	b.PopupMenu.ItemHeight += 2
	return b
}

func comfortableBaseline() ResolvedLayout {
	// comfortable = compact 各内边距 +4、字号 +2
	b := compactBaseline()
	b.Density = "comfortable"
	bumpPadding(&b.Toolbar.Padding, 4)
	bumpPadding(&b.Status.Padding, 4)
	bumpPadding(&b.Tooltip.Padding, 4)
	bumpPadding(&b.PopupMenu.ItemPadding, 4)
	bumpPadding(&b.Toast.Padding, 4)
	b.Tooltip.FontSize += 2
	b.Toast.FontSize += 2
	b.Toolbar.Height += 4
	b.PopupMenu.ItemHeight += 4
	return b
}

func bumpPadding(p *Padding, delta int) {
	p.Top += delta
	p.Right += delta
	p.Bottom += delta
	p.Left += delta
}

// mergeWithDensityBaseline 把用户写出的 layout 字段（Raw 层）叠加到 density 基线上，
// 直接产出 plain int 的 ResolvedLayout。
//
// 合并语义：
//   - 距离/圆角/间隙/边框 类字段（*int）：nil=未写→保留基线；非 nil（含 0）=显式覆盖。
//     因此 `border_radius: 0` / `padding: {top: 0}` 等"显式关闭"语义被正确支持。
//   - 字号/高度/最大宽度/最小宽度无关项及 Scale（plain int / float）：仍沿用零值=回退基线
//     （这些字段 0 无物理意义）。
//   - bool（Visible / Comment.Visible）：基线给合理默认，用户显式 true 即覆盖，
//     显式 false 等同未写（无法区分），本期不暴露 band 关闭能力。
//   - string（Comment.Placement）：非空即覆盖。
//
// P7-5：候选窗已不在 layout（迁 views/behavior），此处仅合并其它窗口。
func mergeWithDensityBaseline(user LayoutSchema) ResolvedLayout {
	d := user.Density
	if d == "" {
		d = "compact"
	}
	base := densityBaseline(d)

	// scale：用户显式给值才覆盖
	if user.Scale != 0 {
		base.Scale = user.Scale
	}

	mergeToolbar(&base.Toolbar, user.Toolbar)
	mergeStatus(&base.Status, user.Status)
	mergeTooltip(&base.Tooltip, user.Tooltip)
	mergePopupMenu(&base.PopupMenu, user.PopupMenu)
	mergeToast(&base.Toast, user.Toast)

	return base
}

// mergePaddingPtr 把 Raw 层 padding（*int）合并到 plain Padding：每边 nil 保留基线，非 nil（含 0）覆盖。
func mergePaddingPtr(dst *Padding, src RawPadding) {
	if src.Top != nil {
		dst.Top = *src.Top
	}
	if src.Right != nil {
		dst.Right = *src.Right
	}
	if src.Bottom != nil {
		dst.Bottom = *src.Bottom
	}
	if src.Left != nil {
		dst.Left = *src.Left
	}
}

func mergeToolbar(dst *ToolbarLayout, src RawToolbarLayout) {
	if src.Height != 0 {
		dst.Height = src.Height
	}
	mergePaddingPtr(&dst.Padding, src.Padding)
	if src.ItemGap != nil {
		dst.ItemGap = *src.ItemGap
	}
}

func mergeStatus(dst *StatusLayout, src RawStatusLayout) {
	if src.BorderWidth != nil {
		dst.BorderWidth = *src.BorderWidth
	}
	mergePaddingPtr(&dst.Padding, src.Padding)
}

func mergeTooltip(dst *TooltipLayout, src RawTooltipLayout) {
	mergePaddingPtr(&dst.Padding, src.Padding)
	if src.FontSize != 0 {
		dst.FontSize = src.FontSize
	}
	if src.BorderRadius != nil {
		dst.BorderRadius = *src.BorderRadius
	}
	if src.MaxWidth != 0 {
		dst.MaxWidth = src.MaxWidth
	}
}

func mergePopupMenu(dst *PopupMenuLayout, src RawPopupMenuLayout) {
	mergePaddingPtr(&dst.ItemPadding, src.ItemPadding)
	if src.ItemHeight != 0 {
		dst.ItemHeight = src.ItemHeight
	}
	if src.SeparatorHeight != nil {
		dst.SeparatorHeight = *src.SeparatorHeight
	}
}

func mergeToast(dst *ToastLayout, src RawToastLayout) {
	mergePaddingPtr(&dst.Padding, src.Padding)
	if src.FontSize != 0 {
		dst.FontSize = src.FontSize
	}
	if src.BorderRadius != nil {
		dst.BorderRadius = *src.BorderRadius
	}
	if src.MaxWidth != 0 {
		dst.MaxWidth = src.MaxWidth
	}
}
