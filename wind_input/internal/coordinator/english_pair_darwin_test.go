//go:build darwin

package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/transform"
	"github.com/huanfeng/wind_input/pkg/config"
)

// TestEnglishModeAutoPair_Darwin 验证 IME 英文模式 (chineseMode=false) 下的成对标点。
//
// 回归点: macOS bridge 的 keyCodeToKeyName 不含 Shift, Shift+9 投递的 data.Key 是 "9"
// 而非 "(", 英文分支又在通用 shiftedKeyMap 解析之前返回, 故必须在英文分支内自行应用
// shiftedKeyMap, 否则括号/花括号/尖括号等 Shift 类配对永远命中不了。
func TestEnglishModeAutoPair_Darwin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Input.AutoPair.English = true
	cfg.Input.AutoPair.EnglishPairs = []string{"()", "[]", "{}", "<>"}

	h := newTestCoordinator(t, withChineseMode(false), withConfig(cfg))
	h.pairTrackerEn = transform.NewPairTracker(cfg.Input.AutoPair.EnglishPairs)

	// Shift+9 → "(": darwin bridge 给 data.Key="9" (keyCodeToKeyName 不含 Shift) + ModShift。
	// 期望经 shiftedKeyMap 解析为 "(" 后插入配对 "()" 并回退光标 1。
	res := h.HandleKeyEvent(bridge.KeyEventData{Key: "9", KeyCode: 0x39, Modifiers: ModShift})
	if res == nil || res.Type != bridge.ResponseTypeInsertTextWithCursor || res.Text != "()" || res.CursorOffset != 1 {
		t.Fatalf("Shift+9 应配对 \"()\" 且 CursorOffset=1, 实际=%+v", res)
	}

	// 智能跳过: 光标在 (|) 时再按 Shift+0 → ")" 栈顶匹配 → MoveCursorRight (跳过已补的右括号)。
	res = h.HandleKeyEvent(bridge.KeyEventData{Key: "0", KeyCode: 0x30, Modifiers: ModShift})
	if res == nil || res.Type != bridge.ResponseTypeMoveCursorRight {
		t.Fatalf("Shift+0 栈顶匹配应智能跳过 (MoveCursorRight), 实际=%+v", res)
	}

	// 非 Shift 括号 "[" 也应配对 "[]" (路径不依赖 shiftedKeyMap)。
	res = h.HandleKeyEvent(bridge.KeyEventData{Key: "[", KeyCode: 0x5B})
	if res == nil || res.Type != bridge.ResponseTypeInsertTextWithCursor || res.Text != "[]" || res.CursorOffset != 1 {
		t.Fatalf("\"[\" 应配对 \"[]\" 且 CursorOffset=1, 实际=%+v", res)
	}
}

// TestEnglishModeAutoPair_Disabled_Darwin 验证 AutoPair.English=false 时英文模式不配对 (透传)。
func TestEnglishModeAutoPair_Disabled_Darwin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Input.AutoPair.English = false
	cfg.Input.AutoPair.EnglishPairs = []string{"()", "[]"}

	h := newTestCoordinator(t, withChineseMode(false), withConfig(cfg))
	h.pairTrackerEn = transform.NewPairTracker(cfg.Input.AutoPair.EnglishPairs)

	if res := h.HandleKeyEvent(bridge.KeyEventData{Key: "9", KeyCode: 0x39, Modifiers: ModShift}); res != nil {
		t.Fatalf("English=false 时 Shift+9 应透传 (nil), 实际=%+v", res)
	}
}
