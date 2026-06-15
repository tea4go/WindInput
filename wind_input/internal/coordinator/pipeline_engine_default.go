// pipeline_engine_default.go — 兜底宿主（正常码表/拼音输入）。
//
// engine_default 是「host 永不为空」（不变量 I1）的默认宿主：启动即它，上屏/ESC/删空后
// host 回落到它。Judge 含 z 键混合回退裁决（decideEngineDefaultZFallback 纯函数）；
// KeyHandlers 贡献 engineDefaultKeyHandler（正常输入键处理 = handleEngineDefaultKey）。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

// 编译期断言：engineDefaultProcessor 实现 Processor 接口、engineDefaultKeyHandler 实现 KeyHandler。
var (
	_ Processor  = (*engineDefaultProcessor)(nil)
	_ KeyHandler = engineDefaultKeyHandler{}
)

type engineDefaultProcessor struct {
	c *Coordinator
}

func newEngineDefaultProcessor(c *Coordinator) *engineDefaultProcessor {
	return &engineDefaultProcessor{c: c}
}

func (p *engineDefaultProcessor) Name() string { return "engine_default" }

// decideEngineDefaultZFallback 是 z 键混合回退的纯判定（无副作用，可表驱动单测）。
// 与现有 zHybridFallback 门禁一致：buffer 以 z 开头、加新键后无前缀匹配、且为码表引擎时，
// 回退到临时拼音，residual 为去掉首 z 后再追加新键的剩余串。
//
//	buffer:            当前主输入缓冲
//	key:               本次按键（应为单个小写字母）
//	hasPrefixWithKey:  ctx.HasPrefix(buffer+key) 的结果
//	isCodeTable:       ctx.EngineIsCodeTable() 的结果
//	zIsTempPinyinTrigger: z 是否配置为临时拼音触发键/混合模式（isZKeyHybridMode || isTempPinyinZTrigger）。
//	    这道门禁与旧 zHybridFallback 一致——缺它会让 z 未配触发键时也误回退。
func decideEngineDefaultZFallback(buffer, key string, hasPrefixWithKey, isCodeTable, zIsTempPinyinTrigger bool) (residual string, doRelease bool) {
	if len(key) != 1 || key[0] < 'a' || key[0] > 'z' {
		return "", false
	}
	if len(buffer) == 0 || buffer[0] != 'z' {
		return "", false
	}
	if !isCodeTable {
		return "", false
	}
	if !zIsTempPinyinTrigger {
		return "", false
	}
	if hasPrefixWithKey {
		return "", false
	}
	return buffer[1:] + key, true
}

func (p *engineDefaultProcessor) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	// 小写字母：可能触发 z 回退，否则正常码表输入（落按键处理链）。
	if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
		buf := ctx.BufferText()
		// z 触发键门禁：与旧 zHybridFallback 一致——z 必须配置为临时拼音触发/混合模式。
		zTrigger := p.c.isZKeyHybridMode() || p.c.isTempPinyinZTrigger()
		if residual, ok := decideEngineDefaultZFallback(buf, key, ctx.HasPrefix(buf+key), ctx.EngineIsCodeTable(), zTrigger); ok {
			return decRelease(residual)
		}
		return decHandle()
	}
	// 其余键（触发键、标点、导航等）第 0 批一律 Pass，交链/旧路径处理。
	return decPass()
}

// Activate engine_default 作为默认 host，Activate 用于从其他宿主回落（带 residual）。
// 第 0 批占位：不接主路径，真正的 buffer 重建在第 1 批落地。
func (p *engineDefaultProcessor) Activate(dec Decision) (string, bool) {
	return "", true
}

func (p *engineDefaultProcessor) Release() {}

func (p *engineDefaultProcessor) BufferText() string { return p.c.inputBuffer }

// KeyHandlers engine_default 的链 = 单个恒 Handle 的整模式 handler（与 urlKeyHandler 同构）。
// 正常输入的全部按键（导航/光标/编辑/上屏/字母含 z 回退/数字/选词/分隔符/标点）由
// handleEngineDefaultKey 统一处理（原 HandleKeyEvent 末尾 switch 的搬迁）。把翻页/高亮拆给共享
// navKeyHandler 是后续批次——届时本 handler 改为对导航键 Pass 让位。
func (p *engineDefaultProcessor) KeyHandlers() []KeyHandler {
	return []KeyHandler{engineDefaultKeyHandler{c: p.c}}
}

// engineDefaultKeyHandler 把 handleEngineDefaultKey 包装成链上的整模式处理单元（Judge 恒 Handle）。
type engineDefaultKeyHandler struct {
	c *Coordinator
}

func (h engineDefaultKeyHandler) Name() string { return "engine_default.key" }

func (h engineDefaultKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	return decHandle()
}

func (h engineDefaultKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	return c.handleEngineDefaultKey(key, data)
}

func (p *engineDefaultProcessor) Capabilities() Capability { return 0 }

func (p *engineDefaultProcessor) UsesExtendedPerPage() bool { return false }

func (p *engineDefaultProcessor) PreferredLayout() config.CandidateLayout { return "" }

func (p *engineDefaultProcessor) AcceptedProviders() []ProviderID { return nil }
