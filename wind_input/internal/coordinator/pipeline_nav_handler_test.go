package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/ipc"
	"github.com/huanfeng/wind_input/internal/ui"
)

// TestPinyinNavKeyHandlerDispatch 验证 KeyHandler 链分解（temp_pinyin 试点）：
//   - 导航键（翻页）经 pinyinNavKeyHandler 认领并推进翻页，与 navPageDown/navPageUp 等价；
//   - 模式特有键（字母）留给 tempPinyinKeyHandler；
//   - 「整模式 handler 对导航键 Pass」⟺「nav handler 对导航键 Handle」同步成立（I11 单一归属）。
func TestPinyinNavKeyHandlerDispatch(t *testing.T) {
	h := newTestCoordinator(t)
	h.config = nil // 用默认翻页键判定（VK_NEXT/VK_PRIOR），绕过测试默认空 PageKeys 配置

	// 跨 2 页候选：14 条、每页 7、当前第 1 页。
	h.candidates = make([]ui.Candidate, 14)
	h.candidatesPerPage = 7
	h.currentPage = 1
	h.totalPages = 2
	h.selectedIndex = 0

	ops := h.tempPinyinOps()
	mode := tempPinyinKeyHandler{c: h.Coordinator}
	nav := navKeyHandler{
		c:            h.Coordinator,
		name:         "temp_pinyin.nav",
		showUI:       func() { h.showPinyinModeUI(ops) },
		hiDownExpand: h.expandCandidates,
	}
	ctx := newDecisionCtx(h.Coordinator, newTempPinyinProcessor(h.Coordinator))

	// 翻页下键：模式 handler Pass、nav handler Handle、Apply 推进到第 2 页。
	pgdn := &bridge.KeyEventData{KeyCode: int(ipc.VK_NEXT)}
	if v := mode.Judge(ctx, "", pgdn).Verdict; v != VerdictPass {
		t.Errorf("mode.Judge(pagedown) = %v, want Pass", v)
	}
	if v := nav.Judge(ctx, "", pgdn).Verdict; v != VerdictHandle {
		t.Errorf("nav.Judge(pagedown) = %v, want Handle", v)
	}
	nav.Apply(h.Coordinator, "", pgdn)
	if h.currentPage != 2 {
		t.Errorf("after nav.Apply(pagedown): currentPage = %d, want 2", h.currentPage)
	}

	// 翻页上键：nav handler Handle、Apply 回到第 1 页。
	pgup := &bridge.KeyEventData{KeyCode: int(ipc.VK_PRIOR)}
	if v := nav.Judge(ctx, "", pgup).Verdict; v != VerdictHandle {
		t.Errorf("nav.Judge(pageup) = %v, want Handle", v)
	}
	nav.Apply(h.Coordinator, "", pgup)
	if h.currentPage != 1 {
		t.Errorf("after nav.Apply(pageup): currentPage = %d, want 1", h.currentPage)
	}

	// 模式特有键（字母 a）：模式 handler Handle、nav handler Pass。
	alpha := &bridge.KeyEventData{Key: "a", KeyCode: int('A')}
	if v := mode.Judge(ctx, "a", alpha).Verdict; v != VerdictHandle {
		t.Errorf("mode.Judge(a) = %v, want Handle", v)
	}
	if v := nav.Judge(ctx, "a", alpha).Verdict; v != VerdictPass {
		t.Errorf("nav.Judge(a) = %v, want Pass", v)
	}
}
