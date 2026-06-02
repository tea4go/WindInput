<!-- Parent: theme-view-p6-bridge-retirement.md -->

# P6 阶段 2a：修 mergeViews 透传独立窗口字段 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`).

**Goal:** 修复 `mergeViews` 丢弃 `Status`/`Tooltip`/`Toolbar`/`Menu` 字段的缺陷，为阶段 2b 在 `ResolveV25` 启用 `defaultViews` merge 扫除地雷。纯防护、零运行时行为变更（`ResolveV25` 本阶段仍 `rv.Views = t.Views` 原样透传，不调用 mergeViews）。

**Architecture:** `mergeViews(base, ov Views) Views` 当前只 merge 9 个候选窗 ViewNode + Metrics（共 10 字段），完全不带 `Status`/`Tooltip`/`Toolbar`/`Menu`（4 个独立窗口的指针字段）。一旦 2b 把 `ResolveV25` 改成 `mergeViews(defaultViews(), *t.Views)`，这 4 个窗口的主题配色会被静默丢弃且现有测试捕获不到。本阶段让 mergeViews 整体透传这 4 个指针（ov 非 nil 取 ov，否则 base 兜底）。

**Tech Stack:** Go。worktree：`d:\Develop\workspace\go_dev\WindInput\.worktrees\theme-v25`，源码在 `wind_input\` 下，go 命令在 `wind_input` 目录执行，`$env:CGO_ENABLED=1`。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `wind_input/pkg/theme/views.go` | `mergeViews`（约 :283-295）末尾透传 4 个独立窗口指针 | 修改 |
| `wind_input/pkg/theme/views_metrics_test.go` | 追加 mergeViews 透传回归测试 | 修改 |

---

## Task 1: mergeViews 透传 Status/Tooltip/Toolbar/Menu

**Files:**
- Modify: `wind_input/pkg/theme/views.go`（`mergeViews`）
- Test: `wind_input/pkg/theme/views_metrics_test.go`（追加）

- [ ] **Step 1: 写失败测试**（追加到 `views_metrics_test.go` 末尾）

```go
func TestMergeViews_PreservesIndependentWindows(t *testing.T) {
	base := defaultViews() // 不含 Status/Tooltip/Toolbar/Menu
	ov := Views{
		Status:  &ViewNode{Color: "${text}"},
		Tooltip: &ViewNode{Color: "${text}"},
		Toolbar: &ToolbarViews{},
		Menu:    &MenuViews{},
	}
	merged := mergeViews(base, ov)
	if merged.Status == nil || merged.Tooltip == nil || merged.Toolbar == nil || merged.Menu == nil {
		t.Errorf("mergeViews 应透传 4 个独立窗口字段, got Status=%v Tooltip=%v Toolbar=%v Menu=%v",
			merged.Status, merged.Tooltip, merged.Toolbar, merged.Menu)
	}
	// ov 未提供这些字段时，结果应回退 base（此处 base 也为 nil，验证兜底逻辑不 panic 且为 nil）
	merged2 := mergeViews(base, Views{})
	if merged2.Status != nil || merged2.Tooltip != nil || merged2.Toolbar != nil || merged2.Menu != nil {
		t.Errorf("base/ov 均无独立窗口字段时结果应为 nil, got %+v", merged2)
	}
	// 候选窗 9 节点 + Metrics 仍正常 merge（回归保护：不破坏既有行为）
	if merged.Metrics == nil {
		t.Error("mergeViews 仍应保留候选窗 Metrics")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run（`wind_input` 下）：`$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestMergeViews_PreservesIndependentWindows' -v`
Expected: FAIL —— `merged.Status == nil`（当前 mergeViews 丢弃这些字段，返回零值 nil）。

- [ ] **Step 3: 改 mergeViews**

`wind_input/pkg/theme/views.go`，把 `mergeViews`（当前直接 `return Views{...10 字段...}`）改为先构造再透传 4 个指针：

```go
func mergeViews(base, ov Views) Views {
	out := Views{
		Window:        mergeViewNode(base.Window, ov.Window),
		PreeditBar:    mergeViewNode(base.PreeditBar, ov.PreeditBar),
		CandidateList: mergeViewNode(base.CandidateList, ov.CandidateList),
		Item:          mergeViewNode(base.Item, ov.Item),
		Index:         mergeViewNode(base.Index, ov.Index),
		Text:          mergeViewNode(base.Text, ov.Text),
		Comment:       mergeViewNode(base.Comment, ov.Comment),
		AccentBar:     mergeViewNode(base.AccentBar, ov.AccentBar),
		FooterBar:     mergeViewNode(base.FooterBar, ov.FooterBar),
		Metrics:       mergeMetrics(base.Metrics, ov.Metrics),
	}
	// 独立窗口 views（Status/Tooltip/Toolbar/Menu）整体透传：ov 非 nil 取 ov，否则 base 兜底。
	// 这 4 个是独立窗口的完整外观定义（P4），不做深度 merge——与现状 rv.Views 整体透传语义一致。
	out.Status = base.Status
	if ov.Status != nil {
		out.Status = ov.Status
	}
	out.Tooltip = base.Tooltip
	if ov.Tooltip != nil {
		out.Tooltip = ov.Tooltip
	}
	out.Toolbar = base.Toolbar
	if ov.Toolbar != nil {
		out.Toolbar = ov.Toolbar
	}
	out.Menu = base.Menu
	if ov.Menu != nil {
		out.Menu = ov.Menu
	}
	return out
}
```

> 注：先 Read `mergeViews` 当前实现确认它确实是"直接 return Views{10 字段}"（阶段 1 Task 3 加了 Metrics 行）。改动只是把 return 拆成 `out := Views{...}` + 4 段透传 + `return out`，**不改动那 10 个字段的 merge 方式**。

- [ ] **Step 4: 跑测试确认通过**

Run: `$env:CGO_ENABLED=1; go test ./pkg/theme/ -run 'TestMergeViews_PreservesIndependentWindows|TestMergeViews_MetricsOverride|TestDefaultViews_Metrics' -v`
Expected: PASS（新增透传测试 + 既有 Metrics 测试都过）。

- [ ] **Step 5: 全包测试 + gofmt + commit**

```
$env:CGO_ENABLED=1; go test ./pkg/theme/ 2>&1 | Select-String -NotMatch 'TestBuiltin|TestListAvailable'
gofmt -w pkg/theme/views.go pkg/theme/views_metrics_test.go
git add wind_input/pkg/theme/views.go wind_input/pkg/theme/views_metrics_test.go
git commit -m "fix(theme): P6 阶段2a mergeViews 透传 Status/Tooltip/Toolbar/Menu"
```
Expected: 除 4 个 pre-existing 失败（TestBuiltin*/TestListAvailable*，缺 build/data/themes）外全绿。

---

## 完成判据
- [ ] `mergeViews` 透传 4 个独立窗口指针，透传 + base 兜底测试通过。
- [ ] 候选窗 9 节点 + Metrics merge 行为不变（既有测试仍过）。
- [ ] `ResolveV25` **未改动**（本阶段不启用 merge）。
- [ ] 纯 theme 包，零渲染影响。

## 非目标（留后续子阶段）
- ❌ 在 `ResolveV25` 启用 `defaultViews` merge（2b）。
- ❌ `ResolveCandidateViews` 颜色/几何下沉 + 主题路径几何指纹（2b）。
- ❌ ui 切换 / 删合成桥 / layout 退场（2c/2e）。

## 关联
- `theme-view-p6-bridge-retirement.md` — P6 总设计
- 阶段 2 探查结论见 `project-view-architecture` 记忆的 P6 段（mergeViews 地雷为本阶段动机）
