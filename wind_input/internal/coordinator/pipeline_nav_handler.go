// pipeline_nav_handler.go — 共享候选窗导航的链上 KeyHandler（KeyHandler 链分解）。
//
// 把翻页/高亮从各模式的「整模式」handler 里抽成决策器链上的独立处理单元。导航逻辑复用
// pipeline_nav.go 的 navPageUp/navPageDown/navHighlightUp/navHighlightDown（字节级等价），
// 宿主差异（翻页键谓词、候选窗刷新 showUI、分级加载 expand）经构造参数注入——故**同一个
// navKeyHandler 类型被多个宿主复用**（true sharedNav）。当前接入 temp_pinyin / special /
// quick_input（拼音 + 基础双上下文）；temp_english（highlight 内联）待后续批次。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

var _ KeyHandler = navKeyHandler{}

// pageKeyPredicate 翻页键谓词的统一签名（适配 isPageUpKey/isQuickInputPageUpKey 的不同入参）。
type pageKeyPredicate func(key string, data *bridge.KeyEventData) bool

// navKeyMatch 是「是否导航键」的唯一判定：翻页（注入谓词）或高亮（通用谓词）。
// mode handler 的让位谓词（isStandardNavKey / isQuickInputBaseNavKey）与 navKeyHandler 的
// 认领谓词都经此函数，保证「Pass ⟺ Handle」对同一键零漂移、与旧 switch 逐字节等价。
func navKeyMatch(c *Coordinator, pageUp, pageDown pageKeyPredicate, key string, data *bridge.KeyEventData) bool {
	vk := uint32(data.KeyCode)
	mods := uint32(data.Modifiers)
	return pageUp(key, data) || pageDown(key, data) ||
		c.isHighlightUpKey(vk, mods) || c.isHighlightDownKey(vk, mods)
}

// stdPageUp/stdPageDown 标准翻页谓词（pinyin/special 用，PageUp/= 等通用键）。
func (c *Coordinator) stdPageUp(key string, data *bridge.KeyEventData) bool {
	return c.isPageUpKey(key, data.KeyCode, uint32(data.Modifiers))
}
func (c *Coordinator) stdPageDown(key string, data *bridge.KeyEventData) bool {
	return c.isPageDownKey(key, data.KeyCode, uint32(data.Modifiers))
}

// quickPageUp/quickPageDown 快捷输入基础上下文专用翻页谓词（排除 -/=/[/] 等输入字符键）。
func (c *Coordinator) quickPageUp(key string, data *bridge.KeyEventData) bool {
	return c.isQuickInputPageUpKey(key, data.KeyCode, uint32(data.Modifiers))
}
func (c *Coordinator) quickPageDown(key string, data *bridge.KeyEventData) bool {
	return c.isQuickInputPageDownKey(key, data.KeyCode, uint32(data.Modifiers))
}

// isStandardNavKey 标准导航键（pinyin / special / quick_input 拼音上下文）：通用翻页 + 高亮。
func (c *Coordinator) isStandardNavKey(key string, data *bridge.KeyEventData) bool {
	return navKeyMatch(c, c.stdPageUp, c.stdPageDown, key, data)
}

// isQuickInputBaseNavKey 快捷输入基础上下文的导航键：专用翻页谓词 + 高亮。
func (c *Coordinator) isQuickInputBaseNavKey(key string, data *bridge.KeyEventData) bool {
	return navKeyMatch(c, c.quickPageUp, c.quickPageDown, key, data)
}

// navKeyHandler 标准导航的链上 KeyHandler。宿主差异经构造参数注入：
//   - pageUp/pageDown：翻页键谓词（标准 stdPageUp/Down 或快捷专用 quickPageUp/Down）。
//   - showUI：该宿主的候选窗刷新（showPinyinModeUI(ops) / showSpecialUI / showQuickInputUI）。
//   - pageDownExpand：navPageDown 翻页后分级加载（pinyin/quick=nil；special=expandSpecialCandidates）。
//   - hiDownExpand：navHighlightDown 翻页后分级加载（pinyin=expandCandidates；special 同 pageDown；quick=nil）。
type navKeyHandler struct {
	c              *Coordinator
	name           string
	pageUp         pageKeyPredicate
	pageDown       pageKeyPredicate
	showUI         func()
	pageDownExpand func()
	hiDownExpand   func()
}

func (h navKeyHandler) Name() string { return h.name }

// Judge：导航键 → Handle，其余 → Pass（交回链上居前的模式特有 handler）。
func (h navKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if navKeyMatch(h.c, h.pageUp, h.pageDown, key, data) {
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
	case h.pageUp(key, data):
		return c.navPageUp(h.showUI)
	case h.pageDown(key, data):
		return c.navPageDown(h.showUI, h.pageDownExpand, false)
	case c.isHighlightUpKey(vk, mods):
		return c.navHighlightUp(h.showUI)
	case c.isHighlightDownKey(vk, mods):
		return c.navHighlightDown(h.showUI, h.hiDownExpand)
	}
	// Judge 已保证命中其一；防御性消费，不应到达。
	return navConsumed()
}
