<!-- Parent: theme-view-architecture.md -->

# P2 切片-1：候选窗颜色 YAML 化

P2 阶段第二个切片。承接切片-0（几何由 views 驱动），把候选窗**静态颜色**也收拢进
`ResolvedViews` 并支持 YAML `views` 配色 token，让候选窗外观完全由主题 views 驱动，
为 `adapter.go` 退役铺路。上位设计见 `theme-view-architecture.md`，切片-0 见
`theme-view-p2-slice0-views.md`。

## 一、目标与范围

**目标**：候选窗静态颜色由 `ResolvedViews` 驱动；build 不再散落读 `cfg.XxxColor`；
YAML `views` 可用颜色 token 配色（解析自 palette）；几何 + 颜色双零回归。

**本切片做**：
- `RVNode` 颜色字段填充（合成桥从 cfg / theme 从 token）+ `ResolvedViews.ShadowColor`。
- build 静态色改从 `r.resolvedViews` 读（替换散落 `cfg.XxxColor` 与 `getCommentColor/getShadowColor`）。
- theme `resolveViewColors`：views 颜色 token → `color.Color`（查 palette）。
- `applyThemeViews` 扩展覆盖颜色字段；**移除 font_size 覆盖**（字号用户全局优先）。
- `default/theme.yaml` views 块写全部静态色 token（= 现 palette 值，零回归）。
- 回归网扩展为「几何 + 颜色」指纹（办法 A）。

**本切片不做（留后续）**：
- 运行时动态色（ModeAccent blend / accent-glow 边框）——保留 build 运行时计算（基于 rv 静态色 + `cfg.ModeAccentColor`）。
- Image / layers / gradient（D5）。
- 其它种子主题（msime 等）迁移 views。
- `adapter.go` 退役（合成桥/adapter 待所有主题迁移完一并删）。

## 二、颜色字段映射（现状 → ResolvedViews）

build 当前用到的静态色，迁移到各具名 View 的 `RVNode` 颜色字段：

| 现状来源（cfg / 运行时方法） | → ResolvedViews 字段 |
|---|---|
| `cfg.BackgroundColor` | `Window.BgColor` |
| `cfg.BorderColor` | `Window.BorderColor` |
| `cfg.InputBgColor` | `PreeditBar.BgColor` |
| `cfg.InputTextColor` | `PreeditBar.TextColor` |
| `cfg.SelectedBgColor` | `Item.SelectedBg` |
| `cfg.HoverBgColor` | `Item.HoverBg` |
| `cfg.IndexBgColor` | `Index.BgColor` |
| `cfg.IndexColor` | `Index.TextColor` |
| `cfg.TextColor` | `Text.TextColor` |
| `r.getCommentColor()` | `Comment.TextColor` |
| `cfg.AccentBarColor` | `AccentBar.BgColor` |
| `r.getShadowColor()` | `ResolvedViews.ShadowColor`（顶层，新增） |

`RVNode` 已有 `BgColor/BorderColor/TextColor/SelectedBg/HoverBg`，足够承载（各 View 用其子集）；
`ResolvedViews` 新增顶层 `ShadowColor color.Color`。

## 三、三来源（沿用切片-0 模式）

- **合成桥**（`renderConfigToViews`，其它主题）：把 `cfg.XxxColor`（已是 `color.Color`，源自 palette via adapter）填入对应 `RVNode` 颜色 + `ShadowColor`。
- **`applyThemeViews`（颜色 token 在 ui 侧解析，default 经 YAML）**：`ResolvedTheme.Views`（即 renderer 持有的 `themeViews`）是 string token 形态。`applyThemeViews(rv, themeViews, candColors)` 接收 `ResolvedTheme.CandidateWindow`（已解析的 `color.Color`，作 token 查表源），把每个 views 颜色字段（hex 直值或 `${name}` token）解析为 `color.Color` 后覆盖 base 对应字段。

> 简化决策：颜色 token 解析放 **ui 侧 `applyThemeViews`**，与几何覆盖同处；token 查表源是
> `ResolvedTheme.CandidateWindow`（renderer 已持有 `resolvedTheme`，与 adapter 同源 → 零回归）。
> 不在 theme 包再建一套 `ResolvedViews` 颜色解析。

## 四、颜色 token 语法与映射

views 颜色字段（`background.color` / `color` / `border.color`）取值二选一：
- **hex 直值**：`"#RRGGBB[AA]"`（同 `ParseHexColor`）。
- **候选窗语义 token**：`"${<name>}"`，映射到 `ResolvedTheme.CandidateWindow.*`（与 adapter 同源，保证零回归）：

| token | → ResolvedTheme.CandidateWindow / Palette |
|---|---|
| `${background}` | `CandidateWindow.BackgroundColor` |
| `${border}` | `CandidateWindow.BorderColor` |
| `${text}` | `CandidateWindow.TextColor` |
| `${index_bg}` | `CandidateWindow.IndexBgColor` |
| `${index_text}` | `CandidateWindow.IndexColor` |
| `${hover_bg}` | `CandidateWindow.HoverBgColor` |
| `${selected_bg}` | `CandidateWindow.SelectedBgColor` |
| `${preedit_bg}` | `CandidateWindow.InputBgColor` |
| `${preedit_text}` | `CandidateWindow.InputTextColor` |
| `${comment}` | `CandidateWindow.CommentColor` |
| `${accent}` | `CandidateWindow.AccentBarColor` |
| `${shadow}` | `CandidateWindow.ShadowColor` |

未知 token / 空值：不覆盖（保留合成桥 base），并在 DEBUG 记录。

## 五、运行时动态色（留 build，不进 YAML）

- **ModeAccent blend**（`buildPreeditBand`）：`bgColor = blendColor(rv.PreeditBar.BgColor, cfg.ModeAccentColor, 35)`（`cfg.ModeAccentColor` 运行时模式色）。
- **accent-glow 边框**（`windowBorder`）：`cfg.ModeAccentColor` 非空时 accent 边框，否则 `rv.Window.BorderColor`。
- 这些基于 rv 静态色 + 运行时状态计算，不静态化。

## 六、字号

维持现状（用户全局 `cfg.FontSize`/`IndexFontSize` 经合成桥进 rv）；**`applyThemeViews` 移除
`FontSize`/`FontWeight` 覆盖**——主题 views 不能改字号（用户偏好/无障碍优先）。views schema
保留 `font_size` 字段但本切片不生效（文档标注）。

## 七、default theme.yaml

views 块补全部静态色 token（= compact-horizontal/windy-blue 现状语义），例：
```yaml
views:
  window:    { background: { color: "${background}" }, border: { color: "${border}", radius: 8 } }
  item:      { selected: { background: { color: "${selected_bg}" } }, hover: { background: { color: "${hover_bg}" } } }
  index:     { background: { color: "${index_bg}" }, color: "${index_text}" }
  text:      { color: "${text}" }
  comment:   { color: "${comment}" }
  accent_bar:{ background: { color: "${accent}" } }
```
（几何字段沿用切片-0 已写部分。）解析后 = 现状颜色，运行时零回归。

## 八、验收（办法 A）

- **几何 + 颜色指纹**：`flattenRects` 扩展为 `flattenNodes`——每节点记录 `Rect` + `bg/text/border` 颜色（RGBA）。横竖 scale=1 golden 零回归（位置+颜色都不变）。
- 合成桥颜色单测（cfg → rv 颜色）+ token 解析单测（`${name}` → CandidateWindow.*）+ `applyThemeViews` 颜色覆盖单测。
- `build` + `go test ./internal/ui/ ./pkg/theme/ ./pkg/config/` 全绿；`go fmt`；AGENTS.md 同步。

## 九、非目标

- ❌ 运行时动态色静态化、Image/layers/gradient、其它主题迁移、adapter 退役（后续切片）。
- ⏸ 字号 YAML 驱动（用户全局优先，本切片不做）。

## 十、关联

- `theme-view-architecture.md`（P0）、`theme-view-p2-slice0-views.md`（切片-0）
- `../../wind_input/pkg/theme/AGENTS.md`、`../../wind_input/internal/ui/AGENTS.md`（需同步）
