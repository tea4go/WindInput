// pipeline_nav_handler.go — 共享候选窗导航的链上 KeyHandler（KeyHandler 链分解）。
//
// 把翻页/高亮从各模式的「整模式」handler 里抽成决策器链上的独立处理单元。导航逻辑复用
// pipeline_nav.go 的 navPageUp/navPageDown/navHighlightUp/navHighlightDown（字节级等价），
// 宿主差异（候选窗刷新 showUI、分级加载 expand）经构造参数注入——故**同一个 navKeyHandler
// 类型被多个宿主复用**（true sharedNav）。当前接入 temp_pinyin + special；quick_input（双
// 上下文 + 专用翻页谓词）/ temp_english（highlight 内联）待后续批次。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

var _ KeyHandler = navKeyHandler{}

// isStandardNavKey 判定按键是否为「标准」导航键（翻页/高亮上下，PageUp/Down 用通用谓词）。
//
// pinyin 模式与 special 模式的导航 case 用的是同一组谓词（isPageUpKey/isPageDownKey/
// isHighlightUpKey/isHighlightDownKey），故共用此判定。quick_input 基础上下文用专用的
// isQuickInputPageUpKey（排除 -/= 等输入字符键），不在此列、待其接入时单独处理。
//
// 谓词与各模式 handleXxxKey switch 的导航 case 逐一对应，保证「整模式 handler 对该键 Pass」
// 与「nav handler 对该键 Handle」同步成立——分发结果与旧 monolith switch 逐字节等价。
func (c *Coordinator) isStandardNavKey(key string, data *bridge.KeyEventData) bool {
	vk := uint32(data.KeyCode)
	mods := uint32(data.Modifiers)
	return c.isPageUpKey(key, data.KeyCode, mods) ||
		c.isPageDownKey(key, data.KeyCode, mods) ||
		c.isHighlightUpKey(vk, mods) ||
		c.isHighlightDownKey(vk, mods)
}

// navKeyHandler 标准导航的链上 KeyHandler。宿主差异经构造参数注入：
//   - showUI：该宿主的候选窗刷新（showPinyinModeUI(ops) / showSpecialUI / ...）。
//   - pageDownExpand：navPageDown 翻页后的分级加载（pinyin=nil；special=expandSpecialCandidates）。
//   - hiDownExpand：navHighlightDown 翻页后的分级加载（pinyin=expandCandidates；special 同 pageDown）。
type navKeyHandler struct {
	c              *Coordinator
	name           string
	showUI         func()
	pageDownExpand func()
	hiDownExpand   func()
}

func (h navKeyHandler) Name() string { return h.name }

// Judge：标准导航键 → Handle，其余 → Pass（交回链上居前的模式特有 handler；本 handler 在 host
// KeyHandlers 中排第二）。
func (h navKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if h.c.isStandardNavKey(key, data) {
		return decHandle()
	}
	return decPass()
}

// Apply：按导航键类别分发到共享导航函数。检查顺序（pageUp→pageDown→highlightUp→highlightDown）
// 与旧 handleXxxKey switch 一致——同键多谓词命中时取首个，保证选择一致、逐字节等价。
func (h navKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	vk := uint32(data.KeyCode)
	mods := uint32(data.Modifiers)
	switch {
	case c.isPageUpKey(key, data.KeyCode, mods):
		return c.navPageUp(h.showUI)
	case c.isPageDownKey(key, data.KeyCode, mods):
		return c.navPageDown(h.showUI, h.pageDownExpand, false)
	case c.isHighlightUpKey(vk, mods):
		return c.navHighlightUp(h.showUI)
	case c.isHighlightDownKey(vk, mods):
		return c.navHighlightDown(h.showUI, h.hiDownExpand)
	}
	// Judge 已保证命中其一；防御性消费，不应到达。
	return navConsumed()
}
