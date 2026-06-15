// handle_quick_input_pinyin.go — 快捷输入模式下的临时拼音子模式
// 在快捷输入模式中，字母键（a-z）在缓冲区为空时进入临时拼音子模式，
// 补足混输模式下没有独立临时拼音入口的能力。
// 按键处理、候选更新、UI 显示等核心逻辑委托给 pinyin_mode_shared.go 中的共享实现。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/candidate"
)

// quickInputPinyinActive 判定快捷输入当前是否处于「拼音上下文」。
//
// 取代旧 quickInputPinyinMode 布尔：拼音上下文由 buffer 内容派生——结构化候选
// （date/calc/number）靠数字/运算符进入、buffer 永不以字母打头；拼音靠字母进入、
// buffer 永远以字母打头（分段上屏后 buffer[ConsumedLength:] 仍以字母打头，分隔符
// "xi'an" 亦然）。故 buffer 首字符是 a-z 即拼音上下文，二者从不共存。
//
// 仅判小写 a-z：拼音 buffer 首字母由调用方（engageQuickInputPinyin 传入 string(lower)、
// handlePinyinModeKey 插入前小写化）保证已小写，大写首字节不视为拼音上下文。
func (c *Coordinator) quickInputPinyinActive() bool {
	return c.quickInputMode && len(c.quickInputBuffer) > 0 &&
		c.quickInputBuffer[0] >= 'a' && c.quickInputBuffer[0] <= 'z'
}

// quickInputAlphaPinyinEnabled 快捷模式字母上下文是否启用拼音源（默认 true）。
// 预留给 F7「拼音可关」场景：Pinyin=false 时字母输入不走 engageQuickInputPinyin 拼音路径，
// 改由通用驱动 + 纯 extras 融合。F3 默认拼音常开，暂未接入调用。
func (c *Coordinator) quickInputAlphaPinyinEnabled() bool {
	return c.config == nil || c.config.Features.QuickInput.AlphaProviders.Pinyin
}

// quickInputRareCharEnabled 生僻字源是否启用（需开关 + 有效实例 id + registry 就绪）。
func (c *Coordinator) quickInputRareCharEnabled() bool {
	return c.config != nil &&
		c.config.Features.QuickInput.AlphaProviders.RareChar &&
		c.config.Features.QuickInput.AlphaProviders.RareCharID != "" &&
		c.specialModeReg != nil
}

// quickInputEnglishEnabled 英文源是否启用。
func (c *Coordinator) quickInputEnglishEnabled() bool {
	return c.config != nil && c.config.Features.QuickInput.AlphaProviders.English
}

// quickInputAlphaExtraProviders 返回字母上下文中「拼音以外」的启用融合源（生僻字/英文）。
// 拼音走有状态主路径(updatePinyinModeCandidates)，故不在此列；二者经 extraCandidates 钩子拼接。
func (c *Coordinator) quickInputAlphaExtraProviders() []CandidateProvider {
	var ps []CandidateProvider
	if c.quickInputRareCharEnabled() {
		ps = append(ps, rareCharProvider{c: c, id: c.config.Features.QuickInput.AlphaProviders.RareCharID})
	}
	if c.quickInputEnglishEnabled() {
		ps = append(ps, englishProvider{c: c})
	}
	return ps
}

// ensureQuickInputAlphaSources 进入字母上下文时 eager 加载启用的非拼音融合源词库
// （英文词库 / 生僻字码表）。幂等；加载失败只记 WARN 不阻断输入（该源候选届时为空）。
// 拼音层挂卸已由决策器 applyEngineDiff 单点管（I3）；这里只管「启用即加载」的幂等只读资源。
func (c *Coordinator) ensureQuickInputAlphaSources() {
	if c.engineMgr == nil {
		return
	}
	if c.quickInputEnglishEnabled() {
		if err := c.engineMgr.EnsureEnglishLoaded(); err != nil {
			c.logger.Warn("Failed to load english dict for quick input fusion", "error", err)
		}
	}
	if c.quickInputRareCharEnabled() {
		id := c.config.Features.QuickInput.AlphaProviders.RareCharID
		inst := c.specialModeReg.get(id)
		if inst == nil {
			// 配了 rare_char_id 但 registry 无此实例：诊断提示，否则用户开了源却一直无候选
			c.logger.Warn("quick input rare-char id not found in registry", "id", id)
		} else if _, err := c.specialModeReg.ensureLoaded(inst); err != nil {
			c.logger.Warn("Failed to load rare-char table for quick input fusion", "id", id, "error", err)
		}
	}
}

// engageQuickInputPinyin 在快捷输入空 buffer 下首次输入字母时切入拼音上下文。
// 不再有独立的子模式布尔/缓冲——状态进统一的 quickInputBuffer，上下文由
// quickInputPinyinActive() 派生。
func (c *Coordinator) engageQuickInputPinyin(firstKey string) *bridge.KeyEventResult {
	// 加载拼音引擎（所有引擎类型都需要），再按引擎类型决定是否交换词库层
	if c.engineMgr != nil {
		if err := c.engineMgr.EnsurePinyinLoaded(); err != nil {
			c.logger.Warn("Failed to load pinyin engine for quick input pinyin", "error", err)
			return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}
		}
	}
	if c.decider != nil {
		c.decider.applyEngineDiff(CapPinyinLayer) // 拼音子上下文挂载拼音层（I3 单点 diff）
	}
	c.ensureQuickInputAlphaSources()

	c.quickInputBuffer = firstKey
	c.quickInputPinyinCursorPos = len(firstKey)
	c.quickInputPinyinCommitted = ""
	c.currentPage = 1
	c.selectedIndex = 0

	c.logger.Debug("Engaged quick input pinyin context", "firstKeyLen", len(firstKey))

	ops := c.quickInputPinyinOps()
	c.updatePinyinModeCandidates(ops)
	c.showPinyinModeUI(ops)

	return c.pinyinModeCompositionResult(ops)
}

// handleQuickInputPinyinKey 处理快捷输入临时拼音子模式下的按键（委托给共享处理器）
func (c *Coordinator) handleQuickInputPinyinKey(key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handlePinyinModeKey(c.quickInputPinyinOps(), key, data)
}

// exitQuickInputPinyinToBase 退出拼音上下文，buffer 清空回到快捷输入基础（空 buffer）状态。
// 由 ops.exitOnBackspaceEmpty 在退格删空 buffer 时调用。
func (c *Coordinator) exitQuickInputPinyinToBase() *bridge.KeyEventResult {
	if c.decider != nil {
		c.decider.applyEngineDiff(0) // 退回基础上下文，卸载拼音层（I3 单点 diff）
	}

	c.quickInputBuffer = ""
	c.quickInputPinyinCommitted = ""
	c.quickInputPinyinCursorPos = 0
	c.preeditDisplay = ""
	c.currentPage = 1
	c.selectedIndex = 0

	c.logger.Debug("Exited quick input pinyin to base mode")

	// 返回快捷输入基础状态
	c.updateQuickInputCandidates()
	c.showQuickInputUI()

	preedit := c.quickInputPrefix()
	return c.modeCompositionResult(preedit, len(preedit))
}

// exitQuickInputPinyinMode 从拼音上下文整体退出快捷输入（上屏 text）。由 ops.exitMode 调用。
func (c *Coordinator) exitQuickInputPinyinMode(commit bool, text string) *bridge.KeyEventResult {
	if c.decider != nil {
		c.decider.applyEngineDiff(0) // 整体退出，卸载拼音层（I3 单点 diff）
	}
	c.preeditDisplay = ""

	// 输入历史在候选最终化点（selectPinyinModeXxx）统一记录, 此处不再记录,
	// 以避免把拼音码、触发键、标点等非候选文本误记
	c.quickInputPinyinCommitted = ""
	c.quickInputPinyinCursorPos = 0

	return c.exitQuickInputMode(commit, text)
}

// quickInputPinyinOps 创建快捷输入拼音上下文的操作回调（buffer 即统一的 quickInputBuffer）
func (c *Coordinator) quickInputPinyinOps() *pinyinModeOps {
	return &pinyinModeOps{
		buffer:    &c.quickInputBuffer,
		cursorPos: &c.quickInputPinyinCursorPos,
		committed: &c.quickInputPinyinCommitted,
		prefix:    c.quickInputPrefix,
		exitMode: func(commit bool, text string) *bridge.KeyEventResult {
			return c.exitQuickInputPinyinMode(commit, text)
		},
		exitOnBackspaceEmpty: func() *bridge.KeyEventResult {
			return c.exitQuickInputPinyinToBase()
		},
		separator: func(key string, keyCode int) bool {
			return c.isPinyinSeparatorForBuffer(c.quickInputBuffer, key, keyCode)
		},
		triggerKey: func(key string, keyCode int) bool {
			return c.isQuickInputTriggerKey(key, keyCode)
		},
		consumeSpaceEmpty: true,
		// 融合：拼音候选后追加生僻字/英文等启用源（默认全关 → 返回空 → 行为不变）
		extraCandidates: func() []candidate.Candidate {
			return mergeProviderCandidates(c.quickInputBuffer, c.quickInputAlphaExtraProviders())
		},
	}
}
