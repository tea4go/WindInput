// pipeline_keyhandler.go — 按键处理层（KeyHandler）接口。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

// KeyHandler 是按键处理层的单元，与宿主解耦。按键经一条有序责任链，链上每个 handler
// 给出纯裁决，决策器选第一个非 Pass 者生效（短路 → 保证单一归属，不变量 I11）。
//
// 链的组装（决策器按 host 动态构建，见 pipeline_decider.go）：
//
//	[全局分流 handlers]      固定最高优先；特殊情况拦截/分流，大多 Pass
//	  ++ host.KeyHandlers()  当前宿主特有键（大小写/拼音分隔符/自动上屏/触发键二次输入）
//	  ++ [共享导航 handlers] 翻页 / 高亮上下 / 数字选候选 / 二三候选键 / ESC / 退格删空
//
// 共享导航 handlers 是消除四套 handleXxxKey 导航重复的载体（第 1 批起逐步抽取）。
type KeyHandler interface {
	Name() string

	// Judge 纯判断：本 handler 对此按键的裁决（Pass/Handle/Activate/Release）。无副作用、可单测。
	Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision

	// Apply 执行：仅当决策器选中本 handler 的 Handle 裁决时调用（持 *Coordinator，有副作用）。
	Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult
}
