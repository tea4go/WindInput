# 主题渲染 v2.6 P4：其它窗口 View 化（状态泡 / Tooltip / 工具栏）

> 设计文档（spec）。承接 P1（候选窗 View 引擎）与 P2（候选窗 YAML 化），把候选窗之外的独立窗口迁移到同一套盒模型 View 引擎，并 YAML 化其外观。**菜单延后单独成阶段**（P4 不含菜单）。

## 背景与目标

候选窗已是盒模型 View 引擎的唯一渲染路径（P1 `f14c8af`），外观由 YAML `views:` 块驱动（P2）。其余 4 类窗口仍各自 GDI/gg 直绘：

| 窗口 | 渲染入口 | 颜色来源 | 复杂度 |
|---|---|---|---|
| 状态泡 | `status_renderer.go:Render` | `ResolvedTheme.ModeIndicator`（2 色）+ 运行时 cfg 覆盖 | ★ |
| Tooltip | `tooltip.go:render` | `ResolvedTheme.Tooltip`（2 色） | ★★ |
| 工具栏 | `toolbar_renderer.go:Render` | `ResolvedTheme.Toolbar`（17 色）+ 齿轮矢量 | ★★★ |
| 菜单 | `popup_menu_render.go:render` | `ResolvedTheme.PopupMenu`（7 色） | ★★★★（延后） |

**目标**：把状态泡 / Tooltip / 工具栏三类窗口的**渲染**迁到 View 引擎，并把其外观 YAML 化（几何 + 颜色 token），为 P5 的 `adapter.go` 退役铺路。窗口生命周期（创建 / 显示 / 命中测试 / 鼠标事件）不动。

**非目标**：菜单（延后）；背景图 / 渐变 / 资源表（无种子主题需要，YAGNI）；窗口交互语义变更。

## 核心原则（沿用 P1/P2）

- **引擎正确性优先，主题后调**：不追像素级 parity，差异用更新主题解决。
- **零回归**：迁移前后默认主题视觉不变。token 默认映射到当前 `ResolvedTheme` 同源颜色，写 token = 写当前值。
- **单步可验证、独立 commit**：每切片端到端跑通再进下一片。

## 架构

### 引擎复用

复用候选窗的引擎核心，**不修改引擎**：
- `Layout(root, x, y, td)`（measure + arrange）
- `PaintTree(root, dc, img, td)`（三趟 paint：shapes / text / overlays）
- `renderTree` / `acquireDrawContext`

每个窗口新增一个 `buildXxxTree(state) *View`，把现有 `render()` / `Render()` 改成「建 View 树 → renderTree → 返回 image」。

### 每窗口独立的 Resolved 结构（决策）

不把多窗口节点塞进候选窗专用的 `ResolvedViews`（会变成上帝结构）。新增三个内聚的小结构：

- `ResolvedStatusViews`
- `ResolvedTooltipViews`
- `ResolvedToolbarViews`

各自承载本窗口的 RVNode 集合 + 必要标量。每个窗口的 build 只依赖自己那一份，符合单一职责，且**完全不碰已验证的候选窗 `ResolvedViews`**。

### 颜色解析器泛化（决策）

候选窗的 `resolveViewColor(s, cand, accent)` 把 `${background}` 写死映射到候选窗颜色集。多窗口下，`${background}` 对不同窗口指不同颜色。

**泛化**：把 token 解析抽象为一个**颜色解析器** `type colorResolver func(name string) color.Color`。每个窗口注入自己的颜色表（从对应 `ResolvedTheme` 子结构构造）。各窗口 YAML 块用**本窗口语义 token**（`${background}` / `${text}` / `${border}` …），互不冲突。

候选窗保持现状（用包装 `cand` + `accent` 的解析器），**零回归**。

### 状态覆盖 + base 回退（决策）

功能按钮 = **一个通用 base + 可选状态覆盖块；不写覆盖即回退 base**（复用候选窗 `Item.Selected`/`Hover` 的 patch 机制，CSS `button {} + button:active {}` 思路）。

- 前期：种子主题只配 button base（外加模式按钮的中/英两态），其余按钮零配置自动继承。
- 可定制性：未来想给某状态单独上色，加一个状态覆盖块即可，无需改代码。
- **schema 粒度（决策）**：按按钮具名 + 状态子块（`mode.chinese` / `mode.english` …），状态名有明确语义、编译期可检查，优于通用 `states map[string]`。

build 时：`effective = mergeViewNode(button.base, button.<当前状态>)`，缺失即回退 base。

## YAML schema 扩展

`Theme.Views` 顶层新增 `status` / `tooltip` / `toolbar` 三个独立子块（与候选窗 `Views` 平级，各自结构，不复用候选窗 `Views`）。

### 状态泡

```yaml
views:
  status:
    background: {color: "${background}"}
    border:     {radius: 8, color: "${border}"}
    color:      "${text}"
    padding:    {top: 4, right: 10, bottom: 4, left: 10}
```

- 字号：用户全局字号优先（与候选窗一致，font 不 token 化）。
- 运行时 `StatusWindowConfig` 的自定义颜色 / 圆角 / 透明度覆盖仍**优先于** views（保持现有"自定义 > 主题 > 默认"优先级）。

> **P4-A 实际实现收窄（已落地）**：状态泡只 token 化**颜色**（`background`/`text`，映射 `ModeIndicator`）。padding(6)/borderRadius/fontSize/opacity 保持现状从运行时 `StatusWindowConfig` 取，**不进 views**——它们本就是运行时配置而非主题，且圆角/字号运行时优先。`views.status` 因此仅含 `background`/`color` 两个 token。几何 YAML 化按需后补。颜色解析未全局重构候选窗 `resolveViewColor`，而是新增通用 `resolveTokenColor(s, resolver)` 入口（各窗口注入自己的颜色表），候选窗保持原状零回归。

### Tooltip

```yaml
views:
  tooltip:
    background: {color: "${background}"}
    border:     {radius: 6}
    color:      "${text}"
    padding:    {top: 6, right: 8, bottom: 6, left: 8}
```

- 多行 + `\t` 列对齐、行截断（尾部 `…`）、行数上限（超 20 行汇总 `+(N)`）逻辑**搬进 build 阶段**：预量算 → 截断后再建 View，View 的 Text 已是最终字符串（沿用候选窗竖排截断的两阶段套路）。

### 工具栏

```yaml
views:
  toolbar:
    background: {color: "${background}"}
    border:     {radius: 6, color: "${border}"}
    grip:       {color: "${grip}"}
    button:                                   # 通用 base，所有按钮默认取这里
      background: {color: "${button_bg}"}
      color:      "${button_text}"
      border:     {radius: 4}
      mode:                                   # 模式按钮:仅覆盖需区分的两态
        chinese: {background: {color: "${mode_cn_bg}"}}
        english: {background: {color: "${mode_en_bg}"}}
      # width / punct 不写 → 继承 button base(符号变、色不变)
    settings:                                 # 齿轮(矢量,自定义色)
      icon: {color: "${settings_icon}"}
      hole: {color: "${settings_hole}"}
```

Go schema（工具栏专属，不复用候选窗 `Views`）：

```go
type ToolbarViews struct {
    Background ViewFill            `yaml:"background,omitempty"`
    Border     ViewBorder          `yaml:"border,omitempty"`
    Grip       ViewNode            `yaml:"grip,omitempty"`
    Button     ToolbarButtonNode   `yaml:"button,omitempty"`
    Settings   ToolbarSettingsNode `yaml:"settings,omitempty"`
}

type ToolbarButtonNode struct {
    ViewNode `yaml:",inline"`          // base: background / color / border
    Mode     *ToolbarModeStates `yaml:"mode,omitempty"`
}

type ToolbarModeStates struct {
    Chinese ViewNode `yaml:"chinese,omitempty"`
    English ViewNode `yaml:"english,omitempty"`
}

type ToolbarSettingsNode struct {
    ViewNode `yaml:",inline"` // 齿轮按钮背景
    Icon     ViewFill `yaml:"icon,omitempty"` // 齿轮本体色
    Hole     ViewFill `yaml:"hole,omitempty"` // 中心孔色
}
```

#### 齿轮图标

保留现有矢量绘制（`drawSettingsButton`：8 齿 + 外圆 + 中心孔），**搬进 View 的自定义 paint 钩子**（类似候选窗 accent-glow 的运行时绘制）。视觉零变化、保留主题变色 + DPI 无损。资源化（位图/SVG）作为未来 schema 扩展点记下，不在 P4 做。

#### 死配置消解（取代"删字段"）

`drawWidthButton` / `drawPunctButton` 当前**无视状态参数**：全/半角永远用 `FullWidthOff*`、中/英标点永远用 `PunctEnglish*`，只有符号 (`●`/`☽`、`。，`/`.,`) 在变、颜色不变。因此 `FullWidthOnBg` / `FullWidthOn` / `PunctChineseBg` / `PunctChinese` 四个字段是死配置（全链路有定义/填充/默认值，但绘制函数从不读）。

在"base + 状态覆盖回退"模型下，这些字段**自然消解**：width / punct 按钮不写状态覆盖 → 继承 `button` base 色（与当前"只换符号不换色"行为一致）。新模型下不再需要这四个字段，P4-C 顺道清理 `theme` 包全链路：
- `theme.go`：`ToolbarColors`（legacy yaml）+ `ResolvedToolbarColors` 删 4 字段；`MustParseHexColor` 填充删 4 处。
- `resolver.go` / `resolved.go` / `palette.go`：`ResolvedToolbarPalette` 删 `FullWidthOnBg` / `PunctChineseBg`（及对应 `*Text`）。
- `adapter.go`：填充删 4 处。
- `toolbar_renderer.go`：默认值删 4 处。
- 同步 `pkg/theme/AGENTS.md`（对外结构变更）。

保留实际在用的色：通用（Background/Border/Grip）、模式中/英两背景 + 文字、设置齿轮三色。删除安全：YAML 旧主题写了被忽略（本就没生效）。

## 切片划分

按复杂度递增，逐片端到端可验证、独立 commit：

### P4-A 状态泡（最简，打通样板）
- `buildStatusTree(state, cfg)` 单行文本 View 树；`status_renderer.go:Render` 改造。
- 新增 `ResolvedStatusViews` + 合成桥（从 `ResolvedTheme.ModeIndicator` 构造默认）+ `views.status` 覆盖。
- 颜色解析器泛化（首次落地，候选窗回归保护）。
- 几何 + 颜色指纹回归网。
- 运行时 cfg 覆盖优先级保持。

### P4-B Tooltip
- `buildTooltipTree(text, maxWidth)` 多行 + 列对齐 + 截断（逻辑搬进 build）。
- `ResolvedTooltipViews` + `views.tooltip`。
- 回归网（含多行 / 列对齐 / 截断指纹）。

### P4-C 工具栏
- `buildToolbarTree(state)` 按钮行 + 齿轮 paint 钩子。
- `ToolbarViews` schema + button base + 状态覆盖（mode 中/英）+ settings。
- `ResolvedToolbarViews`，几何 + 通用色（bg/border/grip）+ settings + 模式中/英 2 色 + 文字**统一走 views token**；`${mode_cn_bg}` / `${mode_en_bg}` 等 token 默认映射到 `ResolvedTheme.Toolbar.ModeChineseBg/EnglishBg`（零回归）。build 按 `ToolbarState` 选用哪一态的解析结果（中文态用 `mode.chinese` 覆盖、英文态用 `mode.english`，缺失回退 button base）。
- 死配置 4 字段全链路清理 + `AGENTS.md` 同步。
- 回归网（含中/英模式两态指纹）。

## 回归网策略

每窗口建几何 + 颜色指纹 golden 测试，沿用候选窗 `flattenNodes`（记录 Rect + bg/bd/tx 颜色）套路：迁移前先以旧渲染路径的等价几何/颜色为基准生成 golden，迁移后比对零回归。工具栏含中/英两态各一份指纹。

## 种子主题迁移

`default` + `msime` 两个种子主题各加 `views.status` / `views.tooltip` / `views.toolbar` 块，颜色用 `${语义}` token（映射到各窗口当前 `ResolvedTheme` 子结构同源值，零回归）。

## 风险与边界

- **保留现有交互语义**：width/punct 按钮"无视状态、只换符号"的行为如实保留，不顺手改。
- **运行时覆盖优先级**：状态泡 `StatusWindowConfig` 自定义色仍优先于 views。
- **集成测试缺口**：worktree 缺 `build/data/themes` 构建产物，真实主题加载无法集成测试（同 P2）；各环节逻辑（YAML 解析 / merge / 颜色解析 / 几何）以单测覆盖，运行时由 `dev.ps1 d1` 人工验证。
- **AGENTS.md**：`pkg/theme` 对外结构变更（删 4 字段 + 新增三 Resolved 结构）需同步对应 AGENTS.md。

## 验收标准

- 三窗口渲染走 View 引擎；旧 gg 直绘 render 逻辑被 View 树取代。
- `default` / `msime` 两主题各窗口 YAML views 驱动，视觉零回归（指纹 golden + 运行时人工核对）。
- 工具栏死配置 4 字段清理完成，色 17 → 13。
- `go build` ✓ / `go fmt` ✓ / 各窗口回归网 ✓ / 候选窗既有测试不破。
- 相关 `AGENTS.md` 同步。
