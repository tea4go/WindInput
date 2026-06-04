# P4-C-2 工具栏 View 化 + 颜色 YAML 化 实现计划

> **For agentic workers:** 用 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。
> 前置：P4-C-1（死配置 4 字段清理，commit 04b7969）已完成。

**Goal:** 工具栏渲染从 gg 直绘迁移到盒模型 View 引擎，颜色经 `views.toolbar` token 驱动（含 `button` base + `mode` 状态覆盖），零回归。

**Architecture:** View 承载**整条背景/边框 + 4 按钮背景框布局 + mode 文字**；grip 点阵 / 全半角符号(●/月牙) / 标点双符号 / 齿轮 这些**矢量符号保留为后处理**，定位用 Layout 后各按钮 `Rect()`。`buildToolbarTree` 返回 root + 各按钮 View 引用。复用 P4-A `resolveTokenColor`/`newSharedDrawContext`。

**Tech Stack:** Go；`pkg/theme` views schema；复用 P4-A/B 基础设施。

**Scope note（沿用收窄）:** 几何（toolbarBaseWidth 116 / Height 30 / gripWidth 10 / buttonWidth 26 / buttonPadding 2 / 各 radius）保持 hardcode×scale。views.toolbar 只 token 化**颜色**。

**零回归基准:** `toolbar_renderer.go:Render`（76-183）。按钮底色现状：mode 用 ModeChineseBg/EnglishBg；width/punct/settings 均 `{230,234,239}`(=FullWidthOffBg)。符号/文字色：mode 文字 ModeText(白)；width/punct 符号 `{89,102,122}`(=FullWidthOffColor)；齿轮 SettingsIcon/Hole。

## 颜色模型（base + 状态覆盖 → 扁平 ResolvedToolbarViews）

`button` base：bg=`${button_bg}`(默认 FullWidthOffBgColor)、符号色=`${button_text}`(默认 FullWidthOffColor)。
`mode` 覆盖：chinese.bg=`${mode_cn_bg}`、english.bg=`${mode_en_bg}`，文字=ModeText(白，单独字段)。
width/punct 不写覆盖 → 继承 button base（bg `{230,234,239}` + 符号 `{89,102,122}`，零回归）。
settings：bg 继承 button base；`${settings_icon}`/`${settings_hole}` 齿轮色。

---

## 文件结构
- **修改** `pkg/theme/views.go`：新增 `ToolbarViews`/`ToolbarButtonNode`/`ToolbarModeStates`/`ToolbarSettingsNode`；`Views` 加 `Toolbar *ToolbarViews`；新增 `ResolvedToolbarViews`（扁平颜色集）。
- **新建** `internal/ui/viewbox_toolbar.go`：`(*ToolbarRenderer).resolveToolbarViews`、`buildToolbarTree`（返回 `toolbarTree`）。
- **修改** `internal/ui/toolbar_renderer.go`：`ToolbarRenderer` 加 `themeViews`；`SetTheme` 存；`Render` 改建树→Layout→PaintTree→后处理矢量；现有 drawGrip/drawWidthSymbol/drawPunct/drawSettingsButton 改签名接收 rect。
- **新建** `internal/ui/viewbox_toolbar_test.go`：颜色解析（base+mode 覆盖）+ buildToolbarTree 几何/状态指纹。
- **修改** `themes/default/theme.yaml`、`themes/msime/theme.yaml`：加 `views.toolbar`。
- **修改** `pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`。

---

## Task 1：theme 包 ToolbarViews schema + ResolvedToolbarViews

**Files:** Modify `pkg/theme/views.go`；Test `pkg/theme/views_yaml_test.go`

- [ ] **Step 1: 失败测试**（追加 `views_yaml_test.go`）

```go
// TestViews_ToolbarParse 验证 views.toolbar 的 button base + mode 状态覆盖解析。
func TestViews_ToolbarParse(t *testing.T) {
	data := `
toolbar:
  background: {color: "${background}"}
  grip: {color: "${grip}"}
  button:
    background: {color: "${button_bg}"}
    color: "${button_text}"
    mode:
      chinese: {background: {color: "${mode_cn_bg}"}}
      english: {background: {color: "${mode_en_bg}"}}
  settings:
    icon: {color: "${settings_icon}"}
    hole: {color: "${settings_hole}"}
`
	var v Views
	if err := yaml.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Toolbar == nil {
		t.Fatal("views.toolbar 应非 nil")
	}
	if v.Toolbar.Button.Background.Color != "${button_bg}" {
		t.Errorf("button base bg token, got %q", v.Toolbar.Button.Background.Color)
	}
	if v.Toolbar.Button.Mode == nil || v.Toolbar.Button.Mode.Chinese.Background.Color != "${mode_cn_bg}" {
		t.Error("mode.chinese bg 覆盖缺失")
	}
	if v.Toolbar.Settings.Icon.Color != "${settings_icon}" {
		t.Errorf("settings icon token, got %q", v.Toolbar.Settings.Icon.Color)
	}
}
```

- [ ] **Step 2: 验证失败** — `go test ./pkg/theme/ -run TestViews_ToolbarParse`（未定义）

- [ ] **Step 3: 加 schema 类型**（`views.go`，`ResolvedTooltipViews` 之后）

```go
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
```

- [ ] **Step 4: `Views` 加 Toolbar 字段**（`Tooltip` 之后）

```go
	Tooltip       *ViewNode     `yaml:"tooltip,omitempty"` // P4-B Tooltip（独立窗口，单节点）
	Toolbar       *ToolbarViews `yaml:"toolbar,omitempty"` // P4-C 工具栏
```

- [ ] **Step 5: 验证通过** — `go test ./pkg/theme/ -run TestViews_ToolbarParse -v`（PASS）
- [ ] **Step 6:** `gofmt -w pkg/theme/views.go pkg/theme/views_yaml_test.go`

---

## Task 2：ui resolveToolbarViews（颜色解析）

**Files:** Create `internal/ui/viewbox_toolbar.go`、`internal/ui/viewbox_toolbar_test.go`；Modify `toolbar_renderer.go`（加 themeViews 字段 + SetTheme）

- [ ] **Step 1: toolbar_renderer.go 加 themeViews + SetTheme**

```go
// ToolbarRenderer struct 内 resolvedTheme 之后：
	resolvedTheme *theme.ResolvedTheme
	themeViews    *theme.Views

// SetTheme：
func (r *ToolbarRenderer) SetTheme(resolved *theme.ResolvedTheme) {
	r.resolvedTheme = resolved
	if resolved != nil {
		r.themeViews = resolved.Views
	} else {
		r.themeViews = nil
	}
}
```

- [ ] **Step 2: 失败测试**（新建 `viewbox_toolbar_test.go`）

```go
package ui

import (
	"image/color"
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// TestResolveToolbarViews_BaseAndMode base 映射 + mode token 覆盖。
func TestResolveToolbarViews_BaseAndMode(t *testing.T) {
	tb := theme.ResolvedToolbarColors{
		BackgroundColor:     color.RGBA{255, 255, 255, 255},
		BorderColor:         color.RGBA{199, 209, 224, 255},
		GripColor:           color.RGBA{153, 173, 199, 179},
		ModeChineseBgColor:  color.RGBA{51, 154, 245, 255},
		ModeEnglishBgColor:  color.RGBA{115, 127, 148, 255},
		ModeTextColor:       color.RGBA{255, 255, 255, 255},
		FullWidthOffBgColor: color.RGBA{230, 234, 239, 255},
		FullWidthOffColor:   color.RGBA{89, 102, 122, 255},
		SettingsBgColor:     color.RGBA{230, 234, 239, 255},
		SettingsIconColor:   color.RGBA{122, 102, 184, 255},
		SettingsHoleColor:   color.RGBA{230, 234, 239, 255},
	}
	r := &ToolbarRenderer{resolvedTheme: &theme.ResolvedTheme{Toolbar: tb}}
	rtv := r.resolveToolbarViews()
	if rtv.ButtonBg != tb.FullWidthOffBgColor {
		t.Error("button base bg 应=FullWidthOffBg")
	}
	if rtv.ButtonText != tb.FullWidthOffColor {
		t.Error("button base text 应=FullWidthOffColor")
	}
	if rtv.ModeChineseBg != tb.ModeChineseBgColor || rtv.ModeText != tb.ModeTextColor {
		t.Error("mode 色映射错误")
	}
	if rtv.SettingsIcon != tb.SettingsIconColor {
		t.Error("settings icon 映射错误")
	}
}
```

- [ ] **Step 3: 实现 resolveToolbarViews**（新建 `viewbox_toolbar.go`，先只含颜色解析，buildToolbarTree 在 Task 3 加）

```go
package ui

// viewbox_toolbar.go — 工具栏 View 树构建与颜色解析（P4-C）。
// View 承载背景条/按钮框/mode 文字；grip/全半角/标点/齿轮矢量符号后处理（定位用 View rect）。

import (
	"image/color"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// resolveToolbarViews 解析工具栏颜色：默认从 ResolvedTheme.Toolbar 映射，views.toolbar token 覆盖。
// button base 默认 = FullWidthOff*（非激活按钮底色/前景，零回归）。
func (r *ToolbarRenderer) resolveToolbarViews() theme.ResolvedToolbarViews {
	tb := r.getToolbarColors() // *ResolvedToolbarColors（含默认）
	rtv := theme.ResolvedToolbarViews{
		BarBg:         tb.BackgroundColor,
		BarBorder:     tb.BorderColor,
		Grip:          tb.GripColor,
		ButtonBg:      tb.FullWidthOffBgColor,
		ButtonText:    tb.FullWidthOffColor,
		ModeChineseBg: tb.ModeChineseBgColor,
		ModeEnglishBg: tb.ModeEnglishBgColor,
		ModeText:      tb.ModeTextColor,
		SettingsBg:    tb.SettingsBgColor,
		SettingsIcon:  tb.SettingsIconColor,
		SettingsHole:  tb.SettingsHoleColor,
	}
	tv := r.themeViews
	if tv == nil || tv.Toolbar == nil {
		return rtv
	}
	t := tv.Toolbar
	res := func(name string) color.Color {
		switch name {
		case "background":
			return tb.BackgroundColor
		case "border":
			return tb.BorderColor
		case "grip":
			return tb.GripColor
		case "button_bg":
			return tb.FullWidthOffBgColor
		case "button_text":
			return tb.FullWidthOffColor
		case "mode_cn_bg":
			return tb.ModeChineseBgColor
		case "mode_en_bg":
			return tb.ModeEnglishBgColor
		case "mode_text":
			return tb.ModeTextColor
		case "settings_icon":
			return tb.SettingsIconColor
		case "settings_hole":
			return tb.SettingsHoleColor
		}
		return nil
	}
	set := func(dst *color.Color, s string) {
		if c := resolveTokenColor(s, res); c != nil {
			*dst = c
		}
	}
	set(&rtv.BarBg, t.Background.Color)
	set(&rtv.BarBorder, t.Border.Color)
	set(&rtv.Grip, t.Grip.Color)
	set(&rtv.ButtonBg, t.Button.Background.Color)
	set(&rtv.ButtonText, t.Button.Color)
	if t.Button.Mode != nil {
		set(&rtv.ModeChineseBg, t.Button.Mode.Chinese.Background.Color)
		set(&rtv.ModeEnglishBg, t.Button.Mode.English.Background.Color)
	}
	set(&rtv.SettingsBg, t.Settings.Background.Color)
	set(&rtv.SettingsIcon, t.Settings.Icon.Color)
	set(&rtv.SettingsHole, t.Settings.Hole.Color)
	return rtv
}
```

- [ ] **Step 4: 验证通过** — `go test ./internal/ui/ -run TestResolveToolbarViews -v`（PASS）
- [ ] **Step 5:** `gofmt -w internal/ui/viewbox_toolbar.go internal/ui/viewbox_toolbar_test.go internal/ui/toolbar_renderer.go`

---

## Task 3：buildToolbarTree + 几何指纹

**Files:** Modify `internal/ui/viewbox_toolbar.go`；Test `internal/ui/viewbox_toolbar_test.go`

设计：root = LayoutRow，CrossAlign 居中，Background(BarBg)+Border(BarBorder,radius 6)。子节点依次：grip 占位(FixedW gripWidth) / mode / width / punct / settings 按钮（各 FixedW=buttonWidth，内含 padding）。按钮框为带 Background+Border(radius 4) 的 View；mode 按钮内含文字叶子。返回各按钮 View 引用供后处理。

- [ ] **Step 1: 失败测试**（追加，几何 + mode 状态色指纹）

```go
// TestBuildToolbarTree_Geometry 验证整条宽高 + mode 按钮背景按 state 选色。
func TestBuildToolbarTree_Geometry(t *testing.T) {
	rtv := theme.ResolvedToolbarViews{
		BarBg: color.RGBA{255, 255, 255, 255}, BarBorder: color.RGBA{1, 2, 3, 255},
		ButtonBg: color.RGBA{230, 234, 239, 255}, ButtonText: color.RGBA{89, 102, 122, 255},
		ModeChineseBg: color.RGBA{51, 154, 245, 255}, ModeEnglishBg: color.RGBA{115, 127, 148, 255},
		ModeText: color.RGBA{255, 255, 255, 255},
		SettingsBg: color.RGBA{230, 234, 239, 255}, SettingsIcon: color.RGBA{1, 1, 1, 255}, SettingsHole: color.RGBA{2, 2, 2, 255},
	}
	m := fixedMeasurer{charW: 10}
	tt := buildToolbarTree(ToolbarState{ChineseMode: true, ModeLabel: "拼"}, rtv, 1.0, m)
	Layout(tt.root, 0, 0, m)
	// 整条宽 = 116, 高 = 30（scale=1）
	if tt.root.Rect().Dx() != 116 || tt.root.Rect().Dy() != 30 {
		t.Errorf("整条尺寸应 116x30, got %dx%d", tt.root.Rect().Dx(), tt.root.Rect().Dy())
	}
	// 中文模式：mode 按钮背景 = ModeChineseBg
	if tt.mode.Background.Color != (color.RGBA{51, 154, 245, 255}) {
		t.Errorf("中文模式 mode 背景应=ModeChineseBg, got %v", tt.mode.Background.Color)
	}
	// 英文模式
	tt2 := buildToolbarTree(ToolbarState{ChineseMode: false}, rtv, 1.0, m)
	if tt2.mode.Background.Color != (color.RGBA{115, 127, 148, 255}) {
		t.Errorf("英文模式 mode 背景应=ModeEnglishBg, got %v", tt2.mode.Background.Color)
	}
}
```

- [ ] **Step 2: 验证失败** — 未定义 `buildToolbarTree`/`toolbarTree`

- [ ] **Step 3: 实现 buildToolbarTree + toolbarTree**（追加 `viewbox_toolbar.go`）

```go
// toolbarTree 持有 root + 各按钮 View 引用（后处理矢量符号定位用其 Rect()）。
type toolbarTree struct {
	root, grip, mode, width, punct, settings *View
}

// modeButtonText 复刻现状 mode 按钮文字选择逻辑。
func modeButtonText(state ToolbarState) string {
	if state.ChineseMode {
		if state.ModeLabel != "" {
			return state.ModeLabel
		}
		return "中"
	}
	if state.CapsLock {
		return "A"
	}
	return "英"
}

// buildToolbarTree 构建工具栏 View 树（整条 LayoutRow：grip + 4 按钮，按钮框走 View、符号后处理）。
func buildToolbarTree(state ToolbarState, rtv theme.ResolvedToolbarViews, scale float64, m TextMeasurer) *toolbarTree {
	h := int(float64(toolbarBaseHeight) * scale)
	gripW := int(float64(gripWidth) * scale)
	btnW := int(float64(buttonWidth) * scale)
	pad := int(float64(buttonPadding) * scale)
	btnRadius := int(4.0 * scale)
	fontSize := 14.0 * scale

	mkButtonFrame := func(bg color.Color) *View {
		return &View{
			FixedW:     btnW - pad*2,
			Margin:     Edges{Top: pad, Right: pad, Bottom: pad, Left: pad},
			Background: Fill{Color: bg},
			Border:     Border{Radius: btnRadius},
			Layout:     LayoutStack,
		}
	}

	grip := &View{FixedW: gripW}

	modeBg := rtv.ModeEnglishBg
	if state.ChineseMode {
		modeBg = rtv.ModeChineseBg
	}
	mode := mkButtonFrame(modeBg)
	mode.Layout = LayoutStack
	mode.Children = []*View{{
		Text:      modeButtonText(state),
		TextStyle: TextStyle{FontSize: fontSize, Color: rtv.ModeText, Align: AlignCenter},
		Stretch:   true,
	}}

	width := mkButtonFrame(rtv.ButtonBg)
	punct := mkButtonFrame(rtv.ButtonBg)
	settings := mkButtonFrame(rtv.SettingsBg)

	root := &View{
		FixedW:     int(float64(toolbarBaseWidth) * scale),
		FixedH:     h,
		Layout:     LayoutRow,
		Background: Fill{Color: rtv.BarBg},
		Border:     Border{Radius: int(6.0 * scale), Color: rtv.BarBorder, Width: 1},
		Children:   []*View{grip, mode, width, punct, settings},
	}
	return &toolbarTree{root: root, grip: grip, mode: mode, width: width, punct: punct, settings: settings}
}
```

- [ ] **Step 4: 验证通过** — `go test ./internal/ui/ -run TestBuildToolbarTree -v`（PASS）
- [ ] **Step 5:** `gofmt -w internal/ui/viewbox_toolbar.go internal/ui/viewbox_toolbar_test.go`

> 注：grip/width/punct/settings 的 FixedW 用 `btnW-pad*2` + margin pad，使布局等价现状 `x+=buttonWidth`。Layout 后各按钮 Rect() 供后处理。若整条宽因 margin 累加 ≠116，Task 5 运行时核对时微调（FixedW 用 btnW、内部 padding 改 Padding 而非 Margin）。

---

## Task 4：Render 改造（建树 + 后处理矢量符号）

**Files:** Modify `internal/ui/toolbar_renderer.go`

将 drawGrip/drawWidthSymbol/drawPunctButton文字/drawSettingsButton 改为接收目标 `image.Rectangle`（用 View 布局结果），Render 改为：

- [ ] **Step 1: 重写 Render**（替换 76-183）

```go
func (r *ToolbarRenderer) Render(state ToolbarState) *image.RGBA {
	scale := GetDPIScale()
	rtv := r.resolveToolbarViews()
	td := r.TextDrawer()

	tt := buildToolbarTree(state, rtv, scale, td)
	Layout(tt.root, 0, 0, td)
	w, h := tt.root.Rect().Dx(), tt.root.Rect().Dy()
	dc, img := newSharedDrawContext(w, h)
	PaintTree(tt.root, dc, img, td) // 背景条 + 按钮框 + mode 文字

	// 后处理矢量符号（坐标用 View 布局 rect）
	r.paintGrip(dc, tt.grip.Rect(), scale, rtv)
	r.paintWidthSymbol(dc, tt.width.Rect(), state.FullWidth, scale, rtv)
	r.paintPunctSymbols(img, td, tt.punct.Rect(), state.ChinesePunct, scale, rtv)
	r.paintGear(dc, tt.settings.Rect(), scale, rtv)

	DrawDebugBanner(img)
	return img
}
```

- [ ] **Step 2: 改写各 paintXxx 接收 rect**（基于现有 drawGrip/drawWidthSymbol/drawSettingsButton 几何，坐标改用传入 rect.Min + rect 尺寸；颜色用 rtv）

实现注记：保留现有几何常量（dotSize/dotGap/齿轮 outerR/innerR/toothHeight、月牙 offset、标点 leftAnchor/rightAnchor/nudge）。把原函数里的 `x,y,w,h` 换成传入 rect 的 `Min.X/Min.Y/Dx/Dy`（float64），颜色 `colors.XxxColor` 换成 `rtv.Xxx`（grip→Grip、width 符号→ButtonText、punct 文字→ButtonText、齿轮→SettingsIcon/SettingsHole）。punct 文字用 `td.DrawString`（在 PaintTree 的 EndDraw 之后需重新 BeginDraw/EndDraw 包裹，或合并进 PaintTree 文本趟——采用独立 td.BeginDraw(img)/EndDraw 包裹 paintPunctSymbols）。删除旧 drawModeButton/drawWidthButton/drawPunctButton（按钮框已由 View 画）；drawGrip/drawWidthSymbol/drawSettingsButton 改造为 paintGrip/paintWidthSymbol/paintGear。

- [ ] **Step 3: 编译 + 测试** — `go build ./... ; go test ./internal/ui/ ./pkg/theme/`
Expected: build ✓；ui PASS；theme pre-existing fail 无关。

- [ ] **Step 4:** `gofmt -w internal/ui/toolbar_renderer.go internal/ui/viewbox_toolbar.go`

---

## Task 5：种子主题 + AGENTS.md + 全量验证 + 运行时核对

**Files:** `themes/default/theme.yaml`、`themes/msime/theme.yaml`、`pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`

- [ ] **Step 1: 两主题加 views.toolbar**（views.tooltip 之后）

```yaml
  toolbar:
    background: {color: "${background}"}
    border: {color: "${border}"}
    grip: {color: "${grip}"}
    button:
      background: {color: "${button_bg}"}
      color: "${button_text}"
      mode:
        chinese: {background: {color: "${mode_cn_bg}"}}
        english: {background: {color: "${mode_en_bg}"}}
    settings:
      icon: {color: "${settings_icon}"}
      hole: {color: "${settings_hole}"}
```

> 注：`${background}`/`${grip}`/`${button_bg}` 等是工具栏命名空间内 token（resolveToolbarViews 的 res 映射），与候选窗/状态泡的同名 token 互不冲突（各窗口注入自己的 resolver）。

- [ ] **Step 2: 主题解析测试**（追加 `pkg/theme/views_yaml_test.go`）

```go
// TestDefaultThemeToolbarParse 验证 default theme.yaml 的 views.toolbar。
func TestDefaultThemeToolbarParse(t *testing.T) {
	data, err := os.ReadFile("../../themes/default/theme.yaml")
	if err != nil {
		t.Skip("default theme.yaml 不可读: " + err.Error())
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if th.Views == nil || th.Views.Toolbar == nil || th.Views.Toolbar.Button.Mode == nil {
		t.Fatal("default theme.yaml 应含 views.toolbar.button.mode")
	}
}
```

- [ ] **Step 3: AGENTS.md 同步**
  - `pkg/theme/AGENTS.md`：补 `ToolbarViews`/`ToolbarButtonNode`/`ToolbarModeStates`/`ToolbarSettingsNode` + `ResolvedToolbarViews`。
  - `internal/ui/AGENTS.md`：加 `viewbox_toolbar.go` 行（buildToolbarTree 按钮框 View + 矢量符号后处理 + resolveToolbarViews base/mode 覆盖），更新 `toolbar_renderer.go` 描述（Render 走引擎）。

- [ ] **Step 4: 全量验证** — `go build ./... ; gofmt -l <改动文件> ; go test ./internal/ui/ ./pkg/theme/`

- [ ] **Step 5: lint** — `pwsh -File scripts/lint_agents_md.ps1`（无 mdlink 真错）

- [ ] **Step 6: 运行时人工核对（交用户）** — `dev.ps1 d1` 后看工具栏：整条背景/边框、grip 点、中/英模式按钮(蓝/灰+白字)、全半角●/月牙、标点。，/.,、齿轮，切换各状态确认与迁移前一致。

---

## 验收清单
- [ ] 工具栏背景条/按钮框/mode 文字走 View 引擎；符号/齿轮矢量后处理（定位用 View rect）。
- [ ] mode 中/英状态覆盖生效；width/punct/settings 继承 button base（零回归）。
- [ ] default/msime 含 views.toolbar；token 映射 ResolvedTheme.Toolbar。
- [ ] build ✓ / 改动文件 gofmt 干净 / 回归网 PASS / AGENTS.md lint 通过。
- [ ] 运行时人工核对（交用户）——尤其整条宽度 116 与各符号定位。
