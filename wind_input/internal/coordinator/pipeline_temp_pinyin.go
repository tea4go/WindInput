// pipeline_temp_pinyin.go — 临时拼音宿主（Processor）。
//
// 实现 Processor 接口，注册到 decider.registry（生产路径）：Judge 裁决触发键激活（标点触发键 +
// z 首次触发 judgeZFirstTrigger），Activate 复用 setupTempPinyinMode，KeyHandlers 贡献模式特有键
// + 共享导航 handler。引擎层（拼音词库层）挂卸由决策器 applyEngineDiff 单点管（I3）。
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
// 激活（getTempPinyinTriggerKey 的 z 渐进仲裁仍被 judgeZFirstTrigger 复用，但已无独立旁路）。
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
	ops := p.c.tempPinyinOps()
	return []KeyHandler{
		tempPinyinKeyHandler{c: p.c},
		navKeyHandler{
			c:              p.c,
			name:           "temp_pinyin.nav",
			pageUp:         p.c.stdPageUp,
			pageDown:       p.c.stdPageDown,
			showUI:         func() { p.c.showPinyinModeUI(ops) },
			pageDownExpand: nil, // 拼音翻页不分级加载（与旧 navPageDown(show, nil, false) 一致）
			hiDownExpand:   p.c.expandCandidates,
		},
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
	if h.c.isStandardNavKey(key, data) {
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
