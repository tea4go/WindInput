// pipeline_nav_handler.go — 共享候选窗导航的链上 KeyHandler（KeyHandler 链分解第一步）。
//
// 把翻页/高亮从各模式的「整模式」handler 里抽成决策器链上的独立处理单元。导航逻辑复用
// pipeline_nav.go 的 navPageUp/navPageDown/navHighlightUp/navHighlightDown（字节级等价），
// 宿主差异（候选窗刷新 showUI、分级加载 expand）经构造参数注入——同一 handler 类型可被多个
// 宿主复用（true sharedNav 的雏形）。本批为 temp_pinyin 试点；quick_input 拼音上下文等后续接入。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

var _ KeyHandler = pinyinNavKeyHandler{}

// isPinyinModeNavKey 判定按键是否为拼音模式导航键（翻页/高亮上下）。
//
// 谓词与 handlePinyinModeKey switch 的导航 case 逐一对应，保证「整模式 handler 对该键 Pass」
// 与「nav handler 对该键 Handle」同步成立——分发结果与旧 monolith switch 逐字节等价。
func (c *Coordinator) isPinyinModeNavKey(key string, data *bridge.KeyEventData) bool {
	vk := uint32(data.KeyCode)
	mods := uint32(data.Modifiers)
	return c.isPageUpKey(key, data.KeyCode, mods) ||
		c.isPageDownKey(key, data.KeyCode, mods) ||
		c.isHighlightUpKey(vk, mods) ||
		c.isHighlightDownKey(vk, mods)
}

// pinyinNavKeyHandler 拼音模式共享导航 handler。ops 提供该宿主的候选窗刷新（showPinyinModeUI），
// expand 固定用 c.expandCandidates（分级加载，与旧导航 case 一致）。
type pinyinNavKeyHandler struct {
	c   *Coordinator
	ops *pinyinModeOps
}

func (h pinyinNavKeyHandler) Name() string { return "pinyin.nav" }

// Judge：导航键 → Handle，其余 → Pass（交回链上居前的模式特有 handler；本 handler 在 host
// KeyHandlers 中排第二）。
func (h pinyinNavKeyHandler) Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision {
	if h.c.isPinyinModeNavKey(key, data) {
		return decHandle()
	}
	return decPass()
}

// Apply：按导航键类别分发到共享导航函数，showUI 走本宿主 ops。检查顺序
// （pageUp→pageDown→highlightUp→highlightDown）与旧 handlePinyinModeKey switch 一致——
// 同键多谓词命中时取首个，保证选择一致、逐字节等价。
func (h pinyinNavKeyHandler) Apply(c *Coordinator, key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	vk := uint32(data.KeyCode)
	mods := uint32(data.Modifiers)
	show := func() { c.showPinyinModeUI(h.ops) }
	switch {
	case c.isPageUpKey(key, data.KeyCode, mods):
		return c.navPageUp(show)
	case c.isPageDownKey(key, data.KeyCode, mods):
		return c.navPageDown(show, nil, false)
	case c.isHighlightUpKey(vk, mods):
		return c.navHighlightUp(show)
	case c.isHighlightDownKey(vk, mods):
		return c.navHighlightDown(show, c.expandCandidates)
	}
	// Judge 已保证命中其一；防御性消费，不应到达。
	return navConsumed()
}
