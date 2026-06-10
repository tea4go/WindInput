# 配置体系重构：实现计划（执行级）

> 状态：可执行（设计依据：docs/design/config-restructure.md，已审查定稿）
> 约定：本文档是执行手册——每个切片给出文件、具体动作、验收命令。执行中与设计文档冲突时以设计文档为准并回报。

## 0. 全局约定

- 分支：`feat/config-restructure`，切片 0/1 连续完成后才可构建验证，切片 2/3 可并行。
- 每切片完成后：`go build ./...`（wind_input 目录）+ 相关 `go test` + `go fmt`；前端切片用 `pnpm build` + `pnpm test`。
- **pkg/config 对外接口大改，切片 1 完成时必须重写 `wind_input/pkg/config/AGENTS.md`**（模板 docs/AGENTS-TEMPLATE.md），提交前跑 `scripts/lint_agents_md.ps1`。
- 命名总表（struct 级，yaml/json tag 见设计文档 §3/§6）：

| v0 struct | v1 struct | 挂载点 |
|---|---|---|
| StartupConfig | **GeneralConfig** | Config.General `yaml:"general"` |
| UIConfig（巨型） | **UIConfig**（纯容器） | 子结构见下 |
| —（新增） | **UICandidateConfig** | UI.Candidate `yaml:"candidate"` |
| —（新增） | **UIFontConfig** | UI.Font `yaml:"font"` |
| —（新增） | **UIThemeConfig** | UI.Theme `yaml:"theme"` |
| StatusIndicatorConfig | 不变 | UI.StatusIndicator |
| TooltipConfig 族 | 不变（Delay 字段迁入 TooltipConfig） | UI.Tooltip |
| ToolbarConfig | 不变 | UI.Toolbar `yaml:"toolbar"` |
| —（新增） | **FeaturesConfig** | Config.Features `yaml:"features"` |
| StatsConfig | 不变 | Features.Stats |
| S2TConfig | 不变 | Features.S2T |
| QuickInputConfig | 不变（删 Enabled/TriggerKey，加 AccentColor） | Features.QuickInput |
| SpecialModeConfig | 不变 | Features.SpecialModes |
| —（新增） | **CmdbarConfig**{CandidatePrefix string} | Features.Cmdbar |
| CapsLockBehaviorConfig | **CapsLockConfig** | Input.CapsLock `yaml:"capslock"` |
| OverflowBehaviorConfig | **OverflowConfig** | Input.Overflow `yaml:"overflow"` |
| AdvancedConfig | **DebugConfig**{LogLevel, PerfSampling bool} | Config.Debug `yaml:"debug"` |
| —（新增） | **CompatConfig**{HostRenderProcesses []string} | Config.Compat `yaml:"compat"` |
| PinyinConfig / FuzzyPinyinConfig | 移出本包 | → internal/engine（切片 1 末步） |

- v1 `Config` 顶层：`General / Schema / Hotkeys / Input / UI / Features / Compat / Debug`（无 version 字段——version 只活在磁盘与 map 层）。

## 切片 0：codec 的 version 机制与迁移链骨架

### 0.1 `pkg/config/codec.go`

1. 加常量与错误：
   ```go
   const currentConfigVersion = 1
   var ErrFutureConfigVersion = errors.New("config version is newer than supported")
   ```
2. 新增 `safeGetInt / safeGetBool / safeGetString / safeGetMap / safeGetSlice`（输入 `map[string]any` + key；断言失败返回零值+false；数值做 int/int64/float64 宽容归一，复用 `yamldiff.go` 的 `toFloat64` 策略）。
3. 新增迁移框架（设计 §4.2 代码骨架照搬）：`configMigration` 类型、`configMigrations` 表（切片 0 先放空表或空实现的 `migrateV0toV1` 占位）、`migrateConfigMap(m map[string]any, fromLegacyYAML bool) (migrated bool, err error)`——version > currentConfigVersion 时返回 `ErrFutureConfigVersion`（注意：判别规则按设计 §4.1：legacy yaml→0；toml 缺 version→currentConfigVersion；否则取值）。
4. `MigratedBackupSuffix`/`renameLegacyFile` 保留（损坏自愈分支、schema_overrides 删除路径仍用），但注释更新为"仅损坏/清理路径使用，正常迁移不再改名（设计 §4.4）"。

### 0.2 `pkg/config/config.go` 的 SaveTo

- 新增私有辅助 `injectVersion(m map[string]any)`：`m["version"] = currentConfigVersion`。
- diff 分支：`ComputeYAMLDiff` 产出 map 后调用 `injectVersion` 再 `marshalForPath`。
- 全量回退分支：struct 先 `toYAMLMap` → `injectVersion` → `marshalTOML`（需要把该分支从直接 marshal struct 改为 map 路径；或新增 `marshalConfigForPath(path, cfg)` 封装）。
- 空 diff 也要写出（文件至少含 `version = 1`）。

### 0.3 `pkg/config/codec_test.go` 扩展

- version 往返：Save → 文件首个非注释行为 `version = 1`（**置顶断言**，锁定 go-toml v2 标量先于表的行为）。
- TOML 缺 version：构造无 version 的 v1 结构 TOML → Load 不迁移、值正确；再 Save → version 补写。
- 未来版本：`version = 99` → `migrateConfigMap` 返回 `ErrFutureConfigVersion`。
- safeGet*：类型错误输入返回 ok=false 不 panic。

验收：`go test ./pkg/config/ -run 'Codec|Version|SafeGet'` 绿；此时 LoadFrom 尚未接入迁移（切片 1 接线），全量测试应仍绿。

## 切片 1：struct 重排 + 三态落地 + v0→v1 迁移 + 预置文件

> 内部子顺序（互为契约）：1A struct 定形 → 1B 迁移函数 → 1C LoadFrom 接线 → 1D 预置文件与文档 → 1E 防漂移与样本测试。全程不 commit 直到本切片验收过。

### 1A. `config.go` struct 重排

1. 按 §0 命名总表重写 Config 树；yaml/json tag 按设计 §3 的 key（json 与 yaml 同名）。
2. **退指针化 9 项**（设计 §5.1）：全部改值类型；同时删除 accessor：`StatsConfig.IsEnabled/IsTrackEnglish`、`TooltipCodeConfig.IsEnabled`、`ToolbarConfig.IsHideInFullscreen`、`UIConfig.GetCmdbarCandidatePrefix`、`TempPinyinConfig.ZIncludeOnCommitEnabled`、`AdvancedConfig.IsPerfSampling`、`boolPtr` 辅助。**编译器此时会列出全部消费方调用点——逐个改为直读字段，这是切片 1→2 的天然工作清单（coordinator/ui/engine 侧的改动顺势完成，不留到切片 2）**。
3. **全量去 omitempty**：标量字段的 yaml/json tag 一律无 omitempty；slice/map 保留 omitempty 豁免（special_modes、global_hotkeys 等）。
4. 删除字段：`UIConfig.StatusIndicatorDuration/OffsetX/OffsetY`、`QuickInputConfig.Enabled/TriggerKey`；`TooltipConfig` 增加 `Delay int`（吸收 tooltip_delay）；`TempPinyinConfig` 增加 `AccentColor string`；`QuickInputConfig` 增加 `AccentColor string`。
5. `DefaultConfig()` 重写：**Layer 1 兜底值逐项保持 v0 现值不变**（含与 data/config.toml 的既有分叉：available=`["wubi86","pinyin"]`、toggle_toolbar=`"none"` 等，见设计 §3 归属说明），仅做路径重排；`MaxCandidateChars`→`MaxChars` 默认 16，`Phrase.MinPrefixLength` 默认 2。
6. `ApplyConfigFallbacks` 瘦身：删除 `migrateQuickInputConfig`/`migrateStatusIndicatorConfig`/theme:"dark" 迁移；保留并调整 clamp 类兜底（schema available/active、MaxChars 8-64、PerPageExtended≤10、MinPrefixLength≥1、ThemeStyle、S2T variant）。
7. `clone.go` 无需改（反射自适应）；`clone_test.go` 跑过即可。

### 1B. 新文件 `pkg/config/migration_v1.go`

实现 `migrateV0toV1(m map[string]any)`，注册进 `configMigrations[0]`：

1. 按设计 §6 映射表逐条搬键（用 safeGet* 取、搬入新路径、删旧键；嵌套子表整体平移用 safeGetMap）。
2. 熔合四个启发式（设计 §4.2/§6 备注）：
   - quick_input：`enabled=false`→`trigger_keys=[]`；`trigger_key` 非空且 `trigger_keys` 缺失→搬入；删两旧键；
   - `ui.theme=="dark"` → `ui.theme.name="default"` + `ui.theme.style="dark"`；
   - status_indicator 三旧键：仅当 `ui.status_indicator` 子 map 中对应**键不存在**时回填（键缺失判定，显式 0 不回填）；
   - font_size_follow_theme：v0 map 中该键缺失（老用户文件）→ 显式写 `false`（保留现字号语义）。
3. 单测 `migration_v1_test.go` + `testdata/v0_full.yaml`（覆盖映射表全部条目）+ `testdata/v0_dirty.yaml`（脏类型样本）。

### 1C. `LoadFrom` / `LoadRuntimeState` 接线

1. `unmarshalConfigData` 改造：normalize 得到 YAML 字节后先解到 `map[string]any`，调 `migrateConfigMap(m, fromLegacyYAML)`，再 `yaml.Marshal(m)` → `yaml.Unmarshal` 进 cfg（保留 TypeError 部分解码语义）。`fromLegacyYAML` 由调用方传入（`readPath` 是否为 legacy 回退路径）。
2. `ErrFutureConfigVersion` → 走现有"损坏"分支（备份 .bak + 回退默认 + 警告）。
3. **迁移成功路径删除 `renameLegacyFile(migratedFrom)` 调用**（config 与 state 两处，设计 §4.4）；损坏自愈分支保留改名/备份行为。
4. Layer 2 系统预置不走迁移（直接 v1 结构）；但若 `GetSystemConfigPath` 回退到旧版 `data/config.yaml`（升级残留），同样走 `migrateConfigMap(fromLegacyYAML=true)` 防御。

### 1D. 预置文件与文档

1. `data/config.toml` 按设计 §3 树全量重写：v1 结构 + `version = 1` 不写（系统预置层非用户文件，version 属用户文件契约；若写也无害——定为**不写**，保持"version=用户文件迁移锚点"语义单一）+ 显式列全面向用户默认值（补 `toggle_s2t`/`take_screenshot`/`numpad_behavior`/`features.cmdbar.candidate_prefix="⚡"` 等）+ 分叉项注释（available、toggle_toolbar）。
2. `build_debug/data/config.toml` 同步（或确认构建脚本自动拷贝）。
3. 重写 `pkg/config/AGENTS.md`。

### 1E. 防漂移与样本测试

- 新文件 `config_contract_test.go`：反射遍历 Config 全树断言①无指针字段②标量 tag 无 omitempty（slice/map 豁免）。
- 迁移保真测试（设计 §9.1/9.2）、diff-save 闭环（§9.3，重点：默认 true 的 bool 显式 false、cmdbar.candidate_prefix 显式 "" 往返）、三层合并（§9.5）。

### 1F. PinyinConfig 移出

`PinyinConfig`/`FuzzyPinyinConfig` 移到 `internal/engine/pinyinconfig.go`（或 manager_config.go 同包），原引用方（`engine/manager_config.go`、`coordinator/reload_handler.go`、`tooltip/provider_pinyin.go`）改 import；编译器捕获全部。

验收：`go build ./... && go test ./...` 全绿；`go fmt`。

## 切片 2：RPC 与热重载路由

1. `pkg/rpcapi/types.go`：`ConfigSection` 枚举更新为 `general/schema/hotkeys/input/ui/features/compat/debug`（删 startup/toolbar/advanced/stats/s2t）。
2. `internal/rpc/config_service.go`：`getSectionMap`/`setSectionFromMap` 的 switch 按新枚举与新字段重写；`resolveKeyPath` 机制不变；`config_service_test.go` 用例 key 更新（如 `input.auto_pair.chinese` 不变、`ui.font_size`→`ui.candidate.font_size`）。
3. `internal/coordinator/reload_handler.go`：`changedSections` 路由重排——`general`→`UpdateGeneralConfig`（原 UpdateStartupConfig 改名）、`ui`→UpdateUIConfig+UpdateToolbarConfig+StatusIndicator 链、`features`→UpdateStatsConfig+UpdateS2TConfig+quick_input/special_modes 相关、`compat`→host render 应用逻辑、`debug`→log level 等；engine 侧 `UpdateFilterMode`/`UpdatePinyinOptions` 等调用点字段路径更新。
4. `internal/coordinator/handle_config.go` 及其余消费方：编译错误驱动的字段路径机械替换（切片 1A 已完成 accessor 部分）。
5. 新增 key 清单导出：`pkg/config/keypaths.go` 提供 `AllKeyPaths() []string`（反射遍历 yaml tag 树，含数组节点规则：special_modes 记为前缀）；测试 `TestExportKeyPaths` 在 `-update` 标志下写出 `wind_setting/frontend/src/generated/config-keys.json`。

验收：`go build ./... && go test ./...`；手动跑一次 `TestExportKeyPaths -update` 生成清单供切片 3 使用。

## 切片 3：前端（wind_setting/frontend）

按设计 §7.2/§10.1：

1. `src/api/settings.ts`：Config 接口树按 §0 命名总表重排 + `getDefaultConfig()` 同步；现有 `boolean | null`（perf_sampling）收敛为 `boolean`；`cmdbar_candidate_prefix?: string | null` → `features.cmdbar.candidate_prefix: string`。
2. `src/schemas/*.schema.ts` + `searchIndex.ts` + `pages/*.search.ts`：key 按设计 §6 映射表替换——**整串带引号字面量、长 key 降序**。
3. Vue 页面：tsc 报错驱动修正 `formData.*` 访问；`getPath/setPath`（schemas/types.ts）加开发期未知路径 `console.warn`。
4. 页面分组重组：新增"扩展功能"页聚合 features.*；advanced 页拆 debug/compat 分区（沿用现有页面组件拆分模式）。
5. 新增 vitest：读 `generated/config-keys.json`，断言全部 schema/搜索索引 key ∈ 清单。

验收：`pnpm build` + `pnpm test` + 设置页全页面手测（含搜索跳转、保存往返）。

## 切片 4：真机验证清单

1. 放置旧结构 `config.yaml`（真实老用户样本）→ 启动 → 验证：`config.toml` 生成且含 `version = 1`、旧 `config.yaml` 保留原地、各设置值语义不变（重点：字号 follow=false、quick_input 触发键、状态提示时长）。
2. diff-save 闭环：设置页改一项 → `config.toml` 只含该项 + version。
3. 手删 `config.toml` 的 version 行 → 重启不重迁移、下次保存补写。
4. `version = 2` 文件 → 备份 .bak + 回退默认 + 不崩溃。
5. 设置热重载：改 features.stats / ui.candidate 等各 section 字段，验证 TSF 端生效（统计推送、候选窗渲染）。

## 风险与回退

- 切片 1 是巨型切片但有编译器全程兜底（accessor 删除、字段改名均为编译错误驱动）；唯一编译器看不见的是 **map 层迁移函数的 key 字符串**——由 §1B 的全覆盖样本测试兜底。
- 任何切片失败可整体回退分支；用户数据零风险（v1 不删不改旧 YAML，TOML 写出前旧文件完好）。
