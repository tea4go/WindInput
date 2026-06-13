// pipeline_quick_input.go — 快捷输入宿主（Processor）。
//
// 第 1 批触发键激活迁移：实现 Processor 接口，注册到 decider.registry（优先级最高）。
// Judge 实现 buffer 空时的触发键激活裁决；Activate 复用 setupQuickInputMode（含 ForceVertical
// 布局处理）。模式内按键仍走旧 handleQuickInputKey（部分接管，不维护 d.host）。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

var (
	_ Processor  = (*quickInputProcessor)(nil)
	_ KeyHandler = quickInputKeyHandler{}
)

type quickInputProcessor struct {
	c *Coordinator
}

func newQuickInputProcessor(c *Coordinator) *quickInputProcessor {
	return &quickInputProcessor{c: c}
}

func (p *quickInputProcessor) Name() string { return "quick_input" }

func (p *quickInputProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if ctx.BufferLen() == 0 && ctx.CandidateCount() == 0 {
		if tk := p.c.matchQuickInputTrigger(key, data.KeyCode); tk != "" {
			return decActivate(tk, -1)
		}
	}
	return decPass()
}

func (p *quickInputProcessor) Activate(dec Decision) (string, bool) {
	return p.c.setupQuickInputMode(dec.TriggerKey)
}

func (p *quickInputProcessor) Release() {}

func (p *quickInputProcessor) BufferText() string { return p.c.quickInputBuffer }

func (p *quickInputProcessor) Capabilities() Capability { return 0 }

// KeyHandlers：quick_input 是自包含模式——成为 host 后所有按键归它（旧 handleQuickInputKey
// 内部已统一处理数字/字母选择/拼音子模式/导航/退出）。用「整模式」薄包装 handler 建立 decide()
// 分发路径，与旧逐条等价；共享导航 handler 抽取留待后续批次。
func (p *quickInputProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{quickInputKeyHandler{c: p.c}}
}

// quickInputKeyHandler 把 handleQuickInputKey 包装成链上的「整模式」处理单元。
// Judge 恒 Handle（host 为 quick_input 时模式内键全部认领，I11 短路于此）；
// Apply 委托回 handleQuickInputKey（含拼音子模式内部分发），行为字节级不变。
type quickInputKeyHandler struct {
	c *Coordinator
}

func (h quickInputKeyHandler) Name() string { return "quick_input.mode" }

func (h quickInputKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decHandle()
}

func (h quickInputKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleQuickInputKey(key, data)
}

func (p *quickInputProcessor) UsesExtendedPerPage() bool { return true }

func (p *quickInputProcessor) PreferredLayout() config.CandidateLayout { return "" }

// AcceptedProviders 返回 nil：快捷输入的候选源（date/calc/number 经
// quickInputBaseProviders；拼音经 pinyinProvider）由 updateQuickInputCandidates 按
// 上下文**硬路由**（拼音 vs 结构化 XOR 互斥），不经白名单驱动 merge。白名单的"拒绝/接纳"
// 语义留待真正多源共存的宿主（url_english/emoji），见 docs/design 第 12 节第 4 批校准。
func (p *quickInputProcessor) AcceptedProviders() []ProviderID { return nil }
