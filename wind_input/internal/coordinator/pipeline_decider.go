// pipeline_decider.go — 统一决策器骨架（第 0 批）。
//
// 第 0 批只搭骨架，不接入 HandleKeyEvent 主路径。decide() 实现求值算法的结构，但宿主迁移
// （executeActivate / applyEngineDiff / CompositionPhase 推导）与共享导航 handler 留待第 1 批。
// 当前 decide() 在无 handler 认领时返回 (nil, false)，表示「未接管，交旧路径」。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

// decider 是统一决策器。host 永不为空（I1）：启动即 engine_default。
type decider struct {
	c    *Coordinator
	host Processor

	registry  []Processor  // 触发激活类宿主，按优先级（高→低）
	sharedNav []KeyHandler // 共享导航 handler（翻页/选候选/导航/删空）
	global    []KeyHandler // 全局分流 handler（预留，第 4 批按需填充）
}

func newDecider(c *Coordinator) *decider {
	d := &decider{c: c}
	d.host = newEngineDefaultProcessor(c) // host 永不为空
	// 触发激活类宿主，按优先级（高→低）。后续批次补 quick_input / special / temp_english。
	d.registry = []Processor{
		newTempPinyinProcessor(c),
	}
	// sharedNav / global 在 1c 起填充。
	return d
}

// keyHandlerChain 按当前 host 动态组装按键处理链：全局分流 + 宿主特有 + 共享导航。
func (d *decider) keyHandlerChain() []KeyHandler {
	chain := make([]KeyHandler, 0, len(d.global)+len(d.sharedNav)+4)
	chain = append(chain, d.global...)
	chain = append(chain, d.host.KeyHandlers()...)
	chain = append(chain, d.sharedNav...)
	return chain
}

// decide 求值算法骨架（全程应在 c.mu 内调用，I7）。
// 返回 (result, handled)：handled=false 表示本决策器未接管，调用方继续旧路径。
func (d *decider) decide(key string, data *bridge.KeyEventData) (*bridge.KeyEventResult, bool) {
	ctx := newDecisionCtx(d.c, d.host)

	// 第一段：活跃宿主迁移裁决（第一拒绝权）。
	switch d.host.Judge(ctx, key, data).Verdict {
	case VerdictActivate, VerdictRelease:
		// 宿主迁移在第 1 批落地（executeActivate/applyEngineDiff/CompositionPhase）。
		// 第 0 批骨架：暂不执行迁移，交旧路径。
		return nil, false
	}

	// 第二段：按键处理链遍历（短路于第一个非 Pass，I11）。
	for _, h := range d.keyHandlerChain() {
		switch hd := h.Judge(ctx, key, data); hd.Verdict {
		case VerdictPass:
			continue
		case VerdictHandle:
			return h.Apply(d.c, key, data), true
		case VerdictActivate, VerdictRelease:
			// 链上迁移裁决，第 1 批落地。
			return nil, false
		}
	}
	return nil, false
}

// shadowLog 第 0b 影子运行：只读地运行宿主迁移裁决并记 DEBUG 日志，零副作用、零行为影响。
// 仅记元数据 + 单按键 + 裁决（DEBUG 级，遵守日志隐私约束，不记 buffer 内容/候选文本）。
func (d *decider) shadowLog(key string, data *bridge.KeyEventData) {
	ctx := newDecisionCtx(d.c, d.host)
	hd := d.host.Judge(ctx, key, data)
	d.c.logger.Debug("shadow decider",
		"host", d.host.Name(),
		"key", key,
		"bufferLen", ctx.BufferLen(),
		"candCount", ctx.CandidateCount(),
		"verdict", hd.Verdict.String(),
	)
}
