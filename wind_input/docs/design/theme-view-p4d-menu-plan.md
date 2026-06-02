# P4-D 菜单 View 化 + 颜色 YAML 化 实现计划

> **For agentic workers:** 用 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。
> P4 最后一片（菜单），完成后候选窗+状态泡+Tooltip+工具栏+菜单全部 View 化。

**Goal:** 弹出菜单渲染从 gg 直绘迁移到盒模型 View 引擎，颜色经 `views.menu` token 驱动，零回归；保留子菜单/勾选/分隔线/hover/disabled 视觉与命中测试布局。

**Architecture:** `buildMenuTree`：root LayoutColumn（padY 上下 padding）+ 每项 LayoutRow（checkCell + text(Grow) + arrowCell，padding 左右 padX/2，hover 满宽背景）；勾选 ✓ / 箭头 ▸ / 文本走 View 文本叶子（统一 PaintTree），分隔线后处理（矢量 DrawLine，定位用分隔项 Rect()）。复用 P4 的 resolveTokenColor + newSharedDrawContext。

**Tech Stack:** Go；复用 P4-A/B/C 基础设施。

**Scope note（沿用收窄）:** 几何（itemHeight/sepH/padX/padY/checkW/arrowW/radius）保持 hardcode×scale（命中测试 popup_menu_event.go 依赖这套布局，不可变）；views.menu 只 token 化颜色（7 色）。

**零回归基准:** `popup_menu_render.go:render`。颜色 `ResolvedPopupMenuColors`（Background/Border/Text/Disabled/HoverBg/HoverText/Separator）。布局：root padY 上下；item `[padX/2 | checkW | text(margin padX/2) | …Grow… | arrowW | padX/2]`；check cx=padX/2+checkW/2、textX=padX+checkW、arrow ax=width-padX/2-arrowW/2；hover 满宽(1..width-2)；分隔线 4*scale..width-4*scale 居中。

常量：menuItemHeight=24 / menuSeparatorHeight=9 / menuPaddingX=24 / menuPaddingY=4 / menuCornerRadius=6 / menuCheckMarkWidth=18 / menuArrowWidth=14。hasChecked/hasChildren 菜单级（决定列是否存在，所有项对齐）。

---

## 文件结构
- **修改** `pkg/theme/views.go`：新增 `MenuViews`/`MenuHoverState`；`Views` 加 `Menu *MenuViews`；新增 `ResolvedMenuViews`（7 色扁平）。
- **新建** `internal/ui/viewbox_menu.go`：`(*PopupMenu).resolveMenuColors`、`buildMenuTree`（返回 `menuTree{root, separators}`）。
- **修改** `internal/ui/popup_menu.go`：`PopupMenu` 加 `themeViews`；`SetTheme` 存（递归 submenu）。
- **修改** `internal/ui/popup_menu_render.go`：`render` 改走引擎 + 分隔线后处理；删旧 gg 直绘 + menuTextItem。
- **新建** `internal/ui/viewbox_menu_test.go`：颜色解析 + 几何/状态指纹。
- **修改** `themes/default/theme.yaml`、`themes/msime/theme.yaml`：加 `views.menu`。
- **修改** `pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`。

---

## Task 1：theme 包 MenuViews schema + ResolvedMenuViews

**Files:** Modify `pkg/theme/views.go`；Test `pkg/theme/views_yaml_test.go`

- [ ] **Step 1: 失败测试**（追加 `views_yaml_test.go`）

```go
// TestViews_MenuParse 验证 views.menu 解析（含 hover 状态）。
func TestViews_MenuParse(t *testing.T) {
	data := `
menu:
  background: {color: "${background}"}
  color: "${text}"
  separator: {color: "${separator}"}
  disabled: "${disabled}"
  hover:
    background: {color: "${hover_bg}"}
    color: "${hover_text}"
`
	var v Views
	if err := yaml.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Menu == nil {
		t.Fatal("views.menu 应非 nil")
	}
	if v.Menu.Color != "${text}" || v.Menu.Disabled != "${disabled}" {
		t.Errorf("menu text/disabled token 错误: %+v", v.Menu)
	}
	if v.Menu.Hover.Background.Color != "${hover_bg}" || v.Menu.Hover.Color != "${hover_text}" {
		t.Error("menu hover 覆盖缺失")
	}
}
```

- [ ] **Step 2: 验证失败** — `go test ./pkg/theme/ -run TestViews_MenuParse`

- [ ] **Step 3: 加 schema**（`views.go`，`ResolvedToolbarViews` 之后）

```go
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
```

- [ ] **Step 4: `Views` 加 Menu 字段**（`Toolbar` 之后）

```go
	Toolbar       *ToolbarViews `yaml:"toolbar,omitempty"` // P4-C 工具栏
	Menu          *MenuViews    `yaml:"menu,omitempty"`    // P4-D 弹出菜单
```

- [ ] **Step 5: 验证通过** — `go test ./pkg/theme/ -run TestViews_MenuParse -v`
- [ ] **Step 6:** `gofmt -w pkg/theme/views.go pkg/theme/views_yaml_test.go`

---

## Task 2：viewbox_menu.go（颜色解析 + buildMenuTree）

**Files:** Create `internal/ui/viewbox_menu.go`、`internal/ui/viewbox_menu_test.go`；Modify `popup_menu.go`（themeViews + SetTheme）

- [ ] **Step 1: popup_menu.go 加 themeViews + SetTheme**

```go
// PopupMenu struct 内 resolvedTheme 之后：
	resolvedTheme *theme.ResolvedTheme
	themeViews    *theme.Views

// SetTheme（保留 submenu 递归）：
func (m *PopupMenu) SetTheme(resolved *theme.ResolvedTheme) {
	m.mu.Lock()
	m.resolvedTheme = resolved
	if resolved != nil {
		m.themeViews = resolved.Views
	} else {
		m.themeViews = nil
	}
	sub := m.submenu
	m.mu.Unlock()
	if sub != nil {
		sub.SetTheme(resolved)
	}
}
```

- [ ] **Step 2: 失败测试**（新建 `viewbox_menu_test.go`）

```go
package ui

import (
	"image/color"
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// TestResolveMenuColors 7 色映射 ResolvedTheme.PopupMenu。
func TestResolveMenuColors(t *testing.T) {
	pm := theme.ResolvedPopupMenuColors{
		BackgroundColor: color.RGBA{255, 255, 255, 255},
		BorderColor:     color.RGBA{199, 199, 199, 255},
		TextColor:       color.RGBA{0, 0, 0, 255},
		DisabledColor:   color.RGBA{161, 161, 161, 255},
		HoverBgColor:    color.RGBA{0, 120, 212, 255},
		HoverTextColor:  color.RGBA{255, 255, 255, 255},
		SeparatorColor:  color.RGBA{219, 219, 219, 255},
	}
	m := &PopupMenu{resolvedTheme: &theme.ResolvedTheme{PopupMenu: pm}}
	rmv := m.resolveMenuColors()
	if rmv.BgColor != pm.BackgroundColor || rmv.HoverBgColor != pm.HoverBgColor ||
		rmv.DisabledColor != pm.DisabledColor || rmv.SeparatorColor != pm.SeparatorColor {
		t.Errorf("menu 7 色映射错误: %+v", rmv)
	}
}

// TestBuildMenuTree_Geometry 验证菜单项布局 + hover/disabled 状态色 + 勾选/箭头。
func TestBuildMenuTree_Geometry(t *testing.T) {
	rmv := theme.ResolvedMenuViews{
		BgColor: color.RGBA{255, 255, 255, 255}, BorderColor: color.RGBA{1, 2, 3, 255},
		TextColor: color.RGBA{0, 0, 0, 255}, DisabledColor: color.RGBA{161, 161, 161, 255},
		HoverBgColor: color.RGBA{0, 120, 212, 255}, HoverTextColor: color.RGBA{255, 255, 255, 255},
		SeparatorColor: color.RGBA{219, 219, 219, 255},
	}
	items := []MenuItem{
		{Text: "项目一", Checked: true},
		{Separator: true},
		{Text: "子菜单", Children: []MenuItem{{Text: "子项"}}},
		{Text: "禁用项", Disabled: true},
	}
	m := fixedMeasurer{charW: 14}
	// hoverIdx=0（项目一 hover），hasChecked=true，hasChildren=true
	mt := buildMenuTree(items, 0, -1, true, true, rmv, 200, 80, 14.0, 24, 1.0)
	Layout(mt.root, 0, 0, m)
	if mt.root.Background.Color != (color.RGBA{255, 255, 255, 255}) {
		t.Error("root bg 应=BgColor")
	}
	if len(mt.root.Children) != 4 {
		t.Fatalf("应 4 项（含分隔）, got %d", len(mt.root.Children))
	}
	// 项0 hover：背景 HoverBg
	if mt.root.Children[0].Background.Color != (color.RGBA{0, 120, 212, 255}) {
		t.Errorf("hover 项背景应=HoverBg, got %v", mt.root.Children[0].Background.Color)
	}
	// 分隔项收集到 separators
	if len(mt.separators) != 1 {
		t.Errorf("应 1 个分隔项, got %d", len(mt.separators))
	}
}
```

- [ ] **Step 3: 实现**（新建 `viewbox_menu.go`，含 resolveMenuColors + buildMenuTree + menuTree）

```go
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

// buildMenuTree 构建菜单 View 树。几何参数为已选定值（itemH 已经 getMenuItemHeight，fontSize 已乘 scale 前的逻辑值）。
// width/height 用预算值（与命中测试一致）。hoverIdx/submenuIdx 决定 hover 态。
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
			FixedH:  itemH,
			Layout:  LayoutRow,
			Stretch: true, // 撑满 root 宽（hover 满宽）
			Padding: Edges{Left: padXHalf, Right: padXHalf},
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
```

> 注：text 用 `Grow` 吸收中间弹性空间（check 与 arrow 之间），文本在 cell 内左对齐（AlignStart 默认）。check/arrow cell `FixedW` 固定，`AlignCenter` 居中符号——复刻现状 check cx=padX/2+checkW/2、arrow ax=width-padX/2-arrowW/2。text 起点 = padXHalf(row pad) + checkW + padXHalf(margin) = padX+checkW（!hasChecked 时 = padX）。

- [ ] **Step 4: 验证通过** — `go test ./internal/ui/ -run 'TestResolveMenuColors|TestBuildMenuTree' -v`
- [ ] **Step 5:** `gofmt -w internal/ui/viewbox_menu.go internal/ui/viewbox_menu_test.go internal/ui/popup_menu.go`

---

## Task 3：render 改造 + 分隔线后处理

**Files:** Modify `internal/ui/popup_menu_render.go`

- [ ] **Step 1: 替换 render**（popup_menu_render.go:20-161）

```go
// render renders the menu to an image（盒模型 View 引擎）。
func (m *PopupMenu) render() *image.RGBA {
	m.mu.Lock()
	items := m.items
	hoverIdx := m.hoverIndex
	submenuIdx := m.submenuIndex
	width := m.width
	height := m.height
	hasChecked := m.hasChecked
	hasChildren := m.hasChildren
	rmv := m.resolveMenuColors()
	td := m.textDrawer
	baseFontSize := m.getMenuFontSize()
	itemHeightLogical := m.getMenuItemHeight()
	scale := m.dpiScale()
	m.mu.Unlock()

	mt := buildMenuTree(items, hoverIdx, submenuIdx, hasChecked, hasChildren, rmv, width, height, baseFontSize, itemHeightLogical, scale)
	Layout(mt.root, 0, 0, td)
	dc, img := newSharedDrawContext(width, height)
	PaintTree(mt.root, dc, img, td)

	// 后处理分隔线（矢量，定位用分隔项 Rect()）
	for _, sep := range mt.separators {
		r := sep.Rect()
		sepY := float64(r.Min.Y) + float64(r.Dy())/2
		dc.SetColor(rmv.SeparatorColor)
		dc.DrawLine(4*scale, sepY, float64(width)-4*scale, sepY)
		dc.Stroke()
	}

	DrawDebugBanner(img)
	return img
}
```

- [ ] **Step 2: 删除 menuTextItem 类型**（popup_menu_render.go:11-17，已不用）

- [ ] **Step 3: 清理 import**（编译指引）—— render 不再直接用 gg/color 时移除；`image` 仍用（返回 *image.RGBA）。updateWindow/trackMouseLeave 保留。

> 注：`menuTextItem` 删除后，若 `color`/`gg` import 在 popup_menu_render.go 不再被引用则移除（updateWindow/trackMouseLeave 用 unsafe/image，不用 gg/color）。

- [ ] **Step 4: 编译 + 测试** — `go build ./... ; go test ./internal/ui/ ./pkg/theme/`
Expected: build ✓；ui PASS；theme pre-existing fail 无关。

- [ ] **Step 5:** `gofmt -w internal/ui/popup_menu_render.go`

---

## Task 4：种子主题 + AGENTS.md + 全量验证

**Files:** `themes/default/theme.yaml`、`themes/msime/theme.yaml`、`pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`

- [ ] **Step 1: 两主题加 views.menu**（views.toolbar 之后）

```yaml
  menu:
    background: {color: "${background}"}
    border: {color: "${border}"}
    color: "${text}"
    separator: {color: "${separator}"}
    disabled: "${disabled}"
    hover:
      background: {color: "${hover_bg}"}
      color: "${hover_text}"
```

- [ ] **Step 2: 主题解析测试**（追加 `pkg/theme/views_yaml_test.go`）

```go
// TestDefaultThemeMenuParse 验证 default theme.yaml 的 views.menu。
func TestDefaultThemeMenuParse(t *testing.T) {
	data, err := os.ReadFile("../../themes/default/theme.yaml")
	if err != nil {
		t.Skip("default theme.yaml 不可读: " + err.Error())
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if th.Views == nil || th.Views.Menu == nil || th.Views.Menu.Hover.Background.Color != "${hover_bg}" {
		t.Fatal("default theme.yaml 应含 views.menu.hover.background")
	}
}
```

- [ ] **Step 3: AGENTS.md 同步**
  - `pkg/theme/AGENTS.md`：补 `MenuViews`/`MenuHoverState` + `ResolvedMenuViews`。
  - `internal/ui/AGENTS.md`：加 `viewbox_menu.go` 行；更新 `popup_menu_render.go` 描述（render 走引擎）。

- [ ] **Step 4: 全量验证** — `go build ./... ; gofmt -l <改动文件> ; go test ./internal/ui/ ./pkg/theme/`

- [ ] **Step 5: lint** — `pwsh -File scripts/lint_agents_md.ps1`（无 mdlink 真错）

- [ ] **Step 6: 运行时人工核对（交用户）** — `dev.ps1 d1` 后右键/设置菜单：背景/边框/分隔线/勾选✓/子菜单箭头▸/hover 高亮(蓝底白字)/禁用项灰字/子菜单展开，逐项确认与迁移前一致。

---

## 验收清单
- [ ] 菜单走 View 引擎；勾选/箭头/文本走 View 文本叶子，分隔线后处理，hover 满宽背景走 View。
- [ ] hover/disabled/checked/子菜单箭头 视觉保持；命中测试布局不变（常量一致）。
- [ ] default/msime 含 views.menu；7 色 token 映射 ResolvedTheme.PopupMenu。
- [ ] build ✓ / 改动文件 gofmt 干净 / 回归网 PASS / AGENTS.md lint 通过。
- [ ] 运行时人工核对（交用户）——含子菜单展开。
