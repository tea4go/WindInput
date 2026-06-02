package theme

import "strings"

// ResolvedToLegacy 把 v2.5 的 ResolvedV25 适配为 v2 的 ResolvedTheme，
// 用于 renderer 在过渡期统一消费同一个 ResolvedTheme 接口。
//
// 字段映射：
//   - Layout: candidate_window → CandidateWindowStyle (尺寸/间距)，工具栏/menu/tooltip 等其它 layout 暂未透传
//     （renderer 端使用的尺寸主要集中在 candidate_window，其它组件仍走自身硬编码）
//   - Palette: light/dark variant → ResolvedTheme 各组件颜色（通过 colorToHex 再 Resolve）
//   - Style.IndexStyle / IndexLabels: IndexLabels 由 layout.candidate_list.index.labels 直接拼接；
//     IndexStyle 仅由 index.circle 决定（circle → "circle" 画圆背景，否则 "text"），与 labels 内容正交
func ResolvedToLegacy(rv *ResolvedV25) *ResolvedTheme {
	if rv == nil {
		return nil
	}

	idx := rv.Layout.CandidateWindow.CandidateList.Index
	indexStyle := "text"
	if idx.Circle {
		indexStyle = "circle"
	}
	indexLabels := buildIndexLabelsFromSlots(idx.Labels)

	out := &ResolvedTheme{
		Meta: rv.Meta,
		Style: ResolvedCandidateWindowStyle{
			IndexStyle:        indexStyle,
			IndexLabels:       indexLabels,
			WindowPaddingX:    float64(rv.Layout.CandidateWindow.WindowPadding.Left),
			WindowPaddingY:    float64(rv.Layout.CandidateWindow.WindowPadding.Top),
			CornerRadius:      float64(rv.Layout.CandidateWindow.BorderRadius),
			RowHeight:         float64(rv.Layout.CandidateWindow.CandidateList.ItemHeight),
			ItemPaddingLeft:   float64(rv.Layout.CandidateWindow.CandidateList.ItemPadding.Left),
			ItemPaddingRight:  float64(rv.Layout.CandidateWindow.CandidateList.ItemPadding.Right),
			IndexMarginRight:  float64(rv.Layout.CandidateWindow.CandidateList.Index.Gap),
			CommentMarginLeft: float64(rv.Layout.CandidateWindow.CandidateList.Comment.Gap),
			IndexFontWeight:   0,
			AccentBarColor:    rv.Palette.CandidateWindow.AccentBar,
			HasAccentBar:      rv.Layout.CandidateWindow.CandidateList.AccentBar.Enabled,
			ShowPageNumber:    true,
			AlwaysShowPager:   false,
		},
		CandidateWindow: ResolvedCandidateWindowColors{
			BackgroundColor: rv.Palette.CandidateWindow.Background,
			BorderColor:     rv.Palette.CandidateWindow.Border,
			TextColor:       rv.Palette.CandidateWindow.Text,
			IndexColor:      rv.Palette.CandidateWindow.IndexText,
			IndexBgColor:    rv.Palette.CandidateWindow.IndexBg,
			HoverBgColor:    rv.Palette.CandidateWindow.HoverBg,
			SelectedBgColor: rv.Palette.CandidateWindow.SelectedBg,
			InputBgColor:    rv.Palette.CandidateWindow.PreeditBg,
			InputTextColor:  rv.Palette.CandidateWindow.PreeditText,
			CommentColor:    rv.Palette.CandidateWindow.Comment,
			ShadowColor:     rv.Palette.Shadow,
		},
		Toolbar: ResolvedToolbarColors{
			BackgroundColor:     rv.Palette.Toolbar.Background,
			BorderColor:         rv.Palette.Toolbar.Border,
			GripColor:           rv.Palette.Toolbar.Grip,
			ModeChineseBgColor:  rv.Palette.Toolbar.ModeChineseBg,
			ModeEnglishBgColor:  rv.Palette.Toolbar.ModeEnglishBg,
			ModeTextColor:       rv.Palette.Toolbar.ModeText,
			FullWidthOnBgColor:  rv.Palette.Toolbar.FullWidthOnBg,
			FullWidthOffBgColor: rv.Palette.Toolbar.FullWidthOffBg,
			FullWidthOnColor:    rv.Palette.Toolbar.FullWidthOnText,
			FullWidthOffColor:   rv.Palette.Toolbar.FullWidthOffText,
			PunctChineseBgColor: rv.Palette.Toolbar.PunctChineseBg,
			PunctEnglishBgColor: rv.Palette.Toolbar.PunctEnglishBg,
			PunctChineseColor:   rv.Palette.Toolbar.PunctChineseText,
			PunctEnglishColor:   rv.Palette.Toolbar.PunctEnglishText,
			SettingsBgColor:     rv.Palette.Toolbar.SettingsBg,
			SettingsIconColor:   rv.Palette.Toolbar.SettingsIcon,
			SettingsHoleColor:   rv.Palette.Toolbar.SettingsHole,
		},
		PopupMenu: ResolvedPopupMenuColors{
			BackgroundColor: rv.Palette.PopupMenu.Background,
			BorderColor:     rv.Palette.PopupMenu.Border,
			TextColor:       rv.Palette.PopupMenu.Text,
			DisabledColor:   rv.Palette.PopupMenu.Disabled,
			HoverBgColor:    rv.Palette.PopupMenu.HoverBg,
			HoverTextColor:  rv.Palette.PopupMenu.HoverText,
			SeparatorColor:  rv.Palette.PopupMenu.Separator,
		},
		Tooltip: ResolvedTooltipColors{
			BackgroundColor: rv.Palette.Tooltip.Background,
			TextColor:       rv.Palette.Tooltip.Text,
		},
		ModeIndicator: ResolvedModeIndicatorColors{
			BackgroundColor: rv.Palette.Toast.Background,
			TextColor:       rv.Palette.Toast.Text,
		},
	}
	if rv.Palette.Background != nil {
		out.Background = &ResolvedThemeBackground{
			Mode:    rv.Palette.Background.Mode,
			Slice:   rv.Palette.Background.Slice,
			Opacity: rv.Palette.Background.Opacity,
			// Image 由 manager 在 resolveTheme 之后 attach（解码图片是 I/O，不放在适配层）
		}
	}
	return out
}

// buildIndexLabelsFromSlots 把逐位标签数组转换为渲染器消费的 /-分隔串。
// 槽位 0→候选序号 1、…、槽位 9→第 10 个候选（index 0）。
// 不足 10 项或某槽为空串的位置回退默认数字（1..9,0）。
// 约束：单个标签不应含 '/'（渲染器以 '/' 切分槽位），此处不做转义。
func buildIndexLabelsFromSlots(labels []string) string {
	digits := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}
	parts := make([]string, 10)
	for i := range 10 {
		if i < len(labels) && labels[i] != "" {
			parts[i] = labels[i]
		} else {
			parts[i] = digits[i]
		}
	}
	return strings.Join(parts, "/")
}
