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

// 编译期断言：tempPinyinProcessor 实现 Processor 接口、tempPinyinKeyHandler 实现 KeyHandler。
var (
	_ Processor  = (*tempPinyinProcessor)(nil)
	_ KeyHandler = tempPinyinKeyHandler{}
)

type tempPinyinProcessor struct {
	c *Coordinator
}

func newTempPinyinProcessor(c *Coordinator) *tempPinyinProcessor {
	return &tempPinyinProcessor{c: c}
}

func (p *tempPinyinProcessor) Name() string { return "temp_pinyin" }

// Judge：作为 registry 候选时的「激活」裁决——buffer 空、无候选时匹配临时拼音触发键 → Activate。
// 复用 matchTempPinyinTrigger（含引擎类型/开关门禁，engineMgr==nil 时安全返回空 → Pass）。
//
// z 首次触发由 judgeZFirstTrigger 单独裁决（matchTempPinyinTrigger 故意排除 z，因 z 还需
// 渐进决策：重复上屏历史 / z 码表前缀）。收编进决策器后，z 与标点触发键一样经 registry
// 激活，不再走 handle_key_event.go 的 getTempPinyinTriggerKey 旁路（decider_enabled 下）。
//
// 作为当前 host 时的「退出」裁决在 1c 接管时补充（1a host 不会是 temp_pinyin）。
func (p *tempPinyinProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if ctx.BufferLen() == 0 && ctx.CandidateCount() == 0 {
		if tk := p.c.matchTempPinyinTrigger(key, data.KeyCode); tk != "" {
			return decActivate(tk, -1)
		}
		if p.c.judgeZFirstTrigger(key, data.KeyCode) {
			return decActivate("z", -1)
		}
	}
	return decPass()
}

// Activate：复用现有 setupTempPinyinMode（含拼音词库层激活）。
// residual（z fallback 拼音码）在 1c 接管时注入；1a 暂忽略。
// 1c 起引擎副作用改由决策器 applyEngineDiff 统一管（I3），届时 setup 内的层激活调用上移。
func (p *tempPinyinProcessor) Activate(dec Decision) (string, bool) {
	return p.c.setupTempPinyinMode(dec.TriggerKey)
}

// Release：1c 接管时由决策器在 CompEnd/CompHot 调用；引擎副作用走 applyEngineDiff（I3），
// 此处仅清模式状态。1a 不接管，不会被调用。
func (p *tempPinyinProcessor) Release() {
	// 1c 落地：清 tempPinyin* 状态（不调 DeactivateTempPinyin，交决策器 diff）。
}

// BufferText 临时拼音的活跃 buffer。
func (p *tempPinyinProcessor) BufferText() string { return p.c.tempPinyinBuffer }

func (p *tempPinyinProcessor) Capabilities() Capability { return CapPinyinLayer }

// KeyHandlers：temp_pinyin 的链 = 模式特有 handler + 共享导航 handler（KeyHandler 链分解试点）。
// 导航键（翻页/高亮）由 pinyinNavKeyHandler 认领（链上居后）；其余模式特有键由
// tempPinyinKeyHandler 认领。pinyinNavKeyHandler 用 tempPinyinOps 提供候选窗刷新回调，
// 与旧 handlePinyinModeKey 的导航 case 逐字节等价。其余三模式的同类分解后续批次推进。
func (p *tempPinyinProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{
		tempPinyinKeyHandler{c: p.c},
		pinyinNavKeyHandler{c: p.c, ops: p.c.tempPinyinOps()},
	}
}

// tempPinyinKeyHandler 把 handleTempPinyinKey 包装成链上的模式特有处理单元。
// Judge 对导航键 Pass（让位 pinyinNavKeyHandler），其余键 Handle（I11 短路于此）；
// Apply 委托回 handleTempPinyinKey——其 switch 仍含导航 case，但导航键已被链上 nav handler
// 在 Apply 前认领，故那些 case 对 temp_pinyin 不再被触达（仍供 quick_input 整模式复用）。
type tempPinyinKeyHandler struct {
	c *Coordinator
}

func (h tempPinyinKeyHandler) Name() string { return "temp_pinyin.mode" }

func (h tempPinyinKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if h.c.isPinyinModeNavKey(key, data) {
		return decPass()
	}
	return decHandle()
}

func (h tempPinyinKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleTempPinyinKey(key, data)
}

func (p *tempPinyinProcessor) UsesExtendedPerPage() bool { return true }

func (p *tempPinyinProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *tempPinyinProcessor) AcceptedProviders() []ProviderID { return nil }
