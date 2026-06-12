// pipeline_temp_pinyin.go — 临时拼音宿主（Processor）。
//
// 第 1 批 1a：实现 Processor 接口，注册到 decider.registry。当前**不接管**主路径（影子仍只读、
// host 仍不切换），故 Activate/Release 尚不会被实际调用；Judge 的「激活」裁决可被单测验证。
// 真正接管（CompHot 热切换、residual 注入、引擎层 diff）在第 1 批 1c 落地。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

// 编译期断言：tempPinyinProcessor 实现 Processor 接口。
var _ Processor = (*tempPinyinProcessor)(nil)

type tempPinyinProcessor struct {
	c *Coordinator
}

func newTempPinyinProcessor(c *Coordinator) *tempPinyinProcessor {
	return &tempPinyinProcessor{c: c}
}

func (p *tempPinyinProcessor) Name() string { return "temp_pinyin" }

// Judge：作为 registry 候选时的「激活」裁决——buffer 空、无候选时匹配临时拼音触发键 → Activate。
// 复用 matchTempPinyinTrigger（含引擎类型/开关门禁，engineMgr==nil 时安全返回空 → Pass）。
// 作为当前 host 时的「退出」裁决在 1c 接管时补充（1a host 不会是 temp_pinyin）。
func (p *tempPinyinProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if ctx.BufferLen() == 0 && ctx.CandidateCount() == 0 {
		if tk := p.c.matchTempPinyinTrigger(key, data.KeyCode); tk != "" {
			return decActivate(tk, -1)
		}
	}
	return decPass()
}

// Activate：复用现有 setupTempPinyinMode（含拼音词库层激活）。
// residual（z fallback 拼音码）在 1c 接管时注入；1a 暂忽略。
// 1c 起引擎副作用改由决策器 applyEngineDiff 统一管（I3），届时 setup 内的层激活调用上移。
func (p *tempPinyinProcessor) Activate(triggerKey, residual string) (string, bool) {
	return p.c.setupTempPinyinMode(triggerKey)
}

// Release：1c 接管时由决策器在 CompEnd/CompHot 调用；引擎副作用走 applyEngineDiff（I3），
// 此处仅清模式状态。1a 不接管，不会被调用。
func (p *tempPinyinProcessor) Release() {
	// 1c 落地：清 tempPinyin* 状态（不调 DeactivateTempPinyin，交决策器 diff）。
}

// BufferText 临时拼音的活跃 buffer。
func (p *tempPinyinProcessor) BufferText() string { return p.c.tempPinyinBuffer }

func (p *tempPinyinProcessor) Capabilities() Capability { return CapPinyinLayer }

func (p *tempPinyinProcessor) KeyHandlers() []KeyHandler { return nil }

func (p *tempPinyinProcessor) UsesExtendedPerPage() bool { return true }

func (p *tempPinyinProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *tempPinyinProcessor) AcceptedProviders() []ProviderID { return nil }
