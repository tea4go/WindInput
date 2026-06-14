// pipeline_nav.go — 共享候选窗导航（翻页/高亮上下）。
//
// 消除四套 handleXxxKey（pinyin_mode_shared / handle_quick_input / handle_temp_english /
// handle_special_mode）各自重复实现的翻页与高亮移动。导航作用于公共候选区
// （candidates/currentPage/totalPages/selectedIndex/candidatesPerPage），差异仅在：
//   - showUI：各模式自己的候选窗刷新（showPinyinModeUI / showQuickInputUI / ...）
//   - expand：分级加载扩展（nil=无；内部自检 hasMore；special 翻页后、temp_english 翻页前）
//
// 这些是**导航辅助函数**（字节级等价的纯状态推进）。链上分发由 pipeline_nav_handler.go 的
// pinyinNavKeyHandler 调用它们——temp_pinyin 已试点（整模式 handler 对导航键 Pass、nav handler
// Handle）；quick_input/temp_english/special 仍内联调用，待后续批次接入链。
package coordinator

import "github.com/huanfeng/wind_input/internal/bridge"

// navConsumed 是导航键的统一返回（消费按键、不产出文本/合成区变更）。
func navConsumed() *bridge.KeyEventResult {
	return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}
}

// navPageUp 上一页：currentPage>1 时回退一页、高亮归零、刷新 UI。各模式逐字节一致（无分级加载）。
func (c *Coordinator) navPageUp(showUI func()) *bridge.KeyEventResult {
	if c.currentPage > 1 {
		c.currentPage--
		c.selectedIndex = 0
		showUI()
	}
	return navConsumed()
}

// navPageDown 下一页：currentPage<totalPages 时前进一页、高亮归零、刷新 UI。
// expand!=nil 时按 expandBefore 在翻页前（temp_english）或翻页后（special）于接近末页时扩展。
func (c *Coordinator) navPageDown(showUI func(), expand func(), expandBefore bool) *bridge.KeyEventResult {
	if expand != nil && expandBefore && c.currentPage >= c.totalPages-1 {
		expand()
	}
	if c.currentPage < c.totalPages {
		c.currentPage++
		c.selectedIndex = 0
		if expand != nil && !expandBefore && c.currentPage >= c.totalPages-1 {
			expand()
		}
		showUI()
	}
	return navConsumed()
}

// navHighlightUp 高亮上移：页内上移；到页首且非首页则回退一页并把高亮置于该页末。无分级加载。
func (c *Coordinator) navHighlightUp(showUI func()) *bridge.KeyEventResult {
	if len(c.candidates) > 0 {
		if c.selectedIndex > 0 {
			c.selectedIndex--
			showUI()
		} else if c.currentPage > 1 {
			c.currentPage--
			startIdx := (c.currentPage - 1) * c.candidatesPerPage
			endIdx := startIdx + c.candidatesPerPage
			if endIdx > len(c.candidates) {
				endIdx = len(c.candidates)
			}
			c.selectedIndex = endIdx - startIdx - 1
			showUI()
		}
	}
	return navConsumed()
}

// navHighlightDown 高亮下移：页内下移；到页尾且非末页则前进一页、高亮归零（expand!=nil 时
// 翻页后于接近末页扩展，对齐 pinyin/special 的分级加载）。
func (c *Coordinator) navHighlightDown(showUI func(), expand func()) *bridge.KeyEventResult {
	if len(c.candidates) > 0 {
		startIdx := (c.currentPage - 1) * c.candidatesPerPage
		endIdx := startIdx + c.candidatesPerPage
		if endIdx > len(c.candidates) {
			endIdx = len(c.candidates)
		}
		pageCount := endIdx - startIdx
		if c.selectedIndex < pageCount-1 {
			c.selectedIndex++
			showUI()
		} else if c.currentPage < c.totalPages {
			c.currentPage++
			c.selectedIndex = 0
			if expand != nil && c.currentPage >= c.totalPages-1 {
				expand()
			}
			showUI()
		}
	}
	return navConsumed()
}
