package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/ui"
)

// TestHandleSelectChar_NavConsumed 验证以词定字路径 (Tab/数字键) 在高亮是
// 数组未展开 nav (IsGroup=true) 时不上屏组名首字符, 而是返回 Consumed,
// 让用户先选 nav 进入展开后再做以词定字。回归点见用户反馈 #5 (2026-05-18)。
func TestHandleSelectChar_NavConsumed(t *testing.T) {
	const groupTpl = `$AA("标点符号", "，。！？")`
	h := newTestCoordinator(t, withEngineMgr())
	h.inputBuffer = "zzbd"
	h.inputCursorPos = len("zzbd")
	h.candidates = []ui.Candidate{{
		Text:          "标点符号",
		Code:          "zzbd",
		IsGroup:       true,
		GroupCode:     "zzbd",
		GroupName:     "标点符号",
		GroupTemplate: groupTpl,
	}}

	res := h.handleSelectChar(0)
	if res == nil {
		t.Fatalf("nav 高亮时 handleSelectChar 应返回 Consumed 而非 nil")
	}
	if res.Type != bridge.ResponseTypeConsumed {
		t.Fatalf("expected ResponseTypeConsumed, got %v (Text=%q)", res.Type, res.Text)
	}
	if h.inputBuffer != "zzbd" {
		t.Fatalf("inputBuffer 应保持不变, got %q", h.inputBuffer)
	}
}
