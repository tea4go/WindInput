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

// KeyHandlers：quick_input 的链 = 模式特有 handler + 共享导航 handler（KeyHandler 链分解）。
// quick_input 有**双上下文**，导航键谓词与 showUI 各异，故按当前上下文构造对应的 navKeyHandler：
//   - 拼音上下文（quickInputPinyinActive）：标准翻页谓词 + showPinyinModeUI（与 temp_pinyin 同款）。
//   - 基础上下文（date/calc/number/重复）：专用翻页谓词 isQuickInputPageUpKey（排除 -/= 等输入字符）
//   - showQuickInputUI、无分级加载。
//
// KeyHandlers 每键调用一次，故据当前（按键前）上下文取对应 nav handler。
func (p *quickInputProcessor) KeyHandlers() []KeyHandler {
	c := p.c
	var nav navKeyHandler
	if c.quickInputPinyinActive() {
		ops := c.quickInputPinyinOps()
		nav = navKeyHandler{
			c:              c,
			name:           "quick_input.pinyin.nav",
			pageUp:         c.stdPageUp,
			pageDown:       c.stdPageDown,
			showUI:         func() { c.showPinyinModeUI(ops) },
			pageDownExpand: nil,
			hiDownExpand:   c.expandCandidates,
		}
	} else {
		nav = navKeyHandler{
			c:              c,
			name:           "quick_input.base.nav",
			pageUp:         c.quickPageUp,
			pageDown:       c.quickPageDown,
			showUI:         c.showQuickInputUI,
			pageDownExpand: nil, // 基础上下文翻页/高亮均无分级加载（与旧 navXxx(show, nil) 一致）
			hiDownExpand:   nil,
		}
	}
	return []KeyHandler{quickInputKeyHandler{c: c}, nav}
}

// quickInputKeyHandler 把 handleQuickInputKey 包装成链上的模式特有处理单元。
// Judge 对当前上下文的导航键 Pass（让位链上居后的 navKeyHandler），其余键 Handle（I11 短路于此）；
// Apply 委托回 handleQuickInputKey（含拼音子模式内部分发）——其导航 case 已被链上 nav handler
// 在 Apply 前认领，对 quick_input 不再被触达（仍供 decider 关闭时的旧路径复用）。
type quickInputKeyHandler struct {
	c *Coordinator
}

func (h quickInputKeyHandler) Name() string { return "quick_input.mode" }

func (h quickInputKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	c := h.c
	// 让位谓词必须与 KeyHandlers 构造的 navKeyHandler 认领谓词按上下文一一对应（同经 navKeyMatch）。
	if c.quickInputPinyinActive() {
		if c.isStandardNavKey(key, data) {
			return decPass()
		}
	} else if c.isQuickInputBaseNavKey(key, data) {
		return decPass()
	}
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
