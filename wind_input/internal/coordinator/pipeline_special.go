// pipeline_special.go — 引导键特殊模式宿主（Processor）。
//
// special 运行时是**单例**（一个 c.specialMode 标志 + 一个 handleSpecialModeKey），尽管配置上
// 有 N 个触发实例（specialModeReg.instances）。故用单个 specialProcessor 承载，模式内键 + 触发
// + host 状态机全接管。
//
// 触发的动态 2 步匹配（specialModeReg.match→id、matchSpecialTrigger→tk）在 Judge 内完成，
// 命中实例 id 经 Decision.ActivateID 交给 Activate=setupSpecialMode。由 decider.tryActivateSpecial
// 在旧 special 触发位置（getXxxTriggerKey 之后）调用，保持 special-last 优先级——故**不混入
// tryActivateFromEmpty 的 registry**（那会把 special 提到 z 首触发之前，改变优先级）。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

var (
	_ Processor  = (*specialProcessor)(nil)
	_ KeyHandler = specialKeyHandler{}
)

type specialProcessor struct {
	c *Coordinator
}

func newSpecialProcessor(c *Coordinator) *specialProcessor {
	return &specialProcessor{c: c}
}

func (p *specialProcessor) Name() string { return "special" }

// Judge：buffer 空 + 无候选时做 2 步动态匹配（specialModeReg.match→id + matchSpecialTrigger→tk），
// 命中则 Activate 带实例 id。供 decider.tryActivateSpecial 在旧 special 触发位置（getXxxTriggerKey
// 之后，保持 special-last 优先级）接管。specialModeReg==nil 安全返回 Pass。
func (p *specialProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if ctx.BufferLen() != 0 || ctx.CandidateCount() != 0 {
		return decPass()
	}
	if p.c.specialModeReg == nil {
		return decPass()
	}
	id := p.c.specialModeReg.match(key, data.KeyCode)
	if id == "" {
		return decPass()
	}
	if tk := p.c.matchSpecialTrigger(id, key, data.KeyCode); tk != "" {
		return decActivateID(tk, id, -1)
	}
	return decPass()
}

// Activate：复用 setupSpecialMode（ActivateID = 命中的码表实例 id）。
func (p *specialProcessor) Activate(dec Decision) (string, bool) {
	return p.c.setupSpecialMode(dec.ActivateID, dec.TriggerKey)
}

func (p *specialProcessor) Release() {}

func (p *specialProcessor) BufferText() string { return p.c.specialBuffer }

// Capabilities：special 码表是独立表（非共享拼音层），无需对称挂卸的引擎资源；ForceVertical
// 布局切换由 setup/exit 既有路径管。故返回 0（不参与 applyEngineDiff/容量 diff）。
func (p *specialProcessor) Capabilities() Capability { return 0 }

// KeyHandlers：special 的链 = 模式特有 handler + 共享导航 handler（KeyHandler 链分解）。
// 导航键（翻页/高亮，与 pinyin 同谓词 isStandardNavKey）由复用的 navKeyHandler 认领，showUI/
// expand 走 special 的 showSpecialUI/expandSpecialCandidates；其余模式特有键由 specialKeyHandler
// 认领。与旧 handleSpecialModeKey 的导航 case 逐字节等价。
func (p *specialProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{
		specialKeyHandler{c: p.c},
		navKeyHandler{
			c:              p.c,
			name:           "special.nav",
			showUI:         p.c.showSpecialUI,
			pageDownExpand: p.c.expandSpecialCandidates, // special 翻页后于接近末页分级加载
			hiDownExpand:   p.c.expandSpecialCandidates,
		},
	}
}

func (p *specialProcessor) UsesExtendedPerPage() bool { return true }

func (p *specialProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *specialProcessor) AcceptedProviders() []ProviderID { return nil }

// specialKeyHandler 把 handleSpecialModeKey 包装成链上的模式特有处理单元。
// Judge 对导航键 Pass（让位链上居后的 navKeyHandler），其余键 Handle（I11 短路于此）；
// Apply 委托回 handleSpecialModeKey——其 switch 仍含导航 case，但导航键已被链上 nav handler
// 在 Apply 前认领，故那些 case 对 special 不再被触达（仍供 decider 关闭时的旧路径复用）。
type specialKeyHandler struct {
	c *Coordinator
}

func (h specialKeyHandler) Name() string { return "special.mode" }

func (h specialKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if h.c.isStandardNavKey(key, data) {
		return decPass()
	}
	return decHandle()
}

func (h specialKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleSpecialModeKey(key, data)
}
