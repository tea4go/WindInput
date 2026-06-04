<!-- Parent: theme-view-p6-bridge-retirement.md -->

# P6 阶段 2b：theme 包 ResolveCandidateViews（颜色+几何下沉）— 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`).

**Goal:** 在 theme 包新增 `ResolveCandidateViews(views, palette) → ResolvedViews`，把候选窗外观（几何 from ViewNode/Metrics、颜色 = palette 默认 ⊕ views token 覆盖）解析为渲染消费形态——但**不含字号/ItemHeight**（运行时值，由 ui 回填）。把 ui 侧 `resolveViewColor` 的 token 映射逻辑下沉到 theme 包。

**Architecture（关键）:** 颜色采用 **palette 默认填 + views token 可选覆盖**：每个 RVNode 颜色先填 palette 对应语义色（无 views 也有色），再用 views 的 `${token}`/hex（若非空）覆盖——等价于合成桥 `buildResolvedViews`(palette 默认) + `applyThemeViews`(token 覆盖) 两步的合并。**本阶段不改 ui、不启用 ResolveV25 merge**（那有渲染影响，移 2c）；`ResolveCandidateViews` 本阶段只被单测调用。

**Tech Stack:** Go、`image/color`。worktree：`d:\Develop\workspace\go_dev\WindInput\.worktrees\theme-v25`，go 命令在 `wind_input` 下，`$env:CGO_ENABLED=1`。

---

## 关键映射表（实现依据，来自合成桥 buildResolvedViews + SetTheme colors.* 提炼）

`cw := palette.CandidateWindow`。每个 RVNode 的**颜色默认值**：

| View | BgColor 默认 | BorderColor 默认 | TextColor 默认 |
|---|---|---|---|
| Window | `cw.Background` | `cw.Border` | — |
| PreeditBar | `cw.PreeditBg` | — | `cw.PreeditText` |
| CandidateList | — | — | — |
| Item | — | — | — |
| Index | `cw.IndexBg` | — | `cw.IndexText` |
| Text | — | — | `cw.Text` |
| Comment | — | — | `cw.Comment` |
| AccentBar | `cw.AccentBar` | — | — |
| FooterBar | — | — | — |

- `Item.SelectedBg` 默认 `cw.SelectedBg`、`Item.HoverBg` 默认 `cw.HoverBg`（来自 states，单独处理）。
- `ResolvedViews.ShadowColor = palette.Shadow`。
- token 映射（`${name}`）：`background→cw.Background / border→cw.Border / text→cw.Text / index_bg→cw.IndexBg / index_text→cw.IndexText / hover_bg→cw.HoverBg / selected_bg→cw.SelectedBg / preedit_bg→cw.PreeditBg / preedit_text→cw.PreeditText / comment→cw.Comment / accent→cw.AccentBar / shadow→palette.Shadow`。
- 几何（逻辑像素，不乘 scale；ui build 端乘）：RVNode margin/padding/border from ViewNode（`edgeOr(ptr, 0)`）；顶层 `ItemSpacing/WindowGap(=BandGap)/ShadowOffset/AccentBarWidth/AccentBarOffset/AccentBarHRatio` from `views.Metrics`。
- **不设**（留 ui 运行时回填，保持零值）：`Text/Index/PreeditBar.FontSize`、`ItemHeight`、`VerticalMaxWidth`（后者归 Behavior）。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `wind_input/pkg/theme/candidate_views.go` | `resolveCandidateViewColor` + `ResolveCandidateViews` | 新建 |
| `wind_input/pkg/theme/candidate_views_test.go` | token 解析 + 几何/颜色映射单测 | 新建 |

---

## Task 1: resolveCandidateViewColor（token 解析下沉）

**Files:** Create `wind_input/pkg/theme/candidate_views.go` + `candidate_views_test.go`

- [ ] **Step 1: 写失败测试** —— `candidate_views_test.go`：

```go
package theme

import (
	"image/color"
	"testing"
)

func testPalette() ResolvedPalette {
	return ResolvedPalette{
		Shadow: color.RGBA{1, 1, 1, 15},
		CandidateWindow: ResolvedCandidateWindowPalette{
			Background: color.RGBA{255, 255, 255, 255},
			Border:     color.RGBA{200, 200, 200, 255},
			Text:       color.RGBA{30, 30, 30, 255},
			Comment:    color.RGBA{150, 150, 150, 255},
			IndexBg:    color.RGBA{66, 133, 244, 255},
			IndexText:  color.RGBA{255, 255, 255, 255},
			HoverBg:    color.RGBA{230, 240, 255, 255},
			SelectedBg: color.RGBA{210, 228, 255, 255},
			PreeditBg:  color.RGBA{240, 240, 240, 255},
			PreeditText: color.RGBA{100, 100, 100, 255},
			AccentBar:  color.RGBA{0, 120, 212, 255},
		},
	}
}

func TestResolveCandidateViewColor(t *testing.T) {
	pal := testPalette()
	cases := map[string]color.Color{
		"${background}":   pal.CandidateWindow.Background,
		"${index_bg}":     pal.CandidateWindow.IndexBg,
		"${selected_bg}":  pal.CandidateWindow.SelectedBg,
		"${comment}":      pal.CandidateWindow.Comment,
		"${accent}":       pal.CandidateWindow.AccentBar,
		"${shadow}":       pal.Shadow,
	}
	for tok, want := range cases {
		if got := resolveCandidateViewColor(tok, pal); got != want {
			t.Errorf("%s → got %v, want %v", tok, got, want)
		}
	}
	// hex 直解
	if got := resolveCandidateViewColor("#FF0000", pal); got == nil {
		t.Error("hex 应解析为非 nil")
	}
	// 空 / 未知 token → nil
	if resolveCandidateViewColor("", pal) != nil {
		t.Error("空串应为 nil")
	}
	if resolveCandidateViewColor("${unknown}", pal) != nil {
		t.Error("未知 token 应为 nil")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**：`$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestResolveCandidateViewColor' -v` → 编译失败 `undefined: resolveCandidateViewColor`。

- [ ] **Step 3: 写实现** —— `candidate_views.go`（先写 token 解析；ResolveCandidateViews 在 Task 2 加）：

```go
package theme

import (
	"image/color"
	"strings"
)

// resolveCandidateViewColor 解析候选窗 views 颜色字段：${name}→palette 候选窗语义色 /
// hex(#RRGGBB[AA]) 直解 / 空或未知 token → nil（调用方据此保留 palette 默认）。
// 从 ui 侧 resolveViewColor 下沉（P6 2b），token 映射表与之一致。
func resolveCandidateViewColor(s string, pal ResolvedPalette) color.Color {
	if s == "" {
		return nil
	}
	cw := pal.CandidateWindow
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		switch s[2 : len(s)-1] {
		case "background":
			return cw.Background
		case "border":
			return cw.Border
		case "text":
			return cw.Text
		case "index_bg":
			return cw.IndexBg
		case "index_text":
			return cw.IndexText
		case "hover_bg":
			return cw.HoverBg
		case "selected_bg":
			return cw.SelectedBg
		case "preedit_bg":
			return cw.PreeditBg
		case "preedit_text":
			return cw.PreeditText
		case "comment":
			return cw.Comment
		case "accent":
			return cw.AccentBar
		case "shadow":
			return pal.Shadow
		}
		return nil
	}
	if c, err := ParseHexColor(s); err == nil {
		return c
	}
	return nil
}
```

> 注：先 grep 确认 `ParseHexColor` 在 theme 包的确切签名（`colors.go`，应为 `func ParseHexColor(s string) (color.Color, error)`）。若签名不同，按实际调整。

- [ ] **Step 4: 跑测试确认通过**：`$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestResolveCandidateViewColor' -v` → PASS。

- [ ] **Step 5: gofmt + commit**：
```
gofmt -w pkg/theme/candidate_views.go pkg/theme/candidate_views_test.go
git add wind_input/pkg/theme/candidate_views.go wind_input/pkg/theme/candidate_views_test.go
git commit -m "feat(theme): P6 阶段2b resolveCandidateViewColor token 解析下沉"
```

---

## Task 2: ResolveCandidateViews（几何+颜色 → ResolvedViews）

**Files:** Modify `candidate_views.go`；Test 追加到 `candidate_views_test.go`

- [ ] **Step 1: 写失败测试**（追加）：

```go
func TestResolveCandidateViews_GeometryAndColor(t *testing.T) {
	pal := testPalette()
	// 用 defaultViews 基线（含 Metrics）+ 给 Window 写颜色 token，验证默认+覆盖
	v := defaultViews()
	v.Window.Background = ViewFill{Color: "${background}"}
	rv := ResolveCandidateViews(v, pal)

	// 几何：Window padding 基线 8（defaultViews）
	if rv.Window.PadLeft != 8 || rv.Window.PadTop != 8 {
		t.Errorf("Window padding 应为 8, got L=%d T=%d", rv.Window.PadLeft, rv.Window.PadTop)
	}
	// 几何：Item border radius 基线 4
	if rv.Item.BorderRadius != 4 {
		t.Errorf("Item radius 应为 4, got %d", rv.Item.BorderRadius)
	}
	// Metrics → 顶层
	if rv.ItemSpacing != 12 || rv.WindowGap != 4 || rv.ShadowOffset != 2 {
		t.Errorf("metrics 顶层错: spacing=%d gap=%d shadow=%d", rv.ItemSpacing, rv.WindowGap, rv.ShadowOffset)
	}
	if rv.AccentBarWidth != 3 || rv.AccentBarOffset != 1 || rv.AccentBarHRatio != 0.6 {
		t.Errorf("accent metrics 错: w=%d off=%d hr=%v", rv.AccentBarWidth, rv.AccentBarOffset, rv.AccentBarHRatio)
	}
	// 颜色默认：Index.BgColor = palette.IndexBg（views 未写 index 颜色 token）
	if rv.Index.BgColor != pal.CandidateWindow.IndexBg {
		t.Errorf("Index.BgColor 默认应=palette.IndexBg, got %v", rv.Index.BgColor)
	}
	// 颜色默认：Text.TextColor = palette.Text
	if rv.Text.TextColor != pal.CandidateWindow.Text {
		t.Errorf("Text.TextColor 默认应=palette.Text, got %v", rv.Text.TextColor)
	}
	// Item selected/hover 默认
	if rv.Item.SelectedBg != pal.CandidateWindow.SelectedBg || rv.Item.HoverBg != pal.CandidateWindow.HoverBg {
		t.Errorf("Item selected/hover 默认错")
	}
	// 颜色覆盖：Window.Background token=${background} → palette.Background
	if rv.Window.BgColor != pal.CandidateWindow.Background {
		t.Errorf("Window.BgColor token 覆盖错, got %v", rv.Window.BgColor)
	}
	// ShadowColor
	if rv.ShadowColor != pal.Shadow {
		t.Errorf("ShadowColor 应=palette.Shadow, got %v", rv.ShadowColor)
	}
	// 字号/ItemHeight/VerticalMaxWidth 不设（运行时回填）：应为零值
	if rv.Text.FontSize != 0 || rv.ItemHeight != 0 || rv.VerticalMaxWidth != 0 {
		t.Errorf("字号/行高/竖排max 本层应为零值（ui 回填）, got fs=%v ih=%v vmax=%v", rv.Text.FontSize, rv.ItemHeight, rv.VerticalMaxWidth)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**：`$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestResolveCandidateViews_GeometryAndColor' -v` → 编译失败 `undefined: ResolveCandidateViews`。

- [ ] **Step 3: 写实现** —— 在 `candidate_views.go` 追加：

```go
// ResolveCandidateViews 把候选窗 Views（已 merge defaultViews 基线，含 Metrics）+ palette
// 解析为渲染消费的 ResolvedViews（几何=逻辑像素、颜色=color.Color）。
// 颜色 = palette 默认 ⊕ views token 覆盖（views 颜色非空才覆盖）。
// 不设字号（Text/Index/PreeditBar.FontSize）、ItemHeight、VerticalMaxWidth——这些是运行时值，由 ui 回填。
func ResolveCandidateViews(views Views, pal ResolvedPalette) ResolvedViews {
	// build 把一个 ViewNode 解析为 RVNode：几何 from node，颜色默认 defBg/defBorder/defText，
	// 若 node 的颜色 token 非空则覆盖。
	build := func(n ViewNode, defBg, defBorder, defText color.Color) RVNode {
		out := RVNode{
			MarginTop:    edgeOr(n.Margin.Top, 0),
			MarginRight:  edgeOr(n.Margin.Right, 0),
			MarginBottom: edgeOr(n.Margin.Bottom, 0),
			MarginLeft:   edgeOr(n.Margin.Left, 0),
			PadTop:       edgeOr(n.Padding.Top, 0),
			PadRight:     edgeOr(n.Padding.Right, 0),
			PadBottom:    edgeOr(n.Padding.Bottom, 0),
			PadLeft:      edgeOr(n.Padding.Left, 0),
			BorderRadius: edgeOr(n.Border.Radius, 0),
			BorderWidth:  edgeOr(n.Border.Width, 0),
			BgColor:      defBg,
			BorderColor:  defBorder,
			TextColor:    defText,
		}
		if c := resolveCandidateViewColor(n.Background.Color, pal); c != nil {
			out.BgColor = c
		}
		if c := resolveCandidateViewColor(n.Border.Color, pal); c != nil {
			out.BorderColor = c
		}
		if c := resolveCandidateViewColor(n.Color, pal); c != nil {
			out.TextColor = c
		}
		return out
	}
	cw := pal.CandidateWindow
	rv := ResolvedViews{
		Window:        build(views.Window, cw.Background, cw.Border, nil),
		PreeditBar:    build(views.PreeditBar, cw.PreeditBg, nil, cw.PreeditText),
		CandidateList: build(views.CandidateList, nil, nil, nil),
		Item:          build(views.Item, nil, nil, nil),
		Index:         build(views.Index, cw.IndexBg, nil, cw.IndexText),
		Text:          build(views.Text, nil, nil, cw.Text),
		Comment:       build(views.Comment, nil, nil, cw.Comment),
		AccentBar:     build(views.AccentBar, cw.AccentBar, nil, nil),
		FooterBar:     build(views.FooterBar, nil, nil, nil),
		ShadowColor:   pal.Shadow,
	}
	// Item selected/hover：默认 palette，states token 覆盖。
	rv.Item.SelectedBg = cw.SelectedBg
	rv.Item.HoverBg = cw.HoverBg
	if views.Item.Selected != nil {
		if c := resolveCandidateViewColor(views.Item.Selected.Background.Color, pal); c != nil {
			rv.Item.SelectedBg = c
		}
	}
	if views.Item.Hover != nil {
		if c := resolveCandidateViewColor(views.Item.Hover.Background.Color, pal); c != nil {
			rv.Item.HoverBg = c
		}
	}
	// Metrics → 顶层几何。
	if m := views.Metrics; m != nil {
		rv.ItemSpacing = edgeOr(m.ItemSpacing, 0)
		rv.WindowGap = edgeOr(m.BandGap, 0)
		rv.ShadowOffset = edgeOr(m.ShadowOffset, 0)
		if m.AccentBar != nil {
			rv.AccentBarWidth = edgeOr(m.AccentBar.Width, 0)
			rv.AccentBarOffset = edgeOr(m.AccentBar.Offset, 0)
			if m.AccentBar.HeightRatio != nil {
				rv.AccentBarHRatio = *m.AccentBar.HeightRatio
			}
		}
	}
	return rv
}
```

- [ ] **Step 4: 跑测试确认通过**：`$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestResolveCandidateViews' -v` → PASS（token + 几何颜色两测试）。

- [ ] **Step 5: 全包测试 + gofmt + commit**：
```
$env:CGO_ENABLED=1; go test ./pkg/theme/ 2>&1 | Select-String -NotMatch 'TestBuiltin|TestListAvailable'
gofmt -w pkg/theme/candidate_views.go pkg/theme/candidate_views_test.go
git add wind_input/pkg/theme/candidate_views.go wind_input/pkg/theme/candidate_views_test.go
git commit -m "feat(theme): P6 阶段2b ResolveCandidateViews 几何+颜色解析"
```
Expected: 除 4 个 pre-existing 失败外全绿。

---

## 完成判据
- [ ] `resolveCandidateViewColor` token/hex/nil 解析正确，单测过。
- [ ] `ResolveCandidateViews` 几何 from ViewNode/Metrics、颜色 palette 默认⊕token 覆盖、字号/ItemHeight/VerticalMaxWidth 留零值，单测过。
- [ ] 纯 theme 包新增，**未改 ResolveV25 / ui / 合成桥**，零渲染影响。

## 非目标（留后续）
- ❌ 启用 `ResolveV25` defaultViews merge（移 2c，与 ui 切换一起，几何指纹守护）。
- ❌ ui 调用 `ResolveCandidateViews` 替代合成桥 + 字号回填（2c）。
- ❌ 删合成桥（2e）。

## 关联
- `theme-view-p6-bridge-retirement.md` — P6 总设计
- 映射表来自合成桥 `viewbox_views_bridge.go` buildResolvedViews + `renderer.go` SetTheme colors.* 提炼（见记忆 P6 段阶段2 探查结论）
