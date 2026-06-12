// pipeline_context.go — 决策上下文（只读视图）。
package coordinator

import "github.com/huanfeng/wind_input/internal/schema"

// DecisionCtx 是 Coordinator 的只读视图，传给各处理单元的 Judge。
// 只暴露 Getter，从编译期保证 Judge 无法写状态（不变量 I2）。
//
// buffer 访问器按 host 路由（BufferText/BufferLen 委派给当前宿主）：engine_default → inputBuffer，
// temp_pinyin → tempPinyinBuffer 等。这是第 0b 影子实测暴露的修正——临时拼音/快捷输入活跃时
// inputBuffer 为空，固定读 inputBuffer 会让 engine_default 误判。候选区是公共的，仍直接读 c。
type DecisionCtx struct {
	c    *Coordinator
	host Processor
}

func newDecisionCtx(c *Coordinator, host Processor) *DecisionCtx {
	return &DecisionCtx{c: c, host: host}
}

// BufferText 当前宿主的活跃 buffer（按 host 路由，非固定 inputBuffer）。
func (ctx *DecisionCtx) BufferText() string { return ctx.host.BufferText() }

// BufferLen 当前宿主活跃 buffer 的长度。
func (ctx *DecisionCtx) BufferLen() int { return len(ctx.host.BufferText()) }

// CandidateCount 当前候选数（公共候选区）。
func (ctx *DecisionCtx) CandidateCount() int { return len(ctx.c.candidates) }

// CurrentPage 当前页码（1-based）。
func (ctx *DecisionCtx) CurrentPage() int { return ctx.c.currentPage }

// SelectedIndex 当前页内高亮索引（0-based）。
func (ctx *DecisionCtx) SelectedIndex() int { return ctx.c.selectedIndex }

// CandidatesPerPage 当前生效的每页候选数。
func (ctx *DecisionCtx) CandidatesPerPage() int { return ctx.c.candidatesPerPage }

// ChineseMode 是否中文模式。
func (ctx *DecisionCtx) ChineseMode() bool { return ctx.c.chineseMode }

// FullWidth 是否全角。
func (ctx *DecisionCtx) FullWidth() bool { return ctx.c.fullWidth }

// EngineIsCodeTable 当前引擎是否为码表类型（如五笔）。
func (ctx *DecisionCtx) EngineIsCodeTable() bool {
	return ctx.c.engineMgr != nil && ctx.c.engineMgr.IsCurrentEngineType(schema.EngineTypeCodeTable)
}

// HasPrefix 码表/短语层中是否存在以 s 为前缀的编码（供 z fallback / engine_default 渐进决策）。
func (ctx *DecisionCtx) HasPrefix(s string) bool {
	return ctx.c.engineMgr != nil && ctx.c.engineMgr.HasPrefix(s)
}
