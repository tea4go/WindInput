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
		CandidateWindow: ResolvedCandidateWindowLayout{
			WindowPadding: Padding{Top: 6, Right: 8, Bottom: 6, Left: 8},
			BandGap:       2,
			BorderWidth:   1,
			BorderRadius:  8,
			PreeditBar: BandLayout{
				Visible:  true,
				Padding:  Padding{Top: 4, Right: 8, Bottom: 2, Left: 8},
				FontSize: 14,
			},
			CandidateList: CandidateListLayout{
				ItemPadding: Padding{Top: 4, Right: 10, Bottom: 4, Left: 8},
				ItemGap:     2,
				ItemHeight:  0,
				ItemRadius:  4,
				Index: IndexLayout{
					Labels:   []string{"1.", "2.", "3.", "4.", "5.", "6.", "7.", "8.", "9.", "0."},
					Circle:   false,
					Gap:      4,
					MinWidth: 18,
					FontSize: 12,
				},
				Text:    TextLayout{FontSize: 14},
				Comment: CommentLayout{Visible: true, FontSize: 12, Gap: 6, Placement: "inline"},
			},
			FooterBar: BandLayout{
				Visible:  true,
				Padding:  Padding{Top: 2, Right: 8, Bottom: 4, Left: 8},
				FontSize: 11,
			},
		},
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
	bumpPadding(&b.CandidateWindow.WindowPadding, 2)
	bumpPadding(&b.CandidateWindow.PreeditBar.Padding, 2)
	bumpPadding(&b.CandidateWindow.CandidateList.ItemPadding, 2)
	bumpPadding(&b.CandidateWindow.FooterBar.Padding, 2)
	bumpPadding(&b.Toolbar.Padding, 2)
	bumpPadding(&b.Status.Padding, 2)
	bumpPadding(&b.Tooltip.Padding, 2)
	bumpPadding(&b.PopupMenu.ItemPadding, 2)
	bumpPadding(&b.Toast.Padding, 2)
	b.CandidateWindow.PreeditBar.FontSize++
	b.CandidateWindow.CandidateList.Text.FontSize++
	b.CandidateWindow.CandidateList.Index.FontSize++
	b.CandidateWindow.CandidateList.Comment.FontSize++
	b.CandidateWindow.FooterBar.FontSize++
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
	bumpPadding(&b.CandidateWindow.WindowPadding, 4)
	bumpPadding(&b.CandidateWindow.PreeditBar.Padding, 4)
	bumpPadding(&b.CandidateWindow.CandidateList.ItemPadding, 4)
	bumpPadding(&b.CandidateWindow.FooterBar.Padding, 4)
	bumpPadding(&b.Toolbar.Padding, 4)
	bumpPadding(&b.Status.Padding, 4)
	bumpPadding(&b.Tooltip.Padding, 4)
	bumpPadding(&b.PopupMenu.ItemPadding, 4)
	bumpPadding(&b.Toast.Padding, 4)
	b.CandidateWindow.PreeditBar.FontSize += 2
	b.CandidateWindow.CandidateList.Text.FontSize += 2
	b.CandidateWindow.CandidateList.Index.FontSize += 2
	b.CandidateWindow.CandidateList.Comment.FontSize += 2
	b.CandidateWindow.FooterBar.FontSize += 2
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
//     （这些字段 0 无物理意义）。注意 Index.MinWidth 已属指针组（*int）。
//   - bool（Visible / Comment.Visible / AccentBar.Enabled）：基线给合理默认，用户显式 true 即覆盖，
//     显式 false 等同未写（无法区分），本期不暴露 band 关闭能力。
//   - string（Index.Style / Comment.Placement）：非空即覆盖。
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

	mergeCandidateWindow(&base.CandidateWindow, user.CandidateWindow)
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

func mergeBand(dst *BandLayout, src RawBandLayout) {
	mergePaddingPtr(&dst.Padding, src.Padding)
	if src.FontSize != 0 {
		dst.FontSize = src.FontSize
	}
}

func mergeCandidateWindow(dst *ResolvedCandidateWindowLayout, src RawCandidateWindowLayout) {
	mergePaddingPtr(&dst.WindowPadding, src.WindowPadding)
	if src.BandGap != nil {
		dst.BandGap = *src.BandGap
	}
	if src.BorderWidth != nil {
		dst.BorderWidth = *src.BorderWidth
	}
	if src.BorderRadius != nil {
		dst.BorderRadius = *src.BorderRadius
	}
	mergeBand(&dst.PreeditBar, src.PreeditBar)
	mergeCandidateList(&dst.CandidateList, src.CandidateList)
	mergeBand(&dst.FooterBar, src.FooterBar)
}

func mergeCandidateList(dst *CandidateListLayout, src RawCandidateListLayout) {
	mergePaddingPtr(&dst.ItemPadding, src.ItemPadding)
	if src.ItemGap != nil {
		dst.ItemGap = *src.ItemGap
	}
	if src.ItemHeight != 0 {
		dst.ItemHeight = src.ItemHeight
	}
	if src.ItemRadius != nil {
		dst.ItemRadius = *src.ItemRadius
	}
	if src.Index.Labels != nil {
		dst.Index.Labels = src.Index.Labels // 数组整体替换，不做 deep-merge
	}
	// Circle 为 bool：与 AccentBar.Enabled 同约定，用户显式 true 即覆盖，false 等同未写（基线默认 false）
	if src.Index.Circle {
		dst.Index.Circle = true
	}
	if src.Index.Gap != nil {
		dst.Index.Gap = *src.Index.Gap
	}
	if src.Index.MinWidth != nil {
		dst.Index.MinWidth = *src.Index.MinWidth
	}
	if src.Index.FontSize != 0 {
		dst.Index.FontSize = src.Index.FontSize
	}
	if src.Text.FontSize != 0 {
		dst.Text.FontSize = src.Text.FontSize
	}
	if src.Comment.FontSize != 0 {
		dst.Comment.FontSize = src.Comment.FontSize
	}
	if src.Comment.Gap != nil {
		dst.Comment.Gap = *src.Comment.Gap
	}
	if src.Comment.Placement != "" {
		dst.Comment.Placement = src.Comment.Placement
	}
	// Comment.Visible 同 BandLayout.Visible 约定

	// AccentBar: bool Enabled 用零值无法区分"未写"和"显式 false"；
	// 约定：基线 Enabled 始终为 false，用户显式写 true 即覆盖；显式 false 等同未写。
	if src.AccentBar.Enabled {
		dst.AccentBar.Enabled = true
	}
	if src.AccentBar.Width != nil {
		dst.AccentBar.Width = *src.AccentBar.Width
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
