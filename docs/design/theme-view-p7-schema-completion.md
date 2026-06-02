<!-- Parent: theme-view-architecture.md -->

# P7：主题 schema 补全（冻结前最后一程）

本文是主题渲染架构 v2.6 的 **P7 阶段** spec。上位设计见 `theme-view-architecture.md`（P0）。
P6 已把**候选窗外观 schema 为两个种子主题定稳**（三权分立 views/palette/behavior、合成桥退役）。
但一次冻结前的综合架构评审（2026-06-01，独立 architect agent + 一手核验）发现：

> **引擎（internal/ui/viewbox.go）能力很强，但 theme.yaml 的 schema 只是引擎能力的"有损投影"。**
> 引擎能画背景图/装饰层/逐元素字体，schema 却声明不出来。以现状 schema 冻结并造编辑器，
> 只能做"配色器+间距微调"，**兑现不了产品目标"复刻搜狗/微软等位图皮肤"**，且日后补能力时
> schema + 编辑器双双返工——正是 P6 立项要避免的代价。

P7 的目标 = **把 schema 补全到"能复刻复杂位图皮肤"的程度，使其可被冻结**，作为 P3 主题编辑器的稳定契约。

## 一、为什么 P6 不够（范围 vs 冻结标准）

不是 P6 做错了，而是**冻结标准比 P6 的交付范围高一层**：
- P6 范围：候选窗 schema 为**两个纯色无图种子主题**定稳。背景图/层/渐变/逐元素字体被有意 YAGNI 延后。
- 冻结标准（用户）：① 灵活性——简单改色 **且** 完整复刻位图皮肤；② 无明显表达不了的自定义需求；③ 自洽。
- 位图皮肤的本质 = 九宫格背景图 + 选中高亮图 + 水印 logo + 装饰层，恰是 P6 延后的能力。

## 二、冻结判据（写死，作为 P7 完成的验收）

> schema 可冻结 ⟺ 同时满足：
> 1. **无假字段**：schema 里每个字段都真实驱动渲染（不存在"有字段、解析器不读"）。
> 2. **无藏起来的能力**：引擎现有能力（Fill.Image / Layers / Shadow / TextStyle.Weight 等）都有 schema 入口。
> 3. **P0 锁定决策 D4/D5 已落到 schema**（layers z 层 / 通用 Image + resources 注册表）。
> 4. **能复刻一个真实复杂皮肤**（建议做一个搜狗/微软风格位图皮肤样例主题作为端到端验收）。

当前四条均未满足。

## 三、能力现状（三关验证：schema 字段 / 解析消费 / 引擎渲染）

| 维度 | schema | 解析 | 引擎 | 实际可用 |
|---|---|---|---|---|
| 几何 padding/margin/border/radius | ✅ | ✅ | ✅ | ✅ |
| 列表级几何（item 间距/band/accent/阴影偏移）| ✅ ViewMetrics | ✅ | ✅ | ✅ |
| 底色（token / hex）| ✅ ViewFill.Color | ✅ | ✅ | ✅ |
| item selected/hover **底色** | ✅ | ⚠️ 仅底色 | ✅ | ⚠️ 仅一个底色 |
| **背景图** | ⚠️ 仅 palette.background（非 views）| resolver 产出但被丢弃 | ✅ Fill.Image | ❌ 运行链断 |
| **装饰层 layers[]** | ❌ 无 | — | ✅ View.Layers | ❌ schema 无入口 |
| **逐元素字体/字重/族** | ⚠️ ViewNode 有字段 | ❌ 解析器不读 | ❌ TextStyle 无 Family | ❌ 假字段 |
| **states（selected/hover/disabled 完整 patch）**| ⚠️ 仅 selected/hover、无 disabled | ❌ 仅取底色 | ✅ | ❌ 残缺 |
| 渐变 | ❌ | — | ❌ | ❌ |
| 阴影 blur/spread | ❌（仅 offset）| — | ❌（仅 offset）| ⚠️ |
| 方向变体（横/竖排分别配）| ❌ | — | 运行时 | ❌ |
| 其它窗口（toolbar/menu/status/tooltip）几何 | ❌ hardcode | — | — | ⚠️ 仅颜色可配 |

## 四、Gap 清单与修法（一手核验过，带证据）

### 🔴 必须（阻断冻结）

**P7-1 打通背景图运行链 + 背景图入 views**
- 证据：`ResolvedV25.Palette.Background *ResolvedBackground` 存在（resolved.go:62），`resolver.go:288-306` 产出，但 `renderer.go` SetTheme 末尾 `r.config.BackgroundImage = nil`（无条件清空，死代码）；`viewbox_build.go` 的 `Image: cfg.BackgroundImage` 永远 nil。
- 修法：① `ViewFill` 加 `Image *ViewImage`（对齐 P0 D5），背景图迁/复制到 `views.window.background.image`；② resolver 解码填入 ResolvedViews；③ 删 SetTheme 清空逻辑，build 从 resolvedViews 取图。`bgimage.go` 的 `LoadBackgroundImage`/`DrawBackground` 原语已就绪。

**P7-2 落地 resources + 通用 Image + `ViewFill.Image` + `ViewNode.Layers`（P0 D4/D5）**
- 证据：`ViewFill` 仅 `Color`（views.go:25-27）；`ViewNode` 无 `Layers`（views.go:37-49）；引擎 `View.Layers []ImageLayer`（viewbox.go:112）+ `Fill.Image` 已实现在用（accent rail/preedit 光标走 Layers）。
- 修法：落地 P0 §三类型——顶层 `resources: map[string]string`（名→路径/data URI）；通用 `ViewImage{ref,mode,slice,opacity,z,anchor,offset,size}`；`ViewNode.Layers []ViewImage`；`ViewFill.Image *ViewImage`。resolver 经 `LoadBackgroundImage` 解码 ref→`RVNode.Layers`。纯 schema+resolver 工，引擎已就绪。

**P7-3 逐元素字体三件套真生效**
- 证据：`ViewNode.FontSize/FontWeight/FontFamily`（views.go:43-44）merge 正确但 `ResolveCandidateViews` 不填 `RVNode`（candidate_views.go 注释"不设字号"）；`RVNode.FontWeight` 恒 0；`TextStyle`（viewbox.go:96-102）**无 Family 字段**。
- 修法：① resolver 填 `RVNode.FontSize/FontWeight`（views 显式优先，nil 跟随运行时派生）；② `TextStyle` 加 `Family string`，文本后端按元素切字体（GDI/DirectWrite 已支持 weight）；③ 定字号语义（相对 delta vs 绝对覆盖）——建议绝对覆盖，nil=跟随全局派生。

**P7-4 states 补成完整 ViewPatch + 加 disabled**
- 证据：`ViewNode` 仅 `Selected/Hover`（无 Disabled，views.go:47-48）；`candidate_views.go:98-107` 仅取 `Item.Selected/Hover.Background.Color`，文字色（palette 有 SelectedText）/字重/边框 patch 全丢；其它节点 states 不解析。
- 修法：把 Selected/Hover 完整 patch（Color/FontWeight/Border/Background.Image）解析进 RVNode 对应态；补 Disabled（cmdbar 禁用项已有需求）。

**P7-5 清 P6 尾巴（自洽收尾）**
- 证据：`ResolvedCandidateWindowLayout`（resolved.go:29）+ `RawCandidateWindowLayout`（layout.go）仍存在且 SetTheme 仍读 rv.Layout.CandidateWindow（IndexStyle/HasAccentBar/themeRowHeight）——P6 §五"候选窗 layout 退场"未完成；候选窗 layout 死字段（FontSize/ItemGap/AccentBar.Width）。
- 修法：候选窗几何/开关来源**全部归 views/behavior**（IndexStyle→views.index、HasAccentBar→views.accent_bar 存在性或 metrics、行高→views/behavior）；删候选窗 layout 字段；SetTheme 不再读 layout 候选窗部分。注意 layout 仍服务其它窗口（toolbar/status/...），density 去留另议（更后续）。

**P7-6 候选窗边框宽度可配**
- 证据：`windowBorder()` 硬编码 width=1（viewbox_build.go:94 / viewbox_build_vertical.go:266），覆盖 `views.window.border.width`（resolver 已填 RVNode.BorderWidth）。
- 修法：`windowBorder` 改读 `rv.Window.BorderWidth`，仅 accent 模式叠加。小改。

### 🟡 字段形状现在预留、渲染可 later（避免日后破坏兼容）

- **渐变**：`Fill`/`Border` 加 `gradient` 字段位（可空），渲染 later。
- **阴影结构化**：shadow 从单标量 `Metrics.ShadowOffset` 改为 `{offset_x,offset_y,blur,spread,color}`，值可只实现 offset，**结构现在定**。
- **方向变体**（D7）：冻结文档声明"Image/views 的 vertical 覆盖是非破坏扩展"，编辑器不写死方向假设。
- **其它窗口几何 View 化**（toolbar/menu）：可迭代，冻结文档标明"仅颜色冻结、几何待定"。

## 五、建议分阶段（每步独立 commit，subagent-driven + quality review，沿用 P6 手法）

1. **P7-A 低风险收尾**：P7-5（清候选窗 layout 尾巴 + 死字段）+ P7-6（边框可配）。先把"假字段/双套 schema"清干净——这是冻结判据①的前提。
2. **P7-B 字体生效**：P7-3（RVNode 填字号/字重 + TextStyle.Family）。
3. **P7-C 图与层（核心）**：P7-1（背景图链路）+ P7-2（resources+Image+layers）。schema 形状是冻结的重头，需谨慎设计 + 几何/颜色指纹守护。
4. **P7-D states 补全**：P7-4。
5. **P7-E 预留字段形状**：渐变/阴影结构体/方向变体声明（只定 schema，不强求渲染）。
6. **P7-F 端到端验收**：做一个真实复杂位图皮肤样例主题（搜狗/微软风格），跑通=冻结判据④。
7. **冻结**：更新 P0 文档锁定 schema 版本，宣布 v2.6 schema 冻结，开 P3 编辑器。

## 六、验证基础（本评审的来源）

- 独立 architect agent（opus，read-only）综合评审，逐字段三关验证 + 文件行号证据。
- 主 agent 一手核验关键断言：TextStyle 无 Family（viewbox.go:96-102）、ResolvedV25.Palette.Background 存在但 SetTheme 丢弃、ResolvedCandidateWindowLayout 仍在、ViewFill 仅 Color。结论一致。

## 七、关联
- `theme-view-architecture.md` — P0 上位设计（D4 layers / D5 通用 Image+resources，P7 补全的就是这两条）
- `theme-view-p6-bridge-retirement.md` — P6（schema 三权分立 + 合成桥退役）
- `../../wind_input/pkg/theme/AGENTS.md` / `../../wind_input/internal/ui/AGENTS.md` — 改 schema/引擎需同步
