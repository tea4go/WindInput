<!-- Parent: AGENTS.md -->
<!-- Generated: 2026-06-04 | Updated: 2026-06-04 -->

# 工具栏几何重构（L1 几何单一真相源 + L2 盒模型化）

## 背景

输入法浮动工具栏（`internal/ui` 的 toolbar 系列，仅 Windows）当前几何还停留在
"线性公式手算按钮坐标 + 几何与鼠标命中各算一套"的状态。候选窗早已迁到盒模型 View
引擎（`viewbox.go` 的 `Layout`/measure/arrange），工具栏渲染虽也接了 `Layout`/`PaintTree`，
但用 `FixedW/FixedH` 把 measure 架空，且命中/边界查询仍走独立公式。

本次按颜色已迁 `toolbar_*` token（v3）之后的延续工作，分两步：**L1 几何单一真相源**
（解耦，零视觉变化）、**L2 盒模型化**（几何进 schema，measure 真正生效）。

## 问题：按钮矩形被算了三遍（+窗口尺寸第四处）

| # | 位置 | 用途 | 形式 |
|---|------|------|------|
| 1 | `viewbox_toolbar.go buildToolbarTree` | 渲染布局 | View `FixedW + Margin` 隐式编码 x |
| 2 | `toolbar_renderer.go HitTest` | 鼠标命中 | 独立线性累加 `gripW + n×buttonW` |
| 3 | `toolbar_renderer.go GetButtonBounds` | tooltip/菜单定位 | 独立线性累加 + padding |
| 4 | `toolbar_window.go:219/547` | Win32 窗口尺寸 | `ScaleIntForDPI(116/30)` 字面量 |

改任一常量（如 `buttonWidth`）需同步改 1/2/3 三处——这是耦合。#4 用的是不同缩放源
（`ScaleIntForDPI` vs 渲染的 `GetDPIScale`），属另一议题，L1 不动缩放基准。

## L1：几何单一真相源（已实现，零视觉变化）

**核心**：一切按钮矩形从同一次 `buildToolbarTree + Layout` 派生，删除 #2/#3 的线性公式。

仿候选窗 `renderTree → RenderResult.Rects → hitRects` 范式，但工具栏只 5 个 View、
几何与状态无关（mode 文字变化不影响布局，因 `FixedW` 固定），故采用**无状态按需布局**，
不引入缓存（无缓存失效 / DPI 过期风险）：

```
computeGeometry() → 用零 state/零色 buildToolbarTree + Layout，提取：
  - size   = root.Rect().Size()                    （GetToolbarSize）
  - bounds = 各按钮 View 的 Rect()（content 矩形）  （GetButtonBounds）
  - hits   = 各按钮 View 的 margin 盒（content 外扩 Margin）（HitTest）
```

**等价性证明（faithful 关键）**：
- 旧 `HitTest` 命中语义 = 按 x 分段、忽略 y、按钮间无间隙 = 每个子 View 的 **margin 盒**。
  `LayoutRow` 使 margin 盒首尾相接 → 平铺整条、满高。逐按钮核对：grip `[0,10)`、
  mode `[10,36)`、width `[36,62)`、punct `[62,88)`、settings `[88,114)`（scale=1），与旧带界一致。
- 旧 `GetButtonBounds` = content 矩形 = View `Rect()`（如 settings `Min.X=90`、宽 `buttonW-2pad=22`）。
- 旧 `GetToolbarSize` = `(116,30)×scale` = root `Rect().Size()`（root `FixedW/FixedH` 即此值）。

`viewOuterRect(v)` = `v.Rect()` 外扩 `v.Margin`（Margin 是 View 自带数据，非重算公式）。

**附带**：`toolbar_window.go` 的窗口尺寸字面量 `116/30` 换成包内常量
`toolbarBaseWidth/toolbarBaseHeight`（消魔数，不改 `ScaleIntForDPI` 缩放源）。

**不变**：渲染（`Render`）与矢量符号后处理已用 `tt.X.Rect()`，本就单源，无需改。

## L2：盒模型化（规划，L1 绿后再开）

L1 解耦后，几何收口到 `buildToolbarTree`，可安全地让 measure 真正生效、几何进主题 schema：

1. **schema 补几何字段**：`ToolbarViews` 增按钮 padding / gap / grip 宽 / 圆角等；默认值=当前常量（零回归）。
2. **弃 `FixedW`**：按钮尺寸由内容 measure + padding 决定，root 尺寸由 measure 汇总；`GetToolbarSize` 自然反映。
3. **补 `ResolveToolbarViews`**：`other_views.go` 当前唯一缺失槽位；走统一 `resolveViewNode`，
   消费目前被忽略的 `ToolbarButtonNode.Border` 等字段。
4. 协调 `GetDPIScale` vs `ScaleIntForDPI`，统一窗口尺寸到 `GetToolbarSize`（#4 收口）。

## L3 愿景（远期，不在本次）

按钮内容动态化：每个状态指定开/关对应显示效果（文字 / 符号 / 图片），支持悬停特效。
当前无视觉 hover（仅记录用于 tooltip）。

## 测试策略

- `TestBuildToolbarTree_Geometry`（既有）：整条 116×30、按钮框高 26、mode 选色、settings `Min.X=90`——L1 后仍逐项绿（几何数值不变）。
- 新增 `TestToolbarHitTest_SingleSource`：① 各按钮带中心点 `HitTest` 返回对应 kind；② `GetButtonBounds` == `buildToolbarTree+Layout` 的对应 `Rect()`；③ `GetToolbarSize` == root 尺寸；④ 命中带平铺无缝（相邻带界相接）。
- `TestResolveToolbarViews_BaseAndMode`（既有）：颜色不变。

## 风险与回滚

- L1 行为零变化由等价性证明 + 测试守护；如真机命中异常，回滚仅涉及 `toolbar_renderer.go` 三方法。
- 缩放源差异（`GetDPIScale`/`ScaleIntForDPI`）刻意留到 L2，避免 L1 夹带 DPI 回归。
