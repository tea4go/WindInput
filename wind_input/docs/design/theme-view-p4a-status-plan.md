# P4-A 状态泡 View 化 + 颜色 YAML 化 实现计划

> **For agentic workers:** 用 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。

**Goal:** 把状态泡（mode indicator bubble）的渲染从 gg 直绘迁移到盒模型 View 引擎，并把其颜色（背景/文字）做成 YAML `views.status` token 驱动，零回归。

**Architecture:** `StatusRenderer`（独立 struct）复用包级 `Layout`/`PaintTree` 引擎核心。新增 `buildStatusTree` 建单节点 View 树；颜色经"基于 resolver 的 token 解析"从 `views.status` 取，默认映射 `ResolvedTheme.ModeIndicator`（零回归），运行时 `StatusWindowConfig` 自定义色仍优先。

**Tech Stack:** Go；`github.com/gogpu/gg`；`pkg/theme` views schema（`*int` 显式语义）。

**Scope note（务实收窄，执行后同步 spec）:** P4-A 只 token 化**颜色**（background/text）。状态泡的 padding(6)/borderRadius(cfg.BorderRadius)/fontSize(cfg.FontSize)/opacity 保持现状从运行时 `StatusWindowConfig` 取，**不进 views**——因它们本就是运行时配置而非主题，且 spec 规定圆角/字号运行时优先。状态泡几何 YAML 化按需后补。

**零回归基准:** 现状 `Render`（status_renderer.go:49-95）：单行文本，水平居中，bg 圆角矩形（无描边），文本基线 `padding + fontSize*0.8`。颜色优先级"自定义 cfg > ModeIndicator 主题 > 默认 `{60,60,60,240}`/`{255,255,255,255}`"。

---

## 文件结构

- **修改** `pkg/theme/views.go`：`Views` 加 `Status *ViewNode`；新增 `ResolvedStatusViews{BgColor, TextColor color.Color}`。
- **新建** `internal/ui/viewbox_status.go`：`resolveTokenColor`（通用 token 解析器入口）、`(*StatusRenderer).resolveStatusColors`、`(*StatusRenderer).buildStatusTree`。
- **修改** `internal/ui/status_renderer.go`：`StatusRenderer` 加 `themeViews *theme.Views` 字段；`SetTheme` 存它；`Render` 改为建树→`Layout`→`PaintTree`。
- **新建** `internal/ui/viewbox_status_test.go`：颜色解析单测 + 几何/颜色指纹回归网。
- **修改** `themes/default/theme.yaml`、`themes/msime/theme.yaml`：加 `views.status` 颜色块。
- **修改** `pkg/theme/AGENTS.md`：记 `Views.Status` + `ResolvedStatusViews`。

---

## Task 1：theme 包加 Status schema + ResolvedStatusViews

**Files:**
- Modify: `wind_input/pkg/theme/views.go`（`Views` 结构 + 新增 `ResolvedStatusViews`）
- Test: `wind_input/pkg/theme/views_yaml_test.go`（追加用例）

- [ ] **Step 1: 写失败测试**（追加到 `views_yaml_test.go` 末尾）

```go
// TestViews_StatusParse 验证 views.status 块解析到 Views.Status（ViewNode）。
func TestViews_StatusParse(t *testing.T) {
	data := `
status:
  background: {color: "${background}"}
  color: "${text}"
`
	var v Views
	if err := yaml.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Status == nil {
		t.Fatal("views.status 应解析为非 nil")
	}
	if v.Status.Background.Color != "${background}" {
		t.Errorf("status background token, got %q", v.Status.Background.Color)
	}
	if v.Status.Color != "${text}" {
		t.Errorf("status text token, got %q", v.Status.Color)
	}
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./pkg/theme/ -run TestViews_StatusParse -v`
Expected: 编译失败 `v.Status undefined`。

- [ ] **Step 3: 加 `Status` 字段到 `Views`**（`views.go` 的 `Views` 结构，`FooterBar` 字段之后）

```go
	FooterBar     ViewNode `yaml:"footer_bar,omitempty"`
	Status        *ViewNode `yaml:"status,omitempty"` // P4-A 状态泡（独立窗口，单节点）
```

- [ ] **Step 4: 加 `ResolvedStatusViews`**（`views.go` 末尾，`ResolvedViews` 之后）

```go
// ResolvedStatusViews 状态泡解析后外观（P4-A）。仅颜色——几何/字号由运行时 StatusWindowConfig 提供。
type ResolvedStatusViews struct {
	BgColor   color.Color
	TextColor color.Color
}
```

- [ ] **Step 5: 运行验证通过**

Run: `go test ./pkg/theme/ -run TestViews_StatusParse -v`
Expected: PASS

- [ ] **Step 6: go fmt**

Run: `gofmt -w pkg/theme/views.go pkg/theme/views_yaml_test.go`

---

## Task 2：ui 包 token 解析器 + 状态泡颜色解析

**Files:**
- Create: `wind_input/internal/ui/viewbox_status.go`
- Test: `wind_input/internal/ui/viewbox_status_test.go`

- [ ] **Step 1: 写失败测试**（新建 `viewbox_status_test.go`）

```go
package ui

import (
	"image/color"
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// TestResolveTokenColor 验证通用 token 解析：hex 直解 / ${name} 走 resolver / 未知与空回退 nil。
func TestResolveTokenColor(t *testing.T) {
	res := func(name string) color.Color {
		if name == "background" {
			return color.RGBA{1, 2, 3, 255}
		}
		return nil
	}
	if c := resolveTokenColor("${background}", res); c != (color.RGBA{1, 2, 3, 255}) {
		t.Errorf("token 应解析为 resolver 值, got %v", c)
	}
	if c := resolveTokenColor("${unknown}", res); c != nil {
		t.Errorf("未知 token 应回退 nil, got %v", c)
	}
	if c := resolveTokenColor("", res); c != nil {
		t.Errorf("空串应回退 nil, got %v", c)
	}
	if c := resolveTokenColor("#FF0000", res); c == nil {
		t.Error("hex 应解析为非 nil")
	}
}

// TestResolveStatusColors 验证状态泡颜色优先级：自定义 cfg > views token(ModeIndicator) > 默认。
func TestResolveStatusColors(t *testing.T) {
	mi := theme.ResolvedModeIndicatorColors{
		BackgroundColor: color.RGBA{10, 20, 30, 255},
		TextColor:       color.RGBA{200, 200, 200, 255},
	}
	rt := &theme.ResolvedTheme{ModeIndicator: mi}
	views := &theme.Views{Status: &theme.ViewNode{
		Background: theme.ViewFill{Color: "${background}"},
		Color:      "${text}",
	}}
	r := &StatusRenderer{resolvedTheme: rt, themeViews: views}

	// views token → ModeIndicator
	rsv := r.resolveStatusColors(StatusWindowConfig{})
	if rsv.BgColor != mi.BackgroundColor {
		t.Errorf("bg 应来自 ModeIndicator, got %v", rsv.BgColor)
	}
	if rsv.TextColor != mi.TextColor {
		t.Errorf("text 应来自 ModeIndicator, got %v", rsv.TextColor)
	}

	// 自定义 cfg 优先
	rsv2 := r.resolveStatusColors(StatusWindowConfig{BackgroundColor: "#FF0000", TextColor: "#00FF00"})
	if rsv2.BgColor == mi.BackgroundColor {
		t.Error("自定义 bg 应覆盖 views token")
	}
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/ui/ -run 'TestResolveTokenColor|TestResolveStatusColors' -v`
Expected: 编译失败（`resolveTokenColor`/`resolveStatusColors`/`themeViews` 未定义）。

- [ ] **Step 3: 实现 `viewbox_status.go` 颜色部分**（新建文件）

```go
package ui

// viewbox_status.go — 状态泡（mode indicator bubble）的 View 树构建与颜色解析（P4-A）。
// 状态泡复用包级 Layout/PaintTree 引擎核心；颜色经 token 解析自 views.status，
// 默认映射 ResolvedTheme.ModeIndicator（零回归），运行时 StatusWindowConfig 自定义色优先。

import (
	"image/color"
	"strings"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// resolveTokenColor 通用颜色 token 解析：hex(#RRGGBB[AA]) 直解；${name} 交给 resolver；
// 空 / 未知 token / 解析失败返回 nil（调用方据此回退）。各窗口注入自己的 resolver。
func resolveTokenColor(s string, resolver func(name string) color.Color) color.Color {
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		return resolver(s[2 : len(s)-1])
	}
	if c, err := theme.ParseHexColor(s); err == nil {
		return c
	}
	return nil
}

// resolveStatusColors 计算状态泡最终颜色，优先级：自定义 cfg > views.status token > ModeIndicator 默认。
func (r *StatusRenderer) resolveStatusColors(cfg StatusWindowConfig) theme.ResolvedStatusViews {
	// base：ModeIndicator 主题色（无主题时用内置默认）
	bg := color.Color(color.RGBA{60, 60, 60, 240})
	text := color.Color(color.RGBA{255, 255, 255, 255})
	if r.resolvedTheme != nil {
		bg = r.resolvedTheme.ModeIndicator.BackgroundColor
		text = r.resolvedTheme.ModeIndicator.TextColor
	}

	// views.status token 覆盖（resolver 映射到 ModeIndicator 同源色）
	if r.themeViews != nil && r.themeViews.Status != nil {
		res := func(name string) color.Color {
			switch name {
			case "background":
				if r.resolvedTheme != nil {
					return r.resolvedTheme.ModeIndicator.BackgroundColor
				}
			case "text":
				if r.resolvedTheme != nil {
					return r.resolvedTheme.ModeIndicator.TextColor
				}
			}
			return nil
		}
		if c := resolveTokenColor(r.themeViews.Status.Background.Color, res); c != nil {
			bg = c
		}
		if c := resolveTokenColor(r.themeViews.Status.Color, res); c != nil {
			text = c
		}
	}

	// 自定义 cfg 优先级最高
	if cfg.BackgroundColor != "" {
		if c, ok := parseHexColor(cfg.BackgroundColor); ok {
			bg = c
		}
	}
	if cfg.TextColor != "" {
		if c, ok := parseHexColor(cfg.TextColor); ok {
			text = c
		}
	}

	return theme.ResolvedStatusViews{BgColor: bg, TextColor: text}
}
```

- [ ] **Step 4: 在 `status_renderer.go` 的 `StatusRenderer` 加 `themeViews` 字段**（struct 内 `resolvedTheme` 之后）

```go
	resolvedTheme *theme.ResolvedTheme
	themeViews    *theme.Views
```

- [ ] **Step 5: 运行验证通过**

Run: `go test ./internal/ui/ -run 'TestResolveTokenColor|TestResolveStatusColors' -v`
Expected: PASS

- [ ] **Step 6: go fmt**

Run: `gofmt -w internal/ui/viewbox_status.go internal/ui/viewbox_status_test.go internal/ui/status_renderer.go`

---

## Task 3：buildStatusTree + 几何/颜色指纹回归网

**Files:**
- Modify: `wind_input/internal/ui/viewbox_status.go`（加 `buildStatusTree`）
- Test: `wind_input/internal/ui/viewbox_status_test.go`（加指纹测试）

- [ ] **Step 1: 写失败测试**（追加到 `viewbox_status_test.go`）

```go
// TestBuildStatusTree_Fingerprint 验证状态泡 View 树几何 + 颜色指纹（零回归基准）。
// 单节点文本 View：FixedW（minWidth 钳制）+ Padding + 居中文本 + bg 圆角。
func TestBuildStatusTree_Fingerprint(t *testing.T) {
	rsv := theme.ResolvedStatusViews{
		BgColor:   color.RGBA{60, 60, 60, 240},
		TextColor: color.RGBA{255, 255, 255, 255},
	}
	// 桩：固定字宽，scale=1，避免依赖真实字体后端
	m := fixedMeasurer{charW: 10}
	root := buildStatusTree("中", rsv, 18.0, 6.0, 8.0, 1.0, m)
	Layout(root, 0, 0, m)

	r := root.Rect()
	// 文本 "中" 宽 10，padding 6*2=12 → 22，minWidth 32 钳制 → FixedW 32
	if r.Dx() != 32 {
		t.Errorf("width 应被 minWidth 钳制为 32, got %d", r.Dx())
	}
	// 高 = fontSize 18 + padding 12 = 30
	if r.Dy() != 30 {
		t.Errorf("height 应为 30, got %d", r.Dy())
	}
	if root.Background.Color != (color.RGBA{60, 60, 60, 240}) {
		t.Errorf("bg 颜色指纹不符, got %v", root.Background.Color)
	}
	if root.TextStyle.Color != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("text 颜色指纹不符, got %v", root.TextStyle.Color)
	}
	if root.TextStyle.Align != AlignCenter {
		t.Error("状态泡文本应水平居中")
	}
}

type fixedMeasurer struct{ charW float64 }

func (f fixedMeasurer) MeasureString(s string, fontSize float64) float64 {
	return float64(len([]rune(s))) * f.charW
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/ui/ -run TestBuildStatusTree_Fingerprint -v`
Expected: 编译失败（`buildStatusTree` 未定义）。

- [ ] **Step 3: 实现 `buildStatusTree`**（追加到 `viewbox_status.go`）

```go
// buildStatusTree 构建状态泡的 View 树（单文本节点：bg 圆角 + padding + 居中文本）。
// 所有尺寸入参为逻辑像素，内部乘 scale 得最终像素（与现状 Render 一致）。
// minWidth=32（逻辑）经 FixedW 钳制；高 = fontSize + padding*2。
func buildStatusTree(text string, rsv theme.ResolvedStatusViews, fontSize, padding, borderRadius, scale float64, m TextMeasurer) *View {
	fs := fontSize * scale
	pad := int(padding * scale)
	minW := int(32.0 * scale)

	tw := m.MeasureString(text, fs)
	w := int(tw) + pad*2
	if w < minW {
		w = minW
	}

	v := &View{
		Text:       text,
		TextStyle:  TextStyle{FontSize: fs, Color: rsv.TextColor, Align: AlignCenter},
		Padding:    Edges{Top: pad, Right: pad, Bottom: pad, Left: pad},
		Background: Fill{Color: rsv.BgColor},
		Border:     Border{Radius: int(borderRadius * scale)},
		FixedW:     w,
	}
	return v
}
```

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/ui/ -run TestBuildStatusTree_Fingerprint -v`
Expected: PASS

- [ ] **Step 5: go fmt**

Run: `gofmt -w internal/ui/viewbox_status.go internal/ui/viewbox_status_test.go`

---

## Task 4：改造 Render 走 View 引擎

**Files:**
- Modify: `wind_input/internal/ui/status_renderer.go`（`Render` + `SetTheme`）

- [ ] **Step 1: 改 `SetTheme` 存 themeViews**（status_renderer.go:197-201）

```go
// SetTheme 设置主题
func (r *StatusRenderer) SetTheme(resolved *theme.ResolvedTheme) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolvedTheme = resolved
	if resolved != nil {
		r.themeViews = resolved.Views
	} else {
		r.themeViews = nil
	}
}
```

- [ ] **Step 2: 改 `Render` 走 View 引擎**（替换 status_renderer.go:49-95 整个 `Render` 方法）

```go
// Render 将状态信息渲染为 RGBA 图像（盒模型 View 引擎）。
func (r *StatusRenderer) Render(state StatusState, cfg StatusWindowConfig) *image.RGBA {
	text := BuildStatusText(state, cfg.ShowMode, cfg.ShowPunct, cfg.ShowFullWidth)
	if text == "" {
		return nil
	}

	scale := GetDPIScale()

	r.mu.Lock()
	td := r.TextDrawer()
	rsv := r.resolveStatusColors(cfg)
	r.mu.Unlock()

	// 透明度应用到背景色（与现状一致）
	rsv.BgColor = applyOpacity(rsv.BgColor, cfg.Opacity)

	// 构建 View 树 + 布局
	root := buildStatusTree(text, rsv, cfg.FontSize, 6.0, cfg.BorderRadius, scale, td)
	Layout(root, 0, 0, td)

	w := root.Rect().Dx()
	h := root.Rect().Dy()
	dc := gg.NewContext(w, h)
	img := dc.Image().(*image.RGBA)
	PaintTree(root, dc, img, td)

	DrawDebugBanner(img)
	return img
}
```

- [ ] **Step 3: 删除被取代的 `getColors`**（status_renderer.go:97-126，逻辑已搬入 `resolveStatusColors`）

删除整个 `getColors` 方法（若编译报 `parseHexColor`/`applyOpacity` 未用需保留——它们仍被 `resolveStatusColors`/`Render` 使用，不删）。

- [ ] **Step 4: 编译 + 全量 ui 测试**

Run: `go build ./... ; go test ./internal/ui/ ./pkg/theme/`
Expected: 编译通过；新增测试 PASS；候选窗既有测试不破（`TestBuiltinDarkMode` 若 pre-existing fail 属已知，与本改动无关）。

- [ ] **Step 5: go fmt**

Run: `gofmt -w internal/ui/status_renderer.go`

---

## Task 5：迁移种子主题 views.status

**Files:**
- Modify: `wind_input/themes/default/theme.yaml`、`wind_input/themes/msime/theme.yaml`

- [ ] **Step 1: default 主题加 `views.status`**（在 `views:` 块内已有节点之后追加）

```yaml
  status:
    background: {color: "${background}"}
    color: "${text}"
```

- [ ] **Step 2: msime 主题加 `views.status`**（同上，追加到 `views:` 块）

```yaml
  status:
    background: {color: "${background}"}
    color: "${text}"
```

- [ ] **Step 3: 加主题解析验证测试**（追加到 `pkg/theme/views_yaml_test.go`）

```go
// TestDefaultThemeStatusParse 验证 default theme.yaml 的 views.status 解析。
func TestDefaultThemeStatusParse(t *testing.T) {
	data, err := os.ReadFile("../../themes/default/theme.yaml")
	if err != nil {
		t.Skip("default theme.yaml 不可读: " + err.Error())
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if th.Views == nil || th.Views.Status == nil {
		t.Fatal("default theme.yaml 应含 views.status")
	}
	if th.Views.Status.Background.Color != "${background}" {
		t.Errorf("status background token, got %q", th.Views.Status.Background.Color)
	}
}
```

- [ ] **Step 4: 运行验证**

Run: `go test ./pkg/theme/ -run TestDefaultThemeStatusParse -v`
Expected: PASS

---

## Task 6：AGENTS.md 同步 + 全量验证 + 运行时人工核对准备

**Files:**
- Modify: `wind_input/pkg/theme/AGENTS.md`

- [ ] **Step 1: 更新 `pkg/theme/AGENTS.md`**

在 views schema 相关段落补充：`Views` 新增 `Status *ViewNode`（状态泡，单节点）；新增导出类型 `ResolvedStatusViews{BgColor, TextColor}`。若该文件有"导出类型清单"或"views 字段表"，同步加入这两项。

- [ ] **Step 2: 全量编译 + fmt + 测试**

Run: `go build ./... ; gofmt -l internal/ui/ pkg/theme/ ; go test ./internal/ui/ ./pkg/theme/`
Expected: build ✓；`gofmt -l` 无输出（已全格式化）；测试 PASS（除已知 pre-existing `TestBuiltinDarkMode`）。

- [ ] **Step 3: AGENTS.md 引用检查**

Run: `pwsh -File scripts/lint_agents_md.ps1`
Expected: 无悬空引用报错。

- [ ] **Step 4: 运行时人工核对（交回用户）**

提示用户用 `dev.ps1 d1` 构建后切到 default/msime 主题，触发状态泡（模式切换提示），确认气泡背景/文字颜色与迁移前一致、位置/大小正常。这是集成验证（worktree 缺 build/data/themes，无法自动集成测试）。

---

## 验收清单
- [ ] 状态泡渲染走 `Layout`/`PaintTree`，旧 `getColors` gg 直绘色逻辑被 `resolveStatusColors` + View 取代。
- [ ] 颜色优先级"自定义 cfg > views token > ModeIndicator 默认"保持。
- [ ] default/msime 含 `views.status`，颜色 token 映射 ModeIndicator（零回归）。
- [ ] `go build` ✓ / `gofmt -l` 空 / 回归网（token 解析 + 颜色优先级 + 几何颜色指纹）PASS。
- [ ] `pkg/theme/AGENTS.md` 同步 + lint 通过。
- [ ] 运行时人工核对气泡零回归（交用户）。
