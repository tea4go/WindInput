<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-01 | Updated: 2026-06-11 -->

# internal/engine/mixed

## Purpose
五笔拼音混合输入引擎。内部持有独立的码表引擎（`*codetable.Engine`）和拼音引擎（`*pinyin.Engine`），根据输入长度选择查询策略，并行查询后按权重合并候选词列表。

查询策略（以最大码长 maxCodeLen=4 为例）：
- 1 码：仅查码表
- 2~4 码：并行查码表+拼音，码表优先（双向夹击权重）
- >4 码：降级为纯拼音（`IsPinyinFallback=true`）

`IsPinyinFallback` 不再是 ">maxCodeLen" 专属标记：**2~maxCodeLen 码内构成合法多音节拼音序列**（`Composition.CompletedSyllables>=2` 且 `isPossiblePinyinSequence` 为真，如 `nihao`/`woai`）时，`convertMixed` 也会置 `IsPinyinFallback=true` 并填充音节分段预编辑区（`ni hao`），码表候选仍并列参与。该置位用 `isPossiblePinyinSequence` **前置门控**（非 overflow 路径"先设后 suppress"），因为 `suppressNonPinyinPreedit` 不重置 `IsPinyinFallback`，正常码长区间须避免单元音五笔码（`aaaa`）/残缺拼音残留该标记；单音节（`CompletedSyllables<2`，如五笔 `an`）保持码表编码显示、不分段。

## Key Files
| File | Description |
|------|-------------|
| `mixed.go` | `Engine`：混输引擎主体；`Config`（`MinPinyinLength`/`CodetableWeightBoost`/`ShowSourceHint`/`PinyinOnlyOverflow`/`TopCodeOverridePinyin`，全为纯数据字段）；`GetConfig`/`ApplyConfig`（`*e.config=*cfg` 整体覆盖，供热更新）；`ConvertEx` 核心转换逻辑（`convertCodetableOnly`/`convertMixed`/`convertPinyinOnly`）；`OnCandidateSelected` 按 `CandidateSource` 路由学习回调；`ConvertResult` 结构体（含 `IsPinyinFallback` 和拼音降级字段） |

## For AI Agents

### Working In This Directory
- **权重策略**（双向夹击）：码表精确匹配 +10M、前缀匹配 +6M；拼音纯辅音简拼按长度递减（3码 -2M，4码 -3.5M），含元音输入保持原值
- **Phrase 独立 tier** (2026-05-16 引入, `PhraseWeightBoost = 1_000_000`): codetableCandidates 切片里 `IsPhrase=true` 的候选在 boost 阶段被分离, 改打 `SourcePhrase` 并仅 +1M, 永远 > 拼音、永远 < 码表词; 详见 docs/design/command-bar-followup.md §2.2
- **Partial 独立 tier** (2026-05-17 引入, `PartialMatchBoost = 500_000`): 码表前缀补全 / 拆分组合 (`Code != input`) 不再加 codetable 6M, 改加 PartialMatchBoost, 让 phrase (1M) 全码命中优先于拆分组合; 解决输入 "date" 时 "d→大" 拆分候选抢在短语之前的问题
- 混输模式**默认禁用码表顶字**（`HandleTopCode` 合法拼音序列时返回 false），超码长输入由拼音降级处理而非顶字上屏
- **顶码歧义裁决**（2026-06-08）：`wang`/`aipu` 这类"既是完整拼音、又是终止性精确五笔全码"的串无法从编码判断意图。`HandleTopCode` 在 `isPossiblePinyinSequence` 为真时，若同时满足 `isWholeSyllablePinyin`（整音节，无残缺尾）+ `isTerminalExactCode`（`HasFullInputMatch && !HasLongerCode`）且开关 `TopCodeOverridePinyin=true`，则**放行顶码倒向五笔**。整音节门禁保证不会切在半个音节上（`zhon`/`yans` 残缺串永不放行，仍受保护）；终止性全码门禁把误伤限定到极小碰撞集。**默认 false**（`Config` 零值、`DefaultConfig`、factory 三处一致），即默认维持原拼音保护、不放行顶码；偏好五笔顶码连打的用户在 schema 设 `topcode_override_pinyin: true` 显式开启。设置程序"混输设置"区有对应开关。
  - **配置单一来源（避免热更新漏接）**：`mixed.Config` 的 spec→config 映射唯一收敛在 `schema.MixedConfigFromSpec`，构建（`factory.createMixedEngine`）与热更新（`engine.Manager.UpdateMixedOptions` → `mixed.Engine.ApplyConfig`）都走它。**新增任何 mixed 标量开关只需在 `MixedConfigFromSpec` 补一行**，构建与热更新自动同步，不必在 `manager_config.go` 逐字段手抄。漂移由 `schema.TestMixedConfigFromSpec_ConstructionEqualsReload` 守护。⚠️ 该"整体覆盖"仅因 `mixed.Config` 全为纯数据字段成立；若引入资源型字段（mmap/词库等），`ApplyConfig` 不能再无脑整体覆盖，需显式处理副作用
- `SetDictManager(dm)` 在引擎创建后由 factory 调用，用于 Shadow 规则访问
- Shadow 规则在各 convert 路径末尾统一应用（幂等操作），防止合并+重排后位置偏移
- `addSourceHints`：仅在拼音候选的 `Comment` 字段添加 `"拼"` 前缀，码表候选不添加标记
- `dedupByText` 去重时保留先出现的（权重较高的）；使用 `sync.Pool` 复用 seen 映射避免 GC 压力
- `convertMixed` 内部使用 `sync.WaitGroup` 并行查询两个引擎

#### ⚠️ 全码顶屏（AutoCommitAtFull）与 Shadow 交互、完整音节守护
混输模式下，码表子引擎设置了 `SkipShadow=true`，Shadow 由 MixedEngine 在合并后统一应用。
新判定逻辑为"全码顶屏：精确唯一 + 无更长后继 + 长度 >= MinAutoCommitLen"，并新增混输守护规则。
修改顶码或 Shadow 相关逻辑时必须注意：

- **不得直接继承** `codetableResult.ShouldCommit`：子引擎的 `checkAutoCommit` 在 Shadow 前执行，若用户通过候选调整删词，子引擎看到的候选数量仍是 Shadow 前的值，会漏判
- 应在 Shadow 应用**之后**调用 `recheckAutoCommit(input, candidates, hasPinyinCandidate)`，从最终候选列表重新评估
- `recheckAutoCommit` 判定条件（**全部满足**才顶屏）：
  1. `AutoCommitAtFull=true` && `len(input) >= MinAutoCommitLen`
  2. 拼音序列守护未否决：当 `AutoCommitBlockOnPinyin=true`（默认）且 `hasPinyinCandidate=true` 时，若 `e.isPossiblePinyinSequence(input)` 为 true（覆盖多音节如 `woai`、单音节、合法尾部前缀如 `nizh`；旧版只用 `trie.Contains` 单音节判定对多音节失效）则否决
  3. 合并候选中 `Code==input` 且 `Source ∈ {SourceCodetable, SourcePhrase}` 的候选恰好 1 个（**来源白名单**：拼音候选即便 Code==input 也不计入，全码顶屏是码表/短语特性，不应被拼音命中触发）
  4. `codetableEngine.HasLongerCode(input) == false`（主码表 + 短语/用户/temp 层任一存在更长后继即否决）
- 调用点（4 条全部）：`convertCodetableOnly` / `convertMixed` / `convertMixedOverflow` / `convertPinyinOnly` 末端均调用 `recheckAutoCommit`，分别按各自路径传 `hasPinyinCandidate`（纯码表 false，其余按拼音候选数 > 0）
- **长码场景**（`len(input) > maxCodeLen`）：`convertMixedOverflow` 与 `convertPinyinOnly` 在 `codetableEngine.HasFullInputMatch(input) || HasLongerCode(input)` 为真时，额外用完整 input 查码表并合并，避免 `abcde→乙` 类长码精确匹配被前 N 码截断吞掉；拼音候选在 `convertPinyinOnly` 此分支同步走 `PinyinTierScale` 归一化以维持 tier 分层
- `HandleEmptyCode` 在 `ClearOnEmptyAt4` 触发清空前调用 `codetableEngine.HasLongerCode(input)` 守护，存在更长后继时不清空
- **空码清空走 ConvertEx 而非 HandleEmptyCode**：协调器只认 `ConvertResult.ShouldClear`（`HandleEmptyCode` 在当前流程未被调用）。`convertMixed` 末端在 `result.IsEmpty`（merged 为空 == 码表与拼音都无候选）时继承 `codetableResult.ShouldClear`（即码表的全码清空决策），并用 `!isPossiblePinyinSequence(input)` 守护：长拼音可能尚未输入完处于暂时无候选状态，此时不清空。**勿改回无条件 `ShouldClear=false`**——那会让简拼关闭时的 4 码空码（如 `ssej`）卡住不清空（回归测试 `TestMixedEngine_ClearOnEmptyAt4`）

#### ⚠️ 学习路由（OnCandidateSelected）与 charBuffer 连续性

`OnCandidateSelected` 按 `CandidateSource` 路由到不同子引擎，但码表的自动造词（`CodeTableAutoPhrase`）依赖连续单字 `charBuffer`，必须感知拼音输入的存在：

- **SourceCodetable**：直接路由到 `codetableEngine.OnCandidateSelected`，单字进 charBuffer，多字词触发 flush
- **SourcePinyin**：路由到 `pinyinEngine.OnCandidateSelected`；**同时**通知码表的造词策略：
  - 拼音单字 → `ls.OnWordCommitted("", text)`（code="" 可为空，flush 时由 `CalcWordCode` 重算）
  - 拼音多字词 → `codetableEngine.OnPhraseTerminated()`，终止当前单字序列触发 flush
- **default（无来源标记，如顶码/自动上屏）**：默认路由到码表，符合预期（顶码只由码表触发）

**如果不遵守上述规则**，拼音输入的字不会进入 charBuffer，导致五笔+拼音交替输入时自动造词只能看到纯五笔子序列，无法正确感知拼音边界。

#### ⚠️ 自动造词功能的历史回退原因及防范

以下问题曾多次因修改其他功能而意外回退，提交前必须验证：

1. **混输方案学习配置**：混输方案**始终**使用主方案（`PrimarySchema`）的学习配置，不维护独立配置（混输本质是主码表 + 辅助拼音）。入口在 `factory.go:createMixedEngine`（`codetableLearningSpec`）和 `manager_config.go:UpdateLearningConfig`（`codetableLS`），两处逻辑对称。**不得删除或改为条件判断**。

2. **拼音 temp layer 的 SetLimits**：混输拼音子引擎使用独立 temp layer（`schemaID="pinyin"`），它不是 `DictManager.activeStoreTemp`，`UpdateActiveTempLimits` **不会**覆盖它。凡是创建/替换拼音 temp layer 的代码路径（factory 初始化 + 热更新），都必须显式调用 `tl.SetLimits`。否则 `promoteCount=0`，`LearnWord` 永远返回 false，临时词永不晋升。

3. **日志字段 `codetableAutoPhrase`**：该字段用类型断言判断 `codetableLearning` 是否为 `*schema.CodeTableAutoPhrase`（而非 `!= nil`）。`ManualLearning{}` 也是 non-nil，若改回 `!= nil` 判断会误报 true。

### Testing Requirements
- `go test ./internal/engine/mixed/`
- `mixed_repro_test.go` 包含复现测试用例
- 新增的学习路由行为建议在 `mixed_repro_test.go` 中补充集成测试（见下文自动化测试建议）

### ⚠️ 自动化测试建议（防止学习/造词/晋升回退）

该模块的学习、造词、晋升逻辑历史上多次因其他改动意外失效，且往往不产生编译错误或崩溃，只在运行时静默失效。推荐以下测试策略：

**集成测试层**（优先，可覆盖多个组件交互）：
- 在 `mixed_repro_test.go` 中构造完整的 `Engine + DictManager + StoreTempLayer`，模拟多次 `OnCandidateSelected`，断言：
  - 达到 `promoteCount` 次后，临时词库条目消失，用户词库中出现对应词
  - 拼音来源单字上屏后，码表 charBuffer 中确实追加了该字（可通过验证最终 flush 结果来验证）
  - 拼音多字词上屏后，charBuffer 被清空（flush 结果为空）

**单元测试层**（作为补充）：
- `StoreTempLayer.LearnWord` + `PromoteWord` 的 promoteCount 边界（已有 `store_layer_test.go`）
- `CodeTableAutoPhrase.OnWordCommitted` 对单字 vs 多字词的路由
- `mixed.Engine.OnCandidateSelected` 对 SourcePinyin 的路由（验证 charBuffer 通知）

**检查清单**（每次修改学习相关代码后运行）：
```
go test ./internal/engine/mixed/...
go test ./internal/dict/...
go test ./internal/schema/...
```

### Common Patterns
- `Engine` 实现 `engine.Engine` 和 `engine.ExtendedEngine` 接口
- `GetCodetableEngine()`/`GetPinyinEngine()` 供 `engine.Manager` 访问内部引擎（用于配置热更新、学习策略注入）
- `candidate.SourceCodetable`/`candidate.SourcePinyin` 标记候选来源，供 `OnCandidateSelected` 路由

## Dependencies
### Internal
- `internal/candidate` — `Candidate`、`CandidateSource`（`SourceCodetable`/`SourcePinyin`）、`Better`
- `internal/dict` — `DictManager`、`ApplyShadowPins`
- `internal/engine/pinyin` — 拼音引擎
- `internal/engine/codetable` — 码表引擎

### External
- 无

<!-- MANUAL: -->
