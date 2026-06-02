# P2 切片-0：default 主题 views schema 实现计划

> **For agentic workers:** 用 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。
> 上位 spec：`theme-view-p2-slice0-views.md`；P0 设计：`theme-view-architecture.md`。

**Goal:** 让候选窗渲染的外观参数从「散落在 RenderConfig + magic number」收拢为结构化
`ResolvedViews`，build 统一从它读；`default` 主题的 `ResolvedViews` 由 YAML `views` 解析而来，
其它主题由 RenderConfig 合成桥产出（渲染零回归）。

**Architecture:** 单 build、双 views 来源（策略 B）。`pkg/theme` 定义 views schema +
解析 + merge + 基线，产出 plain 值的 `theme.ResolvedViews`；`internal/ui` 的 build 消费它。
其它主题经 ui 侧合成桥 `RenderConfig → ResolvedViews`（临时，类 adapter）。

**Tech Stack:** Go、gopkg.in/yaml.v3、现有 viewbox 引擎（measure/arrange/paint）、gg。

**正确性网:** 几何对齐——每个重构 task 后，候选窗各 View 的 `Rect()` 与 Task 0 捕获的基准
逐项一致（横+竖、选中/hover/preedit/翻页/index 圆圈/强调条）。任一 task 打破对齐即回退该 task。

---

## File Structure

- `wind_input/pkg/theme/views.go`（新建）：`Views`/`ViewNode`/`ViewEdges`/`ViewFill`/`ViewBorder`/
  `ResolvedViews` 类型 + `defaultViews()` 基线 + `mergeViews()` + YAML 解析钩子。
- `wind_input/pkg/theme/views_test.go`（新建）：基线值、merge、token 解析单测。
- `wind_input/internal/ui/viewbox_views_bridge.go`（新建）：`renderConfigToViews(cfg) theme.ResolvedViews` 合成桥。
- `wind_input/internal/ui/viewbox_build.go`（改）：`buildHorizontalCandidateTree` 从 `ResolvedViews` 读外观。
- `wind_input/internal/ui/viewbox_build_vertical.go`（改）：同上竖排。
- `wind_input/internal/ui/renderer.go`（改）：`Renderer` 持有 `resolvedViews`；填充入口。
- `wind_input/pkg/config/config.go`（改）：`UIConfig.CandidateIndexLabels`。
- `wind_input/internal/ui/viewbox_geometry_test.go`（新建）：几何对齐回归网。
- 文档：`pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`。

---

## Task 0：几何对齐回归基准网（先建网）

**Files:** Create `wind_input/internal/ui/viewbox_geometry_test.go`

目的：在任何重构前，捕获当前 build 输出的各 View `Rect()` 为「基准」，后续每个 task 跑此测试保绿。
因为重构是纯结构调整、渲染应不变，基准用「同一输入下重构前后 Rect 序列一致」表达。

- [ ] **Step 1: 写几何快照 helper + 横竖基准测试**

```go
package ui

import (
	"fmt"
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
)

// flattenRects 深度优先收集 View 树所有节点的布局矩形，返回稳定字符串序列，
// 作为「几何指纹」。重构前后指纹一致即视为几何对齐。
func flattenRects(v *View) []string {
	if v == nil {
		return nil
	}
	r := v.Rect()
	out := []string{fmt.Sprintf("%d,%d,%d,%d", r.Min.X, r.Min.Y, r.Dx(), r.Dy())}
	for _, c := range v.Children {
		out = append(out, flattenRects(c)...)
	}
	return out
}

func geometryFingerprint(t *testing.T, layout config.CandidateLayout) []string {
	t.Helper()
	cfg := parityConfig()
	cfg.Layout = layout
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	cands := []Candidate{
		{Text: "中文", Index: 1},
		{Text: "中", Index: 2, Comment: "zhōng"},
		{Text: "众", Index: 3},
		{Text: "种", Index: 4},
		{Text: "重", Index: 5},
	}
	var tree *candWindowTree
	if layout == config.LayoutHorizontal {
		tree = r.buildHorizontalCandidateTree(cands, "zhong", 5, 2, 3, 0, 1, "")
	} else {
		tree = r.buildVerticalCandidateTree(cands, "zhong", 5, 2, 3, 0, 1, "")
	}
	Layout(tree.root, 0, 0, r.textDrawer)
	return flattenRects(tree.root)
}

// TestGeometryFingerprint_Horizontal 记录横排几何指纹；重构 task 跑它确保不变。
func TestGeometryFingerprint_Horizontal(t *testing.T) {
	fp := geometryFingerprint(t, config.LayoutHorizontal)
	if len(fp) == 0 {
		t.Fatal("空指纹")
	}
	t.Logf("H fingerprint (%d nodes): %v", len(fp), fp)
}

func TestGeometryFingerprint_Vertical(t *testing.T) {
	fp := geometryFingerprint(t, config.LayoutVertical)
	if len(fp) == 0 {
		t.Fatal("空指纹")
	}
	t.Logf("V fingerprint (%d nodes): %v", len(fp), fp)
}
```

> 注：实现 Task 0 时先核对 `buildHorizontalCandidateTree`/`buildVerticalCandidateTree` 的实际签名
> （`viewbox_build.go:132`、`viewbox_build_vertical.go`），按真实参数顺序调用。

- [ ] **Step 2: 跑测试捕获基准**

Run: `cd wind_input && go test ./internal/ui/ -run TestGeometryFingerprint -v`
Expected: PASS，日志打印两条指纹。**把这两条指纹复制为下方 golden 断言。**

- [ ] **Step 3: 把指纹固化为 golden 断言**

将 Step 2 日志里的指纹数组粘贴为常量 `wantH`/`wantV`，测试改为逐项比对（`reflect.DeepEqual`），
不一致即 `t.Errorf` 打印 diff。这样后续重构 task 一旦改变几何立即报警。

- [ ] **Step 4: Commit**

```bash
git add wind_input/internal/ui/viewbox_geometry_test.go
git commit -m "test(ui): 候选窗几何对齐回归网（P2 切片-0 基准）"
```

---

## Task 1：theme views 类型 + 默认基线

**Files:** Create `wind_input/pkg/theme/views.go`, `wind_input/pkg/theme/views_test.go`

- [ ] **Step 1: 定义类型（views.go）**

```go
package theme

// ViewEdges 四向距离，*int：nil=未写（回退基线），非 nil（含 0）=显式值。
type ViewEdges struct {
	Top, Right, Bottom, Left *int
}

// ViewFill 背景填充。本切片仅 Color；Image/Gradient 留后续切片。
type ViewFill struct {
	Color string // ColorToken: "#RRGGBB[AA]" | "${semantic}" | "transparent"
}

type ViewBorder struct {
	Width  *int
	Color  string
	Radius *int
}

// ViewNode 一个具名 View 的外观属性（盒模型 + Text 属性）。
type ViewNode struct {
	Margin     ViewEdges
	Padding    ViewEdges
	Background ViewFill
	Border     ViewBorder
	FontFamily string
	FontSize   *int
	FontWeight *int
	Color      string   // 文本色 token
	Labels     []string // 仅 index：序号槽位字符（≤10）
	Selected   *ViewNode
	Hover      *ViewNode
}

// Views 具名 View 集合（固定骨架）。
type Views struct {
	Window        ViewNode
	PreeditBar    ViewNode
	CandidateList ViewNode
	Item          ViewNode
	Index         ViewNode
	Text          ViewNode
	Comment       ViewNode
	AccentBar     ViewNode
	FooterBar     ViewNode
}

// ResolvedViews 是 Views 经 merge 基线 + token 解析后的渲染消费形态（plain 值）。
// 本切片字段以"渲染所需的最小集"为准，随实现补全；标量用 0=回退基线的语义沿用 v2.5。
type ResolvedViews struct {
	Raw Views // 暂以合并后的 Views 承载；ui 侧按字段取值。后续切片再细化为纯 plain 结构。
}
```

> 设计取舍：`ResolvedViews` 本切片先以「merge 后的 `Views`」承载，ui 侧用取值 helper 读
> （`*int` nil 时回退 ui 内置默认）。避免一次性铺开完整 plain 结构，保持切片小。

- [ ] **Step 2: defaultViews() 基线 + 取值 helper**

```go
func intp(v int) *int { return &v }

// defaultViews 返回与现 viewbox_build magic number 等价的基线（density=compact）。
// 数值是"逻辑像素"（未乘 DPI scale）；ui 侧乘 scale。
func defaultViews() Views {
	return Views{
		// 占位：实现时对照 viewbox_build.go 逐字段填入（radius=4, item padding L/R=8,
		// index margin right=4, comment margin left=8, window padding=Padding 等）。
		// 由于本切片 ui 侧仍有 RenderConfig 兜底，基线缺省字段回退 ui 默认，可增量补全。
	}
}

// MergedOr 返回指针值或回退默认。
func edgeOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}
```

- [ ] **Step 3: 写基线单测（views_test.go）**

```go
package theme

import "testing"

func TestDefaultViews_Stable(t *testing.T) {
	v := defaultViews()
	// 基线应可重复构造且字段稳定（占位断言，随 Step 2 填充补具体值）
	_ = v
}

func TestEdgeOr(t *testing.T) {
	if edgeOr(nil, 8) != 8 {
		t.Error("nil 应回退默认")
	}
	if edgeOr(intp(0), 8) != 0 {
		t.Error("显式 0 应保留")
	}
}
```

- [ ] **Step 4: Run & Commit**

Run: `cd wind_input && go test ./pkg/theme/ -run "TestDefaultViews|TestEdgeOr" -v` → PASS
```bash
git add wind_input/pkg/theme/views.go wind_input/pkg/theme/views_test.go
git commit -m "feat(theme): views schema 类型 + 默认基线骨架（P2 切片-0）"
```

---

## Task 2：views merge + token 解析

**Files:** Modify `wind_input/pkg/theme/views.go`, `views_test.go`

- [ ] **Step 1: mergeViews（基线 ⊕ 覆盖）**

实现 `mergeViews(base Views, override Views) Views`：逐 ViewNode、逐字段——
`*int` 非 nil 覆盖；string 非空覆盖；`[]string` 非 nil 覆盖；`Selected`/`Hover` 子 patch 递归。
给出完整字段遍历代码（每个 ViewNode 字段一行 if）。

- [ ] **Step 2: resolveViewTokens（颜色 token → 字面色）**

复用现有 `refexpand.go` / palette derive，把 ViewNode 各 `Color`/`Background.Color`/`Border.Color`
的 `${name}` 对 palette 变体解析。产出 `ResolvedViews`。

- [ ] **Step 3: 单测**

```go
func TestMergeViews_PointerOverride(t *testing.T) {
	base := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(4)}}}
	ov := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(8)}}}
	got := mergeViews(base, ov)
	if got.Item.Border.Radius == nil || *got.Item.Border.Radius != 8 {
		t.Errorf("覆盖失败: %v", got.Item.Border.Radius)
	}
}

func TestMergeViews_NilKeepsBase(t *testing.T) {
	base := Views{Item: ViewNode{Border: ViewBorder{Radius: intp(4)}}}
	got := mergeViews(base, Views{})
	if got.Item.Border.Radius == nil || *got.Item.Border.Radius != 4 {
		t.Error("nil 覆盖应保留基线")
	}
}
```

- [ ] **Step 4: Run & Commit** → `feat(theme): views merge + token 解析`

---

## Task 3：ui 合成桥 RenderConfig → ResolvedViews

**Files:** Create `wind_input/internal/ui/viewbox_views_bridge.go`

- [ ] **Step 1: 写合成桥**

```go
package ui

import "github.com/huanfeng/wind_input/pkg/theme"
```

`renderConfigToViews(cfg RenderConfig) theme.ResolvedViews`：把现 build 用到的 cfg 外观字段
（Padding/ItemPadding*/IndexMarginRight/CommentMarginLeft/CornerRadius/各颜色/字号 等）填入
`Views`，再 `mergeViews(defaultViews(), ...)`。**字段映射对照 `viewbox_build.go:146-161` 的
取值**。目标：产出的 ResolvedViews 在 Task 4/5 喂给改造后的 build 后，几何指纹与 Task 0 基准一致。

- [ ] **Step 2: Renderer 持有 resolvedViews**

`renderer.go`：`Renderer` 加字段 `resolvedViews theme.ResolvedViews`；在 build 入口前
（`RenderCandidates` 或 config 应用处）置为 `renderConfigToViews(r.config)`（default 主题后续
Task 7 改为 YAML 来源）。

- [ ] **Step 3: 单测合成桥非空** → `cd wind_input && go test ./internal/ui/ -run Bridge -v`
- [ ] **Step 4: Commit** → `feat(ui): RenderConfig→ResolvedViews 合成桥`

---

## Task 4：改造横排 build 从 ResolvedViews 读

**Files:** Modify `wind_input/internal/ui/viewbox_build.go`

- [ ] **Step 1: 改造取值来源**

把 `buildHorizontalCandidateTree` 内对 `cfg.Padding`/`cfg.ItemPaddingLeft`/`cfg.IndexMarginRight`/
`cfg.CommentMarginLeft`/`cfg.CornerRadius`/`radius=4*scale`/`accent barW=3`/`itemSpacing=12/16` 等
**外观/magic number** 的引用，改为从 `r.resolvedViews` 取（经 Task 1 的 `edgeOr` 等 helper，乘 scale）。
**运行时数据**（candidates/input/cursor/page/selected/hover）与颜色仍按原样（颜色本切片可仍走 cfg，
token 解析在合成桥已等价）。逐字段对照原码替换，不改几何结果。

- [ ] **Step 2: 跑几何网保绿**

Run: `cd wind_input && go test ./internal/ui/ -run "TestGeometryFingerprint_Horizontal" -v`
Expected: PASS（指纹与基准一致）。不一致则逐字段核对映射。

- [ ] **Step 3: Commit** → `refactor(ui): 横排 build 外观取值改走 ResolvedViews`

---

## Task 5：改造竖排 build 从 ResolvedViews 读

**Files:** Modify `wind_input/internal/ui/viewbox_build_vertical.go`

- [ ] **Step 1: 同 Task 4 改造竖排**

对照 `viewbox_build_vertical.go` 的 magic number（含竖排特有：VerticalMaxWidth、每行全宽、
翻页底部居中、commentWidths 预量算），把外观取值改走 `r.resolvedViews`。

- [ ] **Step 2: 跑几何网保绿**

Run: `cd wind_input && go test ./internal/ui/ -run "TestGeometryFingerprint_Vertical" -v` → PASS

- [ ] **Step 3: Commit** → `refactor(ui): 竖排 build 外观取值改走 ResolvedViews`

---

## Task 6：config 全局序号覆盖 + 四层 resolve

**Files:** Modify `wind_input/pkg/config/config.go`, `wind_input/internal/ui/renderer_layout.go`（`indexLabel`）

- [ ] **Step 1: config 字段**

`UIConfig` 加 `CandidateIndexLabels string` yaml `candidate_index_labels,omitempty`，含注释说明
「全局序号标签覆盖，空=用主题」。

- [ ] **Step 2: 四层 resolve**

`indexLabel` 调用处（`viewbox_build.go:169`、`viewbox_build_vertical.go:135`）的标签来源改为：
`cand.IndexLabel(运行时) > cfg.CandidateIndexLabels(全局) > views.index.Labels(主题) > 默认`。
即在传入 `indexLabel` 的 `indexLabels` 实参处按优先级选取。`RenderConfig` 加 `GlobalIndexLabels string`
承接 config 值，在 config 应用处填充。

- [ ] **Step 3: 单测**

```go
// pkg/config
func TestUIConfig_CandidateIndexLabels(t *testing.T) {
	u := UIConfig{CandidateIndexLabels: "①②③④⑤⑥⑦⑧⑨⑩"}
	if u.CandidateIndexLabels == "" {
		t.Error("应保留全局序号覆盖")
	}
}
```

ui 侧加一个 resolve 单测：全局非空时覆盖主题 labels。

- [ ] **Step 4: 跑几何网（默认空覆盖应不变）+ Commit** → `feat(config): 全局序号标签覆盖 + 四层优先级`

---

## Task 7：default 主题接 YAML views 来源

**Files:** Modify `wind_input/pkg/theme/`（解析入口）、`internal/ui/renderer.go`、default 主题 YAML

- [ ] **Step 1: theme 解析 views**

在 `ResolveV25`/`resolver.go` 路径里解析 theme.yaml 的 `views:` 块（存在则）→ `mergeViews(defaultViews(), parsed)`
→ token 解析 → `theme.ResolvedViews`，挂到 `ResolvedV25`。无 `views:` 块时返回基线。

- [ ] **Step 2: default 主题 YAML**

给 `default` 主题 theme.yaml（或 `_views/default.yaml`）加 `views:` 块。因 default≈基线，
块可几乎为空（仅声明启用 views 路径）。定位 default 主题文件：
`data/themes/default/theme.yaml`（或 `_layouts`），实现时确认路径。

- [ ] **Step 3: ui 优先用主题 ResolvedViews**

`renderer.go` 填充 `r.resolvedViews` 时：主题提供了 views → 用 `resolved.Views`；否则用
`renderConfigToViews(cfg)` 合成桥。

- [ ] **Step 4: 验收**

- 几何网（横+竖）保绿（default 走 YAML 后仍对齐基准）。
- Run: `cd wind_input && go test ./internal/ui/ ./pkg/theme/ ./pkg/config/` → 全绿。

- [ ] **Step 5: Commit** → `feat(theme): default 主题接入 YAML views 来源（P2 切片-0 端到端）`

---

## Task 8：文档同步 + 收尾

**Files:** `pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`、spec 状态

- [ ] **Step 1:** `pkg/theme/AGENTS.md` 增 `views.go`/views schema/ResolvedViews 说明。
- [ ] **Step 2:** `internal/ui/AGENTS.md` 标注 build 从 ResolvedViews 读 + 合成桥（临时）。
- [ ] **Step 3:** `go fmt` 全部改动；`go vet ./internal/ui/ ./pkg/theme/`；`scripts/lint_agents_md.ps1`。
- [ ] **Step 4: Commit** → `docs(theme): 同步 views schema 接口说明（P2 切片-0）`

---

## Self-Review 记录

- **Spec 覆盖**：views 类型(T1)、merge/token(T2)、基线(T1)、合成桥+双来源(T3/T7)、build 改造(T4/T5)、
  序号四层+全局覆盖(T6)、几何对齐验收(T0+各 task)、Image/adapter 延后(非目标)。✓
- **几何对齐**贯穿 T0/T4/T5/T6/T7，作为零回归网。
- **占位说明**：T1 `defaultViews()` 与 T4/T5 的逐字段映射须对照现 build 实码填充（已给锚点行号），
  非占位符而是「对照源码搬迁」的实现动作；几何网保证正确性。
