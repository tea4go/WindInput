# 输入处理器流水线（统一决策器 + 按键处理层 + 候选融合）

> 版本：v3。
> - v1 初稿（两层：Processor / Provider）。
> - v2 纳入多方评审：host 永不为空、DecisionCtx 只读视图、Capability 位掩码、Release
>   单次、Activate 原子回滚、词库层延迟卸载、字段读写契约、feature flag + 影子运行、可观测性。
> - v3 纳入「按键处理与宿主解耦」：在决策器与候选之间增加 **KeyHandler 链（按键处理层）**，
>   消除四套 `handleXxxKey` 的导航重复，支持特殊按键分流。
> 评审/版本修订摘要见文末「附录 B」。

## 目标与动机

当前输入流程是「显式进入/退出模式」的硬独占状态机：`tempEnglishMode` /
`tempPinyinMode` / `quickInputMode` / `specialMode` 四个布尔，在 `HandleKeyEvent`
里 `if c.xxxMode { return c.handleXxxKey(...) }` 串行检查（handle_key_event.go:499-516），
进入某模式即锁死，只能由该模式的 `exitXxxMode` 退出。退出统一 `hideUI()` + 清候选 +
重置分页，模式间切换有明显「窗口关→开」边界。此外，四套 `handleXxxKey` 各自重复实现了
翻页、高亮上下、数字选候选、二三候选键、ESC、退格删空等**通用候选窗导航**。

要解决的体验问题：模式切换应**自然、连续**，候选窗不闪烁、不重建。

工程目标：

1. 把分散的触发判定（`decideBufferedTrigger` + 三段 `getXxxTriggerKey`）收敛成**统一决策器**。
2. 把模式从「布尔独占」改造成「优先级排序的处理器列表 + 宿主热切换」。
3. 把「按键处理」从宿主中解耦成**独立的 KeyHandler 链**，消除导航重复、支持分流。
4. 为「多来源候选融合」预留干净扩展点。
5. **不破坏**现有为兼容 Excel/WPS/浏览器/终端积累的全部 composition/焦点补丁。

> 实施基准。第一阶段做决策器骨架 + KeyHandler 接口 + 宿主热切换；融合层（Provider）仅
> 定接口、预留扩展，不实现。

---

## 一、核心模型：三层正交抽象

四轮讨论 + 评审收敛出三个**互相正交**的关注点：

| 层 | 抽象 | 性质 | 同时活跃数 | 现有对应 |
|---|---|---|---|---|
| **宿主迁移** | Processor（宿主） | 有状态，持 buffer/preedit/模式语义；裁决「是否切宿主」 | 任一时刻 **单一** | engine_default / tempEnglish / tempPinyin / quickInput / special |
| **按键处理** | KeyHandler（链） | 无状态处理单元；决定「这个键怎么处理」 | **多个组成链** | 现散落在各 handleXxxKey 里的导航/选候选/特有键 |
| **候选来源** | CandidateProvider | 纯查询、无 UI 副作用，可并行可合并 | **多个** | quick_input 的 date/calc/number、码表/拼音/英文查询 |

三者解耦的依据：

- **按键处理 ≠ 宿主**（v3 核心）：大部分情况按键归当前宿主，但「翻页/选候选/导航」是
  跨宿主一致的通用语义，不该在每个宿主里重复实现；且特殊情况下，按键可由独立 handler
  分流，不必绑定宿主。证据：四套 `handleXxxKey` 重复实现了 VK_BACK/ESCAPE/翻页/高亮/
  数字选候选/二三候选键。
- **候选来源 ≠ 宿主**：`updateQuickInputCandidates`（handle_quick_input.go:380）已是迷你
  融合器——date/calc/number 各自命中即 append，全是纯查询。
- **按键归属仍单一**：同一按键任一时刻只被链上一个 handler 认领（短路），不存在两个
  handler 同抢——「单一归属」由链的有序短路保证，而非「绑死宿主」保证。

> 第一阶段实现 Processor 层 + KeyHandler 接口（共享导航 handler 渐进抽取）。Provider
> 融合为第二阶段。第一阶段候选行为与现状逐条等价。

---

## 二、决策器：四态裁决（宿主迁移）

```go
type Verdict int

const (
    Pass     Verdict = iota // 不认领 → 决策器问下一个
    Handle                  // 认领，在当前状态下处理（不换宿主）
    Activate                // 切换宿主为本处理器，然后处理（可带顶码上屏）
    Release                 // 当前宿主放弃这段 buffer → 触发整链重判
)

type Decision struct {
    Verdict    Verdict
    CommitIdx  int    // Activate：顶码上屏候选索引，-1=不顶（复用 enterModeCommitting）
    TriggerKey string // Activate：触发键标识
    Residual   string // Release：放弃宿主时残留待重判的 buffer（z fallback 的 inputBuffer[1:]+key）
}
```

`Release` 是「自然切换」的钥匙：活跃宿主对**特定按键**说「这个我不要，重新竞争」。
z 键 fallback、退格删空退出、URL 完整前缀夺取，本质都是 `Release → 重判 → 别处 Activate`。

**不变量：一次按键最多一次宿主迁移（一次 Release）。** 禁止 `A→B→C` 链式跳转。

---

## 三、Processor 接口：宿主迁移裁决 + 状态持有

```go
type Processor interface {
    Name() string

    // 宿主迁移裁决：纯函数，只读 DecisionCtx。判断本按键是否触发「切到/切离本宿主」。
    // 约束：极轻量——只做长度检查 / 前缀字符比对，禁止锁竞争、IO、复杂正则。
    Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision

    // Activate：成为宿主。residual 为上一宿主 Release 交接的 buffer（可空）。
    // 必须原子：失败返回 ok=false 时不得遗留任何副作用（见五·回滚）。
    Activate(triggerKey, residual string) (prefix string, ok bool)

    // Release：卸下宿主身份。声明式引擎资源由决策器统一 diff，此处不自行卸。
    Release()

    // KeyHandlers：本宿主贡献的「特有」按键处理单元（如临时英文大小写、拼音分隔符、
    // special 自动上屏）。决策器把它们与共享导航 handler 组装成链（见四）。
    KeyHandlers() []KeyHandler

    // —— 声明式元数据 ——
    Capabilities() Capability                 // 需对称挂卸的引擎资源（拼音层/英文词库/未来 emoji/url…）
    UsesExtendedPerPage() bool                // 是否用扩展档每页候选数
    PreferredLayout() config.CandidateLayout  // 期望布局（""=不强制）；决策器做布局去抖
    AcceptedProviders() []ProviderID          // 接纳哪些 Provider 融合（空=独占）。第一阶段恒空。
}
```

> 注意：Processor 不再有单体 `Handle(key)`。「按键怎么处理」下沉到 KeyHandler 链（四），
> 宿主只通过 `KeyHandlers()` 贡献自己**特有**的处理单元，通用导航交共享 handler。

### Capability 位掩码

不用具体 struct——未来 Emoji/URL/翻译/云词库都来，不应反复改接口：

```go
type Capability uint64

const (
    CapPinyinLayer Capability = 1 << iota // 拼音词库层挂载（污染五笔查询，必须对称）
    CapEnglishDict                        // 英文词库加载
    // 预留：CapEmoji / CapUrl / CapTranslate / CapCloudDict ...
)
```

宿主切换 A→B 时按 diff 对齐（见七·2）：`added := new &^ old; removed := old &^ new`。

---

## 四、KeyHandler 链：按键处理层（v3 核心）

按键处理是一条**有序责任链**，与宿主解耦。链上每个 handler 给出纯裁决，第一个非 Pass 者
生效（短路 → 保证单一归属）。

```go
type KeyHandler interface {
    Name() string
    // 纯判断：本 handler 对此按键的裁决（Pass/Handle/Activate/Release）。无副作用、可单测。
    Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision
    // 执行：仅当决策器选中本 handler 的 Handle 裁决时调用（持 *Coordinator，有副作用）。
    Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult
}
```

### 链的组装（决策器按 host 动态构建）

```
keyHandlerChain(host) =
  [全局分流 handlers]        # 固定最高优先；特殊情况拦截/分流，大多 Pass
    ++ host.KeyHandlers()    # 当前宿主特有键（大小写/拼音分隔符/自动上屏/触发键二次输入）
    ++ [共享导航 handlers]   # 翻页 / 高亮上下 / 数字选候选 / 二三候选键 / ESC / 退格删空
```

- **共享导航 handlers**：所有宿主复用，作用于公共候选区（`candidates`/`currentPage`/
  `selectedIndex`）+ 当前宿主 buffer 抽象。这是消除四套 `handleXxxKey` 重复的载体。
- **host 特有 handlers**：只处理本宿主独有语义。
- **全局分流 handlers**：实现「按键可由独立处理器分流」——例如某键在任何宿主下都要被
  特定逻辑接管，无需每个宿主各自感知。默认 Pass，不干扰常态。

### 与「宿主迁移」的衔接

「触发键 → 切宿主」本身也是链上一个 handler（`ModeTriggerHandler`），其 `Judge` 返回
`Activate`/`Release`。于是**宿主迁移与按键处理统一进同一条链**，决策器只需遍历链一次：

```
某 handler.Judge 返回：
  Pass     → 下一个 handler
  Handle   → handler.Apply（在当前宿主下处理，不切宿主）
  Activate → 决策器执行宿主迁移（见五·executeActivate），切到目标宿主
  Release  → 决策器卸当前宿主，带 residual 重判（一次）
```

> Processor.Judge（三）与 ModeTriggerHandler 是同一裁决的两个视角：第一阶段实现上，
> 「活跃宿主的迁移裁决」走 Processor.Judge（host 第一拒绝权），「触发键激活新宿主」走链上的
> ModeTriggerHandler。两者都产出 `Decision`，决策器统一消费。

### 渐进抽取策略（第一阶段务实）

第一阶段**不要求**立刻把四套 `handleXxxKey` 完全拆成 handler。落地顺序：

1. 定义 `KeyHandler` 接口 + 决策器的链遍历骨架。
2. 先抽取**共享导航 handlers**（翻页/高亮/数字选候选/ESC/删空）——收益最大、跨宿主一致、
   最易验证等价。
3. 各宿主的 `KeyHandlers()` 初期可只返回「特有」部分；其余暂由宿主旧逻辑兜底，逐模式迁移。
4. 全局分流 handler 留空（预留接口），有真实需求（如 URL/某热键级分流）时再填。

---

## 五、决策器求值算法：host 永不为空 + 全程持锁

**host 永不为空。** 启动即 `host = engine_default`；正常码表/拼音输入由它处理。host 生命
周期与 composition 完全对称：上屏/ESC/删空后 host **回落 engine_default**（而非 nil）。

```
decide(key, data) -> *KeyEventResult:    // 全程在 c.mu 下同步执行
    ctx := &DecisionCtx{c}               // 只读视图，零拷贝

    # ── 第一段：活跃宿主迁移裁决（第一拒绝权）──
    d := host.Judge(ctx, key, data)
    switch d.Verdict:
        Handle  -> goto 按键处理链           # 不切宿主，交链处理（高频打字落这里）
        Activate-> return executeActivate(host=自己? 否则切) # 宿主特有触发，罕见
        Release -> residual = d.Residual; markHotSwitch(); host 待定 ; goto 链(触发新宿主 Activate)
        Pass    -> goto 按键处理链

    按键处理链:
        for h in keyHandlerChain(host):    # 全局分流 + host特有 + 共享导航
            d := h.Judge(ctx, key, data)
            switch d.Verdict:
                Pass     -> continue
                Handle   -> return h.Apply(c, key, data)
                Activate -> return executeActivate(targetProcessor, d, residual)  # 切宿主
                Release  -> residual = d.Residual; 重判一次（不变量 I6）
        # 链无人认领 → host 回落 engine_default（若刚 Release）；否则交调用方（标点/透传）
        return nil
```

正常打字时 host=engine_default 的 Judge 返回 Handle/Pass，落按键处理链，由 engine_default
的字母 handler 处理——**第二段宿主列表遍历只在「触发键激活新宿主」时发生**，频率极低，回应
「列表过长影响高频路径」的性能担忧。

### 并发与锁

`HandleKeyEvent` 现已全程持 `c.mu`。`decide` 及全部副作用（Activate/Apply/Release/
applyEngineDiff/CompHot 重绘）**都在锁内同步完成**，`Residual` 锁内原子消费。快速击键的下一
KeyEvent 必等锁，不会在切换半途抢入。

### Activate 原子性与回滚

```
executeActivate(p, d, residual):
    oldHost, oldCaps := host, host.Capabilities()
    applyEngineDiff(oldCaps, p.Capabilities())
    prefix, ok := p.Activate(d.TriggerKey, residual)
    if !ok:
        applyEngineDiff(p.Capabilities(), oldCaps)   # 撤销引擎层变更
        host = oldHost                                # 宿主复位（不切 UI 档位，无残留）
        return oldHost.fallbackResult()
    host = p
    setHostUIState(p)                                # 覆盖式设 ModeLabel/AccentColor/Layout（去抖）
    return composeResult(prefix, d.CommitIdx)         # 据 CompositionPhase 产出
```

### 处理器列表排序（触发激活类）

```
orderedProcessors（高 → 低）:
  1. quick_input        ┐ 触发键激活类（现 triggerModes() 顺序）
  2. temp_pinyin        │
  3. special:<id>...    ┤ 特殊模式实例，按配置序插入
  4. temp_english       ┘
  5. url_english        ← 第二阶段新增，最低触发优先级
  6. engine_default     ← 永远在位的兜底宿主（默认 host）
```

> CodeTable / Mix / Pinyin / Shuangpin 后续可各自拆成独立处理器再融合，**第一阶段先用单一
> `engine_default` 包住现有引擎**，不拆。

---

## 六、Composition 生命周期与宿主切换解耦（兼容补丁的核心）

补丁普查的决定性发现：所有焦点/composition 补丁依赖「新 composition 创建」信号，该信号现由
「进入模式 → `armPendingFirstShow` + C++ `StartComposition`」产生。

**解法：把 composition 边界与宿主切换拆成正交事件。**

```go
type CompositionPhase int

const (
    CompCold   CompositionPhase = iota // 无 → 有：StartComposition + armPendingFirstShow
    CompHot                            // 有 → 有，仅宿主切换：原地换内容，不重启（★ 新增能力）
    CompCommit                         // 有 → 上屏后开新 composition：HasNewComposition + resetCompositionAnchorAfterCommit
    CompEnd                            // 有 → 无：clearState + hideUI
)
```

推导规则（host 永不为空下，「无 composition」= host 是 engine_default 且 buffer 空）：

| 起始 | 结果 | 是否上屏 | Phase |
|---|---|---|---|
| engine_default 空 buffer | Activate 某模式 | 否 | **CompCold** |
| engine_default 有候选 | Activate（顶码 CommitIdx≥0） | 是 | **CompCommit** → 再 Cold 起新 comp |
| 模式 A 活跃 | Release→Activate 模式 B | 否 | **CompHot** ★ |
| 模式 A 活跃 | Release 回 engine_default 空 / 上屏退出 | — | **CompEnd** |
| 同宿主 Handle | — | 否 | 无 phase 变化，原地 show |

**现有补丁不失效**——CompCold/CompCommit/CompEnd 完整保留 `armPendingFirstShow` /
`HasNewComposition` / `resetCompositionAnchorAfterCommit` / `clearState` 的全部触发时机。
`CompHot` 是新增「窗口不变」路径：**不** `hideUI`、**不**重 `StartComposition`/`armPendingFirstShow`
（避免重置 `compositionStartValid` 坐标锁定、避免触发 Excel/WPS cell-edit 重放误判），只换
`c.candidates` + 原地 `ShowCandidates`（`enterTempPinyinFromZBuffer` 已验证此路）。

### 受影响补丁与处置

| 补丁 | 字段 | CompHot 下处置 |
|---|---|---|
| 首次显示推迟 | `pendingFirstShow`/`Token` | **不重新 arm**；沿用已锁坐标。仅 Cold/Commit arm。 |
| 跨焦点重放 | `pendingReplay*`/`shouldDeferClearForReplay` | 不受影响（挂 FocusLost/Gained）。`replayBufferLen` 改读 `host.BufferText()`。 |
| composition 锚点锁定 | `compositionStartValid`/`X/Y` | **CompHot 不清不重设**——正是「窗口不变」要的。仅 End/Commit 清。 |
| 竞态保护 | `HandleCompositionTerminated` raceWindow | 不受影响。 |
| 嵌入式编码 | `isInlinePreedit` | preedit 仍由当前宿主单一产出。 |
| 应用兼容规则 | `activeCompatRule` | 不受影响（静态规则）。 |
| 密码框抑制 | `sensitiveFieldActive` | 不受影响——在决策器**之前**短路（保持现位置）。 |

### CompHot 的 TSF 风险监控

engine_default（buffer 含 `z`）热切 temp_pinyin 时 preedit 由 `z` 变无前缀拼音（内容
重写）。少数严格 undo 栈编辑器可能光标跳动。**第一阶段在 C++ TSF DLL 侧对 CompHot 的
`SetText` 加 telemetry**（宿主进程名 + 前后 preedit 长度），真机灰度观测。

---

## 七、特殊状态补丁清单与迁移策略

### 7.1 焦点 / composition（见六）

核心：CompHot 不重 arm、不重启 composition、不清锚点。

### 7.2 引擎副作用：Capability diff + 延迟卸载

现状：`ActivateTempPinyin`/`DeactivateTempPinyin` 散落 10+ 点，混输短路不对称、
`quickInputPinyinDictSwapped` 多出口易遗漏、z fallback 快速横跳漏卸载污染五笔。

**迁移：副作用由决策器在宿主切换时按 Capability 统一 diff，处理器不自行调。**

```go
func (d *Decider) applyEngineDiff(old, new Capability) {
    added, removed := new&^old, old&^new
    if removed&CapPinyinLayer != 0 { engine.DeactivateTempPinyin() }
    if added&CapPinyinLayer   != 0 { engine.EnsurePinyinLoaded(); engine.ActivateTempPinyin() }
    if added&CapEnglishDict   != 0 { engine.EnsureEnglishLoaded() }
    // old、new 都含某 cap → 不动（★ 去抖：解决 z fallback 反复横跳的层抖动）
}
```

收益：去抖（同需求不动）；对称性（混输 `Capabilities()` 不申报 CapPinyinLayer → diff 永远
no-op）；单点（消灭 `quickInputPinyinDictSwapped`）。

> **保持 eager 挂载**，不要 lazy-activate（评审一致警告失效窗口）。延迟卸载列为后续可选
> 优化（注明污染边界：切回 engine_default 须强制立即对齐拼音层）；第一阶段用即时 diff。

### 7.3 UI / 行为状态：统一卸载契约（修复现有不一致）

真 bug：`exitTempEnglishMode` 漏清 `SetModeAccentColor(nil)`；`lastOutputWasDigit` 不在 exit
清 → 跨模式标点误判；`pairTracker` 仅 `clearState` 清而 exit 不走 → 干扰新模式。

```go
func (c *Coordinator) clearHostUIState() {
    c.uiManager.SetModeLabel("")
    c.uiManager.SetModeAccentColor(nil)
    c.uiManager.SetQuickInputMode(false)
    c.restoreSavedLayout()
    c.pairTracker.Clear(); c.pairTrackerEn.Clear()
    c.lastOutputWasDigit = false
}
```

- **CompEnd**：`Release()` → `clearHostUIState()` + `clearState()`。
- **CompHot**：旧宿主只撤引擎副作用；UI 档位由新宿主覆盖式设置，无中间态闪烁。
- **布局去抖**：`PreferredLayout` 只在目标布局值实际变化时 `SetCandidateLayout`。
- **异常路径**：composition 被系统/宿主强制终止时，经现有 `clearState()` 路由到
  `clearHostUIState()`，保证不残留。重构时全链路走查。

### 7.4 每页候选数与分级加载

- `refreshEffectivePerPage` 改读 `host.UsesExtendedPerPage()`。
- 分级加载三件套现每模式有副本。**第一阶段保留副本**（行为不变），第三批统一为宿主单份。

---

## 八、Processor 字段读写契约（防隐式时序耦合）

`Judge` 纯函数靠 DecisionCtx 只读保障。`Activate`/`Apply`/`Release` 持 `*Coordinator` 会读写
全局状态，需在文件头声明读写集。强约束：**处理器/handler 只写自己模式命名空间 + 公共候选区
（candidates/page/selectedIndex，当前宿主独占写）+ preedit；不写他模式字段。** 公共候选区是
唯一共享写区。

DecisionCtx 为只读视图（指针 + Getter，不泄露内部 slice/map）。**buffer 访问器按 host 路由**
——这是第 0b 影子实测暴露的修正：真实路径进入临时拼音/快捷输入后 `c.inputBuffer` 为空但
`c.candidates` 有候选（那些模式用各自 buffer 字段，如 `tempPinyinBuffer`），若固定读
`inputBuffer`，engine_default 会误判"空 buffer + 字母 → Handle"。故 `BufferText/BufferLen`
委派给当前 host（各宿主自报活跃 buffer），候选区仍读公共 c：

```go
type DecisionCtx struct {
	c    *Coordinator
	host Processor // buffer 访问器委派给它
}
func (ctx *DecisionCtx) BufferText() string      { return ctx.host.BufferText() } // 按 host 路由
func (ctx *DecisionCtx) BufferLen() int          { return len(ctx.host.BufferText()) }
func (ctx *DecisionCtx) CandidateCount() int     { return len(ctx.c.candidates) } // 公共候选区，读 c
func (ctx *DecisionCtx) EngineIsCodeTable() bool { /* ... */ }
func (ctx *DecisionCtx) HasPrefix(s string) bool { /* z fallback / engine_default */ }
// …Judge 只能读。Processor 接口因此新增 BufferText()：engine_default→inputBuffer、
// temp_pinyin→tempPinyinBuffer、quick_input→quickInputBuffer。
```

> 影子日志的黄金验证：`zh` 无字时影子判 `Release` 的**同一帧**，旧路径真实执行了
> `Entered temp pinyin from z fallback`（注册拼音词库层）——证明 z fallback 的 Release 判定
> 时机与旧行为字节级一致，这是第 1 批 CompHot 切换的判定基础。

---

## 九、候选 Provider 融合层（第二阶段，仅预留）

### 9.1 候选带血缘

```go
type Candidate struct {
    Text           string
    Weight         int
    Source         ProviderID // 选中后该谁来 commit
    ConsumedLength int        // 已消耗 buffer 长度（拼音/码表切分不同）
    Payload        any        // provider 私有上下文
}
```

### 9.2 合并器：分段而非全局排序

**不要用统一 Weight 拉平**——结构化候选（日期/计算/URL）靠 provider 级 `Rank` 占固定段位；
同质语言候选才在段内按 Weight 竞争。现 `updateQuickInputCandidates` 的顺序 append 即隐式 Rank。

### 9.3 按键仲裁三定律

1. 唯一宿主独占有语义按键；2. 其他 Provider 只供候选条目、经宿主统一编号选中；3. 选中按
`Source` dispatch commit。**默认仲裁：快捷输入+拼音融合时，数字键归宿主选候选，拼音只能用
翻页键。**

### 9.4 准入白名单取代全局 Exclusive

`AcceptedProviders()`：URL 宿主拒绝一切（=独占），快捷输入接纳 pinyin/date/calc。

### 9.5 首个融合靶子 ✅（已落地，见第 12 节第 4 批 S1-S4）

快捷输入 + 拼音：已消灭 `quickInputPinyinMode`，拼音候选经 `pinyinProvider` 取源。
实证：快捷输入候选是 XOR（拼音 vs 结构化由 buffer 内容互斥），故未触发真正的"多源同列表合并"——
真正的多源融合靶子留待 url_english/emoji 等共存宿主。

---

## 十、测试策略与示例

### 10.1 Judge 表驱动单测（核心红利）

`Judge`（Processor 与 KeyHandler 均）纯函数 → 表驱动覆盖全路径，不建真引擎/UI：

```go
func TestEngineDefaultJudge(t *testing.T) {
    cases := []struct {
        name, buffer string; hasPrefix bool; key string
        want Verdict; wantResid string
    }{
        {"普通字母有前缀", "ji", true, "a", Handle, ""},
        {"z后续失配回退", "z", false, "q", Release, "q"},  // → temp_pinyin Activate
        {"z后续仍匹配", "z", true, "h", Handle, ""},
        {"空buffer触发键让位", "", false, ";", Pass, ""},   // → ModeTriggerHandler Activate quick_input
    }
    for _, tc := range cases {
        ctx := newMockCtx(tc.buffer, tc.hasPrefix)
        got := engineDefault.Judge(ctx, tc.key, mockKeyData(tc.key))
        assertVerdict(t, tc.name, got, tc.want, tc.wantResid)
    }
}
```

### 10.2 mock 与迁移

- `newMockCtx(...)`：构造只读 DecisionCtx 轻量替身。
- 共享导航 handler 的 `Judge` 可脱离宿主单测（给定候选数/页码 → 期望裁决）。
- 集成：feature flag on，跑现有 `routing_basic_test.go`/`mode_trigger_test.go`/`z_*_test.go`
  验证新旧一致。`decideBufferedTrigger` 用例平移为决策器/handler 用例。

---

## 十一、可观测性与性能监控

- **第一拒绝权命中率**：host.Judge 直接落链处理占比，预期 ≥95%。复用 `keyPhaseTimer` 埋点。
- **链遍历深度**：记按键命中链上第几个 handler，监控链增长对高频路径影响。
- **CompHot 计数 + TSF telemetry**：宿主进程名 + 切换前后 preedit 长度。
- **applyEngineDiff 抖动计数**：单位时间拼音层 Activate/Deactivate 次数，验证去抖。
- 均 DEBUG/元数据级，遵守日志隐私（不记输入内容/候选文本）。

---

## 十二、分批实施计划（feature flag + 影子运行）

> **发布状态（2026-06-14）✅ 决策器已成唯一路径，decider-off 退役**：决策器在 `NewCoordinator` 内
> **无条件构造并接管** `HandleKeyEvent`，已无 `decider_enabled` 回退开关。退役前经 **E2E golden A/B**
> （`WIND_E2E_DECIDER=1` 复跑全套 golden）+ 真机长期验证与旧逻辑逐字节等价后，删除了：`wind_dev.toml`
> 双开关（`decider_enabled`/`decider_shadow`）、`dev_config.go`、影子运行 `shadowLog`、E2E 的
> `SetDeciderEnabledForTest`/`WIND_E2E_DECIDER` A/B 脚手架，以及旧并行旁路 `getQuickInputTriggerKey`/
> `getTempEnglishTriggerKey`/`enterQuickInputMode`/`enterTempEnglishModeWithTrigger`/`enterTempPinyinMode`/
> `zHybridFallback`（z 回退判定改由 `engineDefaultProcessor` 的纯函数 `decideEngineDefaultZFallback`
> 单点承载，`z_decision_test.go` 经 `judgeZFallback` 入口回归）。旧 `handleXxxKey` 仍作为决策器链上
> `Apply` 的被调实现保留（删除待 KeyHandler 链分解，目标③）。E2E 不再 A/B——单跑即决策器路径。
>
> **退役理由（alpha 阶段决策）**：双路并存让"主路径进决策器"等后续重构反而更难（须同时维护两套 +
> A/B parity 约束）；alpha 阶段尽早让决策器成为唯一路径、尽早暴露问题，优于长期保留回退分支。
>
> **z 首次触发收编（2026-06-14）**：原散落在 `getTempPinyinTriggerKey` KeyZ 分支的「重复上屏历史 /
> z 码表前缀 / 否则进临时拼音」渐进仲裁，已经 `tempPinyinProcessor.Judge`→`judgeZFirstTrigger` 收编进
> registry，与标点触发键一样经 `tryActivateFromEmpty` 激活——z 不再是决策器的特殊旁路。`judgeZFirstTrigger`
> 直接复用旧 `getTempPinyinTriggerKey` 的判定，保证逐字节等价（A/B golden + `z_first_trigger_test.go`）。
> z 混合回退（②，`judgeZFallback`）此前已收编。**仍留旧路径**：z 重复上屏的选词上屏（`handle_candidates.go`，
> 属正常码表选词，待 KeyHandler 链分解批次）；z 临时拼音**不融合**英文/生僻字（按需求保持）。
>
> **KeyHandler 链分解（2026-06-14，已接入 temp_pinyin / special / quick_input 双上下文）**：把翻页/高亮从
> 「整模式」handler 抽成链上独立的通用 `navKeyHandler`（`pipeline_nav_handler.go`）——模式特有 handler 的
> `Judge` 对导航键 **Pass** 让位、nav handler **Handle**，二者经同一 `navKeyMatch`（注入翻页谓词 + 通用高亮
> 谓词）保证「Pass ⟺ Handle」同步、I11 单一归属、零漂移。**宿主差异（翻页谓词 pageUp/pageDown、showUI、
> pageDownExpand、hiDownExpand）经构造参数注入，同一 navKeyHandler 被 temp_pinyin/special/quick_input 复用**
> （true sharedNav）。quick_input **双上下文**：拼音上下文用标准翻页谓词 + showPinyinModeUI，基础上下文用专用
> `isQuickInputPageUpKey`（排除 -/= 输入字符）+ showQuickInputUI，按 `quickInputPinyinActive` 选用。A/B golden
> （`mode_temp_pinyin_paging`/`special_highlight_nav`/`mode_quick_input_pinyin_highlight`/`mode_quick_input_base_highlight`，
> 含高亮真状态变化）+ 单测验证逐字节等价。**temp_english（2026-06-14，四模式收官）**：其导航语义不同（翻页
> expandBefore=true、高亮 expand 在移动前 + showUI 无条件刷新），故高亮抽成 `tempEnglishHighlightUp/Down` 方法、
> 用**专用** `tempEnglishNavKeyHandler` 包装精确逻辑（不复用通用 navKeyHandler），行为字节级不变；mode handler Judge
> 对 allow_symbols 符号字符 Handle 优先（保旧 switch 顺序）。golden `mode_temp_english_highlight`。**至此四模式
> （temp_pinyin/special/quick_input/temp_english）导航全部从整模式 handler 移入决策器链。** **后续待办**：模式特有键
> 进一步细分（字母/数字/选词等拆为独立 handler）；引擎副作用 `applyEngineDiff` 单点化（I3）；url_english 处理器。
>
> **engine_default 正常输入进决策器（2026-06-15，终态骨架）**：原 `HandleKeyEvent` 中文模式末尾的大 switch
> （导航/光标/编辑/上屏/字母含 z 回退/数字/以词定字/拼音分隔符/标点）**逐字节搬入** `handleEngineDefaultKey`
> （`handle_engine_default.go`），包成恒 Handle 的 `engineDefaultKeyHandler` 进 `engine_default.KeyHandlers()`；
> `HandleKeyEvent` 尾部仅余一行 `dispatchHostChain`。**至此五受管宿主 + engine_default 兜底宿主全部经决策器链，
> switch 不再是内联特例，I1（host 永不为空）名副其实。** 因 decider-off 已退役、A/B parity 约束消失，本步骤
> 直接进行、零开关；安全网为新增的 e2e golden（`engine_default_cursor_move`/`_delete`/`_highlight_nav`/`_escape`/
> `_enter_raw`/`_pinyin_separator`）+ 既有核心 golden。`prevDigitState` 经 `c.keyPrevDigitState` 透传（Apply 签名固定）。
> `dispatchManagedHost` 因不再限于受管宿主，更名 `dispatchHostChain`。**后续**：engine_default/url 仍是单一整模式
> handler，把翻页/高亮拆给共享 `navKeyHandler`（如其它受管宿主）是下一步内部分解。
>
> **applyEngineDiff 引擎层副作用单点化（I3，2026-06-15）**：决策器新增 `mounted Capability` 字段 + `applyEngineDiff(needed)`，
> 用既有 `computeCapabilityDiff` 据 `mounted` 去抖驱动**需对称管理**的引擎层挂卸（目前仅 `CapPinyinLayer`：码表引擎下
> `ActivateTempPinyin` 移码表层挂拼音层、`DeactivateTempPinyin` 逆操作；混输引擎内部 no-op）。临时拼音/快捷拼音子上下文
> 的进出各入口（setup/exit/clearState/rewind/engage/exit-to-base）统一改调 `applyEngineDiff(CapPinyinLayer)` / `(0)`，
> 删除旧 `setQuickInputPinyinLayer` + `quickInputPinyinDictSwapped` + 散落的 6 处 `Activate/DeactivateTempPinyin`。
> **关键认知**：引擎方法本就幂等 + 混输 no-op（manager_temp_pinyin.go），故此为「单点对称、防泄漏」的优雅化而非修错；
> 各调用点**时机不变**（仅统一「怎么挂卸」），零时机风险。English/拼音引擎/生僻字是只加载不卸载的幂等资源，不进 diff。
> 可观测性：`State.PinyinLayerMounted`（读 `d.mounted&CapPinyinLayer`），e2e `TestEngineLayerSymmetry` 显式断言 +
> 8 个 `*_pinyin_*` golden 的 `pinyin_layer_mounted` 字段把「进=挂载/退=卸载」对称无泄漏变成可见且强制的不变量。
> 真机日志亦佐证（临时拼音「移除码表层 / 恢复码表层」13/13 对称）。

**贯穿全程：开关控制，可回退旧 `HandleKeyEvent`。** 第 0b 影子运行的开关放在独立开发
配置 `wind_dev.toml`（`decider_shadow`，见 `internal/coordinator/dev_config.go`），**不进主配置
流程**（无 const-gen / 前端 / 版本迁移桥接），避免临时调试开关污染用户主配置；第 1 批起真正
接管主路径的开关再定（沿用 `wind_dev.toml` 或转正式配置）。

### 第 0 批：地基 + 影子运行，零行为变化
- 定义 `Verdict`/`Decision`/`Processor`/`KeyHandler`/`DecisionCtx`/`Capability`/`CompositionPhase`。
- 建决策器骨架 + registry + 链遍历骨架。`engine_default` 作默认 host 接入。
- **flag off**：旧路径照常；新决策器**影子运行**——并行算 `Decision` 仅记日志比对，不接管输出。
- 验证 shadow 裁决与旧行为一致。安全网。

### 第 1 批：打通 engine_default ⇄ temp_pinyin 热切换（细分三步，逐步降风险）

**1a（基础，不接管，纯新增 + 单测）**：
- Processor 接口加 `BufferText()`；DecisionCtx 按 host 路由 buffer（修正第 0b 影子发现的失真）。
- 实现 `temp_pinyin` Processor：`Judge`（候选角色=触发键激活裁决）/ `Activate`（复用
  `setupTempPinyinMode`）/ `Release`（1c 落地）/ `BufferText`(=tempPinyinBuffer) /
  `Capabilities`(=CapPinyinLayer)。注册到 `decider.registry`。影子仍只读、host 仍不切换，零行为变化。

**1b（影子增强，验证切换裁决）**：
- 影子模式下让决策器**模拟**宿主切换（独立影子 host 状态，仍不影响真实输出），记日志验证
  engine_default→temp_pinyin→engine_default 完整裁决链与旧路径一致。

**1c（真正接管，CompHot）**：
- flag on（`wind_dev.toml` `decider_enabled`），决策器接管 engine_default ⇄ temp_pinyin。
- 抽取**共享导航 handlers**（翻页/高亮移动等纯导航可共享，UI 刷新/选候选 commit 由 host 回调）。
- 用 `Release(residual) → temp_pinyin.Activate(residual)` 落地 z fallback 走 CompHot。
- 落地 CompositionPhase 推导 + CompHot「不 hideUI/不重 arm」+ Activate 回滚。真机验证。

### 第 2 批：副作用 diff + 统一清理
- 迁 quick_input 为 Processor。落地 `applyEngineDiff`，删 `quickInputPinyinDictSwapped`。
- 落地 `clearHostUIState`，修复 7.3 跨模式污染 bug。

### 第 3 批：special / temp_english 收尾
- 迁余下两模式，各自 `KeyHandlers()` 完成抽取。统一分级加载状态为宿主单份。

### 第 4 批：融合（第二阶段）

按切片实施（S1-S4 已落地，逐片单测+审查+真机手测后提交）：
- **S1 ✅** `CandidateProvider` 接口（`ID`/`Rank`/`Query` 纯查询）+ `mergeProviderCandidates`（Rank 分段拼接 + Text 去重）。**候选血缘复用既有 `candidate.Source`/`ConsumedLength`，不另造平行类型**（commit 归属靠 `Source` 区分：拼音 `SourcePinyin && ConsumedLength>0` 分段上屏 / 结构化 `Source` 空整条上屏）。
- **S2 ✅** date/calc/number 抽成 `dateProvider`/`calcProvider`/`numberProvider`，`updateQuickInputCandidates` 改走 `mergeProviderCandidates`（字节对拍测试锁等价）。
- **S3 ✅** `pinyinProvider`（Rank 40），`Query` 委托 `ConvertWithPinyin`；具体方法 `query()` 兼返 `PreeditDisplay`。
- **S4 ✅** 消灭 `quickInputPinyinMode` 布尔 + 独立 buffer：拼音上下文由 `quickInputPinyinActive()` 从 buffer 内容派生（首字母 a-z ⟺ 拼音）；共享 `updatePinyinModeCandidates` 内部经 `pinyinProvider.query()` 取源（temp_pinyin + quick_input 统一）；`setQuickInputPinyinLayer` 幂等挂卸词库层。

> **实现校准（重要）**：快捷输入的候选实为 **XOR**——拼音上下文 vs 结构化（date/calc/number）由 buffer 内容互斥，**从不同时呈现**。故 §9.2 的"分段合并"对本宿主退化为"结构化段合并 + 拼音段单独路由"，§9.3 的"数字键仲裁"由**上下文路由天然解决**（拼音上下文数字选候选、结构化上下文数字追加 buffer），**无需独立仲裁层**；§9.4 的 `AcceptedProviders` 白名单"拒绝"语义只在引入 url_english 等**真正多源共存**宿主时才有意义。

**URL 临时输入模式 ✅（2026-06-14 实现）**：`urlProcessor`（`pipeline_url.go`）+ `handle_url.go` + `config.UrlInputConfig`（默认关，前缀 www./http/https/ftp.）。**悲观全匹配**：正常输入下 `inputBuffer+本键` 恰好等于某前缀即夺取（`urlActivationResidual` 钩子，须置于 buffered-trigger 与标点处理**之前**，使带 '.' 前缀的 '.' 在被当标点前被拦截——单钩子覆盖字母结尾与点结尾两类前缀）。前缀作初始 buffer，模式内任意 ASCII 可见字符追加不上屏、仅空格/回车上屏、退格删空退出。受管宿主但**不入 registry**（激活非触发键驱动）。无候选纯 preedit。e2e `mode_url_http`/`mode_url_www`/`mode_url_esc`/`mode_url_rewind`（首次退格回退）/`mode_url_rewind_back_to_prefix`（编辑后删回前缀回退）/`mode_url_rewind_after_content`（打 http:// 删回前缀回退）。**简化**：未走设计原定的 engine_default `Release`（engine_default 正常输入尚未入决策器），改用正常输入路径显式钩子，等价效果。

**统一夺取回退 ✅（2026-06-14）**：「夺取式激活」（z 键混合切入、URL 前缀夺取）是推断进入、可能误判的，故配一个对称的「一键撤销」出口（本质是夺取 Release→Activate 的逆，见第八节 §82）。**决策器统一持有**快照 + 执行：`armRewind`（夺取 host 进入时登记：快照=夺取前 inputBuffer、hostText=进入后 host buffer、cleanup 闭包）/ `canRewind`（当前 host buffer 与登记一致=未编辑）/ `rewindHijack`（清 host 状态 + 还原正常输入 + 回落 engine_default）。触发在 `HandleKeyEvent` 模式分发**之前**（故用 cleanup 闭包而非 stale 的 `host.Release()`）：`rewindArmed` 时退格且 `canRewind`（当前 host buffer == 登记的 hostText）→ 回退。**单键 vs 多键夺取的作废语义不同（2026-06-15 修正）**：z 是单键夺取，模式内任意其它键即作废登记（确认用临时拼音）；**URL 是多键夺取**（前缀由多键逐字打成），续打网址内容**不**作废登记——使退格删回前缀边界（urlBuffer 退回 == residual 前缀）时再退格仍能撤销夺取、回正常输入流（前缀本是被夺取的正常输入，删进它即撤销）。URL 仅正常退出（上屏/ESC/删空）时由 `exitUrlMode` 作废。z 旧的 `tempPinyinRewindBuffer/Key` + `rewindTempPinyinToNormal` 已删并迁入此机制。

**推迟**：
- `AcceptedProviders` 白名单驱动 merge（当前各宿主返回 nil，候选源由宿主按上下文硬路由；url 独占=拒绝一切，待 emoji 等真正多源共存宿主出现时落地白名单）。
- `applyEngineDiff` 统一引擎副作用 diff（当前 `setQuickInputPinyinLayer` / `setup`/`exit`/`clearState` 既有路径管，对称性经真机日志验证 22/22）。

> 加词模式（`addWordActive`）是功能模式，**不参与**。命令直通车是候选选中后的解析/动作层，
> **不是**处理器。

---

## 十三、已决决策点

1. **engine_default 降格为处理器** —— ✅ 并定为**永不为空的默认 host**。靠表驱动测试 +
   feature flag + 影子运行兜底。
2. **宿主第一拒绝权** —— ✅「活跃宿主动态最高、仅 Release 让位」。边界待真机测试补充。
3. **URL 渐进方向** —— ✅ **悲观，仅完整前缀全匹配时才处理**。前缀字符作 `Residual` 经
   Release→url Activate 转入，复用 z fallback 机制，无需 rewind。
4. **分级加载副本统一时机** —— ✅ 第一阶段保留副本，第三批统一。
5. **按键处理解耦（v3）** —— ✅ 引入 KeyHandler 链，按键与宿主解耦、支持分流；第一阶段先
   定接口 + 抽共享导航 handler，逐模式迁移。

---

## 十四、长期演进方向（备忘）

终态四层流水线：

```
Key → Decision(决策器) → KeyHandler(按键处理) → Processor(宿主状态) → Provider(候选) → Merge
    → Renderer(UI) → CompositionController(TSF)
```

`CompositionController` 把散落的 composition 生命周期收成独立层（`CompositionPhase` 是雏形）；
Renderer 把各 `showXxxUI` 重复（caret 兜底/分页切片/InlinePreedit 空壳）收成单一入口。第 4 批
之后的事，此处仅记录方向。

---

## 附录 A：关键不变量（实现时必须守住）

- **I1**：host **永不为空**。启动 `host = engine_default`；上屏/ESC/删空后回落 engine_default。
- **I2**：`Judge`（Processor 与 KeyHandler）无副作用、极轻量（无锁/IO/复杂正则），可多次调用。
- **I3**：引擎副作用只在决策器 `applyEngineDiff` 单点变更，处理器/handler 不自行 Activate/Deactivate。
- **I4**：CompHot 路径绝不调用 `hideUI`/`armPendingFirstShow`/`StartComposition`。
- **I5**：所有宿主卸载经 `clearHostUIState`；异常终止路径亦须路由到它。
- **I6**：一次按键最多一次宿主迁移（一次 Release）。禁止链式跳转。
- **I7**：`decide` 及全部副作用在 `c.mu` 内同步完成；`Residual` 锁内原子消费。
- **I8**：处理器/handler 只写自己模式命名空间 + 公共候选区（当前宿主独占）+ preedit。
- **I9**：第一阶段 `AcceptedProviders()` 恒空，候选行为与现状逐条等价。
- **I10**：`Activate` 失败（ok=false）必须无残留回滚到上一健康宿主。
- **I11**：按键归属单一——链上第一个非 Pass 的 handler 认领，短路；不存在两 handler 同抢。

## 附录 B：版本修订摘要

| 项 | v1 | v2 | v3 |
|---|---|---|---|
| 核心层 | Processor / Provider 两层 | 同 v1 | **+ KeyHandler 链三层**，按键与宿主解耦 |
| 按键处理 | 宿主单体 Handle | 同 v1 | **KeyHandler 链**：全局分流 + 宿主特有 + 共享导航 |
| 导航重复 | 四套 handleXxxKey 各自实现 | 同 v1 | **共享导航 handler** 复用，渐进抽取 |
| host 生命周期 | 可空，engine_default 非 host | **永不为空**，默认 host | 同 v2 |
| DecisionCtx | snapshot | **只读视图** | 同 v2 |
| 引擎副作用声明 | EngineRequirement{bool} | **Capability 位掩码** | 同 v2 |
| 宿主迁移次数 | 未约束 | **每键最多一次（I6）** | 同 v2 |
| Activate 失败 | 未定义 | **原子回滚（I10）** | 同 v2 |
| 锁语义 | 未明示 | **decide 全程持 c.mu（I7）** | 同 v2 |
| 字段读写契约 | 无 | **第八节** | 同 v2 |
| 测试/可观测性 | 仅提"可表驱动" | **第十、十一节** | handler 亦可单测 |
| 第 0 批 | engine_default 走新路 | **feature flag + 影子运行** | 同 v2 |
| URL 方向 | 未定 | **悲观/全匹配** | 同 v2 |
| 单一归属保证 | 绑死宿主 | 同 v1 | **链短路（I11）**，非绑死宿主 |
