package theme

import "image/color"

// ResolvedV25 是 v2.5 schema 解析后的最终形态，所有引用已展开、所有缺省字段已派生填充。
// P5 起为渲染层唯一解析结果来源（adapter/ResolvedTheme/v2 已退役）。
type ResolvedV25 struct {
	Meta     ThemeMeta
	Layout   ResolvedLayout
	Palette  ResolvedPalette
	Views    *Views           // 盒模型 View 外观（v2.6 P2）；nil=主题未提供 views，渲染器用合成桥
	Behavior ResolvedBehavior // 行为配置（v2.6 P6）：defaultBehavior ⊕ 主题 behavior（用户 override 在 ui/config 层）

	// Resources 图片资源注册表（v2.6 P7-C）：名→绝对路径 / data: URI（相对路径已按 theme 目录解析）。
	// 渲染器据此把 ViewImage.ref 解码为位图（一次性缓存，非每帧）。
	Resources map[string]string
}

// ResolvedLayout layout 的解析形态（与 LayoutSchema 同形，标量类型不变）。
// P7-5：候选窗几何/序号/强调条/行高已全部归口 views/behavior，layout 不再承载候选窗，仅剩其它窗口。
type ResolvedLayout struct {
	Density string
	Scale   float64

	Toolbar   ToolbarLayout
	Status    StatusLayout
	Tooltip   TooltipLayout
	PopupMenu PopupMenuLayout
	Toast     ToastLayout
}

// ResolvedPalette palette 的解析形态，所有颜色已 ParseColor 完成
type ResolvedPalette struct {
	IsDark  bool
	Primary color.Color

	// 顶层语义色
	Bg       color.Color
	Surface  color.Color
	Border   color.Color
	Text     color.Color
	TextDim  color.Color
	TextHint color.Color
	Accent   color.Color
	OnAccent color.Color
	Shadow   color.Color

	CandidateWindow ResolvedCandidateWindowPalette
	Toolbar         ResolvedToolbarPalette
	PopupMenu       ResolvedPopupMenuPalette
	Tooltip         ResolvedTooltipPalette
	Status          ResolvedStatusPalette
	Toast           ResolvedToastPalette
}

type ResolvedCandidateWindowPalette struct {
	Background   color.Color
	Border       color.Color
	Text         color.Color
	Comment      color.Color
	IndexBg      color.Color
	IndexText    color.Color
	HoverBg      color.Color
	SelectedBg   color.Color
	SelectedText color.Color
	PreeditBg    color.Color
	PreeditText  color.Color
	AccentBar    color.Color
}

type ResolvedToolbarPalette struct {
	Background       color.Color
	Border           color.Color
	Grip             color.Color
	ModeChineseBg    color.Color
	ModeEnglishBg    color.Color
	ModeText         color.Color
	FullWidthOffBg   color.Color
	FullWidthOffText color.Color
	PunctEnglishBg   color.Color
	PunctEnglishText color.Color
	SettingsBg       color.Color
	SettingsIcon     color.Color
	SettingsHole     color.Color
}

type ResolvedPopupMenuPalette struct {
	Background color.Color
	Border     color.Color
	Text       color.Color
	Disabled   color.Color
	HoverBg    color.Color
	HoverText  color.Color
	Separator  color.Color
}

type ResolvedTooltipPalette struct {
	Background color.Color
	Text       color.Color
}

type ResolvedStatusPalette struct {
	Background color.Color
	Border     color.Color
	Text       color.Color
}

type ResolvedToastPalette struct {
	Background color.Color
	Text       color.Color
}
