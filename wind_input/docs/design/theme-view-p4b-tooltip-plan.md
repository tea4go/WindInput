# P4-B Tooltip View 化 + 颜色 YAML 化 实现计划

> **For agentic workers:** 用 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。

**Goal:** 把 Tooltip（候选悬停编码提示）渲染从 gg 直绘迁移到盒模型 View 引擎，颜色经 `views.tooltip` token 驱动，零回归；多行 / `\t` 列对齐 / 行截断 / 行数上限逻辑搬进 build。

**Architecture:** 新增 `buildTooltipTree`：预处理（splitLines→行数上限 20→列宽计算→截断）后建 `LayoutColumn`（行）+ `LayoutRow`（多列 cell，`FixedW` 列对齐，缺列空占位）View 树。颜色经 `resolveTokenColor` 从 `views.tooltip` 取，默认映射 `ResolvedTheme.Tooltip`。复用 P4-A 的 `newSharedDrawContext`。

**Tech Stack:** Go；`pkg/theme` views schema；复用 P4-A `resolveTokenColor`/`newSharedDrawContext`。

**Scope note（沿用 P4-A 收窄）:** 只 token 化颜色（background/text）。padding(6)/borderRadius(4)/fontSize(14)/lineSpacing(2)/colGap(16)/maxLines(20) 保持 hardcode（逻辑像素，build 内乘 scale）。`views.tooltip` 仅含 background/color 两 token。

**零回归基准:** `tooltip.go:render`（343-499）。颜色 `ResolvedTheme.Tooltip` / 默认 `{60,60,60,240}`/`{255,255,255,255}`。

---

## 文件结构
- **修改** `pkg/theme/views.go`：`Views` 加 `Tooltip *ViewNode`；新增 `ResolvedTooltipViews{BgColor, TextColor}`。
- **新建** `internal/ui/viewbox_tooltip.go`：`(*TooltipWindow).resolveTooltipColors`、`buildTooltipTree`。
- **修改** `internal/ui/tooltip.go`：`TooltipWindow` 加 `themeViews`；`SetTheme` 存它；`render` 改走引擎；删旧多列/单列 gg 直绘块。
- **新建** `internal/ui/viewbox_tooltip_test.go`：颜色 + 单列/多列/截断/行数上限几何指纹。
- **修改** `themes/default/theme.yaml`、`themes/msime/theme.yaml`：加 `views.tooltip`。
- **修改** `pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`。

---

## Task 1：theme 包加 Tooltip schema + ResolvedTooltipViews

**Files:** Modify `pkg/theme/views.go`；Test `pkg/theme/views_yaml_test.go`

- [ ] **Step 1: 失败测试**（追加 `views_yaml_test.go`）

```go
// TestViews_TooltipParse 验证 views.tooltip 解析到 Views.Tooltip。
func TestViews_TooltipParse(t *testing.T) {
	var v Views
	if err := yaml.Unmarshal([]byte("tooltip:\n  background: {color: \"${background}\"}\n  color: \"${text}\"\n"), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Tooltip == nil || v.Tooltip.Background.Color != "${background}" || v.Tooltip.Color != "${text}" {
		t.Fatalf("tooltip token 解析错误: %+v", v.Tooltip)
	}
}
```

- [ ] **Step 2: 验证失败** — `go test ./pkg/theme/ -run TestViews_TooltipParse`（`v.Tooltip undefined`）

- [ ] **Step 3: 加字段**（`Views` 结构，`Status` 之后）

```go
	Status        *ViewNode `yaml:"status,omitempty"`  // P4-A 状态泡（独立窗口，单节点）
	Tooltip       *ViewNode `yaml:"tooltip,omitempty"` // P4-B Tooltip（独立窗口，单节点）
```

- [ ] **Step 4: 加 ResolvedTooltipViews**（`views.go`，`ResolvedStatusViews` 之后）

```go
// ResolvedTooltipViews Tooltip 解析后外观（P4-B）。仅颜色——几何由 render 内置默认（hardcode）。
type ResolvedTooltipViews struct {
	BgColor   color.Color
	TextColor color.Color
}
```

- [ ] **Step 5: 验证通过** — `go test ./pkg/theme/ -run TestViews_TooltipParse -v`（PASS）
- [ ] **Step 6:** `gofmt -w pkg/theme/views.go pkg/theme/views_yaml_test.go`

---

## Task 2：viewbox_tooltip.go（颜色解析 + buildTooltipTree）

**Files:** Create `internal/ui/viewbox_tooltip.go`、`internal/ui/viewbox_tooltip_test.go`

- [ ] **Step 1: 失败测试**（新建 `viewbox_tooltip_test.go`）

```go
package ui

import (
	"image/color"
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// TestResolveTooltipColors token 映射 ResolvedTheme.Tooltip；无 views 用默认。
func TestResolveTooltipColors(t *testing.T) {
	tt := theme.ResolvedTooltipColors{
		BackgroundColor: color.RGBA{11, 22, 33, 255},
		TextColor:       color.RGBA{210, 210, 210, 255},
	}
	rt := &theme.ResolvedTheme{Tooltip: tt}
	views := &theme.Views{Tooltip: &theme.ViewNode{
		Background: theme.ViewFill{Color: "${background}"}, Color: "${text}",
	}}
	w := &TooltipWindow{resolvedTheme: rt, themeViews: views}
	rtv := w.resolveTooltipColors()
	if rtv.BgColor != tt.BackgroundColor || rtv.TextColor != tt.TextColor {
		t.Fatalf("tooltip 颜色应映射 ResolvedTheme.Tooltip, got %+v", rtv)
	}
}

// TestBuildTooltipTree_SingleCol 单列多行：列布局，行高/总尺寸指纹。
func TestBuildTooltipTree_SingleCol(t *testing.T) {
	m := fixedMeasurer{charW: 10}
	rtv := theme.ResolvedTooltipViews{BgColor: color.RGBA{1, 1, 1, 255}, TextColor: color.RGBA{2, 2, 2, 255}}
	root := buildTooltipTree("ab\ncde", 0, rtv, 1.0, m)
	Layout(root, 0, 0, m)
	r := root.Rect()
	// 行宽: "ab"=20, "cde"=30 → max 30; +padding 6*2 = 42
	if r.Dx() != 42 {
		t.Errorf("width 应 42, got %d", r.Dx())
	}
	// 高: fontSize 14*2 + lineSpacing 2*(2-1) + padding 12 = 28+2+12 = 42
	if r.Dy() != 42 {
		t.Errorf("height 应 42, got %d", r.Dy())
	}
	if len(root.Children) != 2 || root.Layout != LayoutColumn {
		t.Errorf("应为 2 行的列布局, got layout=%d children=%d", root.Layout, len(root.Children))
	}
	if root.Background.Color != (color.RGBA{1, 1, 1, 255}) {
		t.Error("bg 颜色指纹不符")
	}
}

// TestBuildTooltipTree_MultiCol 多列：每行 LayoutRow，列宽对齐 + 缺列空占位。
func TestBuildTooltipTree_MultiCol(t *testing.T) {
	m := fixedMeasurer{charW: 10}
	rtv := theme.ResolvedTooltipViews{BgColor: color.RGBA{1, 1, 1, 255}, TextColor: color.RGBA{2, 2, 2, 255}}
	// 行1 两列 "a\tbb"，行2 一列 "ccc"（缺第2列）
	root := buildTooltipTree("a\tbb\nccc", 0, rtv, 1.0, m)
	Layout(root, 0, 0, m)
	if len(root.Children) != 2 {
		t.Fatalf("应 2 行, got %d", len(root.Children))
	}
	row0 := root.Children[0]
	if row0.Layout != LayoutRow || len(row0.Children) != 2 {
		t.Fatalf("行0 应为 2 列的 Row, got layout=%d cells=%d", row0.Layout, len(row0.Children))
	}
	// 列0 宽 = max("a"=10, "ccc"=30) = 30
	if row0.Children[0].FixedW != 30 {
		t.Errorf("列0 宽应 30（列对齐取最大）, got %d", row0.Children[0].FixedW)
	}
	row1 := root.Children[1]
	if len(row1.Children) != 2 || row1.Children[1].Text != "" {
		t.Errorf("行1 缺第2列应补空占位 cell")
	}
}
```

- [ ] **Step 2: 验证失败** — `go test ./internal/ui/ -run 'TestResolveTooltipColors|TestBuildTooltipTree'`（未定义）

- [ ] **Step 3: 实现**（新建 `viewbox_tooltip.go`）

```go
package ui

// viewbox_tooltip.go — Tooltip（候选编码提示）的 View 树构建与颜色解析（P4-B）。
// 复用包级 Layout/PaintTree + newSharedDrawContext；颜色经 token 解析自 views.tooltip，
// 默认映射 ResolvedTheme.Tooltip。多行 / \t 列对齐 / 行截断 / 行数上限逻辑在 build 内预处理。

import (
	"image/color"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// resolveTooltipColors 计算 Tooltip 颜色：views.tooltip token > ResolvedTheme.Tooltip > 默认。
func (w *TooltipWindow) resolveTooltipColors() theme.ResolvedTooltipViews {
	bg := color.Color(color.RGBA{60, 60, 60, 240})
	text := color.Color(color.RGBA{255, 255, 255, 255})
	if w.resolvedTheme != nil {
		bg = w.resolvedTheme.Tooltip.BackgroundColor
		text = w.resolvedTheme.Tooltip.TextColor
	}
	if w.themeViews != nil && w.themeViews.Tooltip != nil {
		res := func(name string) color.Color {
			if w.resolvedTheme == nil {
				return nil
			}
			switch name {
			case "background":
				return w.resolvedTheme.Tooltip.BackgroundColor
			case "text":
				return w.resolvedTheme.Tooltip.TextColor
			}
			return nil
		}
		if c := resolveTokenColor(w.themeViews.Tooltip.Background.Color, res); c != nil {
			bg = c
		}
		if c := resolveTokenColor(w.themeViews.Tooltip.Color, res); c != nil {
			text = c
		}
	}
	return theme.ResolvedTooltipViews{BgColor: bg, TextColor: text}
}

// buildTooltipTree 构建 Tooltip View 树（LayoutColumn 行 + 多列 LayoutRow cell）。
// 几何为 hardcode 逻辑像素 × scale。maxContentWidth<=0 不限宽。无行返回 nil。
func buildTooltipTree(text string, maxContentWidth float64, rtv theme.ResolvedTooltipViews, scale float64, m TextMeasurer) *View {
	fontSize := 14.0 * scale
	padding := int(6.0 * scale)
	lineSpacing := int(2.0 * scale)
	colGap := int(16.0 * scale)
	radius := int(4.0 * scale)
	const maxLines = 20

	lines := splitLines(text)
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > maxLines {
		hidden := len(lines) - (maxLines - 1)
		kept := append([]string{}, lines[:maxLines-1]...)
		lines = append(kept, "… (+"+itoaCompact(hidden)+")")
	}

	innerMax := maxContentWidth - float64(padding*2)

	// 拆列，求最大列数
	rows := make([][]string, len(lines))
	numCols := 1
	for i, line := range lines {
		cells := splitTabs(line)
		rows[i] = cells
		if len(cells) > numCols {
			numCols = len(cells)
		}
	}

	mkText := func(s string) *View {
		return &View{Text: s, TextStyle: TextStyle{FontSize: fontSize, Color: rtv.TextColor}}
	}

	root := &View{
		Layout:     LayoutColumn,
		Gap:        lineSpacing,
		Padding:    Edges{Top: padding, Right: padding, Bottom: padding, Left: padding},
		Background: Fill{Color: rtv.BgColor},
		Border:     Border{Radius: radius},
	}

	if numCols == 1 {
		for _, line := range lines {
			if innerMax > 0 && m.MeasureString(line, fontSize) > innerMax {
				line = truncateLineToWidth(textMeasurerDrawer{m}, line, fontSize, innerMax)
			}
			root.Children = append(root.Children, mkText(line))
		}
		return root
	}

	// 多列：列宽 = 每列最大；总宽超 innerMax 则截断最后一列
	colWidth := make([]float64, numCols)
	for _, cells := range rows {
		for k := 0; k < numCols && k < len(cells); k++ {
			if lw := m.MeasureString(cells[k], fontSize); lw > colWidth[k] {
				colWidth[k] = lw
			}
		}
	}
	if innerMax > 0 {
		var fixed float64
		for k := 0; k < numCols-1; k++ {
			fixed += colWidth[k]
		}
		fixed += float64(numCols-1) * float64(colGap)
		lastBudget := innerMax - fixed
		if lastBudget < 0 {
			lastBudget = 0
		}
		if colWidth[numCols-1] > lastBudget {
			colWidth[numCols-1] = 0
			for i, cells := range rows {
				if len(cells) < numCols {
					continue
				}
				if m.MeasureString(cells[numCols-1], fontSize) > lastBudget {
					rows[i][numCols-1] = truncateLineToWidth(textMeasurerDrawer{m}, cells[numCols-1], fontSize, lastBudget)
				}
				if lw := m.MeasureString(rows[i][numCols-1], fontSize); lw > colWidth[numCols-1] {
					colWidth[numCols-1] = lw
				}
			}
		}
	}

	for _, cells := range rows {
		rowView := &View{Layout: LayoutRow, Gap: colGap}
		for k := 0; k < numCols; k++ {
			cell := &View{FixedW: int(colWidth[k] + 0.5)}
			if k < len(cells) {
				cell.Text = cells[k]
				cell.TextStyle = TextStyle{FontSize: fontSize, Color: rtv.TextColor}
			}
			rowView.Children = append(rowView.Children, cell)
		}
		root.Children = append(root.Children, rowView)
	}
	return root
}
```

- [ ] **Step 4: 加 `splitTabs` + `textMeasurerDrawer` 适配器**（`viewbox_tooltip.go`）

```go
// splitTabs 按 \t 拆列（薄封装，单列即单元素）。
func splitTabs(line string) []string { return splitOnTab(line) }
```

> 实现注记：`truncateLineToWidth` 形参是 `TextDrawer`，但 build 只需 `TextMeasurer`。为可单测（注入 `fixedMeasurer`），加一个把 `TextMeasurer` 适配成 `TextDrawer` 的薄包装 `textMeasurerDrawer`，仅 `MeasureString` 有效、其余 draw 方法空实现。若 `truncateLineToWidth` 实测只调用 `MeasureString`，此适配安全。Step 3 实现前先 `Read truncateLineToWidth`（tooltip.go:526）确认它只用度量；若它还调用绘制方法，则改为新增一个 `truncateToWidthM(m TextMeasurer, ...)` 纯度量版本并让 `truncateLineToWidth` 委托它（DRY）。`splitOnTab` 用 `strings.Split(line, "\t")`。

- [ ] **Step 5: 验证通过** — `go test ./internal/ui/ -run 'TestResolveTooltipColors|TestBuildTooltipTree' -v`（PASS）
- [ ] **Step 6:** `gofmt -w internal/ui/viewbox_tooltip.go internal/ui/viewbox_tooltip_test.go`

---

## Task 3：tooltip.go render 改造

**Files:** Modify `internal/ui/tooltip.go`

- [ ] **Step 1: `TooltipWindow` 加 `themeViews`**（struct，`resolvedTheme` 之后）

```go
	resolvedTheme *theme.ResolvedTheme
	themeViews    *theme.Views
```

- [ ] **Step 2: `SetTheme` 存 themeViews**（tooltip.go:117-121）

```go
func (w *TooltipWindow) SetTheme(resolved *theme.ResolvedTheme) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.resolvedTheme = resolved
	if resolved != nil {
		w.themeViews = resolved.Views
	} else {
		w.themeViews = nil
	}
}
```

- [ ] **Step 3: 替换整个 `render`**（tooltip.go:346-499 → 下方）

```go
// render 将 tooltip 文本渲染到图像（盒模型 View 引擎，支持 \n 换行 + \t 列对齐）。
// maxContentWidth 为可用内容区最大像素宽度（不含 padding）；<=0 表示不限制。
func (w *TooltipWindow) render(text string, maxContentWidth float64) *image.RGBA {
	scale := GetDPIScale()
	w.mu.Lock()
	td := w.TextDrawer()
	rtv := w.resolveTooltipColors()
	w.mu.Unlock()

	root := buildTooltipTree(text, maxContentWidth, rtv, scale, td)
	if root == nil {
		return nil
	}
	Layout(root, 0, 0, td)
	dc, img := newSharedDrawContext(root.Rect().Dx(), root.Rect().Dy())
	PaintTree(root, dc, img, td)
	DrawDebugBanner(img)
	return img
}
```

- [ ] **Step 4: 清理无用 import**（编译报错指引）

删 `render` 后若 `gg` 不再被 tooltip.go 其它函数使用则移除其 import；`getTooltipColors` 若不再被引用则删除（逻辑已入 `resolveTooltipColors`）。`splitLines`/`truncateLineToWidth`/`itoaCompact` 仍被 build 使用，保留。

- [ ] **Step 5: 编译 + 测试** — `go build ./... ; go test ./internal/ui/ ./pkg/theme/`
Expected: build ✓；新测试 PASS；候选窗既有测试不破；pkg/theme `TestBuiltin*` 4 个 pre-existing fail（worktree 缺 build/data/themes）无关。

- [ ] **Step 6:** `gofmt -w internal/ui/tooltip.go`

---

## Task 4：种子主题 + AGENTS.md + 全量验证

**Files:** `themes/default/theme.yaml`、`themes/msime/theme.yaml`、`pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`

- [ ] **Step 1: 两主题加 `views.tooltip`**（`views.status` 之后）

```yaml
  tooltip:
    background: {color: "${background}"}
    color: "${text}"
```

- [ ] **Step 2: 主题解析测试**（追加 `pkg/theme/views_yaml_test.go`）

```go
// TestDefaultThemeTooltipParse 验证 default theme.yaml 的 views.tooltip。
func TestDefaultThemeTooltipParse(t *testing.T) {
	data, err := os.ReadFile("../../themes/default/theme.yaml")
	if err != nil {
		t.Skip("default theme.yaml 不可读: " + err.Error())
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if th.Views == nil || th.Views.Tooltip == nil || th.Views.Tooltip.Background.Color != "${background}" {
		t.Fatal("default theme.yaml 应含 views.tooltip 且 background token")
	}
}
```

- [ ] **Step 3: AGENTS.md 同步**
  - `pkg/theme/AGENTS.md`：P4-A 段补 `Tooltip *ViewNode` + `ResolvedTooltipViews`。
  - `internal/ui/AGENTS.md`：加 `viewbox_tooltip.go` 行（`buildTooltipTree` 多列对齐 + `resolveTooltipColors`），并注明 `tooltip.go:render` 改走引擎。

- [ ] **Step 4: 全量验证** — `go build ./... ; gofmt -l internal/ui/viewbox_tooltip.go internal/ui/tooltip.go pkg/theme/views.go ; go test ./internal/ui/ ./pkg/theme/`
Expected: build ✓；`gofmt -l` 上述文件无输出；测试 PASS（除 pre-existing）。

- [ ] **Step 5: lint** — `pwsh -File scripts/lint_agents_md.ps1`（无 mdlink 真错）

- [ ] **Step 6: 运行时人工核对（交用户）** — `dev.ps1 d1` 后悬停候选触发编码 tooltip（含多行拆字/拼音列对齐场景），确认背景/文字/列对齐/截断与迁移前一致。

---

## 验收清单
- [ ] Tooltip 渲染走 `Layout`/`PaintTree` + `newSharedDrawContext`，旧多列/单列 gg 直绘块删除。
- [ ] 单列截断 / 多列对齐 / 缺列占位 / 行数上限行为保持（几何指纹验证）。
- [ ] default/msime 含 `views.tooltip`，token 映射 `ResolvedTheme.Tooltip`（零回归）。
- [ ] build ✓ / 改动文件 gofmt 干净 / 回归网 PASS / AGENTS.md lint 通过。
- [ ] 运行时人工核对（交用户）。
