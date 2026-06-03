# 设置端配置保存：快照 diff + 按 key 最小化提交

## 背景与问题

设置端（wind_setting）的全局「保存」按钮当前走 `App.vue saveConfig → wailsApi.saveConfig → App.SaveConfig → rpcClient.ConfigSetAll`，把整个 `formData` 序列化后整份覆盖服务端配置。

但 `formData` 并不是完整配置的真实镜像：它经过 `mergeWithDefaults`（App.vue:237）只重建了 8 个段（startup / schema / hotkeys / ui / toolbar / input / advanced / s2t），**不含 stats、dictionary 等走独立接口的段**。

### 已确认的根因（null 覆盖链）

1. 用户在统计页关闭「统计英文模式」→ 走独立的 `Stats.UpdateConfig`，正确把 `track_english=false` 写入内存和文件。
2. 用户点全局「保存」→ `formData` 不含 stats。
3. `App.SaveConfig(cfg *config.Config)` 接收的是 **Go 完整 `config.Config` 结构体**，wails 把「无 stats 的前端对象」反序列化进去时，`Stats` 的 `*bool` 字段变成 `nil`，再 `json.Marshal` 序列化成 `"track_english": null`。
4. 后端 `ConfigService.SetAll` 的 `json.Unmarshal(args, newCfg)` 用 `null` 把已存在的 `&false` **覆盖回 `nil`**。
5. `IsTrackEnglish()` 对 `nil` 返回 `true`，并触发 `UpdateStatsConfig(true)`；`config.Save` 时 `nil + omitempty` → stats 段消失 → 重载读默认 `true`。

问题本质是一个**语义错配**：全局保存用「全量覆盖」语义，但提交方（formData）不是完整配置的镜像，缺失字段被当作 `null`/清空，从而把「前端没管的东西」冲掉。只要这个错配还在，任何「前端全局表单不管理、又走独立接口」的配置段都会重蹈覆辙。

## 目标 / 非目标

**目标**
- 全局保存只提交用户**真正改动过的配置项**（按 key 最小化提交），而非整份覆盖。
- 统一机制：所有 formData 管理的页面共用同一套「快照 diff + 提交」路径，无需逐控件加标记。
- 独立段（stats / dictionary）天然隔离，永不被全局保存触碰。
- 提升正确性：不覆盖外部并发修改的其他项；根治 stats 被冲掉的 bug。

**非目标**
- 不改动 Stats / Dictionary 的专用即时保存路径。
- 不引入显式的「脏标记」UI 状态（dirty 集由快照 diff 自动得出）。
- 不重构后端 RPC / config 包的现有接口（全部复用）。

## 方案概览

采用「快照 diff + 按 key 最小化提交」：利用 App.vue 已有的两份数据——`config`（加载时快照）与 `formData`（编辑中）——保存时对两者做 deep-diff，得出「改动过的 key 集」，只用现成的 `Config.Set`（按 key）提交这些项。

- **dirty 粒度** = key 级
- **标记方式** = 不需要显式标记，`deepDiff(config, formData)` 自动产出
- **与 hasUnsavedChanges 的关系** = 同源（都基于 `config` vs `formData` 的差异）
- **后端配合** = 复用现成 `Config.Set`；`SetAll` 退出全局保存路径

## 详细设计

### 1. 核心数据流（App.vue `saveConfig`）

```
1. items = diffConfigToItems(config.value, formData.value)   // 改动过的 key 列表
2. if (items.length === 0) { toast("当前无改动"); return; }   // 空 diff：提示并跳过
3. reply = await wailsApi.setConfigItems(items)              // 按 key 提交（走 Config.Set）
4. await wailsApi.saveTSFLogConfig(tsfLogConfig.value)        // 不变
5. toast(reply.requires_restart ? "保存成功（部分设置需重启生效）" : "保存成功")
6. config.value = clone(formData.value)                      // 重置基线
   savedTSFLogConfig.value = clone(tsfLogConfig.value)        // 不变
   rebuildEngines(formData.value)                             // 不变
```

独立段（Stats / Dictionary）完全不参与此路径，继续各自专用即时保存接口。

### 2. deep-diff 函数（前端新增，可独立测试）

签名：`diffConfigToItems(base: any, current: any, prefix = ""): ConfigSetItem[]`

规则：
- **对象** → 递归进入，key 路径追加 `.<字段名>`。
- **数组 / 标量** → 视为叶子。用 `JSON.stringify` 比较；不等则产出一项 `{ key: 点路径, value: current 值 }`（数组整体作为值提交，不做元素级 diff）。
- **类型不同**（如对象变成 null）→ 视为叶子，整体提交 current。
- 只遍历 `current`（formData）实际拥有的字段。stats / dictionary 不在 formData → 永不产出。

key 路径形如 `ui.font_size`、`input.auto_pair.chinese`、`hotkeys.global_hotkeys`，与后端 `resolveKeyPath` / `setNestedKey` 的点路径约定对齐。

> 该函数在概念上与后端的 `config.ComputeYAMLDiff` 一致（都是「相对基线只保留变化字段」），但作用在前端 `config` vs `formData` 之间，产出 `Config.Set` 所需的 `{key, value}` 列表。

### 3. 新增接口（两个薄封装）

- **wind_setting Go**：新增 `App.SetConfigItems(items []rpcapi.ConfigSetItem) (*SaveConfigResult, error)`，内部调用 `a.rpcClient.ConfigSet(items)`，返回 `SaveConfigResult{RequiresRestart: reply.RequiresRestart}`。
- **前端 wails.ts**：新增 `setConfigItems(items: ConfigSetItem[]): Promise<SaveConfigResult>` → `App.SetConfigItems`，并补 `ConfigSetItem` 类型（`{ key: string; value: any }`）。
- **后端 RPC / config 包**：零改动。`ConfigSetArgs{Items:[{Key,Value}]}`、`ConfigSetReply{Applied, RequiresRestart}`、`rpcClient.ConfigSet`、`ConfigService.Set`（嵌套 `setNestedKey` + `diffSections` 精准热更新）均已存在。

### 4. hasUnsavedChanges / 基线

- `hasUnsavedChanges` 保持不变：`JSON.stringify(formData) !== JSON.stringify(config)`。
- dirty 集 = `diffConfigToItems` 结果，与 `hasUnsavedChanges` 同源，无需额外显式标记。
- 保存成功后 `config.value = clone(formData.value)` 重置基线（沿用现有逻辑）。
- `refreshConfigAndEngines`（外部 config 变更时的静默刷新）逻辑不变：仅在「无未保存改动」时刷新 formData，避免覆盖用户正在编辑的内容。

### 5. `SetAll` 的处置

- 全局保存退出 `SetAll` 路径。`wailsApi.saveConfig`(整份)目前只有 App.vue:332 一处调用，改用 `setConfigItems` 后即无前端调用方。
- **清理**：删除前端 `wailsApi.saveConfig`（整份）这一死调用封装；wind_setting 的 `App.SaveConfig` 同步删除（若确认无其他引用）。
- **保留兜底**：后端 `ConfigService.SetAll` 及其内部 `newCfg.Stats = s.cfg.Stats` 防御性保护**保留**——`SetAll` 仍是合法 RPC，保留可防「将来再次以整份方式调用」时重蹈覆辙。对应回归测试 `TestConfigSetAll_PreservesStats` 一并保留。

### 6. 边界与错误处理

- **空 diff**：不发 RPC，toast「当前无改动」后返回。
- **保存失败**：沿用现有 `try/catch + toast`，失败时**不重置基线**（`config` 保持旧值，`formData` 保留用户改动，`hasUnsavedChanges` 仍为 true）。
- **数组**：整体提交，不做元素级 diff（可接受；数组类配置如 `schema.available`、`hotkeys.global_hotkeys` 体量小）。
- **嵌套对象部分修改**：精确到改动的叶子 key（如只改 `input.auto_pair.chinese` 则只提交该 key）。
- **hotkey 冲突**：沿用现有 `hotkeyConflicts` 前置校验，冲突时阻断保存。

## 测试

- **deep-diff 单元测试**（前端）：覆盖无变化（空结果）、标量改动、嵌套对象部分改动、数组改动、新增字段、类型变化等用例。
- **后端回归**：保留 `TestConfigSetAll_PreservesStats`（守护 `SetAll` 兜底）。
- **手动验证**：在「外观/输入」页改一个字段 + 在统计页关闭「统计英文模式」→ 点全局保存 → 触发重载，确认：① 改的字段生效；② `track_english` 保持关闭；③ `D:\UserData\输入法数据\config.yaml` 出现 `stats:\n    track_english: false`。

## 影响范围

- 前端：新增 `diffConfigToItems` 工具 + `wails.ts` 的 `setConfigItems`/`ConfigSetItem`；改写 `App.vue saveConfig`；删除整份 `saveConfig` 死调用。
- wind_setting Go：新增 `App.SetConfigItems`；删除 `App.SaveConfig`（若无其他引用）。需同步更新对应目录 AGENTS.md（对外方法变更）。
- 后端 RPC / config 包：无改动（保留既有兜底 + 测试）。

---

## 后续：stats 配置并入全局保存

### 背景
上述重构后，stats 仍走独立的即时保存（`Stats.UpdateConfig`），其它 8 段走全局保存（diff + `setConfigItems`）——两套机制不统一。本节把 stats 也并入全局保存，实现单一保存路径。注：「保存设置」按钮在 sidebar 全局可见（含 stats tab），现状下它并不管 stats（stats 即时保存），并入后该按钮才名副其实。

### 决策
- **交互模型**：完全并入。stats 开关改的是 `formData`，需点「保存设置」才生效，与其它设置一致；废弃 stats 即时保存。
- **废弃范围**：彻底删除 stats 配置专用链（前后端整条）。
- **设计文档/分支**：本节追加于本文件；实现与上一轮在同一 worktree/分支 `feature/settings-incremental-save` 继续，最后统一提交。

### `*bool` 往返正确性（关键）
- **保存方向**：`Config.Set("stats.track_english", false)` 是按 key 单点设置——`getSectionMap` 从当前 cfg 读出含已有值的 map、`setNestedKey` 只改该 key、`setSectionFromMap` 往返回结构体，`false` 正确存为 `&false`，未改字段（如 `enabled`）保留。不同于 `SetAll` 整段覆盖（缺字段→null）。
- **读取方向（坑）**：`ConfigGetAll` 把 `*bool` 的 nil（默认 true 语义）序列化成 `null`。`mergeWithDefaults` 的 stats 段**必须用 `??` 兜底**（`cfg.stats?.track_english ?? defaults.stats.track_english`），否则简单 spread 会把默认 true 丢成 null。这与本文档开头的 null 覆盖问题同源，方向相反。

### 改动清单
1. **前端 Config 类型与默认**（`src/api/settings.ts`）：`Config` 加 `stats: { enabled: boolean; retain_days: number; track_english: boolean }`；`getDefaultConfig` 加 stats 默认（`enabled:true, retain_days:0, track_english:true`）。
2. **mergeWithDefaults**（`App.vue`）：加 stats 段，**用 `??` 兜底 null**。
3. **StatsPage 改造**（`StatsPage.vue`）：
   - 移除 `statsConfig` ref、`loadData` 中的 `getStatsConfig`、`handleStatsEnabledChange`/`handleTrackEnglishChange` 的即时保存、局部 `saveConfig`。
   - 改接收 `:formData` props，两个开关 `v-model` 绑 `formData.stats.enabled` / `formData.stats.track_english`。
   - **保留**统计数据相关：`getStatsSummary`、`getDailyStats`、`clearStats`、`clearStatsBefore`、`onStatsEvent`/`offStatsEvent`。
   - App.vue 模板把 `<StatsPage>` 的 `:isWailsEnv` 之外补 `:formData="formData"`。
4. **彻底删除 stats 配置专用链**：
   - 前端 `src/api/wails.ts`：`getStatsConfig`、`saveStatsConfig`（及仅此处使用的 `StatsConfig` interface）。
   - wind_setting `app_stats.go`：`GetStatsConfig`、`SaveStatsConfig`。
   - 后端 `internal/rpc/server.go`：`Stats.GetConfig`、`Stats.UpdateConfig` 注册。
   - 后端 `internal/rpc/stats_service.go`：`GetConfig`、`UpdateConfig` 方法。
   - `pkg/rpcapi/client.go`：`StatsGetConfig`、`StatsUpdateConfig`。
   - `pkg/rpcapi/types.go`：`StatsConfigReply`、`StatsConfigUpdateArgs`（**确认无其它引用后**删）。
   - `wails generate module` 重新生成绑定（移除 GetStatsConfig/SaveStatsConfig）。
   - `StatsService` 的 `GetSummary`/`GetDaily`/`Clear`/`Prune` **保留**。
5. **后端兜底**（`config_service.go` 的 `SetAll` 内 `newCfg.Stats = s.cfg.Stats` + `TestConfigSetAll_PreservesStats`）：**保留**（`SetAll` 仍存在，继续防御）。

### 测试
- **后端**：新增测试验证 `Config.Set` 设 `stats.track_english=false` → 内存/落盘正确 `false`，且未改的 `enabled` 保留；设 `stats.enabled=false` 不影响 `track_english`。
- **前端**：`mergeWithDefaults` 对 `stats.track_english=null` 兜底成默认 true（vitest）；`diffConfigToItems` 对 stats 段产出正确 key（可复用现有 diff 测试机制，补 stats 专项用例）。
- **手动**：统计页关「统计英文」→ 点保存 → 重载 → `track_english:false` 持久；改 `enabled` 不影响 `track_english`；stats 开关随全局保存生效（不再即时）。

### 影响范围
前端：`settings.ts`、`App.vue`、`StatsPage.vue`、`wails.ts`、生成绑定。后端：`app_stats.go`、`server.go`、`stats_service.go`、`client.go`、`rpcapi/types.go`。AGENTS.md 同步：`wind_setting/`（app_stats.go 方法移除）、`src/api/`（wails.ts 导出变更）。
