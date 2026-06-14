// pipeline_url.go — URL 临时输入宿主（Processor）。
//
// URL 的触发不是「buffer 空时的触发键」，而是「正常输入下 inputBuffer 恰好完成某前缀」
// （悲观全匹配，Release→Activate），由 handle_key_event 的 urlActivationResidual 钩子在
// 正常输入路径夺取（调 enterUrlMode）。故 url **不入 decider.registry**（registry 是触发键
// 激活类）；但它是受管宿主——模式内键经 dispatchManagedHost 走链、host 由决策器维护。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

var (
	_ Processor  = (*urlProcessor)(nil)
	_ KeyHandler = urlKeyHandler{}
)

type urlProcessor struct {
	c *Coordinator
}

func newUrlProcessor(c *Coordinator) *urlProcessor { return &urlProcessor{c: c} }

func (p *urlProcessor) Name() string { return "url" }

// Judge：url 不经 registry/触发键激活（其激活在正常输入路径的 urlActivationResidual 钩子），
// 故作为裁决候选恒 Pass。
func (p *urlProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decPass()
}

// Activate：url 经专用入口 enterUrlMode 进入（带 residual 前缀），此占位不被调用。
func (p *urlProcessor) Activate(dec Decision) (string, bool) { return "", false }

func (p *urlProcessor) Release() {}

func (p *urlProcessor) BufferText() string { return p.c.urlBuffer }

func (p *urlProcessor) Capabilities() Capability { return 0 }

// KeyHandlers：url 自包含模式——成为 host 后所有按键归它（handleUrlKey 内部统一处理输入/上屏/退出）。
func (p *urlProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{urlKeyHandler{c: p.c}}
}

func (p *urlProcessor) UsesExtendedPerPage() bool { return false }

func (p *urlProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *urlProcessor) AcceptedProviders() []ProviderID { return nil }

// urlKeyHandler 把 handleUrlKey 包装成链上的整模式处理单元（url 无候选导航，无需分解）。
type urlKeyHandler struct {
	c *Coordinator
}

func (h urlKeyHandler) Name() string { return "url.mode" }

func (h urlKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decHandle()
}

func (h urlKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleUrlKey(key, data)
}
