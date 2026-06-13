// pipeline_nav_test.go — 共享导航辅助方法的表驱动单测（脱离引擎/UI，纯状态机）。
package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/ui"
)

// navTestCoord 构造一个仅含候选分页状态的 Coordinator。
func navTestCoord(candCount, perPage, page, totalPages, sel int) *Coordinator {
	return &Coordinator{
		candidates:        make([]ui.Candidate, candCount),
		candidatesPerPage: perPage,
		currentPage:       page,
		totalPages:        totalPages,
		selectedIndex:     sel,
	}
}

func TestNavPageUp(t *testing.T) {
	// 非首页：回退一页、高亮归零、刷新。
	c := navTestCoord(20, 5, 3, 4, 2)
	shown := 0
	c.navPageUp(func() { shown++ })
	if c.currentPage != 2 || c.selectedIndex != 0 || shown != 1 {
		t.Errorf("page>1: page=%d sel=%d shown=%d, want 2/0/1", c.currentPage, c.selectedIndex, shown)
	}
	// 首页：无变化、不刷新。
	c = navTestCoord(20, 5, 1, 4, 3)
	shown = 0
	c.navPageUp(func() { shown++ })
	if c.currentPage != 1 || c.selectedIndex != 3 || shown != 0 {
		t.Errorf("page==1: page=%d sel=%d shown=%d, want 1/3/0(不刷新)", c.currentPage, c.selectedIndex, shown)
	}
}

func TestNavPageDown(t *testing.T) {
	// 非末页、无 expand：前进一页。
	c := navTestCoord(20, 5, 1, 4, 2)
	shown := 0
	c.navPageDown(func() { shown++ }, nil, false)
	if c.currentPage != 2 || c.selectedIndex != 0 || shown != 1 {
		t.Errorf("page<total: page=%d sel=%d shown=%d, want 2/0/1", c.currentPage, c.selectedIndex, shown)
	}

	// expandBefore=true：翻页**前**调 expand（看到旧 currentPage）。
	c = navTestCoord(20, 5, 3, 4, 0)
	var expandSawPage int
	c.navPageDown(func() {}, func() { expandSawPage = c.currentPage }, true)
	if expandSawPage != 3 || c.currentPage != 4 {
		t.Errorf("expandBefore: expandSawPage=%d page=%d, want 3/4（翻页前扩展）", expandSawPage, c.currentPage)
	}

	// expandBefore=false：翻页**后**调 expand（看到新 currentPage）。
	c = navTestCoord(20, 5, 3, 4, 0)
	expandSawPage = 0
	c.navPageDown(func() {}, func() { expandSawPage = c.currentPage }, false)
	if expandSawPage != 4 || c.currentPage != 4 {
		t.Errorf("expandAfter: expandSawPage=%d page=%d, want 4/4（翻页后扩展）", expandSawPage, c.currentPage)
	}

	// 末页：无 page 变化（expandBefore 仍可在接近末页触发，但 page 不动）。
	c = navTestCoord(20, 5, 4, 4, 1)
	shown = 0
	c.navPageDown(func() { shown++ }, nil, false)
	if c.currentPage != 4 || shown != 0 {
		t.Errorf("page==total: page=%d shown=%d, want 4/0(不刷新)", c.currentPage, shown)
	}
}

func TestNavHighlightUp(t *testing.T) {
	// 页内上移。
	c := navTestCoord(20, 5, 2, 4, 3)
	shown := 0
	c.navHighlightUp(func() { shown++ })
	if c.selectedIndex != 2 || c.currentPage != 2 || shown != 1 {
		t.Errorf("页内: sel=%d page=%d shown=%d, want 2/2/1", c.selectedIndex, c.currentPage, shown)
	}
	// 页首且非首页：回退一页、高亮置该页末。
	c = navTestCoord(18, 5, 2, 4, 0) // page2 末页起 5*1=5..9，满页 → 末项 idx4
	shown = 0
	c.navHighlightUp(func() { shown++ })
	if c.currentPage != 1 || c.selectedIndex != 4 || shown != 1 {
		t.Errorf("跨页上: page=%d sel=%d shown=%d, want 1/4/1", c.currentPage, c.selectedIndex, shown)
	}
	// 最顶（首页页首）：无变化、不刷新。
	c = navTestCoord(20, 5, 1, 4, 0)
	shown = 0
	c.navHighlightUp(func() { shown++ })
	if c.currentPage != 1 || c.selectedIndex != 0 || shown != 0 {
		t.Errorf("最顶: page=%d sel=%d shown=%d, want 1/0/0(不刷新)", c.currentPage, c.selectedIndex, shown)
	}
	// 空候选：不刷新。
	c = navTestCoord(0, 5, 1, 1, 0)
	shown = 0
	c.navHighlightUp(func() { shown++ })
	if shown != 0 {
		t.Errorf("空候选: shown=%d, want 0", shown)
	}
}

func TestNavHighlightDown(t *testing.T) {
	// 页内下移。
	c := navTestCoord(20, 5, 1, 4, 2)
	shown := 0
	c.navHighlightDown(func() { shown++ }, nil)
	if c.selectedIndex != 3 || c.currentPage != 1 || shown != 1 {
		t.Errorf("页内: sel=%d page=%d shown=%d, want 3/1/1", c.selectedIndex, c.currentPage, shown)
	}
	// 页尾且非末页、翻页后接近末页（currentPage>=totalPages-1）：前进一页、高亮归零、调 expand。
	c = navTestCoord(20, 5, 3, 4, 4) // page3 满页末项 idx4，翻到 page4 触发 4>=3 扩展
	shown = 0
	expandCalled := false
	c.navHighlightDown(func() { shown++ }, func() { expandCalled = true })
	if c.currentPage != 4 || c.selectedIndex != 0 || shown != 1 || !expandCalled {
		t.Errorf("跨页下: page=%d sel=%d shown=%d expand=%v, want 4/0/1/true", c.currentPage, c.selectedIndex, shown, expandCalled)
	}
	// 页尾且非末页、但未接近末页：前进一页、不 expand。
	c = navTestCoord(20, 5, 1, 4, 4)
	expandCalled = false
	c.navHighlightDown(func() {}, func() { expandCalled = true })
	if c.currentPage != 2 || expandCalled {
		t.Errorf("跨页下未近末页: page=%d expand=%v, want 2/false", c.currentPage, expandCalled)
	}
	// 末页页尾：无变化、不刷新。
	c = navTestCoord(20, 5, 4, 4, 4)
	shown = 0
	c.navHighlightDown(func() { shown++ }, nil)
	if c.currentPage != 4 || shown != 0 {
		t.Errorf("末页页尾: page=%d shown=%d, want 4/0(不刷新)", c.currentPage, shown)
	}
}
