<!-- Parent: theme-view-architecture.md -->

# P6：合成桥退役 + 候选窗 views 统一 + Behavior 覆盖架构

本文是主题渲染架构 v2.6 的 **P6 阶段** spec。上位设计见 `theme-view-architecture.md`（P0）。
P6 的目标是**把候选窗外观的 schema 定稳**，使其成为主题 Web 编辑器（P3）可依赖的稳定契约，
并消除当前 `layout` / `views` / `RenderConfig` 三套来源纠缠的临时合成桥。

> 驱动：编辑器本质是 `theme.Views`/`Behavior` schema 的可视化前端。若 P6 之后 schema 仍要变，
> 编辑器必返工。故"先把 schema 定稳"必须排在 P3 之前。

## 一、现状真相（已审计 + 一手验证）

候选窗实际渲染**唯一**消费 `r.resolvedViews`（`ResolvedViews`），其数据链路：
`layout → SetTheme 填 RenderConfig →（部分搬运）→ buildResolvedViews 造 base → applyThemeViews 用主题 views 覆盖`。

由此得到三条经 grep 验证的结论：

1. **layout 候选窗部分已名存实亡**。`SetTheme` 只读 11 个 layout 字段（`Index.Circle`/`AccentBar.Enabled`/
   `ItemPadding.L+R`/`ItemRadius`/`WindowPadding.L+T`/`BorderRadius`/`ItemHeight`/`Index.Gap`/`Comment.Gap`）。
   死字段：`BorderWidth`/`BandGap`/`ItemGap`/`AccentBar.Width`/`MaxWidth`/`Index.MinWidth`/
   `ItemPadding.Top/Bottom`/全部 5 个 `FontSize`。而活着的 11 个里凡 views 也定义的（window/item padding、
   radius、index/comment gap），最终又被 `applyThemeViews` 覆盖——**views 才是事实权威**。
2. **字号系统完全独立于主题**。候选窗字号 100% 由用户全局 `baseFontSize`（来自 `config.UI.FontSize`）派生：
   主文本 = base、序号 = base−4、行高 = max(32, base×1.8)，再 × DPI scale。layout/views 的字号字段全是摆设。
   推论：**density（compact/comfortable）的字号差异对候选窗无效**，只有 padding 差异生效。
3. **一批几何被 hardcode 写死、不可配**：item 间距（12/16）、accent 宽（3）、band 间距（4）、
   阴影偏移（2）、竖排最大宽（恒 600）。横排最大宽 `HorizontalMaxWidth` 字段存在但从不消费。

## 二、目标架构

候选窗外观拆成四个职责清晰、互不重叠的来源：

| 维度 | 目标权威 | 说明 |
|---|---|---|
| **外观几何** | `views`（单一来源） | padding/margin/border/圆角 + 此前 hardcode 的 item 间距/band 间距/accent 条/阴影偏移，进 views schema（竖排最大宽归 Behavior，因可被用户覆盖） |
| **颜色** | `palette` | 现状已对（P2 切片-1 + P5），不动 |
| **行为** | `Behavior` 块（三层覆盖） | 字号基准、page 策略 + 可被用户覆盖的项 |
| **运行时派生** | 引擎 | DPI scale、字号二级派生（序号=base−4、行高等）保留在引擎，不进主题 |

**三层合并**（与现有 `defaultViews()` 基线机制对称）：

```
外观:  defaultViews() 基线 ──→ 主题 views 覆盖 ──→ ResolvedViews（主题独占，用户不覆盖）
行为:  defaultBehavior() ──→ 主题 behavior 覆盖 ──→ 用户 override 注入(nil 跳过) ──→ ResolvedBehavior
颜色:  palette（现状）
       ↓
       引擎: × DPI scale + 字号二级派生 → 最终渲染
```

关键原则：
- **Behavior 块 = "可覆盖白名单"的结构化表达**。出现在 Behavior 的字段才允许用户覆盖；纯 `ViewNode`/几何字段天然不可覆盖。编辑器据此决定给用户开放哪些覆盖控件。
- **用户 override 一律 `*T` 指针，`nil`=跟随主题**（落地"跟随主题默认"语义，换主题时未覆盖项随主题变）。
- **Behavior 声明 + 默认值内置在引擎**（`defaultBehavior()`），主题/用户都只按需覆盖；绝大多数主题不写 `behavior:` 块。

## 三、Schema 变更

### 3.1 Behavior 块（新增，theme.yaml 顶层，与 layout/palette/views 并列）

```yaml
behavior:                    # 全部可选；未写走 defaultBehavior()
  font_size: 18              # 候选基准字号默认（引擎据此派生序号/行高 + scale）
  always_show_pager: false   # 单页时也显示翻页区
  show_page_number: true     # 翻页区显示 "1/3"
  vertical_max_width: 600     # 竖排候选最大宽（逻辑像素）
```

- 引擎内置 `defaultBehavior()`：`font_size:18 / always_show_pager:false / show_page_number:true / vertical_max_width:600`（= 现状 hardcode 值，零回归）。
- 用户 override：见 §3.3。

### 3.2 views 新增候选窗"列表级"几何

现 `Views` 是具名 `ViewNode` 集合。新增的几何里，padding/margin/border/圆角可继续挂在对应 `ViewNode`
（已支持）；但"列表布局级"参数（item 间距、band 间距、阴影偏移、accent 条尺寸）不属于单个节点。

**提案（待 review 定夺）**：Views 顶层加一个候选窗几何小块 `metrics`，承载列表级参数，保持 `ViewNode` 纯节点外观：

```yaml
views:
  # ...现有 window/item/index/text/comment/accent_bar 等 ViewNode 不变...
  metrics:                   # 候选窗列表级几何（新增）
    item_spacing: 12         # 候选项间距（原 hardcode 12/16，按 index 模式）
    band_gap: 4              # band 间距（原 WindowGap）
    shadow_offset: 2
    accent_bar: {width: 3, offset: 1, height_ratio: 0.6}
```

> 备选方案 B：把这些塞进对应 `ViewNode`（item_spacing→item、accent 尺寸→accent_bar 节点扩展）。
> 缺点：`ViewNode` 要长出"非盒模型"的尺寸字段，语义混杂。**推荐 metrics 块**——列表级与节点级分离更干净，编辑器也更好组织。

注：`vertical_max_width` 归 **Behavior**（用户可覆盖），不进 metrics。

### 3.3 用户 override 存储（config）

- `config.UI.FontSize`：语义改为"**跟随主题 / 自定义覆盖**"。采用可空表达（`*int` 指针 `nil`=跟随，或等价的显式 mode 字段——具体依 config 现有序列化形态在实现计划中定），跟随 = 用主题 `behavior.font_size`，有值 = 用户固定覆盖。
  迁移：**不靠"旧值=18"猜测**（会误判主动设 18 的用户），改为依 config 文件是否显式含该字段来判定"是否设过"；细则在实现计划阶段对照 config 序列化定。
- `config.UI.PagerDisplayMode`：已是枚举（Never/Auto/Always/Default）。`Default`（空）= 跟随主题 `behavior.{always_show_pager,show_page_number}`；其余按用户。**无需改 schema**，只需把 `Default` 的兜底从"renderer hardcode false/true"改为"读主题 behavior"。
- 字体 family：**本期不纳入**（待确认）。

## 四、合成桥退役

删除 `internal/ui/viewbox_views_bridge.go` 全部（`buildResolvedViews` / `applyThemeViews` /
`refreshResolvedViews` / `resolveViewColor`），候选窗 build 直接消费 theme 包解析出的 `ResolvedViews` + `ResolvedBehavior`。

- 颜色 token 解析（现 ui 侧 `resolveViewColor`）下沉到 theme 包（与其它 5 窗口一致：theme 包产出已解析 `color.Color`）。
- `RenderConfig` 候选窗外观字段（padding/颜色/spacing 等）退役；`RenderConfig` 仅保留真运行时值（DPI scale、用户字号 override、cmdbar 前缀、hover/cursor 等运行时数据）。
- 死字段清理：`HorizontalMaxWidth`/`VerticalMaxWidth`（恒 600）/`Horizontal/VerticalMinWidth`/`TextMarginRight`/`CommentMarginRight` 等一并删。

## 五、layout 候选窗退场

- 删除 `RawCandidateWindowLayout` / `ResolvedCandidateWindowLayout` 及候选窗相关 layout 字段；`SetTheme` 不再从 layout 填候选窗 RenderConfig。
- **density 对候选窗失效**（本就无效）：候选窗不再受 density 影响。
- **边界**：layout 还服务其它窗口（toolbar/status/tooltip/menu/toast）。这些窗口的 layout 字段是否同样大半死、layout 维度能否整体退役，**未在本期审计范围**。P6 只退役 **候选窗** 的 layout 部分；`layout` 整体退役（连同 density）作为更大的后续工作（需先审计其它窗口的 layout 消费），见 §9。

## 六、迁移与零回归

- **几何零回归**：`viewbox_geometry_test.go` 的横/竖指纹 golden（scale=1）是安全网。所有 hardcode 值
  （12/16/4/3/2/600）迁入 `defaultBehavior()`/`defaultViews().metrics` 后，scale=1 指纹必须逐项不变。
- **字号零回归**：default/msime 不写 `behavior.font_size`，引擎默认 18，与现状 baseFontSize 默认一致；
  用户已设 FontSize 的迁移见 §3.3。
- **现有基线断言迁移**：`viewbox_views_bridge_test.go` / `viewbox_applythemeviews_test.go` 固化了 18/14/32/600
  等 base 值；合成桥删除后，这些断言迁移到 theme 包的 `defaultBehavior`/`defaultViews` 测试。

## 七、编辑器影响（P3 契约）

P6 完成后，编辑器对接的稳定 schema = `views`（外观，含 metrics）+ `behavior`（可覆盖行为）+ `palette`（颜色）。
`Behavior` 块即"用户可覆盖字段白名单"，编辑器据此区分"主题作者编辑项"与"用户可覆盖项"。不再有 layout 候选窗维度。

## 八、非目标（YAGNI）

- ❌ layout 整体退役 / density 整体去留（仅候选窗部分退场；其它窗口待后续审计）。
- ❌ 字体 family 纳入 Behavior（待确认）。
- ❌ 其它窗口（toolbar/status/...）的 Behavior 化（本期只候选窗；Behavior 机制设计成通用，未来可接）。
- ❌ FooterBar band 渲染（现状未渲染，不在本期补）。

## 九、分步实施（草案，详见后续 implementation plan）

1. theme 包：定义 `Behavior` schema + `defaultBehavior()` + merge；`Views` 加 `metrics` + `defaultViews().metrics`；颜色 token 解析下沉。
2. theme 包：`ResolvedV25` 加 `Behavior ResolvedBehavior` + `ResolvedViews.Metrics`；解析链填充。
3. ui 候选窗：build 改吃 `ResolvedViews`+`ResolvedBehavior`，删合成桥（`viewbox_views_bridge.go`）。
4. ui：`RenderConfig` 瘦身（删退役外观字段）；字号 override（config.UI.FontSize nil 语义）+ page 策略读 behavior。
5. layout：删候选窗 layout 字段；`SetTheme` 候选窗映射删除。
6. 两种子主题：default/msime 增补 `behavior`（如需）+ `views.metrics`（零回归值）；几何指纹回归网验证。
7. 用户配置迁移（FontSize 语义）+ wind_setting 前端（若涉及字号/page 设置项）。
8. AGENTS.md 同步（pkg/theme、internal/ui、pkg/config）。

## 十、关联

- `theme-view-architecture.md` — P0 上位设计
- `theme-view-p5-adapter-retirement.md` — P5（adapter/ResolvedTheme 退役，本期同类手法）
- `../../wind_input/pkg/theme/AGENTS.md` / `../../wind_input/internal/ui/AGENTS.md` — 需同步
- `enum-constraint.md` — 全局枚举约束（PagerDisplayMode 已是枚举，本期不新增枚举）

## 十一、已定决策（2026-05-31 review）

1. **views.metrics 块**（§3.2）：✅ 采用独立 `metrics` 块（列表级几何与节点级 ViewNode 分离，编辑器更好组织）。
2. **vertical_max_width 归属**：✅ 归 **Behavior**（可被用户覆盖）。
3. **FontSize 迁移**（§3.3）：✅ 采用"跟随主题 / 自定义覆盖"显式语义，**不靠"旧值=18"猜测**；可空表达与迁移细则在实现计划中依 config 序列化形态定。
4. **字体 family**：✅ **本期不纳入** Behavior（Behavior 机制设计为通用，未来可加；§八 非目标）。
