package theme

// PaletteSchema 描述主题的颜色，与 v2.5 spec §五 1:1 对应。
// 不含任何尺寸；尺寸由 LayoutSchema 描述。
type PaletteSchema struct {
	Meta    PaletteMeta  `yaml:"meta" json:"meta"`
	Primary string       `yaml:"primary" json:"primary"`
	Derive  DeriveConfig `yaml:"derive" json:"derive"`

	Light PaletteVariant `yaml:"light" json:"light"`
	Dark  PaletteVariant `yaml:"dark" json:"dark"`

	Background *PaletteBackground `yaml:"background,omitempty" json:"background,omitempty"`
}

// PaletteMeta 标识一个共享 palette 零件
type PaletteMeta struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

// DeriveConfig 颜色派生配置
type DeriveConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Algorithm string `yaml:"algorithm" json:"algorithm"` // hct | hsl-shift | none
}

// PaletteVariant 一个变体（light/dark）的完整颜色集
type PaletteVariant struct {
	// 顶层语义色
	Bg       string `yaml:"bg" json:"bg"`
	Surface  string `yaml:"surface" json:"surface"`
	Border   string `yaml:"border" json:"border"`
	Text     string `yaml:"text" json:"text"`
	TextDim  string `yaml:"text_dim" json:"text_dim"`
	TextHint string `yaml:"text_hint" json:"text_hint"`
	Accent   string `yaml:"accent" json:"accent"`
	OnAccent string `yaml:"on_accent" json:"on_accent"`
	Shadow   string `yaml:"shadow" json:"shadow"`

	// 组件覆盖（缺省回退到语义色）
	CandidateWindow CandidateWindowPalette `yaml:"candidate_window" json:"candidate_window"`
	Toolbar         ToolbarPalette         `yaml:"toolbar" json:"toolbar"`
	PopupMenu       PopupMenuPalette       `yaml:"popup_menu" json:"popup_menu"`
	Tooltip         TooltipPalette         `yaml:"tooltip" json:"tooltip"`
	Status          StatusPalette          `yaml:"status" json:"status"`
	Toast           ToastPalette           `yaml:"toast" json:"toast"`
}

// CandidateWindowPalette 候选窗口色板
type CandidateWindowPalette struct {
	Background   string `yaml:"background" json:"background"`
	Border       string `yaml:"border" json:"border"`
	Text         string `yaml:"text" json:"text"`
	Comment      string `yaml:"comment" json:"comment"`
	IndexBg      string `yaml:"index_bg" json:"index_bg"`
	IndexText    string `yaml:"index_text" json:"index_text"`
	HoverBg      string `yaml:"hover_bg" json:"hover_bg"`
	SelectedBg   string `yaml:"selected_bg" json:"selected_bg"`
	SelectedText string `yaml:"selected_text" json:"selected_text"`
	PreeditBg    string `yaml:"preedit_bg" json:"preedit_bg"`
	PreeditText  string `yaml:"preedit_text" json:"preedit_text"`
	AccentBar    string `yaml:"accent_bar" json:"accent_bar"` // 强调条颜色；空则用 ${accent}
}

// ToolbarPalette 工具栏色板（字段对应 v2 ToolbarColors，去掉 _color 后缀）
type ToolbarPalette struct {
	Background       string `yaml:"background" json:"background"`
	Border           string `yaml:"border" json:"border"`
	Grip             string `yaml:"grip" json:"grip"`
	ModeChineseBg    string `yaml:"mode_chinese_bg" json:"mode_chinese_bg"`
	ModeEnglishBg    string `yaml:"mode_english_bg" json:"mode_english_bg"`
	ModeText         string `yaml:"mode_text" json:"mode_text"`
	FullWidthOffBg   string `yaml:"full_width_off_bg" json:"full_width_off_bg"`
	FullWidthOffText string `yaml:"full_width_off_text" json:"full_width_off_text"`
	PunctEnglishBg   string `yaml:"punct_english_bg" json:"punct_english_bg"`
	PunctEnglishText string `yaml:"punct_english_text" json:"punct_english_text"`
	SettingsBg       string `yaml:"settings_bg" json:"settings_bg"`
	SettingsIcon     string `yaml:"settings_icon" json:"settings_icon"`
	SettingsHole     string `yaml:"settings_hole" json:"settings_hole"`
}

// PopupMenuPalette 弹出菜单色板
type PopupMenuPalette struct {
	Background string `yaml:"background" json:"background"`
	Border     string `yaml:"border" json:"border"`
	Text       string `yaml:"text" json:"text"`
	Disabled   string `yaml:"disabled" json:"disabled"`
	HoverBg    string `yaml:"hover_bg" json:"hover_bg"`
	HoverText  string `yaml:"hover_text" json:"hover_text"`
	Separator  string `yaml:"separator" json:"separator"`
}

// TooltipPalette tooltip 色板
type TooltipPalette struct {
	Background string `yaml:"background" json:"background"`
	Text       string `yaml:"text" json:"text"`
}

// StatusPalette 状态提示色板
type StatusPalette struct {
	Background string `yaml:"background" json:"background"`
	Border     string `yaml:"border" json:"border"`
	Text       string `yaml:"text" json:"text"`
}

// ToastPalette Toast 色板，本期预留
type ToastPalette struct {
	Background string `yaml:"background" json:"background"`
	Text       string `yaml:"text" json:"text"`
}

// PaletteBackground 背景图配置
// Opacity 使用 *float64 以区分"未设置"（nil，回退默认 1.0）与"显式 0"（全透明）。
type PaletteBackground struct {
	Image     string   `yaml:"image" json:"image"` // 文件路径或 data: URI
	Mode      string   `yaml:"mode" json:"mode"`   // nine_slice | stretch | tile | center
	Slice     Padding  `yaml:"slice" json:"slice"` // 仅 nine_slice 用
	Opacity   *float64 `yaml:"opacity,omitempty" json:"opacity,omitempty"`
	DarkImage string   `yaml:"dark_image" json:"dark_image"` // 暗色专用，可选
}
