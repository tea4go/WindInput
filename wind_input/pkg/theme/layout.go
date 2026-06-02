package theme

import "strings"

// BuildIndexLabelsFromSlots 把序号槽位 []string 拼成 "/" 分隔串（候选窗 IndexLabels）。
// 槽位 0→候选序号 1、…、槽位 9→第 10 个候选；不足 10 或空槽回退默认数字（1..9,0）。
// 约束：单个标签不应含 '/'（渲染器以 '/' 切分槽位），此处不做转义。
func BuildIndexLabelsFromSlots(labels []string) string {
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

// LayoutSchema 描述主题的尺寸与布局，与 v2.5 spec §四 的 yaml 字段 1:1 对应。
// 不含任何颜色；颜色由 PaletteSchema 描述。
//
// 这是 *原始解析层*（Raw 层）：距离/圆角/间隙/边框 类字段用 `*int` 表示，
// nil 表示"用户未在 yaml 中写该字段"（回退 density 基线），非 nil（含 0）表示"用户显式给值"。
// 经 mergeWithDensityBaseline 合并后产出 plain int 的 ResolvedLayout。
type LayoutSchema struct {
	Meta    LayoutMeta `yaml:"meta" json:"meta"`
	Density string     `yaml:"density" json:"density"` // compact | cozy | comfortable
	Scale   float64    `yaml:"scale" json:"scale"`     // 整体缩放，DPI 适配后再乘

	// P7-5：候选窗几何/序号/强调条/行高已迁 views/behavior，layout 不再承载候选窗（yaml 中残留的
	// candidate_window: 块会被静默忽略）。
	Toolbar   RawToolbarLayout   `yaml:"toolbar" json:"toolbar"`
	Status    RawStatusLayout    `yaml:"status" json:"status"`
	Tooltip   RawTooltipLayout   `yaml:"tooltip" json:"tooltip"`
	PopupMenu RawPopupMenuLayout `yaml:"popup_menu" json:"popup_menu"`
	Toast     RawToastLayout     `yaml:"toast" json:"toast"` // 预留，本期不消费
}

// LayoutMeta 标识一个共享 layout 零件
type LayoutMeta struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

// intPtr 返回 v 的指针，用于构造 Raw* 结构体字面量与测试。
func intPtr(v int) *int { return &v }

// ---- Raw 层（原始解析）：距离/圆角/间隙/边框 用 *int 支持"显式 0" ----

// RawPadding 通用四边内边距（原始解析层，nil=未写）
type RawPadding struct {
	Top    *int `yaml:"top" json:"top"`
	Right  *int `yaml:"right" json:"right"`
	Bottom *int `yaml:"bottom" json:"bottom"`
	Left   *int `yaml:"left" json:"left"`
}

// RawToolbarLayout 工具栏布局（原始解析层）
type RawToolbarLayout struct {
	Height  int        `yaml:"height" json:"height"`
	Padding RawPadding `yaml:"padding" json:"padding"`
	ItemGap *int       `yaml:"item_gap" json:"item_gap"`
}

// RawStatusLayout 状态提示（原始解析层）
type RawStatusLayout struct {
	BorderWidth *int       `yaml:"border_width" json:"border_width"`
	Padding     RawPadding `yaml:"padding" json:"padding"`
}

// RawTooltipLayout 候选提示（原始解析层）
type RawTooltipLayout struct {
	Padding      RawPadding `yaml:"padding" json:"padding"`
	FontSize     int        `yaml:"font_size" json:"font_size"`
	BorderRadius *int       `yaml:"border_radius" json:"border_radius"`
	MaxWidth     int        `yaml:"max_width" json:"max_width"`
}

// RawPopupMenuLayout 弹出菜单（原始解析层）
type RawPopupMenuLayout struct {
	ItemPadding     RawPadding `yaml:"item_padding" json:"item_padding"`
	ItemHeight      int        `yaml:"item_height" json:"item_height"`
	SeparatorHeight *int       `yaml:"separator_height" json:"separator_height"`
}

// RawToastLayout Toast 通知（原始解析层），本期预留不消费
type RawToastLayout struct {
	Padding      RawPadding `yaml:"padding" json:"padding"`
	FontSize     int        `yaml:"font_size" json:"font_size"`
	BorderRadius *int       `yaml:"border_radius" json:"border_radius"`
	MaxWidth     int        `yaml:"max_width" json:"max_width"`
}

// ---- Plain 层（供 Resolved 复用，标量类型保持 plain int）----

// Padding 通用四边内边距
type Padding struct {
	Top    int `yaml:"top" json:"top"`
	Right  int `yaml:"right" json:"right"`
	Bottom int `yaml:"bottom" json:"bottom"`
	Left   int `yaml:"left" json:"left"`
}

// ToolbarLayout 工具栏布局
type ToolbarLayout struct {
	Height  int     `yaml:"height" json:"height"`
	Padding Padding `yaml:"padding" json:"padding"`
	ItemGap int     `yaml:"item_gap" json:"item_gap"`
}

// StatusLayout 状态提示（temp_pinyin 等）
type StatusLayout struct {
	BorderWidth int     `yaml:"border_width" json:"border_width"`
	Padding     Padding `yaml:"padding" json:"padding"`
}

// TooltipLayout 候选提示
type TooltipLayout struct {
	Padding      Padding `yaml:"padding" json:"padding"`
	FontSize     int     `yaml:"font_size" json:"font_size"`
	BorderRadius int     `yaml:"border_radius" json:"border_radius"`
	MaxWidth     int     `yaml:"max_width" json:"max_width"`
}

// PopupMenuLayout 弹出菜单
type PopupMenuLayout struct {
	ItemPadding     Padding `yaml:"item_padding" json:"item_padding"`
	ItemHeight      int     `yaml:"item_height" json:"item_height"`
	SeparatorHeight int     `yaml:"separator_height" json:"separator_height"`
}

// ToastLayout Toast 通知，本期预留不消费
type ToastLayout struct {
	Padding      Padding `yaml:"padding" json:"padding"`
	FontSize     int     `yaml:"font_size" json:"font_size"`
	BorderRadius int     `yaml:"border_radius" json:"border_radius"`
	MaxWidth     int     `yaml:"max_width" json:"max_width"`
}
