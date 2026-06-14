// pipeline_temp_english.go — 临时英文宿主（Processor）。
//
// 第 1 批触发键激活迁移：实现 Processor 接口，注册到 decider.registry（优先级低于临时拼音）。
// Judge 实现 buffer 空时的触发键激活裁决（仅触发键路径；Shift+字母进临时英文仍走旧逻辑）。
// Activate 复用 setupTempEnglishMode。模式内按键仍走旧 handleTempEnglishKey（部分接管）。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

var (
	_ Processor  = (*tempEnglishProcessor)(nil)
	_ KeyHandler = tempEnglishKeyHandler{}
)

type tempEnglishProcessor struct {
	c *Coordinator
}

func newTempEnglishProcessor(c *Coordinator) *tempEnglishProcessor {
	return &tempEnglishProcessor{c: c}
}

func (p *tempEnglishProcessor) Name() string { return "temp_english" }

func (p *tempEnglishProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if ctx.BufferLen() == 0 && ctx.CandidateCount() == 0 {
		if tk := p.c.matchTempEnglishTrigger(key, data.KeyCode); tk != "" {
			return decActivate(tk, -1)
		}
	}
	return decPass()
}

func (p *tempEnglishProcessor) Activate(dec Decision) (string, bool) {
	return p.c.setupTempEnglishMode(dec.TriggerKey)
}

func (p *tempEnglishProcessor) Release() {}

func (p *tempEnglishProcessor) BufferText() string { return p.c.tempEnglishBuffer }

// Capabilities：开启英文候选时需英文词库（与 setupTempEnglishMode 的 EnsureEnglishLoaded 一致）。
func (p *tempEnglishProcessor) Capabilities() Capability {
	if p.c.config != nil && p.c.config.Input.ShiftTempEnglish.ShowEnglishCandidates {
		return CapEnglishDict
	}
	return 0
}

// KeyHandlers：temp_english 的链 = 模式特有 handler + 专用导航 handler（KeyHandler 链分解）。
// temp_english 的导航语义与其它模式**不同**（翻页 expandBefore=true、高亮 expand 在移动前 +
// showUI 无条件刷新），故用专用 `tempEnglishNavKeyHandler` 包装其精确逻辑，而非复用通用
// navKeyHandler——行为字节级不变。
func (p *tempEnglishProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{
		tempEnglishKeyHandler{c: p.c},
		tempEnglishNavKeyHandler{c: p.c},
	}
}

// tempEnglishKeyHandler 把 handleTempEnglishKey 包装成链上的模式特有处理单元。
// Judge 对导航键 Pass（让位链上居后的 tempEnglishNavKeyHandler），其余键 Handle（I11 短路于此）。
// 例外：allow_symbols 开启时符号字符优先入 buffer（旧 switch 中 allowSymbols 符号 case 在翻页
// case 之前），故对其 Handle 不让位——保持旧 switch 的判定顺序。
type tempEnglishKeyHandler struct {
	c *Coordinator
}

func (h tempEnglishKeyHandler) Name() string { return "temp_english.mode" }

func (h tempEnglishKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	c := h.c
	if c.tempEnglishAllowSymbols() && len(key) == 1 && isTempEnglishSymbolChar(key[0]) {
		return decHandle()
	}
	if c.isStandardNavKey(key, data) {
		return decPass()
	}
	return decHandle()
}

func (h tempEnglishKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleTempEnglishKey(key, data)
}

// tempEnglishNavKeyHandler temp_english 的专用导航 handler。导航键谓词与标准一致
// （isStandardNavKey：通用翻页 + 高亮），但 Apply 调用 temp_english 特有的导航实现：
// 翻页 navPageDown 用 expandBefore=true，高亮走 tempEnglishHighlightUp/Down（保留其特有时序）。
// 与旧 handleTempEnglishKey 的导航 case 逐字节等价。allow_symbols 符号优先已由 mode handler
// 的 Judge 守卫（符号 Handle、不让位），故本 handler 只在真正的导航键上被触达。
type tempEnglishNavKeyHandler struct {
	c *Coordinator
}

func (h tempEnglishNavKeyHandler) Name() string { return "temp_english.nav" }

func (h tempEnglishNavKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if h.c.isStandardNavKey(key, data) {
		return decHandle()
	}
	return decPass()
}

func (h tempEnglishNavKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	vk := uint32(data.KeyCode)
	mods := uint32(data.Modifiers)
	switch {
	case c.isPageUpKey(key, data.KeyCode, mods):
		return c.navPageUp(c.showTempEnglishUI)
	case c.isPageDownKey(key, data.KeyCode, mods):
		return c.navPageDown(c.showTempEnglishUI, c.expandTempEnglishCandidates, true)
	case c.isHighlightUpKey(vk, mods):
		return c.tempEnglishHighlightUp()
	case c.isHighlightDownKey(vk, mods):
		return c.tempEnglishHighlightDown()
	}
	// Judge 已保证命中其一；防御性消费，不应到达。
	return navConsumed()
}

func (p *tempEnglishProcessor) UsesExtendedPerPage() bool { return false }

func (p *tempEnglishProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *tempEnglishProcessor) AcceptedProviders() []ProviderID { return nil }
