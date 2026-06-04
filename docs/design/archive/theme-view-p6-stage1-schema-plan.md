<!-- Parent: theme-view-p6-bridge-retirement.md -->

# P6 阶段 1：theme 包 schema 地基 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `pkg/theme` 建立 `Behavior` 行为配置 schema（三层覆盖的前两层：内置默认 + 主题覆盖）与 `views.metrics` 几何 schema，并把 `Behavior` 接入 `ResolvedV25` 解析链——全部纯 theme 包、单测覆盖、不触碰任何渲染代码。

**Architecture:** `Behavior`（YAML，全可空指针）→ `defaultBehavior()` 基线 ⊕ `mergeBehavior()` 主题覆盖 → `ResolvedBehavior`（解析后具体值），由 `ResolveV25` 填入 `rv.Behavior`。`ViewMetrics` 作为 `Views` 的新字段承载列表级几何 + `defaultViews().Metrics` 基线 + `mergeViews` 支持；本阶段**只立 schema 与 merge 单测**，不在 `ResolveV25` 启用 `defaultViews` merge（那会牵动 ui 合成桥，属阶段 2）。用户 override 层属阶段 3。

**Tech Stack:** Go、`gopkg.in/yaml.v3`、`go test`。worktree：`d:\Develop\workspace\go_dev\WindInput\.worktrees\theme-v25`，源码在 `wind_input\` 子目录。所有 `go` 命令在 `wind_input\` 下执行，`$env:CGO_ENABLED=1`。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `wind_input/pkg/theme/behavior.go` | `Behavior`/`ResolvedBehavior` 类型 + `defaultBehavior()` + `mergeBehavior()` | 新建 |
| `wind_input/pkg/theme/behavior_test.go` | Behavior 默认值 + merge + YAML 解析单测 | 新建 |
| `wind_input/pkg/theme/theme.go` | `Theme` 加 `Behavior *Behavior` 字段 | 修改 (:19-29) |
| `wind_input/pkg/theme/resolved.go` | `ResolvedV25` 加 `Behavior ResolvedBehavior` 字段 | 修改 (:7-12) |
| `wind_input/pkg/theme/resolver.go` | `ResolveV25` 填 `rv.Behavior` | 修改 (:56-69) |
| `wind_input/pkg/theme/views.go` | `ViewMetrics`/`AccentBarMetrics` 类型 + `Views.Metrics` + `defaultViews().Metrics` + `mergeViews` 支持 | 修改 (:52-66, :199-310) |
| `wind_input/pkg/theme/views_metrics_test.go` | metrics 默认值 + merge + YAML 解析单测 | 新建 |

---

## Task 1: Behavior schema + 默认值 + merge

**Files:**
- Create: `wind_input/pkg/theme/behavior.go`
- Test: `wind_input/pkg/theme/behavior_test.go`

- [ ] **Step 1: 写失败测试**

`wind_input/pkg/theme/behavior_test.go`:

```go
package theme

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultBehavior(t *testing.T) {
	b := defaultBehavior()
	if b.FontSize != 18 {
		t.Errorf("FontSize 默认应为 18, got %d", b.FontSize)
	}
	if b.AlwaysShowPager != false {
		t.Errorf("AlwaysShowPager 默认应为 false, got %v", b.AlwaysShowPager)
	}
	if b.ShowPageNumber != true {
		t.Errorf("ShowPageNumber 默认应为 true, got %v", b.ShowPageNumber)
	}
	if b.VerticalMaxWidth != 600 {
		t.Errorf("VerticalMaxWidth 默认应为 600, got %d", b.VerticalMaxWidth)
	}
}

func TestMergeBehavior_NilKeepsBase(t *testing.T) {
	base := defaultBehavior()
	got := mergeBehavior(base, nil)
	if got != base {
		t.Errorf("nil override 应原样返回基线, got %+v", got)
	}
}

func TestMergeBehavior_PartialOverride(t *testing.T) {
	base := defaultBehavior()
	fs := 22
	apn := true
	ov := &Behavior{FontSize: &fs, AlwaysShowPager: &apn}
	got := mergeBehavior(base, ov)
	if got.FontSize != 22 {
		t.Errorf("FontSize 应被覆盖为 22, got %d", got.FontSize)
	}
	if got.AlwaysShowPager != true {
		t.Errorf("AlwaysShowPager 应被覆盖为 true, got %v", got.AlwaysShowPager)
	}
	// 未覆盖字段保持基线
	if got.ShowPageNumber != true || got.VerticalMaxWidth != 600 {
		t.Errorf("未覆盖字段应保持基线, got ShowPageNumber=%v VerticalMaxWidth=%d", got.ShowPageNumber, got.VerticalMaxWidth)
	}
}

func TestBehavior_YAMLParse(t *testing.T) {
	src := []byte("font_size: 16\nshow_page_number: false\n")
	var b Behavior
	if err := yaml.Unmarshal(src, &b); err != nil {
		t.Fatalf("yaml 解析失败: %v", err)
	}
	if b.FontSize == nil || *b.FontSize != 16 {
		t.Errorf("font_size 应解析为 16, got %v", b.FontSize)
	}
	if b.ShowPageNumber == nil || *b.ShowPageNumber != false {
		t.Errorf("show_page_number 应解析为 false, got %v", b.ShowPageNumber)
	}
	// 未写字段应为 nil（跟随主题语义）
	if b.AlwaysShowPager != nil {
		t.Errorf("未写的 always_show_pager 应为 nil, got %v", b.AlwaysShowPager)
	}
}
```

- [ ] **Step 2: 跑测试，确认编译失败**

Run（在 `wind_input\` 下）:
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestDefaultBehavior|TestMergeBehavior|TestBehavior_YAMLParse' -v
```
Expected: 编译失败，`undefined: defaultBehavior` / `undefined: Behavior` 等。

- [ ] **Step 3: 写最小实现**

`wind_input/pkg/theme/behavior.go`:

```go
package theme

// Behavior 主题行为配置 YAML schema（P6）。全部为可空指针，nil=未指定、走 defaultBehavior。
// 这是"可被用户覆盖"字段的白名单：出现在此结构的字段才允许用户 override（用户 override 层见阶段 3）。
type Behavior struct {
	FontSize         *int  `yaml:"font_size,omitempty" json:"font_size,omitempty"`
	AlwaysShowPager  *bool `yaml:"always_show_pager,omitempty" json:"always_show_pager,omitempty"`
	ShowPageNumber   *bool `yaml:"show_page_number,omitempty" json:"show_page_number,omitempty"`
	VerticalMaxWidth *int  `yaml:"vertical_max_width,omitempty" json:"vertical_max_width,omitempty"`
}

// ResolvedBehavior 解析后的行为配置（所有字段已填具体值）。
type ResolvedBehavior struct {
	FontSize         int
	AlwaysShowPager  bool
	ShowPageNumber   bool
	VerticalMaxWidth int
}

// defaultBehavior 引擎内置行为基线。值与重构前 hardcode 现状一致（零回归）：
// 字号 18（= 旧 baseFontSize 默认）、单页不显翻页区、显示页码、竖排最大宽 600。
func defaultBehavior() ResolvedBehavior {
	return ResolvedBehavior{
		FontSize:         18,
		AlwaysShowPager:  false,
		ShowPageNumber:   true,
		VerticalMaxWidth: 600,
	}
}

// mergeBehavior 用主题 behavior（非 nil 字段）覆盖基线，返回解析后行为。ov 为 nil 时原样返回基线。
func mergeBehavior(base ResolvedBehavior, ov *Behavior) ResolvedBehavior {
	if ov == nil {
		return base
	}
	out := base
	if ov.FontSize != nil {
		out.FontSize = *ov.FontSize
	}
	if ov.AlwaysShowPager != nil {
		out.AlwaysShowPager = *ov.AlwaysShowPager
	}
	if ov.ShowPageNumber != nil {
		out.ShowPageNumber = *ov.ShowPageNumber
	}
	if ov.VerticalMaxWidth != nil {
		out.VerticalMaxWidth = *ov.VerticalMaxWidth
	}
	return out
}
```

- [ ] **Step 4: 跑测试，确认通过**

Run:
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestDefaultBehavior|TestMergeBehavior|TestBehavior_YAMLParse' -v
```
Expected: PASS（4 个测试全过）。

- [ ] **Step 5: gofmt + commit**

```
gofmt -w pkg/theme/behavior.go pkg/theme/behavior_test.go
git add wind_input/pkg/theme/behavior.go wind_input/pkg/theme/behavior_test.go
git commit -m "feat(theme): P6 阶段1 Behavior schema + defaultBehavior + mergeBehavior"
```

---

## Task 2: Behavior 接入 Theme / ResolvedV25 / ResolveV25

**Files:**
- Modify: `wind_input/pkg/theme/theme.go:19-29`
- Modify: `wind_input/pkg/theme/resolved.go:7-12`
- Modify: `wind_input/pkg/theme/resolver.go:56-69`
- Test: `wind_input/pkg/theme/behavior_test.go`（追加）

- [ ] **Step 1: 写失败测试（追加到 behavior_test.go 末尾）**

照搬 `resolver_test.go` 既有 `TestResolveV25_Inline` 的构造方式（`setupTestThemes`/`makeTestManager`/`sampleLayoutYAML`/`samplePaletteYAML` 均为 `package theme` 内既有 helper，同包测试可直接用，避免依赖真实零件目录）：

```go
func TestResolveV25_BehaviorDefault(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)
	var lm, pm map[string]any
	_ = yaml.Unmarshal([]byte(sampleLayoutYAML), &lm)
	_ = yaml.Unmarshal([]byte(samplePaletteYAML), &pm)
	th := &Theme{Meta: ThemeMeta{Name: "t"}, Layout: lm, Palette: pm}
	rv, err := m.ResolveV25(th, false, tmp)
	if err != nil {
		t.Fatalf("ResolveV25: %v", err)
	}
	if rv.Behavior != defaultBehavior() {
		t.Errorf("未提供 behavior 时应为 defaultBehavior, got %+v", rv.Behavior)
	}
}

func TestResolveV25_BehaviorOverride(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)
	var lm, pm map[string]any
	_ = yaml.Unmarshal([]byte(sampleLayoutYAML), &lm)
	_ = yaml.Unmarshal([]byte(samplePaletteYAML), &pm)
	fs := 20
	th := &Theme{Meta: ThemeMeta{Name: "t"}, Layout: lm, Palette: pm, Behavior: &Behavior{FontSize: &fs}}
	rv, err := m.ResolveV25(th, false, tmp)
	if err != nil {
		t.Fatalf("ResolveV25: %v", err)
	}
	if rv.Behavior.FontSize != 20 {
		t.Errorf("主题 behavior.font_size 应覆盖为 20, got %d", rv.Behavior.FontSize)
	}
	if rv.Behavior.ShowPageNumber != true {
		t.Errorf("未覆盖字段应保持基线 true, got %v", rv.Behavior.ShowPageNumber)
	}
}
```

> 注：`behavior_test.go` 顶部已 `import "gopkg.in/yaml.v3"`（Task 1），无需重复。`setupTestThemes`/`makeTestManager`/`sampleLayoutYAML`/`samplePaletteYAML` 见 `resolver_test.go`（同包，直接引用）。

- [ ] **Step 2: 跑测试，确认失败**

Run:
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestResolveV25_Behavior' -v
```
Expected: 编译失败 `th.Behavior undefined` / `rv.Behavior undefined`。

- [ ] **Step 3a: Theme 加 Behavior 字段**

`wind_input/pkg/theme/theme.go`，在 `Views` 字段后（:27 之后）加：

```go
	Views     *Views     `yaml:"views,omitempty" json:"views,omitempty"` // 盒模型 View 外观（v2.6 P2）
	Behavior  *Behavior  `yaml:"behavior,omitempty" json:"behavior,omitempty"` // 行为配置（v2.6 P6，可被用户覆盖）
	Overrides *Overrides `yaml:"overrides,omitempty" json:"overrides,omitempty"`
```

- [ ] **Step 3b: ResolvedV25 加 Behavior 字段**

`wind_input/pkg/theme/resolved.go`，`ResolvedV25` 结构（:7-12）加字段：

```go
type ResolvedV25 struct {
	Meta     ThemeMeta
	Layout   ResolvedLayout
	Palette  ResolvedPalette
	Views    *Views           // 盒模型 View 外观（v2.6 P2）；nil=主题未提供 views，渲染器用合成桥
	Behavior ResolvedBehavior // 行为配置（v2.6 P6）：defaultBehavior ⊕ 主题 behavior（用户 override 在 ui/config 层）
}
```

- [ ] **Step 3c: ResolveV25 填 rv.Behavior**

`wind_input/pkg/theme/resolver.go`，在 `rv.Views = t.Views`（:65）之后、`return rv, nil`（:69）之前加：

```go
	// 6. behavior（v2.6 P6）：defaultBehavior 基线 ⊕ 主题 behavior（非 nil 字段覆盖）。
	// 用户 override 不在此处——它在 ui/config 层注入（nil=跟随主题）。
	rv.Behavior = mergeBehavior(defaultBehavior(), t.Behavior)
```

- [ ] **Step 4: 跑测试，确认通过**

Run:
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestResolveV25_Behavior' -v
```
Expected: PASS。若因 Manager 零件路径失败，按 Step 1 注释调整 Manager 构造后再跑。

- [ ] **Step 5: 全包测试 + gofmt + commit**

```
$env:CGO_ENABLED=1; go test ./pkg/theme/ 2>&1 | Select-String -NotMatch 'TestBuiltin|TestListAvailable'
gofmt -w pkg/theme/theme.go pkg/theme/resolved.go pkg/theme/resolver.go pkg/theme/behavior_test.go
git add wind_input/pkg/theme/theme.go wind_input/pkg/theme/resolved.go wind_input/pkg/theme/resolver.go wind_input/pkg/theme/behavior_test.go
git commit -m "feat(theme): P6 阶段1 Behavior 接入 Theme/ResolvedV25/ResolveV25"
```
Expected: 除已知 pre-existing 的 `TestBuiltin*`/`TestListAvailable*`（worktree 缺 build/data/themes）外全过。

---

## Task 3: views.metrics 几何 schema + 基线 + merge

**Files:**
- Modify: `wind_input/pkg/theme/views.go`（类型 :52-66 区域加 Metrics；`defaultViews` :199-210；`mergeViews` :283-310）
- Test: `wind_input/pkg/theme/views_metrics_test.go`

> 说明：本任务只立 schema + 基线 + merge 单测。`ResolveV25` 目前 `rv.Views = t.Views` 原样透传、**不** merge `defaultViews()`（resolver.go:61-65 注释）；启用 defaultViews merge 会牵动 ui 合成桥，属**阶段 2**，本任务不动 `ResolveV25`。

- [ ] **Step 1: 写失败测试**

`wind_input/pkg/theme/views_metrics_test.go`:

```go
package theme

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultViews_Metrics(t *testing.T) {
	v := defaultViews()
	if v.Metrics == nil {
		t.Fatal("defaultViews().Metrics 不应为 nil")
	}
	if v.Metrics.ItemSpacing == nil || *v.Metrics.ItemSpacing != 12 {
		t.Errorf("item_spacing 基线应为 12, got %v", v.Metrics.ItemSpacing)
	}
	if v.Metrics.BandGap == nil || *v.Metrics.BandGap != 4 {
		t.Errorf("band_gap 基线应为 4, got %v", v.Metrics.BandGap)
	}
	if v.Metrics.ShadowOffset == nil || *v.Metrics.ShadowOffset != 2 {
		t.Errorf("shadow_offset 基线应为 2, got %v", v.Metrics.ShadowOffset)
	}
	if v.Metrics.AccentBar == nil || v.Metrics.AccentBar.Width == nil || *v.Metrics.AccentBar.Width != 3 {
		t.Errorf("accent_bar.width 基线应为 3, got %+v", v.Metrics.AccentBar)
	}
}

func TestMergeViews_MetricsOverride(t *testing.T) {
	base := defaultViews()
	sp := 16
	ov := Views{Metrics: &ViewMetrics{ItemSpacing: &sp}}
	merged := mergeViews(base, ov)
	if merged.Metrics == nil || merged.Metrics.ItemSpacing == nil || *merged.Metrics.ItemSpacing != 16 {
		t.Errorf("item_spacing 应被覆盖为 16, got %+v", merged.Metrics)
	}
	// 未覆盖项保持基线
	if merged.Metrics.BandGap == nil || *merged.Metrics.BandGap != 4 {
		t.Errorf("band_gap 应保持基线 4, got %v", merged.Metrics.BandGap)
	}
}

func TestViewMetrics_YAMLParse(t *testing.T) {
	src := []byte("metrics:\n  item_spacing: 14\n  accent_bar:\n    width: 5\n")
	var v Views
	if err := yaml.Unmarshal(src, &v); err != nil {
		t.Fatalf("yaml 解析失败: %v", err)
	}
	if v.Metrics == nil || v.Metrics.ItemSpacing == nil || *v.Metrics.ItemSpacing != 14 {
		t.Errorf("item_spacing 应解析为 14, got %+v", v.Metrics)
	}
	if v.Metrics.AccentBar == nil || v.Metrics.AccentBar.Width == nil || *v.Metrics.AccentBar.Width != 5 {
		t.Errorf("accent_bar.width 应解析为 5, got %+v", v.Metrics)
	}
}
```

- [ ] **Step 2: 跑测试，确认失败**

Run:
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestDefaultViews_Metrics|TestMergeViews_MetricsOverride|TestViewMetrics_YAMLParse' -v
```
Expected: 编译失败 `v.Metrics undefined` / `undefined: ViewMetrics`。

- [ ] **Step 3a: 加 ViewMetrics / AccentBarMetrics 类型 + Views.Metrics 字段**

`wind_input/pkg/theme/views.go`，在 `Views` 结构（:52-66）内 `Menu` 字段后加 `Metrics` 字段：

```go
	Menu          *MenuViews    `yaml:"menu,omitempty"`    // P4-D 弹出菜单
	Metrics       *ViewMetrics  `yaml:"metrics,omitempty"` // P6 候选窗列表级几何
}
```

在 `Views` 结构定义之后加类型（紧随 :66 后）：

```go
// ViewMetrics 候选窗"列表级"几何（P6）：不便归入单个 ViewNode 的尺寸。全部可空指针，nil=走 defaultViews 基线。
type ViewMetrics struct {
	ItemSpacing  *int              `yaml:"item_spacing,omitempty"`  // 横排候选框间距基数（旧 hardcode 12/16）
	BandGap      *int              `yaml:"band_gap,omitempty"`      // band 间距（旧 WindowGap）
	ShadowOffset *int              `yaml:"shadow_offset,omitempty"` // 窗口投影偏移
	AccentBar    *AccentBarMetrics `yaml:"accent_bar,omitempty"`    // 强调条尺寸
}

// AccentBarMetrics 强调条尺寸（P6）。
type AccentBarMetrics struct {
	Width       *int     `yaml:"width,omitempty"`        // 条宽
	Offset      *int     `yaml:"offset,omitempty"`       // 左缘偏移
	HeightRatio *float64 `yaml:"height_ratio,omitempty"` // 条高 = ItemHeight × 此比例
}
```

- [ ] **Step 3b: defaultViews 加 Metrics 基线**

`wind_input/pkg/theme/views.go`，`defaultViews()`（:199-210）的返回 `Views{...}` 内，`FooterBar` 后加：

```go
		AccentBar:  ViewNode{},
		FooterBar:  ViewNode{},
		Metrics: &ViewMetrics{
			ItemSpacing:  intp(12),
			BandGap:      intp(4),
			ShadowOffset: intp(2),
			AccentBar:    &AccentBarMetrics{Width: intp(3), Offset: intp(1), HeightRatio: f64p(0.6)},
		},
	}
}
```

并在文件内（`intp` 旁，:186 附近）加 float64 指针 helper（若已存在同名 helper 则复用，先 grep `func f64p` 确认）：

```go
func f64p(v float64) *float64 { return &v }
```

- [ ] **Step 3c: mergeViews 支持 Metrics**

`wind_input/pkg/theme/views.go`，`mergeViews`（:283-)的返回 `Views{...}` 内，所有 `mergeViewNode(...)` 行之后加 Metrics 合并字段：

```go
		FooterBar:     mergeViewNode(base.FooterBar, ov.FooterBar),
		Metrics:       mergeMetrics(base.Metrics, ov.Metrics),
```

> 注：`mergeViews`（views.go:283-295）当前仅 merge 9 个 ViewNode（Window..FooterBar），**不含** Status/Tooltip/Toolbar/Menu/Metrics 字段。本步即在其返回的 `Views{}` 末尾追加 `Metrics: mergeMetrics(base.Metrics, ov.Metrics)` 一行（共 10 个字段）。

在 `mergeViews` 之后加合并函数：

```go
// mergeMetrics 用 ov 的非 nil 字段覆盖 base（ov/base 任一为 nil 时取另一方；均非 nil 时逐字段覆盖）。
func mergeMetrics(base, ov *ViewMetrics) *ViewMetrics {
	if ov == nil {
		return base
	}
	if base == nil {
		return ov
	}
	out := *base
	if ov.ItemSpacing != nil {
		out.ItemSpacing = ov.ItemSpacing
	}
	if ov.BandGap != nil {
		out.BandGap = ov.BandGap
	}
	if ov.ShadowOffset != nil {
		out.ShadowOffset = ov.ShadowOffset
	}
	if ov.AccentBar != nil {
		out.AccentBar = mergeAccentBarMetrics(out.AccentBar, ov.AccentBar)
	}
	return &out
}

// mergeAccentBarMetrics 同 mergeMetrics 的逐字段覆盖语义。
func mergeAccentBarMetrics(base, ov *AccentBarMetrics) *AccentBarMetrics {
	if ov == nil {
		return base
	}
	if base == nil {
		return ov
	}
	out := *base
	if ov.Width != nil {
		out.Width = ov.Width
	}
	if ov.Offset != nil {
		out.Offset = ov.Offset
	}
	if ov.HeightRatio != nil {
		out.HeightRatio = ov.HeightRatio
	}
	return &out
}
```

- [ ] **Step 4: 跑测试，确认通过**

Run:
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestDefaultViews_Metrics|TestMergeViews_MetricsOverride|TestViewMetrics_YAMLParse' -v
```
Expected: PASS（3 个测试全过）。

- [ ] **Step 5: 全包测试 + gofmt + commit**

```
$env:CGO_ENABLED=1; go test ./pkg/theme/ 2>&1 | Select-String -NotMatch 'TestBuiltin|TestListAvailable'
gofmt -w pkg/theme/views.go pkg/theme/views_metrics_test.go
git add wind_input/pkg/theme/views.go wind_input/pkg/theme/views_metrics_test.go
git commit -m "feat(theme): P6 阶段1 views.metrics 几何 schema + 基线 + merge"
```
Expected: 除 pre-existing 的 `TestBuiltin*`/`TestListAvailable*` 外全过。

---

## 阶段 1 完成判据

- [ ] `Behavior`/`ResolvedBehavior`/`defaultBehavior`/`mergeBehavior` 就位，单测过。
- [ ] `Theme.Behavior` + `ResolvedV25.Behavior` 接入，`ResolveV25` 填 `rv.Behavior`（默认 + 主题覆盖），单测过。
- [ ] `ViewMetrics`/`AccentBarMetrics` + `Views.Metrics` + `defaultViews().Metrics` 基线 + `mergeViews` 支持，单测过。
- [ ] `go build ./...`（wind_input）通过；`pkg/theme` 除 pre-existing TestBuiltin*/TestList* 外全绿。
- [ ] 全程纯 theme 包，未改任何 `internal/ui` / 渲染代码（零渲染影响）。

## 阶段 1 非目标（留后续阶段）

- ❌ 在 `ResolveV25` 启用 `defaultViews()` merge（阶段 2，牵动合成桥）。
- ❌ ui 候选窗消费 `Behavior`/`Metrics`、删合成桥（阶段 2）。
- ❌ 用户 override 注入（config 层 nil=跟随，阶段 3）。
- ❌ 主题 yaml 增补 behavior/metrics 块（阶段 2 零回归接入时按需）。
- ❌ AGENTS.md 同步（随阶段 2/3 schema 落地一并更新）。

## 关联
- `theme-view-p6-bridge-retirement.md` — P6 总设计（本计划实现其 §3.1/§3.2/§9-1·2 的 theme 包部分）
