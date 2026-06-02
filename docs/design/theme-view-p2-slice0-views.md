<!-- Parent: theme-view-architecture.md -->

# P2 切片-0：default 主题 views schema 端到端

本文是主题渲染架构 P2 阶段的**第一个切片（切片-0）** spec，约束其实现。上位设计见
`theme-view-architecture.md`（P0），本切片只取 P2 的最小端到端子集，验证
「YAML `views` → View 树」整条链路走通，架构反馈拿到后再扩到全部种子主题与 `adapter.go` 退役。

## 一、目标与范围

**目标**：让 `default` 主题（横排 + 竖排）端到端走「YAML `views` → View 树」渲染，与现
`viewbox_build` 输出**几何对齐**。打通 schema → resolver → 渲染消费整条链路。

**本切片做**：
- `views` schema 的 Go 类型（具名 View 集合）+ YAML 解析。
- 代码内置**默认 Views 基线**（从现 `viewbox_build` magic number 精确提炼）。
- YAML 覆盖 merge（指针字段非 nil 生效，沿用 v2.5 显式语义）。
- `buildTreeFromViews`：`ResolvedViews` + palette token + 运行时数据 → `candWindowTree`（横 + 竖）。
- `default` 主题接入 views 路径；`RenderCandidates` 在该主题下走新 build。
- 序号四层优先级模型 + 用户全局序号覆盖（`config.UI.CandidateIndexLabels`）。

**本切片不做（留后续切片）**：
- `Image` / `layers` / `gradient` 的解析与绘制（`default` 无背景图；非破坏性扩展，留带图主题切片）。
- 其它种子主题（compact-horizontal / msime / msime-tight 等）迁移。
- `adapter.go` 退役（所有主题迁移完后再删）。
- 编辑器镜像（P3）。

## 二、数据流

```
theme.yaml (default)
  ├─ palette: id ───────→ ResolvedPalette            （现有，保留）
  └─ views: {…}  ──[新]→ 解析 + merge 内置默认基线 → ResolvedViews（具名 View 模板集）

RenderCandidates(候选, page, hover, cursorPos, Layout 方向, cmdbarPrefix, …运行时参数)
  └─[新]→ buildTreeFromViews(ResolvedViews, palette, 运行时数据) → candWindowTree
                                                                  → 现有 renderTree（布局/绘制/命中）
```

**单 build、双 views 来源**（实现策略，DRY）：`viewbox_build` 统一从 `ResolvedViews` 读外观，
不复刻第二套 build 逻辑。`ResolvedViews` 有两个来源：
- `default` 主题：从 YAML `views` 解析 + merge 基线。
- 其它主题：由 `RenderConfig` 合成（临时桥，类 `adapter`）。

切片期 `RenderConfig` 保留。其它主题的合成桥保证 `ResolvedViews` = 现状取值，**渲染零回归**；
合成桥与 `adapter.go` 待所有主题迁移完后一并删除（P2 后续切片）。

## 三、关键边界：View 树 = views 外观模板 ⊗ 运行时数据

- `views` 只描述**外观骨架**：margin / padding / background.color / border / states / 字体 / 颜色 token。
- **运行时数据**（候选文本、序号、注释、当前页、hover/selected 索引、横竖方向、cmdbarPrefix）
  仍从 `RenderCandidates` 参数来——它们不是主题外观。
- 序号字符是混合来源，见第五节。

## 四、views 类型（`pkg/theme/views.go`）

```text
ViewEdges  { Top, Right, Bottom, Left : *int }        # 复用 v2.5 "显式0" 指针语义
ViewFill   { Color : ColorToken }                      # Image 本切片不含
ViewBorder { Width : *int; Color : ColorToken; Radius : *int }
ViewNode   {
  Margin, Padding : ViewEdges
  Background      : ViewFill
  Border          : ViewBorder
  FontFamily      : string          # Text 属性（非 Text 节点留空）
  FontSize        : *int
  FontWeight      : *int
  Color           : ColorToken      # 文本色
  Labels          : []string        # 仅 index 节点：序号槽位字符（≤10）
  States          : { Selected, Hover : *ViewNode }   # ViewPatch=指针字段部分覆盖
}
Views { Window, PreeditBar, CandidateList, Item, Index, Text, Comment, AccentBar, FooterBar : ViewNode }
```

**默认基线 merge**：代码内置 `defaultViews()`（从现 `viewbox_build` magic number 精确提炼：
ItemHeight、各 padding、index 圆半径、accent 条宽、preedit 间距等）作基线；YAML 覆盖基线。
**merge 规则**：指针字段（`*int`，距离/圆角/边框宽）非 nil 时覆盖（保留显式 0 语义）；
string 字段（颜色 token / 字体名）非空时覆盖；slice 字段（`Labels`）非 nil 时覆盖；
`States` 子 patch 逐字段同规则递归。`default` 主题外观≈基线，其 views YAML 近乎为空，
恰好验证「基线即现状」。

**token 解析**：颜色 `${name}` 对 palette 变体解析，复用现有 `refexpand.go` / derive；本切片无图，
不涉及 resources。

## 五、序号四层优先级模型

序号字符生效优先级（高→低）：
1. `cand.IndexLabel`（运行时 per-候选覆盖，如临时拼音 a/b/c）
2. **`config.UI.CandidateIndexLabels`**（用户全局覆盖，本切片新增，空=不覆盖）
3. `views.index.Labels`（主题默认）
4. 内置默认 `1-9,0`

`config.UI` 新增字段 `CandidateIndexLabels string`（yaml `candidate_index_labels,omitempty`）。
build 解析序号时按上述优先级 resolve；`/` 分隔多字符槽位的现有规则不变。

> 历史背景：layout 早期的 `index.style` 已废弃，`labels`（槽位字符）是序号显示唯一来源，
> 「风格」只是编辑器侧填充预设。全局覆盖沿用同一 labels 模型，不引入新枚举。

## 六、验收

1. **几何对齐**：新 views 路径产出的 `candWindowTree` 各 View `Rect()` 与现 `viewbox_build`
   输出**逐项一致**（横 + 竖；覆盖 selected / hover / preedit / 翻页 / index 圆圈 / 强调条）。
   比 PNG 像素对比更可断言。实现前先以现 build 输出为基准捕获各 Rect。
2. **全局序号覆盖单测**：`CandidateIndexLabels` 非空时覆盖主题；空时回退主题/默认。
3. **build + `go test ./internal/ui/ ./pkg/theme/ ./pkg/config/` 全绿**；`go fmt`。
4. 同步 `pkg/theme/AGENTS.md`、`internal/ui/AGENTS.md`、`pkg/config` 字段说明。

## 七、非目标（YAGNI）

- ❌ Image / layers / gradient 解析与绘制（本切片）。
- ❌ 其它种子主题迁移、`adapter.go` 退役（P2 后续切片）。
- ❌ 完整任意 View 树 / data-binding（P0 D1 已排除）。
- ⏸ 编辑器镜像（P3）。

## 八、关联

- `theme-view-architecture.md` — P0 上位设计（类型契约、骨架、分阶段）
- `../../wind_input/pkg/theme/AGENTS.md` — 主题包对外接口（本切片需同步）
- `enum-constraint.md` — 全局枚举约束（本切片不新增枚举）
