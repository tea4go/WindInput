# 配置体系重构：层级重排 + version 机制 + 三态语义统一

> 状态：设计稿（待审阅定稿后实施）
> 关联：docs/design/enum-constraint.md、wind_input/pkg/config/AGENTS.md

## 1. 背景与时机

配置已完成 YAML→TOML 桥接迁移（da32d4ea），但 **TOML 格式尚未随正式版发布**。这是重排配置结构的唯一低成本窗口：

- 已发布正式版用户的配置文件全部是**旧结构 YAML**，反正要做一次"YAML→TOML"迁移，把"旧结构→新结构"熔进同一次迁移，用户侧零额外成本；
- TOML 格式一旦随正式版发布，再改层级就需要"TOML 旧结构→TOML 新结构"的第二套迁移，且 key 兼容包袱永久化。

本次一次性定稿 **version 1** 的最终格式：层级重排 + 引入 `version` 字段与版本化迁移链 + 三态语义统一。

### 现状问题（摘要）

1. **顶层分类标准不统一**：`ui`/`toolbar` 按界面区域、`input`/`hotkeys` 按行为域、`s2t`/`stats` 按功能、`advanced` 兜底——四种分类法混用，新配置项无明确归属规则。
2. **UIConfig 巨型化**：30+ 字段混杂字体渲染、候选窗布局、主题覆盖、状态提示、零散开关五个子域。
3. **"未设置"有三种并存编码**：`*bool`+omitempty（nil=默认 true）、值 bool 禁止 omitempty（FollowTheme 族，靠长注释守护）、零值哨兵（`0=禁用`）。曾导致 fontsize 探针迁移坑。
4. **迁移靠启发式**：无版本号，`migrateQuickInputConfig`/`migrateStatusIndicatorConfig`/`theme:"dark"` 迁移靠"字段为零/为空"探测，不可枚举、deprecated 字段永久陪葬。
5. **归属错位**：`advanced.host_render_processes` 本质是按进程兼容规则；`status_indicator_*` 三个旧字段连 `data/config.toml` 自己还在用。

## 2. 设计原则

### 2.1 分类原则：用户心智域 = 设置页

**一个配置项放在哪个 section，由"用户在设置界面哪一页找它"决定。** 顶层 section 与设置页一一对应，归属争论就此消失。

`input` 与 `features` 的边界判定：

- **input**：与按键处理流水线紧耦合的行为（打字过程中的按键语义）——标点、选键、回车行为、临时拼音、临时英文、自动配对等；
- **features**：可整体拔掉而不影响基础打字的自包含增值功能（通常有 enabled 总开关或 trigger_keys=空即关闭）——统计、简入繁出、快捷输入、特殊模式、命令直通车。

### 2.2 文件层级：二维矩阵（成文约定，本次不动文件划分）

| | 全局 | 按方案 | 按外部应用 |
|---|---|---|---|
| **用户意图**（手改/设置页） | config.toml | schema_overrides.toml | compat.toml |
| **程序状态**（自动写） | state.toml | user_data.db | state.toml（pin 位置） |

判据：**谁写入**决定文件族（防止用户编辑覆盖运行时数据），**作用域**决定具体文件。`host_render_processes` 按此矩阵属"按外部应用的用户意图"，但因其有设置页入口且与 compat.toml 的 `[[apps]]` 规则粒度不同（白名单 vs 规则集），折中放 config.toml 的 `[compat]` 节（见 §3）。

### 2.3 三态语义规范（替代现有三种编码）

> 本节经审阅迭代过一次：初稿规则为"默认 true 的 bool 一律指针化"，后发现这是过度设计——**diff-save 模型下值类型不加 omitempty 本来就正确**（用户改值≠base → 写出；未改 → diff 掉 → 键缺失 → 加载回默认；FollowTheme 族现状即此模式且运行良好）。指针化的唯一增量收益是区分"显式设置成默认值 vs 从未设置"，兑现它要求 nil 贯穿内存 → JSON 下发 null → 设置前端全量 null 处理 + 默认值知识，为一个极边缘能力（系统默认翻转时钉住用户意图）付出全局成本。故规则修正如下。

统一规则（可机检）：

| 规则 | 内容 |
|---|---|
| **R1** | 配置 struct **禁用指针类型**。"未设置=继承默认"由磁盘**键缺失**表达，由 diff-save（写端）+ DefaultConfig/三层合并（读端）天然承担，不需要内存里的 nil。 |
| **R2** | 标量字段（bool/数值/string/枚举）**一律不加 omitempty**——omitempty 基于 Go 零值而非默认值，会把显式 false/0/"" 丢键。slice/map 豁免（nil 与空集语义一致）。 |
| **R3** | 确需"未设置 ≠ 一切显式值"的字段，用**显式枚举值**表达而非指针——项目已有成熟先例：hotkeys 的 `"none"`（禁用）、`pinyin_separator` 的 `"auto"`、`theme_style` 的 `"system"`、`pager_bar_display` 的 `""`（跟随主题）。bool 需要三态时升格为字符串枚举。TOML 无 null，枚举比指针自文档化且无解引用风险。 |
| **R4** | 零值哨兵仅当"零值本身是合法语义"时保留（`retain_days=0` 永久、`per_page_extended=0` 禁用、`max_readings=0` 不限）；"零值=回退默认"类哨兵退役（`MaxCandidateChars` 0→16 改为默认 16 + clamp）。 |

- **核心红线：bool 永不为 null**。任何场景下 bool 字段都是值类型二态；若某 bool 将来真需要第三种状态，按 R3 升格为字符串枚举，而不是指针化。
- **配套实践：默认值显式落在系统预置层**。`data/config.toml`（Layer 2）显式写出全部面向用户的默认值（如 `candidate_prefix = "⚡"`），自文档化且发布后可调（无需重编译）；代码 `DefaultConfig()`（Layer 1）保持完整兜底（预置文件丢失时程序仍可用）。diff-save 的 base 本就是 Layer1+Layer2 合并结果，默认值前移到 Layer 2 不改变任何往返行为。
- **防漂移测试**：反射扫描 Config 全树，断言"无指针字段"+"标量字段无 omitempty（slice/map 豁免）"，让规则机器执行。规则极简，测试也极简。
- **收益**：`IsXxx()` accessor 样板全部退役；nil 解引用风险归零（原计划的 ast-grep 裸访问扫描不再需要）；`marshalTOML` 的防御性 nil 剔除失去存在前提；设置前端类型保持非 nullable，零 null 处理成本。
- 现有 9 个指针字段全部退指针化，逐项核查见 §5。

> 不熔合 FollowTheme 配对字段为单字段（如 `font_size` 单值 nil/auto=跟随）的原因：设置页 UX 需要"关掉跟随后记住用户上次的自定义值"，单字段会在切回跟随时丢失记忆。保留 `值字段 + bool follow` 双字段模式（两个都是值类型）。

## 3. version 1 目标层级（全量）

```toml
version = 1

[general]                        # ← 原 [startup]
remember_last_state = false
default_chinese_mode = true
default_full_width = false
default_chinese_punct = true

[schema]                         # 不变
active = "wubi86"
available = ["wubi86", "wubi86_pinyin"]
# primary_codetable / primary_pinyin（可选）

[hotkeys]                        # key 路径不变。注意：toggle_s2t / take_screenshot 在现有
                                 # data/config.toml 中缺失（只活在代码默认），v1 重写预置文件时
                                 # 按 §2.3 配套实践补齐显式列出
toggle_mode_keys = ["lshift", "rshift"]
commit_on_switch = true
switch_engine = "ctrl+shift+e"
toggle_full_width = "shift+space"
toggle_punct = "ctrl+."
delete_candidate = "ctrl+shift+number"
pin_candidate = "ctrl+number"
toggle_toolbar = 'ctrl+shift+\'
open_settings = "ctrl+shift+]"
add_word = "ctrl+equal"
toggle_s2t = "ctrl+shift+j"
take_screenshot = "ctrl+shift+f11"
global_hotkeys = []

[input]                          # 按键流水线行为
punct_follow_mode = false
filter_mode = "smart"
select_key_groups = ["semicolon_quote"]
page_keys = ["pageupdown", "minus_equal"]
highlight_keys = ["arrows", "tab"]
select_char_keys = []
smart_punct_after_digit = true
smart_punct_list = ".,:"
enter_behavior = "commit"
space_on_empty_behavior = "commit"
numpad_behavior = "direct"       # 既有字段（现仅存在于代码默认），路径不变，预置文件补齐显式列出
pinyin_separator = "auto"

[input.shift_temp_english]       # 不变
[input.capslock]                 # ← 原 input.capslock_behavior（去 _behavior 后缀）
[input.temp_pinyin]              # 不变 + 吸收 ui.temp_pinyin_accent_color → accent_color
[input.auto_pair]                # 不变
[input.punct_custom]             # 不变
[input.overflow]                 # ← 原 input.overflow_behavior
[input.phrase]                   # 不变

[ui]                             # （顶层不留散字段）

[ui.candidate]                   # 候选窗布局与行为
font_size = 18                   # + font_size_follow_theme（值 bool，true=跟随主题）
per_page = 7                     # ← candidates_per_page
per_page_extended = 0            # ← candidates_per_page_extended（0=禁用，去 omitempty）
max_chars = 16                   # ← max_candidate_chars（哨兵退役，默认 16）
layout = "horizontal"            # ← candidate_layout
inline_preedit = true
preedit_mode = "top"
flip_when_above = false          # ← flip_layout_when_above
hide_window = false              # ← hide_candidate_window
index_labels = ""                # ← candidate_index_labels
mode_accent_border = true        # ← ui.mode_accent_border（值 bool，§5.1）
# behavior 覆盖层：FollowTheme 族 4 对（值字段 + 独立 bool follow 键，true=跟随主题，全部值类型）
font_size_follow_theme = true            # 与上方 font_size 配对
always_show_pager = false
always_show_pager_follow_theme = true
show_page_number = true
show_page_number_follow_theme = true
vertical_max_width = 600
vertical_max_width_follow_theme = true
# 强制覆盖枚举（空=不覆盖，在 follow 层之后应用——两组是分层关系，本次保留，见 §7）：
pager_bar_display = ""
page_number_display = ""

[ui.font]                        # 字体与文本渲染
family = ""                      # ← font_family
path = ""                        # ← font_path
render_mode = "directwrite"      # ← text_render_mode
gdi_weight = 500                 # ← gdi_font_weight
gdi_scale = 1.0                  # ← gdi_font_scale
menu_weight = 500                # ← menu_font_weight
menu_size = 12.0                 # ← menu_font_size

[ui.theme]
name = "default"                 # ← ui.theme
style = "system"                 # ← theme_style
editor_auto_start = false        # ← theme_editor_auto_start

[ui.status_indicator]            # 原 ui.status_indicator 子结构整体平移
# enabled/duration/display_mode/schema_name_style/show_mode/show_punct/
# show_full_width/position_mode/offset_x/offset_y/custom_x/custom_y/
# font_size/opacity/background_color/text_color/border_radius
# （ui.status_indicator_duration/_offset_x/_offset_y 三个旧顶层字段：struct 删除，v0→v1 迁移吸收）

[ui.tooltip]                     # 原 ui.tooltip 子结构平移 + 吸收 tooltip_delay
delay = 200                      # ← ui.tooltip_delay
[ui.tooltip.code]                # enabled（值 bool 默认 true，§5.1）
[ui.tooltip.pinyin]              # enabled/heteronyms/max_readings
[ui.tooltip.chaizi]
[ui.tooltip.debug]

[ui.toolbar]                     # ← 原顶层 [toolbar]
visible = true
hide_in_fullscreen = true        # 值 bool 默认 true（§5.1）

[features]                       # 自包含可选功能

[features.stats]                 # ← 原顶层 [stats]
enabled = true                   # 值 bool 默认 true（§5.1）
retain_days = 0                  # 0=永久（合法语义值，R4 保留）
track_english = true             # 值 bool 默认 true（§5.1）

[features.s2t]                   # ← 原顶层 [s2t]
enabled = false
variant = "s2t"

[features.quick_input]           # ← 原 input.quick_input（deprecated 字段 enabled/trigger_key 删除）
trigger_keys = ["semicolon"]
force_vertical = true
decimal_places = 6
accent_color = ""                # ← ui.quick_input_accent_color

[[features.special_modes]]       # ← 原 input.special_modes
# id/name/trigger_keys/table/auto_commit/fixed_length/force_vertical/
# accent_color/show_all_on_entry + 预留 code_charset/schemes/engines

[features.cmdbar]
candidate_prefix = "⚡"          # ← ui.cmdbar_candidate_prefix（值 string 默认"⚡"，""=不显示，伪三态退指针见 §5.1）

[compat]                         # 进程级兼容（与 compat.toml 概念对齐）
host_render_processes = ["SearchHost.exe"]   # ← advanced.host_render_processes

[debug]                          # ← 原 [advanced]
log_level = "info"
perf_sampling = false            # 值 bool 默认 false（§5.1；隐私提示属文档/UI 层职责）
```

归属说明（审阅关注点）：

- **temp_pinyin 留 input、quick_input/special_modes 进 features**：三者都是引导键触发，但临时拼音是输入流水线内的引擎切换（z 键语义、编码包含规则与码表流深耦合），后两者是可整体拔掉的独立功能。
- **s2t 进 features 而非 input**：有 enabled 总开关、输出侧转换、可整体禁用，符合 features 判定；其热键绑定 `toggle_s2t` 留在 hotkeys。
- **accent_color 按功能就近**（`input.temp_pinyin.accent_color`、`features.quick_input.accent_color`），与 special_modes 每实例自带 accent_color 的既有模式一致；总开关 `mode_accent_border` 是候选窗视觉，留 `ui.candidate`。
- **PinyinConfig/FuzzyPinyinConfig**（未挂 Config，是 config↔engine 适配中介）：随本次移出 `pkg/config`，迁至 `internal/engine` 侧（消费方 `engine/manager_config.go`、`coordinator/reload_handler.go`）。`pkg/config` 只留应用级配置。
- **§3 树中的值 = Layer 2 预置层取值**，不代表 `DefaultConfig()`（Layer 1 兜底）的值。两层既有分叉（如 `schema.available`：预置 `["wubi86","wubi86_pinyin"]` vs 代码 `["wubi86","pinyin"]`；`toggle_toolbar`：预置 `'ctrl+shift+\'` vs 代码 `"none"`）是分层设计的正常形态（Layer 2 本就为覆盖 Layer 1 而存在），**不是笔误**。实施时重写 DefaultConfig 须保持 Layer 1 现值不变，仅做路径重排；分叉项在 data/config.toml 中加注释说明。

## 4. version 机制

### 4.1 版本定义：格式即版本边界

- **version 0 = YAML**：一切 `.yaml` 旧文件（已发布正式版用户）。YAML 文件永远不会有 version 键，格式本身就是 v0 的判别依据。
- **version 1 = TOML 新结构**：TOML 尚无任何正式用户，内测期旧结构 TOML 不提供迁移路径（开发机残留按损坏自愈分支处理即可）。
- 顶层裸键 `version = 1`。置顶依据是 **go-toml v2 的 marshal 行为**（map 序列化时顶层标量恒先于表输出，已实测），并非 TOML 格式天然如此（TOML 仅要求裸键位于其所属表 header 之前）——codec 测试断言 marshal 输出首键为 version，将该库行为锁定，未来换 marshal 库需重验。

判别规则（按序；实施修订：**显式 version 键最高优先**，缺失时才按格式判定——SaveTo 对 YAML 路径也注入 version，带 version 的 YAML 按其值处理不重迁移）：

1. 文件含显式 `version = N`（1 ≤ N ≤ current）→ 从 N 起执行迁移链（与扩展名无关）；
2. `version` 缺失且来源为 `.yaml` → **v0**，走 `migrateV0toV1` + 格式迁移；
3. `version` 缺失且来源为 `.toml` → **按 v1 处理**（安全默认）。因为 v0 只可能是 YAML，TOML 缺 version 必然是用户手编时误删——不执行任何迁移，下次 diff-save 自动补写 `version = 1`。这天然规避了"误删 version 导致 v1 文件被当 v0 重迁移"的风险，无需特征键探针；
4. `version > current`（程序回滚）→ 降级防护：备份 `.bak` + 回退默认 + 警告。

> 未来引入 v2 时规则 2 需重新评估（届时"缺 version 的 TOML"可能是 v1 或 v2 手编损伤），可在 v2 迁移函数中用 v2 独有特征键做探针——这是 v2 的问题，v1 不预支复杂度。

**辅助文件不写 version（v1 范围收窄）**：初稿曾约定 `state.toml`/`compat.toml`/`schema_overrides.toml` 同步写 `version = 1`，审查发现 `schema_overrides.toml` 顶层是 `map[schemaID]→覆盖项`，裸 `version` 键会被解读为一个名为 "version" 的方案 ID，污染命名空间且解码报错——物理不可行。统一收窄为：**v1 仅用户 `config.toml` 携带 version**；三个辅助文件本次无结构变化、不写 version，未来各自需要结构迁移时再引入（state/compat 顶层是 struct 可直接加键；schema_overrides 届时需包一层 `{version, schemas}` 结构，属于那次迁移自身的工作）。系统预置 `data/config.toml` 也**不写** version——它随包分发、不走迁移，version 的语义锚点保持"用户文件迁移状态"单一职责。

### 4.2 迁移链：map 层、版本驱动

迁移在**桥接管线的 map 层**做（`normalizeToYAML` 之后、`yaml.Unmarshal` 进 struct 之前）：

```go
// codec.go 新增
type configMigration func(m map[string]any)
var configMigrations = map[int]configMigration{
    0: migrateV0toV1,   // key 重映射 + 旧字段语义熔合
}

// fromLegacyYAML：来源是否为 .yaml 回退文件（§4.1 格式即版本边界）
func migrateConfigMap(m map[string]any, fromLegacyYAML bool) (migrated bool) {
    v := currentConfigVersion          // TOML 缺 version 的安全默认 = 当前版本起点 v1
    if fromLegacyYAML {
        v = 0                          // YAML 永远是 v0
    } else if n, ok := safeGetInt(m, "version"); ok {
        v = n
    }
    for ; v < currentConfigVersion; v++ {
        configMigrations[v](m)
        migrated = true
    }
    m["version"] = currentConfigVersion
    return
}
```

注意：**Config struct 不含 version 字段**——version 只存活于磁盘文件与 map 层（驱动迁移循环、SaveTo 注入），unmarshal 进 struct 时被自然丢弃。这保证内存配置对象与版本机制完全解耦。

选 map 层而非 struct 层的理由：

1. **struct 上不留尸体**——`QuickInputConfig.Enabled/TriggerKey`、`UIConfig.StatusIndicator{Duration,OffsetX,OffsetY}` 等 deprecated 字段从 struct 彻底删除，新代码不可能再引用；
2. 旧 key 只在迁移函数里以字符串出现一次，可单测可枚举；
3. 与桥接架构契合：TOML/YAML 都先归一化为 map，迁移对两种来源格式天然同构。

`migrateV0toV1` 内容 = §6 映射表逐条搬键 + 熔合现有三个启发式迁移（`migrateQuickInputConfig`、`migrateStatusIndicatorConfig`、`theme:"dark"`；实施时核实 FontSizeFollowTheme 旧探针已移除，缺失=默认 true，直接平移）。迁移后 `ApplyConfigFallbacks` 只剩值域 clamp 类兜底，所有 `migrate*` 函数删除。

**map 弱类型防护**：迁移函数禁止裸类型断言（`m["x"].(bool)` 一旦类型不符即 panic），统一走 `safeGetBool/safeGetInt/safeGetString/safeGetMap` 辅助函数——断言失败返回零值+false，迁移按"该键缺失"处理（用户手编脏数据降级为丢该项，不崩启动）。注意按 §4.1 规则，v0→v1 的输入 map **只来自 yaml.v3 解析**（v0 必为 YAML），数字类型面单一（int），无需处理 go-toml 的 int64 差异；但辅助函数仍做宽容数值归一（int/int64/float64），与 `yamldiff.go` 的 `toFloat64` 同策略，防未来迁移函数复用时踩坑。

### 4.3 与现有加载/保存流程的交集

- **加载**（`LoadFrom` 三层合并）：Layer 2 系统预置 `data/config.toml` 随包分发直接改为 v1 结构（不走迁移）；Layer 3 用户文件 normalize 后先 `migrateConfigMap` 再 unmarshal。检测到 `migrated=true` 时复用现有"加载成功即写出 TOML"通路，但**不再改名旧文件**（§4.4 修订：旧 YAML 保留原地，`renameLegacyFile` 在 config/state 迁移成功路径上的调用移除）。
- **保存**（diff-save）：`version` 是元数据不参与 diff——`SaveTo` 在 `ComputeYAMLDiff` 产出 map 后**强制注入** `version = currentConfigVersion` 再 marshal。空 diff 也写出 version（文件至少含一行，可作迁移完成标记）。
- **降级防护**：加载遇到 `version > currentConfigVersion`（用户回滚旧版程序）时按现有"损坏"分支处理：备份 `.bak` + 回退默认并警告，不尝试解读未来格式。

### 4.4 混版本共存（网盘同步场景）

真实场景：用户经网盘（OneDrive/Dropbox）在多设备间同步配置目录，设备 A 已升级（v1 TOML），设备 B 仍是旧版（只认 `config.yaml`）。

现行迁移行为（旧 YAML 改名 `*.migrated.bak`）在此场景下有体验劣化：A 迁移后 `config.yaml` 消失 → 同步到 B → **旧版程序找不到配置，回退默认**。

**修订决策：v0→v1 迁移成功后保留旧 `config.yaml` 原文件，不再改名 `.migrated.bak`。**

- 无重复迁移问题：`readFileWithLegacyFallback` 本就 TOML 优先，`config.toml` 存在时旧 YAML 根本不会被读，保留它零成本；
- 混版本期设备 B 旧版继续读写 `config.yaml`，不退默认；设备 A 新版只认 `config.toml`——两端各自工作；
- 已知且接受的局限：混版本期两文件**配置分叉**（B 上的修改不会进 A 的 TOML）。这是同步场景的固有矛盾，任何方案都无法既让旧版可写又让新版吸收；B 升级后其 YAML 也不会再次迁移（TOML 已存在），分叉的 B 端改动丢弃——可接受，因为该场景下 A 端 TOML 才是用户最近主力配置的概率更高，且旧 YAML 仍在原地可手工找回；
- 损坏自愈分支维持现状（损坏文件仍备份 `.bak`）；
- 旧 YAML 的清理留给未来版本（如 v2 时或间隔一年的清理逻辑），v1 不删用户文件。

`state.toml` 等同理改为保留旧文件。（注：state 含按显示器的窗口位置，跨设备同步本身就语义存疑，超出本设计范围。）

## 5. 三态统一字段清单：现有指针字段逐项核查与退指针化

### 5.1 "真三态"核查（指针存留的唯一理由）

判据：**nil 是否承载任何值类型表达不了的语义**。逐项核查现有 9 个指针字段：

| 字段（v0） | 现类型 | 表面语义 | 核查结论 | v1 落位 |
|---|---|---|---|---|
| stats.enabled | `*bool` | nil=true | 二态。历史防御注释（"避免 yaml 反序列化归零"）在"unmarshal 进已填充 struct"管线下不成立 | 值 bool 默认 true |
| stats.track_english | `*bool` | nil=true | 同上 | 值 bool 默认 true |
| ui.tooltip.code.enabled | `*bool` | nil=true（注释"向后兼容"） | 二态，兼容语义由 v0→v1 迁移吸收 | 值 bool 默认 true |
| toolbar.hide_in_fullscreen | `*bool` | nil=true | 二态 | 值 bool 默认 true |
| ui.mode_accent_border | `*bool` | 注释称 nil=开，**实测消费方（coordinator.modeAccentColor）nil=关** | 二态；实施时核实：以消费方行为为准，默认关 | 值 bool 默认 false |
| input.temp_pinyin.z_include_on_commit | `*bool` | nil=true | 二态（内部预留开关） | 值 bool 默认 true |
| advanced.perf_sampling | `*bool` | nil=false | 二态且默认即 Go 零值，最平凡 | 值 bool 默认 false |
| input.quick_input.enabled | `*bool` | deprecated | — | 删除（迁移熔合） |
| ui.cmdbar_candidate_prefix | `*string` | nil=默认"⚡"、""=不显示、其他=自定义 | **表面三态，实为伪三态**：值 string 默认 "⚡" 完全等价——磁盘缺键=默认"⚡"（diff-save），显式 ""=不显示（R2 禁 omitempty 保证 "" 不丢键）。nil 与显式 "⚡" 的行为差异（内置默认未来变化时是否跟随）正是 §2.3 论证过的边缘能力 | 值 string 默认 "⚡" |

**核查结果：真三态集合为空，v1 配置 struct 实现零指针。**

伪三态的通用判别（写给未来新增字段）：凡"nil=默认 X、显式值=覆盖"且 X 是固定值的，都是伪三态——"缺键=默认"由 diff-save 天然表达；只有"未设置时跟随一个**运行时动态源**（如主题、另一字段）且任何静态值都无法表达跟随"才是真三态，而这种场景按 R3 用枚举哨兵值（`"auto"`/`""`）或双字段（值+follow 开关）表达，仍然不需要指针。

### 5.2 其余类型调整

| 字段（新路径） | 调整 | 说明 |
|---|---|---|
| FollowTheme 族 4 个（ui.candidate.*_follow_theme） | 保持值 bool，删除"不可 omitempty"防御注释 | R2 成为全局规则后特例消失 |
| ui.candidate.max_chars | int 哨兵退役，默认 16 + clamp | R4 |
| ui.candidate.per_page_extended | 去 omitempty，0=禁用语义保留 | R4（0 是合法语义值） |
| input.phrase.min_prefix_length | 去 omitempty，默认 2 + clamp(≥1) | R4（0 非合法值，哨兵退役） |
| 全部标量字段 | yaml/json tag 统一去 omitempty | R2；slice/map 豁免 |

`IsXxx()` accessor 全部删除，消费方改直读字段（编译器捕获全部调用点）；`Clone()` 反射深拷贝自动适应；防漂移测试断言"无指针 + 标量无 omitempty"。

## 6. v0→v1 key 映射表（迁移函数实现依据）

未列出 = 路径与字段名均不变。

| v0 路径 | v1 路径 | 备注 |
|---|---|---|
| startup.* | general.* | section 改名 |
| input.capslock_behavior.* | input.capslock.* | |
| input.overflow_behavior.* | input.overflow.* | |
| input.quick_input.{trigger_keys,force_vertical,decimal_places} | features.quick_input.* | |
| input.quick_input.enabled / trigger_key | （删除） | 熔合旧启发式：enabled=false→trigger_keys=[]；trigger_key 非空且 trigger_keys 空→搬入 |
| input.special_modes | features.special_modes | 数组整体平移；实例内全部字段（含预留 code_charset/schemes/engines）同名不动 |
| stats.* | features.stats.* | |
| s2t.* | features.s2t.* | |
| toolbar.* | ui.toolbar.* | |
| advanced.log_level / perf_sampling | debug.* | |
| advanced.host_render_processes | compat.host_render_processes | |
| ui.font_size / font_size_follow_theme | ui.candidate.* | 直接平移。（实施时核实：旧"老用户缺失→false"探针已于现行代码移除，缺失=继承默认 true，无需启发式） |
| ui.candidates_per_page | ui.candidate.per_page | |
| ui.candidates_per_page_extended | ui.candidate.per_page_extended | |
| ui.max_candidate_chars | ui.candidate.max_chars | |
| ui.candidate_layout | ui.candidate.layout | |
| ui.inline_preedit / preedit_mode | ui.candidate.* | |
| ui.flip_layout_when_above | ui.candidate.flip_when_above | |
| ui.hide_candidate_window | ui.candidate.hide_window | |
| ui.candidate_index_labels | ui.candidate.index_labels | |
| ui.mode_accent_border | ui.candidate.mode_accent_border | |
| ui.always_show_pager(_follow_theme) | ui.candidate.* | |
| ui.show_page_number(_follow_theme) | ui.candidate.* | |
| ui.vertical_max_width(_follow_theme) | ui.candidate.* | |
| ui.pager_bar_display / page_number_display | ui.candidate.* | |
| ui.font_family | ui.font.family | |
| ui.font_path | ui.font.path | |
| ui.text_render_mode | ui.font.render_mode | |
| ui.gdi_font_weight / gdi_font_scale | ui.font.gdi_weight / gdi_scale | |
| ui.menu_font_weight / menu_font_size | ui.font.menu_weight / menu_size | |
| ui.theme | ui.theme.name | 含 theme:"dark"→name:"default"+style:"dark" 熔合 |
| ui.theme_style | ui.theme.style | |
| ui.theme_editor_auto_start | ui.theme.editor_auto_start | |
| ui.tooltip_delay | ui.tooltip.delay | |
| ui.tooltip.* | ui.tooltip.*（子表平移） | |
| ui.status_indicator.* | ui.status_indicator.*（平移） | |
| ui.status_indicator_duration / _offset_x / _offset_y | ui.status_indicator.duration / offset_x / offset_y | 仅当 `ui.status_indicator` 子 map 中对应**键不存在**时回填（map 层判定=键缺失，显式 0 不回填——与旧 struct 层"值==0 视为未设"判定不同，map 层语义更精确）（熔合 migrateStatusIndicatorConfig） |
| ui.temp_pinyin_accent_color | input.temp_pinyin.accent_color | |
| ui.quick_input_accent_color | features.quick_input.accent_color | |
| ui.cmdbar_candidate_prefix | features.cmdbar.candidate_prefix | |

## 7. 影响面与同步改动

### 7.1 Go 侧

- `pkg/config`：Config struct 全树重排、`DefaultConfig()`、`ApplyConfigFallbacks` 瘦身、codec 迁移链、防漂移测试；`PinyinConfig`/`FuzzyPinyinConfig` 移出。
- **RPC 点路径**（`internal/rpc/config_service.go` `resolveKeyPath`/`setNestedKey`）：前端 `ConfigSetItem.key` 是点路径字符串，随新路径整体换名；RPC 测试用例同步。
- **热重载 section 路由**（`coordinator/reload_handler.go` `changedSections`）：顶层 section 集合变为 `general/schema/hotkeys/input/ui/features/compat/debug`；`UpdateStartupConfig`→`UpdateGeneralConfig`，`UpdateStatsConfig`/`UpdateS2TConfig` 改挂 features 路由，`UpdateToolbarConfig` 改挂 ui 路由。
- `coordinator/handle_config.go` 各 `Update*` 方法签名内字段访问路径机械替换（编译器全量捕获）。
- `data/config.toml` 重写为 v1 结构（顺带消灭其中的 status_indicator 旧字段用法），并按 §2.3 配套实践显式列全面向用户的默认值（含 `features.cmdbar.candidate_prefix = "⚡"` 等原先只存在于代码默认的项）。

### 7.2 前端（wind_setting/frontend）

消费高度集中（schema 驱动 + 点路径），同步面：

1. `src/api/settings.ts`：Config TS 接口全树重排（TS 编译器捕获类型引用）；
2. `src/schemas/*.schema.ts`：约 47 处 `key: "..."` 点路径字符串，按 §6 映射表驱动替换；
3. 各 Vue 页面少量 `formData.xxx` 硬编码访问（`HotkeyPage.vue` 8 处等）；
4. `searchIndex.ts` / `pages/*.search.ts` 搜索索引中的 key 引用。

**替换与验收护栏**（防部分匹配误伤与漂移）：

- **不引入前端 LegacyKeyMap 运行时映射层**：旧 key 会以"兼容层"形式永久驻留前端，违背本次"不留尸体"原则；且 Go 侧已在 map 层一次性迁移，前端没有旧 key 数据源，映射层无对象可映射。
- **替换以完整带引号字面量为单位**：对 schema key 替换 `'ui.tooltip_delay'` 整串（含引号），并按 key 长度降序处理，前缀 key（`ui.tooltip`）不可能误伤更长 key（`ui.tooltip_delay`）——纯子串替换确实危险，整串替换则不存在该问题。
- **tsc 是主要捕获网**：`formData` 保持严格的 `Config` 类型（不得 any 化），接口重排后所有旧路径属性链访问（`formData.ui.tooltip_delay`）直接编译报错，"动态访问不报错"仅剩 `getPath/setPath` 字符串路径一类。
- **getPath/setPath 开发期警告**：对解析不到的路径 `console.warn`（缺失路径本就该警告），运行设置页即暴露漏改的字符串 key。
- **端到端 key 一致性校验**（切片 3 验收核心）：Go 侧新增测试工具，反射遍历 Config yaml tag 树导出全量 v1 点路径清单（写入 testdata 或 go:generate 产物）；前端测试断言 schema 文件与搜索索引中出现的全部 key ∈ 该清单。任何拼写漂移在 CI 一次性抓出，不依赖人工全文搜索作为最终验收。

### 7.3 明确不做（标记为后续候选）

- **pager 两组配置合并**：`always_show_pager(+follow)` 与 `pager_bar_display` 经查证是分层覆盖关系（behavior 覆盖层先应用，枚举层后强制覆盖，`renderer.applyBehaviorOverrides` / `manager.applyPagerOverride`），两组均有活跃消费。语义上仍有冗余，但合并涉及渲染行为变化，不搭本次格式重排的车；v1 仅按新路径搬家。
- **schema_overrides.toml 类型化**、**filter_mode 应用层/方案层归一**：属方案配置体系演进，另案处理。
- **compat.toml 与 state.toml 的 pin 数据耦合**：维持现状。

## 8. 实施切片

| 切片 | 内容 | 验收 |
|---|---|---|
| 0 | codec：`version` 读写 + `migrateConfigMap` 迁移链骨架 + 降级防护；SaveTo 强制注入 version | codec_test 扩展：version 往返、超版本回退 |
| 1 | Config struct 重排 + 三态规范落地（9 指针字段退值类型 + accessor 删除 + 全量去 omitempty）+ DefaultConfig/Fallbacks 重写 + `migrateV0toV1` + `LoadFrom` 整合（含 §4.4：config/state 迁移成功后**不再调用** `renameLegacyFile`，保留旧 YAML）+ data/config.toml 重写 + PinyinConfig 移出。内部子顺序：先 struct+DefaultConfig 定形 → 再迁移函数（二者互为契约）→ 最后预置文件与测试 | 全量 go test；新增防漂移反射测试；旧结构 YAML 真实样本迁移 round-trip 测试 |
| 2 | RPC 点路径 + reload_handler 路由 + coordinator Update* 适配 | go build 全仓 + rpc/coordinator 测试 |
| 3 | 前端：settings.ts 接口 + schema key + Vue 硬编码 + 搜索索引 | 前端构建 + 设置页全页面手测（含搜索跳转） |
| 4 | 真机验证：旧版 YAML 用户配置升级路径、diff-save 闭环（改一项→文件只含该项+version） | 手测 + 检查 config.toml 生成且旧 config.yaml 保留原地（§4.4） |

切片 0/1 必须同一分支连续完成（struct 与迁移函数互为契约）；2/3 可并行。

## 9. 关键测试清单

1. **迁移保真**：构造覆盖映射表全部条目的 v0 YAML 样本 → Load → 断言新 struct 各字段值与旧语义一致（含 quick_input 启发式、theme:"dark"、status_indicator 回填、font_size_follow 老用户=false）；迁移成功后旧 YAML 保留原地、TOML 写出（§4.4）。
2. **脏数据宽容**：v0 样本中故意放类型错误的键（字符串放进 bool 位等）→ 迁移不 panic，该项按缺失处理（safeGet 防护）。
3. **diff-save 闭环**：每个"默认 true 的 bool"显式 false、每个"默认非空的 string"显式 ""（如 cmdbar.candidate_prefix）→ Save → 重新 Load 值不变（防 omitempty 丢键回归）；显式值=默认 → 文件不含该键；空 diff 也写出 version。
4. **version 规则**：marshal 输出首键必为 `version`（置顶断言，防 go-toml 键序回归）；TOML 缺 version → 按 v1 加载、不执行迁移、下次保存补写；version=2 文件 → 备份 + 回退默认 + 不崩溃。
5. **三层合并**：系统预置改默认 → 用户未 touch 字段跟随、touch 过字段不动。
6. **防漂移**：反射断言"Config 全树无指针字段"、"标量字段无 omitempty（slice/map 豁免）"（§2.3/§5）。
7. **key 一致性**：Go 反射导出 v1 全量点路径清单，前端 schema/搜索索引 key 必须 ∈ 清单（§7.2）。

## 10. wind_setting：本次变化与演进路线

### 10.1 本次变化（与 §7.2 互补，按工作项视角）

| 改动面 | 内容 | 捕获网 |
|---|---|---|
| `api/settings.ts` | Config TS 接口全树重排 + `getDefaultConfig()` 同步 | tsc 报错全部旧路径 |
| `schemas/*.schema.ts` | ~47 处 key 点路径按 §6 映射表整串替换 | CI key 一致性校验 |
| Vue 页面硬编码 | `formData.xxx` 属性链访问修正 | tsc（formData 严格 Config 类型） |
| 搜索索引 | `searchIndex.ts` / `pages/*.search.ts` key 同步 | CI key 一致性校验 |
| **页面分组** | 按"设置页=配置节"原则重组：新增"扩展功能"页聚合 features.*（stats/s2t/quick_input/cmdbar；special_modes 暂无 UI 留位）；advanced 页对应拆为 debug/compat 分区 | 手测 |
| RPC 层 | 机制不变（点路径 patch），仅 key 字符串变 | rpc 测试 |
| 三态处理 | **零额外成本**（§5.1 零指针决议的直接收益）：JSON 下发永远是已解析的非 null 值，前端类型保持非 nullable；现有零星 `boolean \| null` 标注（perf_sampling 等）随退指针化收敛为 `boolean` | tsc |

### 10.2 演进路线（不在本次范围，按依赖顺序）

1. **单一事实源代码生成**：key/类型/默认值目前在四处手写重复（Go struct+DefaultConfig、settings.ts 接口+getDefaultConfig、schema key、搜索索引）。v1 的"CI key 一致性校验"是过渡形态，终态是 `go:generate` 反射 Config 树直接生成 TS 类型 + key 清单，settings.ts 手写接口退役——加配置项只改 Go 一处。
2. **默认值动态下发**：前端 `getDefaultConfig()` 手写默认值是漂移源。增加 RPC `ConfigGetDefaults`（下发 `SystemDefaultConfig()`），前端用于"恢复默认"与默认值提示——系统预置 `data/config.toml` 改默认，前端展示自动跟随，无需发版。
3. **"已自定义"可视化 + 单项重置**：diff-save 使 Go 天然知道用户 touch 过哪些键（用户文件中存在的键）。RPC 增加 per-key reset（删除用户层键）后，设置页可给已修改项加徽标、提供单项"恢复默认"。依赖 v1 的 key 路径稳定，是 version 化 + diff-save 闭环后自然解锁的能力。
4. **搜索索引从 schema 派生**：`pages/*.search.ts` 的 label/key/options 在 schema 定义中已有，可生成大部分，只留同义词人工补充。
5. **表单原语趋同**：与主题编辑器（WindInputThemeEditor）沉淀的表单原语体系长期收敛复用，两端表单交互一致化。

演进 1/2 建议在 v1 落地后的下一迭代做（先由 CI 校验堵住漂移）；3 不急。
