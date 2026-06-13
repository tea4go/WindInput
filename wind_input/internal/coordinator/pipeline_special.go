// pipeline_special.go — 引导键特殊模式宿主（Processor）。
//
// special 运行时是**单例**（一个 c.specialMode 标志 + 一个 handleSpecialModeKey），尽管配置上
// 有 N 个触发实例（specialModeReg.instances）。故用单个 specialProcessor 承载，内部键全接管。
//
// **触发仍走旧路径**（不入 decider.registry）：special 触发是动态 2 步匹配——
// specialModeReg.match(key)→id、再 matchSpecialTrigger(id)→tk、setupSpecialMode(id, tk)——
// 其中 id 无法塞进现有 Processor.Activate(triggerKey, residual) 签名（需扩 Decision/接口，
// 留后续批次）。本宿主只接管 host 状态机 + 模式内键，故 Judge/Activate 为占位实现。
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

// Judge：special 不入 registry，激活裁决不经决策器（触发走旧 2 步匹配）。占位 Pass——
// 作为 host 时 shadowLog 会调它，返回 Pass 无副作用。
func (p *specialProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decPass()
}

// Activate：占位，不被调用（special 不在 registry，进入经旧 setupSpecialMode 直接调）。
func (p *specialProcessor) Activate(triggerKey, residual string) (string, bool) {
	return "", false
}

func (p *specialProcessor) Release() {}

func (p *specialProcessor) BufferText() string { return p.c.specialBuffer }

// Capabilities：special 码表是独立表（非共享拼音层），无需对称挂卸的引擎资源；ForceVertical
// 布局切换由 setup/exit 既有路径管。故返回 0（不参与 applyEngineDiff/容量 diff）。
func (p *specialProcessor) Capabilities() Capability { return 0 }

// KeyHandlers：special 自包含模式——成为 host 后所有按键归它。整模式薄包装 handler，
// Apply=handleSpecialModeKey，与旧逐条等价。
func (p *specialProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{specialKeyHandler{c: p.c}}
}

func (p *specialProcessor) UsesExtendedPerPage() bool { return true }

func (p *specialProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *specialProcessor) AcceptedProviders() []ProviderID { return nil }

// specialKeyHandler 把 handleSpecialModeKey 包装成链上的「整模式」处理单元。
// Judge 恒 Handle（host 为 special 时模式内键全部认领，I11 短路于此）；Apply 委托回
// handleSpecialModeKey，行为字节级不变。
type specialKeyHandler struct {
	c *Coordinator
}

func (h specialKeyHandler) Name() string { return "special.mode" }

func (h specialKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decHandle()
}

func (h specialKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleSpecialModeKey(key, data)
}
