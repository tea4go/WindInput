// routing_basic_test.go — HandleKeyEvent 最浅层路由的烟雾测试.
//
// 覆盖"不触达 engineMgr / uiManager"的早期直通分支, 主要目的是验证
// testhelper_test.go 提供的脚手架本身可用. 真正的输入路径用例（候选生成、
// 临时拼音、z 混合决策等）依赖 engine fixture, 等后续提交补.
package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/candidate"
)

// 英文模式下字母键应直通给宿主, HandleKeyEvent 返回 nil, 不进入 IME 流水线.
func TestRouting_EnglishMode_LetterPassThrough(t *testing.T) {
	h := newTestCoordinator(t, withChineseMode(false))
	r := h.pressKey("a")

	if r != nil {
		t.Fatalf("expected nil pass-through for English-mode letter, got %+v", r)
	}
	if h.inputBuffer != "" {
		t.Errorf("inputBuffer should remain empty, got %q", h.inputBuffer)
	}
}

// 英文模式 + 全角开启时, 字母应被转为全角并 InsertText.
func TestRouting_EnglishMode_FullWidthLetter(t *testing.T) {
	h := newTestCoordinator(t, withChineseMode(false))
	h.fullWidth = true

	r := h.pressKey("a")
	if r == nil {
		t.Fatal("expected InsertText for full-width English-mode letter, got nil")
	}
	if r.Type != bridge.ResponseTypeInsertText {
		t.Errorf("result type = %v, want InsertText", r.Type)
	}
	// "ａ" = 全角 a (U+FF41)
	if r.Text != "ａ" {
		t.Errorf("Text = %q, want %q", r.Text, "ａ")
	}
}

// Ctrl+字母组合在没有 pending input 时应直通（返回 nil）, 让宿主消费快捷键.
func TestRouting_CtrlCombo_PassThrough(t *testing.T) {
	h := newTestCoordinator(t)
	r := h.HandleKeyEvent(bridge.KeyEventData{
		Key:       "c",
		KeyCode:   int('C'),
		Modifiers: ModCtrl,
	})
	if r != nil {
		t.Errorf("expected nil pass-through for Ctrl+C without pending input, got %+v", r)
	}
}

// 回归: macOS ⌘ 映射为 ModWin. 中文模式下 ⌘C/⌘V 等系统快捷键在无 pending input 时
// 必须直通（返回 nil）, 否则 'c'/'v' 会被当成拼音字母消费, 导致复制/粘贴失效.
func TestRouting_WinCombo_PassThrough(t *testing.T) {
	for _, key := range []string{"c", "v", "x", "a"} {
		h := newTestCoordinator(t) // 默认中文模式
		r := h.HandleKeyEvent(bridge.KeyEventData{
			Key:       key,
			KeyCode:   int(key[0] - 'a' + 'A'),
			Modifiers: ModWin,
		})
		if r != nil {
			t.Errorf("⌘%s: expected nil pass-through without pending input, got %+v", key, r)
		}
		if h.inputBuffer != "" {
			t.Errorf("⌘%s: inputBuffer should remain empty, got %q", key, h.inputBuffer)
		}
	}
}

// 回归: 中文模式输入态下按 ⌘ 组合键, 应取消组字并返回 ClearComposition 让宿主消费快捷键,
// 而非把按键追加进 buffer 继续打字.
func TestRouting_WinCombo_DuringComposing_ClearsAndPassThrough(t *testing.T) {
	// clearState 会调用 engineMgr.InvalidateCommandCache, 故需挂上最小 engine fixture.
	h := newTestCoordinator(t, withEngineMgr(), withCandidates("ni", candidate.Candidate{Text: "你", Code: "ni"}))
	r := h.HandleKeyEvent(bridge.KeyEventData{
		Key:       "c",
		KeyCode:   int('C'),
		Modifiers: ModWin,
	})
	if r == nil || r.Type != bridge.ResponseTypeClearComposition {
		t.Fatalf("expected ClearComposition for ⌘C during composing, got %+v", r)
	}
	if h.inputBuffer != "" {
		t.Errorf("inputBuffer should be cleared, got %q", h.inputBuffer)
	}
}
