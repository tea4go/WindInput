<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-13 | Updated: 2026-06-10 -->

# pkg/config

## Purpose
应用配置的完整定义、加载/保存逻辑、版本化迁移、路径管理和运行时状态持久化。配置文件为 **TOML 格式**（`config.toml` / `state.toml` / `compat.toml` / `schema_overrides.toml`），存储在用户数据目录（Windows `%APPDATA%\WindInput\`、macOS `~/Library/Application Support/WindInput/`）。

**v1 配置结构**（设计权威：docs/design/config-restructure.md）：顶层节 `general / schema / hotkeys / input / ui / features / compat / debug`，分类原则「设置页在哪就放哪」；ui 拆分 candidate/font/theme/status_indicator/tooltip/toolbar 子节；features 集中自包含可选功能（stats/s2t/quick_input/special_modes/cmdbar）。

**版本机制**：用户 `config.toml` 顶层裸键 `version = 1`（仅用户配置文件携带；系统预置与辅助文件不写）。格式即版本边界：旧版 `.yaml` = v0（结构迁移 `migrateV0toV1` 在桥接 map 层执行），TOML 缺 version 按当前版本处理（手编误删保护）。**Config struct 不含 version 字段**。迁移成功后旧 YAML **保留原地不改名**（网盘混版本共存兜底，§4.4）。

**三态规范**（防漂移测试机器守护，见 `config_contract_test.go`）：struct **禁指针**、**bool 永不为 null**、标量 tag **禁 omitempty**（slice/map 豁免）；"未设置=继承默认"由磁盘键缺失 + diff-save 表达；需"未设置"语义用枚举哨兵值（`"none"`/`"auto"`/`""`）或双字段（值+follow 开关）。

采用**桥接式编解码**（`codec.go`）：TOML 仅作磁盘表面格式，struct ↔ map 统一走 yaml tag 管线，**无 toml tag**。三层配置合并：代码默认（Layer 1）→ `data/config.toml`（Layer 2，显式列全面向用户默认值）→ 用户配置（Layer 3，diff 保存）。

## Key Files
| File | Description |
|------|-------------|
| `config.go` | v1 `Config` 结构树（General/Schema/Hotkeys/Input/UI/Features/Compat/Debug）、`Load()`/`LoadFrom()`/`Save()`/`SaveTo()`/`DefaultConfig()`/`ApplyConfigFallbacks()`，三层加载 + 迁移接线 + version 注入 |
| `migrate.go` | 版本机制：`currentConfigVersion`/`ErrFutureConfigVersion`/`migrateConfigMap`（map 层迁移链）/`injectVersion`/`safeGet*` 弱类型取值辅助 |
| `migration_v1.go` | `migrateV0toV1`：v0→v1 全量 key 重映射 + 三个启发式熔合（quick_input 旧字段、theme:"dark"、status_indicator 旧顶层键回填）；`renameKeyV1`/`putIfAbsent`/`ensureMapV1`/`moveKeysLazy` 搬移辅助 |
| `codec.go` | TOML 桥接编解码层：`IsTOMLPath`/`LegacyYAMLPath`/`normalizeToYAML`/`marshalTOML`/`marshalForPath`/`readFileWithLegacyFallback`/`renameLegacyFile`（仅损坏/清理路径用，正常迁移不再改名） |
| `keypaths.go` | `AllKeyPaths()` 反射导出全量 v1 点路径；`keypaths_test.go -update` 写出 `wind_setting/frontend/src/generated/config-keys.json` 供前端 key 一致性校验 |
| `clone.go` | `Config.Clone()` 反射式深拷贝。**红线：异步持久化（`go config.Save(...)`）必须先 Clone，禁止浅拷贝**——共享底层 map/slice 并发修改会硬 panic |
| `accessor.go` | cmdbar `config.get/set/toggle` 的字段注册表（`Fields`，v1 点路径索引如 `"ui.candidate.layout"`、`"features.s2t.enabled"`）；**key 重命名需同步本表**（用户已保存的 cmdbar 命令字符串引用这些路径） |
| `paths.go` | 路径常量与辅助函数（`GetConfigDir`、`GetDataDir`、`GetSystemConfigPath`（toml 优先回退旧 yaml）等） |
| `config_hotkey.go` | 快捷键匹配与冲突检测（`IsToggleModeKey`/`IsSelectKey2/3`/`IsPageUp/DownKey`/`ValidateHotkeyConflicts`） |
| `state.go` | `RuntimeState`（state.toml）：模式/全角/标点/工具栏位置/候选窗 pin 位置；迁移后旧 state.yaml 保留原地 |
| `compat.go` | `AppCompat` 按进程兼容规则（compat.toml，系统+用户层合并） |
| `schema_overrides.go` | 方案覆盖（schema_overrides.toml，`map[schemaID]→覆盖项`；**顶层不可写裸 version 键**——会被当方案 ID） |

## For AI Agents

### Working In This Directory
- **新增配置项**：在对应子结构体添加字段（**值类型**，yaml/json tag 同名、**禁 omitempty**），`DefaultConfig()` 给 Layer 1 默认值，`data/config.toml` 显式列出，`ApplyConfigFallbacks()` 只做值域 clamp；归属按「设置页在哪就放哪」，input vs features 边界=「按键流水线耦合 vs 可整体拔掉」
- **禁止**：给配置 struct 加指针字段或 toml tag；绕过桥接直接 toml.Unmarshal 进 struct；在迁移函数里裸类型断言（必须走 `safeGet*`）
- **改 key 路径**（罕见，需 version bump）：更新 §6 式映射表 + 新迁移函数 + `accessor.go` 注册表 + RPC `ConfigSection`/前端 schema key + 重新生成 config-keys.json
- **三层加载**（`LoadFrom`）：Layer 2 经 `GetSystemConfigPath()`；Layer 3 `readFileWithLegacyFallback`（TOML 优先回退 yaml），map 层 `migrateConfigMap` 后 unmarshal（保留 yaml.TypeError 部分解码自愈）；`ErrFutureConfigVersion`（程序回滚）走损坏分支（备份+默认）
- **diff-save**（`SaveTo`）：与 `SystemDefaultConfig()` 求 diff 后**强制注入 version** 再写出；空 diff 也写出 `version = 1`
- **热重载路由**：`coordinator/reload_handler.go` 按 v1 节路由——general→UpdateStartupConfig、ui→UpdateUIConfig+UpdateToolbarConfig、features→UpdateFeaturesConfig（含 stats 推送/s2t/special_modes registry 重建/cmdbar 前缀）、compat/debug→需重启；RPC 节枚举见 `rpcapi.ConfigSection`
- `RuntimeState` 与 `Config` 分开存储（state.toml），避免用户编辑配置时覆盖运行时状态

### Testing Requirements
- `config_contract_test.go`：三态规范防漂移（无指针/无 omitempty），新字段自动覆盖
- `migration_v1_test.go` + `testdata/v0_full.yaml`：v0→v1 全映射表保真、启发式熔合、脏数据宽容、幂等
- `version_test.go`：version 判别/置顶断言/未来版本降级/safeGet
- `codec_test.go`：桥接往返、diff-save、旧版迁移（旧文件保留原地）、损坏自愈
- 用 `setTestConfigDir(t)` 把配置目录重定向到隔离临时目录

### Common Patterns
- 路径函数返回 `(string, error)`，调用方在错误时回退到 exeDir
- 枚举字段用具名常量（enums.go），含 `"auto"`/`"none"`/`""` 类哨兵值表达"未设置/跟随"
- FollowTheme 双字段模式：值字段 + `*_follow_theme` bool（true=跟随主题），两者皆值类型

## Dependencies
### Internal
- 无

### External
- `gopkg.in/yaml.v3` — struct ↔ map 编解码管线（yaml tag 驱动）+ 旧版 YAML 解析
- `github.com/pelletier/go-toml/v2` — TOML 磁盘格式编解码（仅 map 级，经桥接使用；version 置顶依赖其"顶层标量先于表"的 marshal 行为，测试锁定）

<!-- MANUAL: -->
