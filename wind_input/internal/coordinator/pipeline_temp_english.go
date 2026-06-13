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

func (p *tempEnglishProcessor) Activate(triggerKey, residual string) (string, bool) {
	return p.c.setupTempEnglishMode(triggerKey)
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

// KeyHandlers：temp_english 是自包含模式——成为 host 后所有按键归它（旧 handleTempEnglishKey
// 内部已统一处理字母/退出/上屏）。用「整模式」薄包装 handler 建立 decide() 分发路径，与旧逐条
// 等价；共享导航 handler 抽取留待后续批次。
func (p *tempEnglishProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{tempEnglishKeyHandler{c: p.c}}
}

// tempEnglishKeyHandler 把 handleTempEnglishKey 包装成链上的「整模式」处理单元。
// Judge 恒 Handle（host 为 temp_english 时模式内键全部认领，I11 短路于此）；
// Apply 委托回 handleTempEnglishKey，行为字节级不变。
type tempEnglishKeyHandler struct {
	c *Coordinator
}

func (h tempEnglishKeyHandler) Name() string { return "temp_english.mode" }

func (h tempEnglishKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decHandle()
}

func (h tempEnglishKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleTempEnglishKey(key, data)
}

func (p *tempEnglishProcessor) UsesExtendedPerPage() bool { return false }

func (p *tempEnglishProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *tempEnglishProcessor) AcceptedProviders() []ProviderID { return nil }
