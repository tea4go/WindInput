<!-- 用途: 主题尺寸字段「继承 vs 显式 0」语义审计 + 编辑器 round-trip 契约 -->

# 主题尺寸字段「继承 vs 显式 0」语义 — 现状审计与编辑器契约

> 状态：设计审计（草案）。结论先行见第零节。
> 关联：`theme-v3-freeze-report.md`（v3 冻结契约 / base 继承）、`theme-capability-schema.md`（能力三态）、`pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`。

## 零、结论先行（对早期判断的修正）

早期讨论里把「尺寸字段零值歧义」列为"影响最深远、需要把所有 `Dimension` 值字段大扫成 `*Dimension`"的架构改造。**审计后这个前提不成立**：

- **后端继承/合并层已三重正确地表达「nil=继承」与「显式 0=覆盖」**（第一节证据）。无需任何 schema 字段类型迁移。
- 真正残留的缺口收敛为 **3 个具体、互相独立、可窄修**的点（第三节）。没有一个是"大扫迁移"。
- 唯一的跨仓工作是**编辑器（独立仓 `WindInputThemeEditor`）的 round-trip 契约**：未触碰字段必须"省略"而非"写 0"（第四节）。`wind_setting` 不涉及——它只编辑 `ui.*` 高层覆盖，不碰 `views` 盒模型字段（第四节实证）。

## 一、现状：后端已三重正确表达 nil 语义

「某个尺寸字段没配 = 继承上游」「显式写 0 = 覆盖成 0」这组语义，后端在三个独立层次都已正确实现：

### 1. base 单链继承 —— raw map deepMerge（按 key 存在性）

`theme-v3-freeze-report.md:149` 明确：继承"先合并、作用于 raw"——
```
merged = deepMerge(rawBase 链, rawSelf)   —— 纯 raw schema 合并，不求值
```
这是 **YAML map 层**的合并：子主题没写某 key → 继承父主题的 key；写了（哪怕 `0`）→ 覆盖。map 的"键存在性"天然区分"缺省"与"显式 0"，无歧义。

### 2. defaultViews 基线 ⊕ 主题 —— `*Dimension` nil-check

YAML schema 层（`pkg/theme/views.go`）的几何字段**全部是 `*Dimension` / `*int` 指针**：

```go
// views.go:22-29
type ViewEdges struct {
    Top    *Dimension `yaml:"top,omitempty"`   // nil=未写(回退基线)，非 nil(含 0)=显式值
    ...
}
// ViewBorder.Width/Radius、ViewNode.Gap/RowGap/Width/Offset/LineSpacing/ColGap/TitleGap … 同为 *Dimension
```

`mergeEdges` / `mergeViewNode`（views.go:379-507）一律"指针非 nil 才覆盖"：

```go
// views.go:379-395
func mergeEdges(base, ov ViewEdges) ViewEdges {
    out := base
    if ov.Top != nil { out.Top = ov.Top }   // nil=不覆盖(继承)，&{0}=覆盖成 0
    ...
}
```

→ `&Dimension{0}`（显式 0）与 `nil`（未写）严格区分。

### 3. 序列化 round-trip —— `omitempty`

`*Dimension` + `omitempty`：nil 指针 → 字段省略；`&{0}` → 序列化为 `0`。Go 层 JSON/YAML 往返保留 nil/0 区别（`dimension.go` 的 Marshal/Unmarshal 按值处理，不影响指针的 omit 行为）。

**小结**：在"配置→继承→合并"这条链上，歧义不存在。

## 二、为什么 resolve 扁平化不丢信息

`resolveViewNode`（candidate_views.go:39-95）把 `*Dimension` 拍平为 `RVNode` 的值类型 `Dimension`：

```go
PadTop: dimOr(n.Padding.Top, Dimension{}),   // 默认恒为 Dimension{} (零)
```

`dimOr(p, Dimension{})`：nil → `{0}`，`&{0}` → `{0}`，`&{N}` → `{N}`。**nil 与显式 0 在此塌缩成同一个 0**——但这是**正确**的：到达 `resolveViewNode` 时，`views` 已是 `defaultViews ⊕ base 链 ⊕ 主题` 的**完全合并结果**，继承语义已在上游应用完毕。`RVNode` 是"求值后"的渲染态，本就不该再携带继承信息。nil 在这里等价于"含 defaults 在内都没人配过它"→ 渲染器内置默认（通常即 0 或派生公式）。

> 因此 `RVNode` 用值类型 `Dimension` 是**正确的设计**，不是缺陷。把 `RVNode` 也改成指针毫无收益。

## 三、真正残留的 3 个缺口（窄、独立、可单修）

### GAP 1 — 状态态（selected/hover/disabled）的几何是「假字段」⚠️ 真 bug

schema 允许 `item.selected.padding` / `.margin` / `.font_size`（`ViewNode.Selected/Hover/Disabled` 是完整 `*ViewNode`），但**渲染两道关卡都把它丢掉**：

**关卡 A — `resolveState` 的"有无覆盖"判定不看几何**（candidate_views.go:171-179）：
```go
if resolveColor(node.Background.Color) != nil || node.Background.Image != nil ||
    resolveColor(node.Color) != nil || resolveColor(node.Border.Color) != nil ||
    node.Border.Width != nil || node.FontWeight != nil {
    has = true   // ← 只认 颜色/图/边框色/边框宽/字重；Padding/Margin/Radius/FontSize 完全没看
}
if !has { return nil }   // 只改 padding 的 selected 态 → has=false → 整个 patch 被丢弃
```

**关卡 B — `effectiveNode` 合并 patch 时不碰几何**（viewbox_build.go:170-200）：
```go
// 只合并 BgColor/BgImage/BorderColor/BorderWidth/BorderRadius/TextColor/FontWeight/FontFamily
// 从不合并 Padding/Margin/FontSize
```

**影响**：`item.selected.padding`、`item.hover.margin`、`item.selected.font_size` 等是声明可写、渲染必忽略的假字段。
**严重度**：中（功能缺失，非崩溃；多数主题不依赖"选中态改间距"）。
**修法二选一**：
- (推荐·低风险) **对齐现实**：能力矩阵把 `item`/`index`/`comment`/`menu.item` 的状态态几何标 `unsupported`，schema 注释注明"状态态仅支持颜色/边框/字重覆盖"。`resolveState` 加注释说明判定范围。零渲染改动。
- (高成本) **补齐渲染**：`resolveState` 判定纳入几何字段、`effectiveNode` 合并 Padding/Margin/FontSize。需重做 golden（状态态几何会改变布局）。

### GAP 2 — `RVNode` 消费层 `!= Dimension{}` 门控：状态无法表达"显式 0"

`effectiveNode`（viewbox_build.go:185/188）与 `applyNodeBox`（viewbox_build.go:209）用零值门控：
```go
if st.BorderWidth != (theme.Dimension{}) { out.BorderWidth = st.BorderWidth }
```
→ 状态态想把 border-width 显式设回 0 无法表达（被当"未覆盖"）。
**影响**：极窄。border-width=0 = 无边框，与"不覆盖"在视觉上等价，基本 moot。padding 的状态覆盖本就被 GAP 1 拦在更上游，轮不到这层。
**严重度**：低（理论一致性问题）。
**修法**：随 GAP 1 的"补齐渲染"方案一并处理；单独修无意义。

### GAP 3 — 渲染兜底"0 = 用内置默认"站点：显式 0 无法表达

少数消费点把"解析出 0"当作"用 hardcode 默认"，导致显式 0 配置失效：
```go
// viewbox_menu.go:57-60  menu.root 上下 padding
if padTop == 0 && padBottom == 0 { padTop, padBottom = menuPaddingY, menuPaddingY }  // 想配 0 padding → 反被兜底
// viewbox_menu.go:64-67  menu.item 左右 padding 同理
// buildPager footer 箭头 padding: if v := fb.PadLeft.Scaled(); v != 0 { use } else { 兜底 6 }
```
**影响**：这些位置"显式 0"会被还原成内置默认值——例如无法把菜单做成 0 内边距。
**严重度**：低-中（少数节点、少数人想要 0）。
**修法**：若要支持，把判定从"resolved 值 == 0"改回"schema `*Dimension` 是否为 nil"（即把兜底下沉到 `defaultViews` 基线、或在 resolve 前判 nil）。属窄修，逐站点处理。

## 四、编辑器 round-trip 契约（独立仓 `WindInputThemeEditor`）

### wind_setting 不涉及（实证）

`wind_setting` 的外观页只编辑 `ui.*` 运行时覆盖，**不碰 `views` 盒模型**：
- `frontend/src/schemas/appearance.schema.ts`：全部 key 形如 `ui.theme_style`、`ui.candidates_per_page`、`ui.status_indicator.border_radius`、`toolbar.visible`……无一条是 `views.*.padding/border`。
- `AppearancePage.vue` 的主题相关能力是**选择/导入/删除主题** + **启停主题编辑器服务**（`startThemeServer`/`stopThemeServer`），并不直接编辑盒模型字段。

→ 盒模型字段（padding/border/间距）的可视化编辑在独立仓 `WindInputThemeEditor`。本契约面向它。

### 契约：未触碰字段必须"省略"，不得"写 0"

后端 nil 语义只有在序列化保留时才有意义。编辑器**绝不能把用户未填的尺寸字段写成 `0`**——那会把"继承"悄悄变成"显式 0 覆盖"，污染 base 继承链。

**三态数值控件**（与 nil 语义一一对应）：

| 控件状态 | 含义 | 序列化 |
|---|---|---|
| 空（placeholder 显示**继承来的有效值**，灰字） | 继承 base/默认 | **省略该 key** |
| 填 `0` | 显式覆盖为 0 | 写 `0` |
| 填 `N` / `"Npx"` | 显式覆盖为 N（dp / px） | 写 `N` 或 `"Npx"` |

铁律：**清空控件 ⇒ 删除 JSON key**（回到继承），而不是写 0。加载主题时，缺省的 key 渲染为"空+placeholder（有效值）"，非"0"。

### 与能力三态正交

两套三态各管一维，组合使用：

| 维度 | 三态 | 决定 |
|---|---|---|
| 能力声明（`theme-capabilities.json`） | supported / reserved / unsupported | 控件**显不显示**（reserved 灰显角标、unsupported 隐藏） |
| 继承（本契约） | 空 / 0 / N | 控件**填没填值**（空=继承 placeholder） |

例：`item.background_gradient` = supported 且用户未填 → 显示控件、空态、placeholder 为继承值。`item.selected.padding`（GAP 1 修为 unsupported）→ 控件不显示。

### 带单位（dp / px）的表达

`Dimension` 支持 `8`（dp）/`"8px"`（设备像素）。编辑器数值控件宜配单位切换（dp 默认 / px），分别序列化为裸数字 / `"Npx"`（见 `dimension.go` Marshal）。这是**正交于继承**的第三维（值本身的单位），不要与"空=继承"混淆。

## 五、建议处置（按性价比）

| # | 动作 | 成本 | 建议 |
|---|---|---|---|
| GAP 1 | 能力矩阵标状态态几何 `unsupported` + schema 注释（对齐现实方案） | 小 | **先做**：消除假字段，零渲染风险 |
| 契约 | 把第四节写进 `theme-capability-schema.md` / freeze-report，供独立编辑器对齐 | 小 | **先做**：钉死 round-trip 纪律 |
| GAP 3 | 若产品要支持 0 padding：兜底站点改 nil 判定 | 中 | 视需求，可延后 |
| GAP 1' | 若产品要支持"选中态改间距"：补齐 `resolveState`+`effectiveNode`+golden | 大 | 仅在有真实需求时 |
| GAP 2 | 随 GAP 1' 一并 | — | 不单独做 |

## 六、回归判据

- 处置 GAP 1（对齐现实）：`TestCapabilities_WellFormed` + `theme-capabilities.json` golden 重落；`TestV3GoldenSnapshot` 不变。
- 处置 GAP 3：新增"显式 0 padding 生效"单测（menu root/item、footer arrow）；既有 golden 不变（默认未配走原兜底）。
- 编辑器契约：独立仓侧新增 round-trip 测试（加载→不动→保存，diff 必须为空；清空字段→key 消失）。

## 七、变更规则

- 本文档若与 `theme-v3-freeze-report.md` / `theme-capability-schema.md` 冲突，以冻结报告的 schema 契约为准。
- 任何把状态态几何从 `unsupported` 转 `supported` 的改动，必须同时落地 `resolveState` + `effectiveNode` 的真实消费（不得空转声明），并重做相关 golden。
