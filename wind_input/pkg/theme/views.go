package theme

// views.go — 盒模型 View 主题 schema（v2.6 P2）。
//
// 两类形态：
//   - YAML schema（Views/ViewNode，*int 显式语义）：主题文件 views: 块的解析目标，
//     沿用 v2.5「nil=未写回退基线，非 nil（含 0）=显式值」。
//   - 渲染消费形态（ResolvedViews/RVNode，plain 值 + 已解析颜色）：候选窗 build 直接读取。
//
// 本切片（切片-0）YAML 可配字段 = margin/padding/border/background.color/font/color/labels/states；
// 其余尺寸（item height、window gap、shadow、accent 宽、派生公式输入）由渲染器从运行时配置/
// 内置默认提供，ResolvedViews 以额外字段承载。Image/layers/gradient 留后续切片。

import "image/color"

// ViewEdges 四向距离（逻辑像素）。*int：nil=未写（回退基线），非 nil（含 0）=显式值。
type ViewEdges struct {
	Top    *int `yaml:"top,omitempty"`
	Right  *int `yaml:"right,omitempty"`
	Bottom *int `yaml:"bottom,omitempty"`
	Left   *int `yaml:"left,omitempty"`
}

// ViewFill 背景填充。本切片仅 Color；Image/Gradient 留后续切片。
type ViewFill struct {
	Color string `yaml:"color,omitempty"` // ColorToken: "#RRGGBB[AA]" | "${semantic}" | "transparent"
}

// ViewBorder 边框。
type ViewBorder struct {
	Width  *int   `yaml:"width,omitempty"`
	Color  string `yaml:"color,omitempty"`
	Radius *int   `yaml:"radius,omitempty"`
}

// ViewNode 一个具名 View 的外观属性（盒模型 + Text 属性）。
type ViewNode struct {
	Margin     ViewEdges  `yaml:"margin,omitempty"`
	Padding    ViewEdges  `yaml:"padding,omitempty"`
	Background ViewFill   `yaml:"background,omitempty"`
	Border     ViewBorder `yaml:"border,omitempty"`
	FontFamily string     `yaml:"font_family,omitempty"`
	FontSize   *int       `yaml:"font_size,omitempty"`
	FontWeight *int       `yaml:"font_weight,omitempty"`
	Color      string     `yaml:"color,omitempty"`  // 文本色 token
	Labels     []string   `yaml:"labels,omitempty"` // 仅 index：序号槽位字符（≤10）
	Selected   *ViewNode  `yaml:"selected,omitempty"`
	Hover      *ViewNode  `yaml:"hover,omitempty"`
}

// Views 具名 View 集合（固定骨架，设计文档 D3）。
type Views struct {
	Window        ViewNode      `yaml:"window,omitempty"`
	PreeditBar    ViewNode      `yaml:"preedit_bar,omitempty"`
	CandidateList ViewNode      `yaml:"candidate_list,omitempty"`
	Item          ViewNode      `yaml:"item,omitempty"`
	Index         ViewNode      `yaml:"index,omitempty"`
	Text          ViewNode      `yaml:"text,omitempty"`
	Comment       ViewNode      `yaml:"comment,omitempty"`
	AccentBar     ViewNode      `yaml:"accent_bar,omitempty"`
	FooterBar     ViewNode      `yaml:"footer_bar,omitempty"`
	Status        *ViewNode     `yaml:"status,omitempty"`  // P4-A 状态泡（独立窗口，单节点）
	Tooltip       *ViewNode     `yaml:"tooltip,omitempty"` // P4-B Tooltip（独立窗口，单节点）
	Toolbar       *ToolbarViews `yaml:"toolbar,omitempty"` // P4-C 工具栏
	Menu          *MenuViews    `yaml:"menu,omitempty"`    // P4-D 弹出菜单
}

// RVNode 渲染消费形态的单个 View 外观（plain 逻辑像素 + 颜色）。
// 各字段为该 View 实际用到的子集；零值表示「用渲染器内置默认」。
// 切片-0 只填几何字段（margin/padding/border 尺寸 + FontSize/FontWeight）；
// 颜色字段（BgColor/BorderColor/TextColor/SelectedBg/HoverBg）留颜色迁移切片，
// 本切片颜色仍由渲染器从 RenderConfig 读（颜色不影响几何对齐）。
type RVNode struct {
	MarginTop, MarginRight, MarginBottom, MarginLeft int
	PadTop, PadRight, PadBottom, PadLeft             int
	BorderRadius                                     int
	BorderWidth                                      int
	BorderColor                                      color.Color
	BgColor                                          color.Color
	FontSize                                         float64
	FontWeight                                       int
	TextColor                                        color.Color
	SelectedBg                                       color.Color
	HoverBg                                          color.Color
}

// ResolvedViews 候选窗各具名 View 的解析后外观（plain 逻辑像素，渲染器直接读）。
// 派生布局公式（index 圆直径、comment/pager 字号、inputH 等）留在渲染器，其输入从这里取。
// 几何杂项（Window/Item 等不便归入单 View margin/padding 的尺寸）以顶层字段承载。
type ResolvedViews struct {
	Window        RVNode
	PreeditBar    RVNode
	CandidateList RVNode
	Item          RVNode
	Index         RVNode
	Text          RVNode
	Comment       RVNode
	AccentBar     RVNode
	FooterBar     RVNode

	// 几何杂项（逻辑像素 / 倍率）
	WindowGap        int         // window 列间距（band 之间）
	ShadowOffset     int         // 窗口投影偏移
	ItemHeight       float64     // 行高（rowH = round(ItemHeight)）
	ItemSpacing      int         // 横排候选框间距基数（已按 isTextIndex 选定 12/16）
	AccentBarWidth   int         // 强调条宽
	AccentBarOffset  int         // 强调条左缘偏移
	AccentBarHRatio  float64     // 强调条高 = ItemHeight * 此比例
	VerticalMaxWidth float64     // 竖排最大宽（逻辑像素）
	ShadowColor      color.Color // 窗口投影颜色（P2 切片-1）
}

// ResolvedStatusViews 状态泡解析后外观（P4-A）。仅颜色——几何/字号由运行时 StatusWindowConfig 提供。
type ResolvedStatusViews struct {
	BgColor   color.Color
	TextColor color.Color
}

// ResolvedTooltipViews Tooltip 解析后外观（P4-B）。仅颜色——几何由 render 内置默认（hardcode）。
type ResolvedTooltipViews struct {
	BgColor   color.Color
	TextColor color.Color
}

// ToolbarViews 工具栏 YAML schema（P4-C）。button base + mode 状态覆盖 + settings 齿轮色。
type ToolbarViews struct {
	Background ViewFill            `yaml:"background,omitempty"`
	Border     ViewBorder          `yaml:"border,omitempty"`
	Grip       ViewNode            `yaml:"grip,omitempty"`
	Button     ToolbarButtonNode   `yaml:"button,omitempty"`
	Settings   ToolbarSettingsNode `yaml:"settings,omitempty"`
}

// ToolbarButtonNode 按钮通用 base（background/color）+ mode 状态覆盖。
type ToolbarButtonNode struct {
	Background ViewFill           `yaml:"background,omitempty"`
	Color      string             `yaml:"color,omitempty"`
	Border     ViewBorder         `yaml:"border,omitempty"`
	Mode       *ToolbarModeStates `yaml:"mode,omitempty"`
}

// ToolbarModeStates 模式按钮中/英两态覆盖（仅 background）。
type ToolbarModeStates struct {
	Chinese ViewNode `yaml:"chinese,omitempty"`
	English ViewNode `yaml:"english,omitempty"`
}

// ToolbarSettingsNode 设置按钮：background（继承 button base 若空）+ 齿轮 icon/hole 色。
type ToolbarSettingsNode struct {
	Background ViewFill `yaml:"background,omitempty"`
	Icon       ViewFill `yaml:"icon,omitempty"`
	Hole       ViewFill `yaml:"hole,omitempty"`
}

// ResolvedToolbarViews 工具栏解析后扁平颜色集（P4-C）。几何 hardcode；mode 中/英 build 按 state 选。
type ResolvedToolbarViews struct {
	BarBg, BarBorder, Grip                 color.Color
	ButtonBg, ButtonText                   color.Color // base（width/punct/settings 共用）
	ModeChineseBg, ModeEnglishBg, ModeText color.Color
	SettingsBg, SettingsIcon, SettingsHole color.Color
}

// MenuViews 弹出菜单 YAML schema（P4-D）。7 色：背景/边框/文本/分隔/禁用 + hover 状态。
type MenuViews struct {
	Background ViewFill       `yaml:"background,omitempty"`
	Border     ViewBorder     `yaml:"border,omitempty"`
	Color      string         `yaml:"color,omitempty"`     // 普通文本
	Separator  ViewFill       `yaml:"separator,omitempty"` // 分隔线色（用 .Color）
	Disabled   string         `yaml:"disabled,omitempty"`  // 禁用文本
	Hover      MenuHoverState `yaml:"hover,omitempty"`
}

// MenuHoverState 菜单项 hover 覆盖：背景 + 文本。
type MenuHoverState struct {
	Background ViewFill `yaml:"background,omitempty"`
	Color      string   `yaml:"color,omitempty"`
}

// ResolvedMenuViews 菜单解析后扁平 7 色集（P4-D）。
type ResolvedMenuViews struct {
	BgColor, BorderColor, TextColor             color.Color
	DisabledColor, HoverBgColor, HoverTextColor color.Color
	SeparatorColor                              color.Color
}

func intp(v int) *int { return &v }

// edgeOr 返回指针值或回退默认（保留显式 0）。
func edgeOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// defaultViews 返回与现 viewbox_build magic number 等价的 YAML 基线（density=compact 量级，
// 逻辑像素，未乘 DPI scale）。仅覆盖本切片 YAML 可配字段；颜色/字号由 palette 与运行时提供，
// 故基线不含颜色。主题 views 块以此为基线覆盖。
func defaultViews() Views {
	return Views{
		Window:     ViewNode{Padding: ViewEdges{Top: intp(8), Right: intp(8), Bottom: intp(8), Left: intp(8)}, Border: ViewBorder{Radius: intp(8)}},
		PreeditBar: ViewNode{Padding: ViewEdges{Right: intp(8), Left: intp(8)}, Border: ViewBorder{Radius: intp(4)}},
		Item:       ViewNode{Padding: ViewEdges{Right: intp(8), Left: intp(8)}, Border: ViewBorder{Radius: intp(4)}},
		Index:      ViewNode{},
		Text:       ViewNode{Margin: ViewEdges{Left: intp(4)}}, // index→text 间距
		Comment:    ViewNode{Margin: ViewEdges{Left: intp(8)}}, // text→comment 间距
		AccentBar:  ViewNode{},
		FooterBar:  ViewNode{},
	}
}

// mergeEdges 用 ov 的非 nil 字段覆盖 base（保留显式 0）。
func mergeEdges(base, ov ViewEdges) ViewEdges {
	out := base
	if ov.Top != nil {
		out.Top = ov.Top
	}
	if ov.Right != nil {
		out.Right = ov.Right
	}
	if ov.Bottom != nil {
		out.Bottom = ov.Bottom
	}
	if ov.Left != nil {
		out.Left = ov.Left
	}
	return out
}

// mergeViewNode 用 ov 覆盖 base：指针字段非 nil、string 非空、slice 非 nil 时覆盖；
// Selected/Hover 子 patch 递归同规则。
func mergeViewNode(base, ov ViewNode) ViewNode {
	out := base
	out.Margin = mergeEdges(base.Margin, ov.Margin)
	out.Padding = mergeEdges(base.Padding, ov.Padding)
	if ov.Background.Color != "" {
		out.Background.Color = ov.Background.Color
	}
	if ov.Border.Width != nil {
		out.Border.Width = ov.Border.Width
	}
	if ov.Border.Color != "" {
		out.Border.Color = ov.Border.Color
	}
	if ov.Border.Radius != nil {
		out.Border.Radius = ov.Border.Radius
	}
	if ov.FontFamily != "" {
		out.FontFamily = ov.FontFamily
	}
	if ov.FontSize != nil {
		out.FontSize = ov.FontSize
	}
	if ov.FontWeight != nil {
		out.FontWeight = ov.FontWeight
	}
	if ov.Color != "" {
		out.Color = ov.Color
	}
	if ov.Labels != nil {
		out.Labels = ov.Labels
	}
	if ov.Selected != nil {
		var baseSel ViewNode
		if base.Selected != nil {
			baseSel = *base.Selected
		}
		merged := mergeViewNode(baseSel, *ov.Selected)
		out.Selected = &merged
	}
	if ov.Hover != nil {
		var baseHover ViewNode
		if base.Hover != nil {
			baseHover = *base.Hover
		}
		merged := mergeViewNode(baseHover, *ov.Hover)
		out.Hover = &merged
	}
	return out
}

// mergeViews 把 override（通常来自主题 YAML）逐具名 View 合并到 base（通常是 defaultViews 基线）。
func mergeViews(base, ov Views) Views {
	return Views{
		Window:        mergeViewNode(base.Window, ov.Window),
		PreeditBar:    mergeViewNode(base.PreeditBar, ov.PreeditBar),
		CandidateList: mergeViewNode(base.CandidateList, ov.CandidateList),
		Item:          mergeViewNode(base.Item, ov.Item),
		Index:         mergeViewNode(base.Index, ov.Index),
		Text:          mergeViewNode(base.Text, ov.Text),
		Comment:       mergeViewNode(base.Comment, ov.Comment),
		AccentBar:     mergeViewNode(base.AccentBar, ov.AccentBar),
		FooterBar:     mergeViewNode(base.FooterBar, ov.FooterBar),
	}
}
