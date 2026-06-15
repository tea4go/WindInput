// pipeline_decider.go — 统一决策器。
//
// 决策器是 HandleKeyEvent 键事件的唯一主路径：host 状态机（markEntered/reconcileHost/
// dispatchHostChain）、触发激活（tryActivateFromEmpty/tryActivateSpecial）、z 回退判定、
// 统一夺取回退（armRewind/canRewind/rewindHijack）、引擎层单点挂卸（applyEngineDiff）。
// decide() 为早期求值骨架，现仅余 tryActivate*/judgeZFallback/dispatchHostChain 等具体入口被调用。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

// decider 是统一决策器。host 永不为空（I1）：启动即 engine_default。
type decider struct {
	c    *Coordinator
	host Processor

	engineDefault Processor // 兜底宿主单例（host 回落目标）
	tempPinyin    Processor // 受管宿主单例（decide() 全接管）
	quickInput    Processor // 受管宿主单例（decide() 全接管）
	tempEnglish   Processor // 受管宿主单例（decide() 全接管）
	special       Processor // 受管宿主单例（模式内键全接管；触发仍走旧 2 步匹配，不入 registry）
	url           Processor // 受管宿主单例（激活走正常输入路径前缀夺取钩子，不入 registry）

	registry  []Processor  // 触发激活类宿主，按优先级（高→低）
	sharedNav []KeyHandler // 共享导航 handler（翻页/选候选/导航/删空）
	global    []KeyHandler // 全局分流 handler（预留，第 4 批按需填充）

	// ── 夺取回退（统一）─────────────────────────────────────────────
	// "夺取式激活"（z 键混合回退、URL 前缀夺取）是从正常输入推断进入的，可能误判，故需一个
	// 对称的"一键撤销"出口：刚夺取、未编辑时第一次退格 → Release 当前 host、还原夺取前的正常
	// 输入流（本质是夺取 Release→Activate 的逆，见设计文档第八节）。状态与执行由决策器统一持有，
	// 各夺取 host 在进入 funnel 调 armRewind 登记；触发在 handleKeyEvent 模式分发前（两路一致）。
	rewindBuffer   string // 夺取前的正常输入 inputBuffer 快照（回退时还原）
	rewindHostText string // 夺取瞬间的 host buffer（判定"未编辑"：当前 host buffer 与此一致才可回退）
	rewindCleanup  func() // 清当前夺取 host 的模式状态 + 引擎层（各 host 注入）

	// mounted 是当前已挂载的「需对称管理」引擎资源位集（I3 单点 diff 的唯一真值源）。
	// 目前仅 CapPinyinLayer（拼音词库层，码表引擎下交换、混输 no-op）。English/拼音引擎/生僻字
	// 是只加载不卸载的幂等资源，不进 diff。各挂卸入口统一调 applyEngineDiff，杜绝散落不对称。
	mounted Capability
}

func newDecider(c *Coordinator) *decider {
	d := &decider{c: c}
	d.engineDefault = newEngineDefaultProcessor(c)
	d.tempPinyin = newTempPinyinProcessor(c)
	d.quickInput = newQuickInputProcessor(c)
	d.tempEnglish = newTempEnglishProcessor(c)
	d.special = newSpecialProcessor(c)
	d.url = newUrlProcessor(c)
	d.host = d.engineDefault // host 永不为空
	// 触发激活类宿主，按优先级（高→低），对齐旧 getXxxTriggerKey 顺序。special 暂走旧逻辑。
	// 受管宿主（quick_input/temp_pinyin/temp_english）复用单例，使激活后 d.host 与 registry 实例同一。
	d.registry = []Processor{
		d.quickInput,
		d.tempPinyin,
		d.tempEnglish,
	}
	// sharedNav / global 在后续批次填充。
	return d
}

// isManaged 标识 decide() **全接管**的宿主：其内部按键走 decide()、d.host 由决策器维护、
// 退出经 reconcileHost 回落。当前 temp_pinyin + quick_input + temp_english + special。
// special 的触发仍走旧 2 步匹配（不在 registry），但模式内键 + host 由决策器全接管。
func (d *decider) isManaged(p Processor) bool {
	return p == d.tempPinyin || p == d.quickInput || p == d.tempEnglish || p == d.special || p == d.url
}

// modeActive 据模式真值源判定受管宿主 p 是否仍活跃。模式真值源仍是各 c.xxxMode 布尔
// （被 refreshEffectivePerPage/lifecycle/showUI 等多处读取，不可替换）；d.host 是其镜像。
// engine_default 与非受管宿主恒返回 false（不参与 reconcile 的「仍活跃」判定）。
func (d *decider) modeActive(p Processor) bool {
	switch p {
	case d.tempPinyin:
		return d.c.tempPinyinMode
	case d.quickInput:
		return d.c.quickInputMode
	case d.tempEnglish:
		return d.c.tempEnglishMode
	case d.special:
		return d.c.specialMode
	case d.url:
		return d.c.urlMode
	default:
		return false
	}
}

// reconcileHost 在受管宿主的一次 Apply 之后，据模式真值源回填 d.host：当前 host 的模式已退出
// （mode→false）即回落 engine_default。注意查的是**外层**模式标志——如 quick_input 的拼音
// 上下文（buffer 以字母打头，由 quickInputPinyinActive 派生）退出回基础时 quickInputMode 仍为
// true，host 应保持 quick_input。
func (d *decider) reconcileHost() {
	if d.host == d.engineDefault {
		return
	}
	if !d.modeActive(d.host) {
		d.logSwitch(d.host, d.engineDefault)
		d.host = d.engineDefault
	}
}

// markEntered 在受管宿主经某入口进入后把 d.host 对齐到该单例。仅当对应模式真置位时设，
// 避免 setup 失败（如 enterTempPinyinFromZBuffer 引擎加载失败返回 nil）时误切 host。
func (d *decider) markEntered(p Processor) {
	if d.modeActive(p) && d.host != p {
		d.logSwitch(d.host, p)
		d.host = p
	}
}

// onTempPinyinEntered / onQuickInputEntered / onTempEnglishEntered 是 markEntered 的具名薄
// 包装，供主路径各进入 funnel 调用（z 回退 / 触发键 / buffer 非空顶码 / 热键 / Shift+字母），
// 语义更直观。
func (d *decider) onTempPinyinEntered()  { d.markEntered(d.tempPinyin) }
func (d *decider) onQuickInputEntered()  { d.markEntered(d.quickInput) }
func (d *decider) onTempEnglishEntered() { d.markEntered(d.tempEnglish) }
func (d *decider) onSpecialEntered()     { d.markEntered(d.special) }
func (d *decider) onUrlEntered()         { d.markEntered(d.url) }

// keyHandlerChain 按当前 host 动态组装按键处理链：全局分流 + 宿主特有 + 共享导航。
func (d *decider) keyHandlerChain() []KeyHandler {
	chain := make([]KeyHandler, 0, len(d.global)+len(d.sharedNav)+4)
	chain = append(chain, d.global...)
	chain = append(chain, d.host.KeyHandlers()...)
	chain = append(chain, d.sharedNav...)
	return chain
}

// decide 求值算法骨架（全程应在 c.mu 内调用，I7）。
// 返回 (result, handled)：handled=false 表示本决策器未接管，调用方继续旧路径。
func (d *decider) decide(key string, data *bridge.KeyEventData) (*bridge.KeyEventResult, bool) {
	ctx := newDecisionCtx(d.c, d.host)

	// 第一段：活跃宿主迁移裁决（第一拒绝权）。
	switch d.host.Judge(ctx, key, data).Verdict {
	case VerdictActivate, VerdictRelease:
		// 宿主迁移在第 1 批落地（executeActivate/applyEngineDiff/CompositionPhase）。
		// 第 0 批骨架：暂不执行迁移，交旧路径。
		return nil, false
	}

	// 第二段：按键处理链遍历（短路于第一个非 Pass，I11）。
	for _, h := range d.keyHandlerChain() {
		switch hd := h.Judge(ctx, key, data); hd.Verdict {
		case VerdictPass:
			continue
		case VerdictHandle:
			return h.Apply(d.c, key, data), true
		case VerdictActivate, VerdictRelease:
			// 链上迁移裁决，第 1 批落地。
			return nil, false
		}
	}
	return nil, false
}

// judgeZFallback 用决策器（engine_default 宿主裁决）判定 z 键混合回退。返回 (residual, true)
// 表示应回退临时拼音，residual 为初始拼音 buffer（判定逻辑在 decideEngineDefaultZFallback，含 z
// 触发键门禁）。执行复用 enterTempPinyinFromZBuffer（CompHot 原地切换，不 hideUI）。
func (d *decider) judgeZFallback(key string, data *bridge.KeyEventData) (string, bool) {
	ctx := newDecisionCtx(d.c, d.host)
	if dec := d.host.Judge(ctx, key, data); dec.Verdict == VerdictRelease {
		return dec.Residual, true
	}
	return "", false
}

// tryActivateSpecial 在旧 special 触发位置（getXxxTriggerKey 之后，保持 special-last 优先级，
// 不混入 tryActivateFromEmpty 的 registry 以免被提到 z 首触发之前）走决策器接管 special 激活。
// 返回 (result, true) 已接管；(nil, false) 交旧路径。
func (d *decider) tryActivateSpecial(key string, data *bridge.KeyEventData) (*bridge.KeyEventResult, bool) {
	ctx := newDecisionCtx(d.c, d.host)
	if dec := d.special.Judge(ctx, key, data); dec.Verdict == VerdictActivate {
		return d.executeActivate(d.special, dec)
	}
	return nil, false
}

// tryActivateFromEmpty 在 buffer 空/无候选时遍历 registry（按优先级 quick > temp_pinyin >
// temp_english，含 z 首次触发），第一个判 Activate 的宿主接管激活。返回 (result, true) 表示已接管；
// (nil, false) 未接管（如 special——不在 registry，由 tryActivateSpecial 单独处理）。
func (d *decider) tryActivateFromEmpty(key string, data *bridge.KeyEventData) (*bridge.KeyEventResult, bool) {
	ctx := newDecisionCtx(d.c, d.host)
	for _, p := range d.registry {
		if dec := p.Judge(ctx, key, data); dec.Verdict == VerdictActivate {
			return d.executeActivate(p, dec)
		}
	}
	return nil, false
}

// executeActivate 执行宿主激活（触发键路径，buffer 空 → 无候选，preedit=prefix），等价旧
// enterXxxMode（setupXxxMode + modeCompositionResult）。
//
// 受管宿主（isManaged：temp_pinyin/quick_input/temp_english/special）激活后经 markEntered 设
// d.host=p，使模式内键经 dispatchHostChain 走 decide()。markEntered 自带 modeActive 守卫，
// 仅在模式真置位时设 host；非受管宿主（仅 engine_default）不会进 registry/此路径。
func (d *decider) executeActivate(p Processor, dec Decision) (*bridge.KeyEventResult, bool) {
	prefix, ok := p.Activate(dec)
	if !ok {
		return nil, false
	}
	if d.isManaged(p) {
		d.markEntered(p) // 自带 modeActive 守卫
	}
	return d.c.modeCompositionResult(prefix, len(prefix)), true
}

// dispatchHostChain 驱动当前 d.host（受管宿主 + engine_default 兜底宿主）的按键处理链。
// 遍历链取第一个非 Pass 者执行（I11 短路），Apply 后 reconcileHost 据模式真值源回填 host
// （受管宿主退出→回落 engine_default；engine_default 自身 reconcile 为 no-op）。各宿主的链当前
// 均含恒 Handle 的整模式 handler（engineDefaultKeyHandler/各 xxxKeyHandler/urlKeyHandler），故必中
// 第一个；兜底分支防御链意外全 Pass（不应发生）。全程在 c.mu 内（I7，调用方 HandleKeyEvent 已持锁）。
//
// **组链前先 reconcileHost**：链外路径（输入态 Ctrl/Alt 透传、失焦/IME 停用、CapsLock 通知、
// numpad direct）经 clearState 清模式标志但不经本函数，d.host 可能滞留陈旧受管宿主；若不先回填，
// engine_default 尾部分发会用陈旧 host 组链，把正常输入键误路由回已退出的模式（且引擎层已卸载）。
// reconcileHost 仅在 modeActive(d.host)=false 时降级，模式仍活跃的正常模式内分发不受影响。
func (d *decider) dispatchHostChain(key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	d.reconcileHost()
	ctx := newDecisionCtx(d.c, d.host)
	for _, h := range d.keyHandlerChain() {
		if h.Judge(ctx, key, data).Verdict == VerdictPass {
			continue
		}
		res := h.Apply(d.c, key, data)
		// 慢请求归因：受管宿主模式内键处理（拼音候选已在 updatePinyinModeCandidates 单独标记，
		// 此处兜底 quick/special/url 等未单独标记的模式路径，避免全归 p_teardown）。
		d.c.markKeyPhase("mode_dispatch")
		d.reconcileHost()
		return res
	}
	// 兜底：链意外全 Pass（受管宿主 KeyHandler 恒 Handle，不应发生）——记 WARN，消费按键防泄漏。
	if d.c.logger != nil {
		d.c.logger.Warn("dispatchHostChain: empty chain verdict", "host", d.host.Name())
	}
	d.reconcileHost()
	return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}
}

// armRewind 夺取进入时登记回退。snapshot=夺取前的正常输入 inputBuffer（回退时还原），
// hostText=进入后的 host buffer（用于判定"未编辑"），cleanup=清该 host 模式状态 + 引擎层。
func (d *decider) armRewind(snapshot, hostText string, cleanup func()) {
	d.rewindBuffer = snapshot
	d.rewindHostText = hostText
	d.rewindCleanup = cleanup
}

// clearRewind 作废回退登记（用户确认要用该模式 / 模式退出 / 回退执行后）。
func (d *decider) clearRewind() {
	d.rewindBuffer = ""
	d.rewindHostText = ""
	d.rewindCleanup = nil
}

// rewindArmed 是否已登记回退（夺取进入后、尚未作废）。
func (d *decider) rewindArmed() bool { return d.rewindCleanup != nil }

// canRewind 当前是否可回退：已登记，且当前 host buffer 与登记时一致（即未做任何编辑）。
func (d *decider) canRewind(currentHostText string) bool {
	return d.rewindCleanup != nil && currentHostText == d.rewindHostText
}

// rewindHijack 执行夺取回退：清当前夺取 host 状态（cleanup，含引擎层）→ 还原夺取前的正常
// 输入流 → host 回落 engine_default（decider 开时）。本质是夺取 Release→Activate 的逆。
// 全程在 c.mu 内（调用方 HandleKeyEvent 已持锁，I7）。
func (d *decider) rewindHijack() *bridge.KeyEventResult {
	c := d.c
	pre := d.rewindBuffer
	cleanup := d.rewindCleanup
	d.clearRewind()
	if cleanup != nil {
		cleanup() // 清模式状态 + 引擎层（如 DeactivateTempPinyin）
	}
	// 还原正常输入流
	c.inputBuffer = pre
	c.inputCursorPos = len(pre)
	c.preeditDisplay = ""
	if c.uiManager != nil {
		c.uiManager.SetModeLabel("")
		c.uiManager.SetModeAccentColor(nil)
	}
	c.updateCandidates()
	c.showUI()
	d.reconcileHost() // 模式标志已被 cleanup 清零 → 回落 engine_default
	return c.compositionUpdateResult()
}

// applyEngineDiff 据 needed 与已挂载资源的差集，单点驱动需对称管理的引擎层挂卸（I3）。
// 各挂卸入口（setup/exit/clearState/rewind/quick_input 拼音子上下文）统一改调本方法，传入「此刻
// 需要的资源位集」，由 mounted 去抖：仅在跨边界时真正调引擎，杜绝散落不对称 + 重复挂卸。
//
// CapPinyinLayer：码表引擎下 ActivateTempPinyin 移码表层挂拼音层、DeactivateTempPinyin 逆操作；
// 混输引擎下引擎内部 no-op（见 manager_temp_pinyin.go）。引擎调用本身幂等，mounted 仅作去抖。
// English/拼音引擎/生僻字是只加载不卸载的幂等资源，不在此 diff（保留各自 EnsureXxxLoaded 调用）。
func (d *decider) applyEngineDiff(needed Capability) {
	if d.c.engineMgr == nil {
		d.mounted = needed
		return
	}
	added, removed := computeCapabilityDiff(d.mounted, needed)
	if added&CapPinyinLayer != 0 {
		d.c.engineMgr.ActivateTempPinyin()
	}
	if removed&CapPinyinLayer != 0 {
		d.c.engineMgr.DeactivateTempPinyin()
	}
	d.mounted = needed
}

// logSwitch 在宿主切换边界记 DEBUG 遥测：宿主名 + 容量 diff（应挂载/卸载的引擎资源）+
// CompositionPhase。容量 diff 的真实挂卸已由 applyEngineDiff 在各入口落地（I3）；本方法仅
// 只读观测去抖统计（设计文档第十一节）。遵守日志隐私：仅元数据，不记 buffer/候选文本。
func (d *decider) logSwitch(from, to Processor) {
	if d.c.logger == nil || from == to {
		return
	}
	added, removed := computeCapabilityDiff(from.Capabilities(), to.Capabilities())
	phase := deriveCompositionPhase(from, to, d.engineDefault)
	d.c.logger.Debug("decider host switch",
		"from", from.Name(),
		"to", to.Name(),
		"phase", phase.String(),
		"capAdded", uint64(added),
		"capRemoved", uint64(removed),
	)
}
