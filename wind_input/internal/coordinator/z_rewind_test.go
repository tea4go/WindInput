// z_rewind_test.go — 验证 z 键混合切入后的"原子回退"行为（统一夺取回退机制）。
//
// 场景: 用户配置 z 临时拼音 + 有 zzhb 等长前缀短语. 输入 zzh 时还匹配命令前缀, 输入 zzha 时
// zHybridFallback 触发切到临时拼音 buffer="zha". 用户此刻按 backspace, 期望回到 zzh 的正常输入态
// 而不是从临时拼音 buffer 删字符. 一旦用户在临时拼音里敲了任何新字符, 回退路径作废.
//
// 回退已统一到决策器（decider.armRewind 登记 / HandleKeyEvent 模式分发前拦截首次退格 →
// rewindHijack）；本测试经 armRewind 模拟切入登记，再驱动 HandleKeyEvent 验证。
package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/ipc"
)

// armZRewind 模拟 z 混合切入临时拼音瞬间的状态 + 统一回退登记。
//   - tempPinyinBuffer = "zh" + "a" = "zha"（hostText）
//   - 快照 = 切入前的 inputBuffer "zzh"
func armZRewind(h *testCoordinator) {
	h.tempPinyinMode = true
	h.tempPinyinBuffer = "zha"
	h.tempPinyinCursorPos = len("zha")
	h.tempPinyinTriggerKey = "z"
	// 对齐 d.host=tempPinyin（生产中 z 回退经 onTempPinyinEntered 完成），使后续模式内键经
	// dispatchManagedHost 走临时拼音链。决策器恒开后这是必需的；缺它模式内键会落到 engine_default。
	h.decider.onTempPinyinEntered()
	h.decider.armRewind("zzh", "zha", h.clearTempPinyinModeStateForRewind)
}

// 切入瞬间的 backspace 应当原子回退: tempPinyinMode 清零, inputBuffer 恢复, 回退登记作废.
func TestZRewind_BackspaceRestoresPreSwitchBuffer(t *testing.T) {
	h := newTestCoordinator(t,
		withEngineMgr(withCodetableEntry("zzhb", "$")),
		withZHybridSchema(true),
	)
	armZRewind(h)

	h.pressKeyCode(int(ipc.VK_BACK))

	if h.tempPinyinMode {
		t.Errorf("tempPinyinMode should be false after rewind backspace")
	}
	if h.tempPinyinBuffer != "" {
		t.Errorf("tempPinyinBuffer = %q, want empty", h.tempPinyinBuffer)
	}
	if h.inputBuffer != "zzh" {
		t.Errorf("inputBuffer = %q, want %q", h.inputBuffer, "zzh")
	}
	if h.decider.rewindArmed() {
		t.Errorf("decider rewind should be cleared after rewind")
	}
}

// 用户切入后又敲了新字符, 回退登记作废; 此后 backspace 走标准临时拼音删字符路径,
// 不应当再回退到 inputBuffer.
func TestZRewind_NewCharInvalidatesRewind(t *testing.T) {
	h := newTestCoordinator(t,
		withEngineMgr(withCodetableEntry("zzhb", "$")),
		withZHybridSchema(true),
	)
	armZRewind(h)

	// 在临时拼音里敲 'b' → 模式分发前的触发器作废回退登记, 'b' 入 buffer → "zhab"
	h.pressKey("b")
	if h.tempPinyinBuffer != "zhab" {
		t.Fatalf("after typing 'b': tempPinyinBuffer = %q, want %q",
			h.tempPinyinBuffer, "zhab")
	}
	if h.decider.rewindArmed() {
		t.Fatalf("rewind should be cleared after typing")
	}

	// 再 backspace → 走临时拼音标准删字符, 不再回退到 inputBuffer
	h.pressKeyCode(int(ipc.VK_BACK))

	if !h.tempPinyinMode {
		t.Errorf("tempPinyinMode should remain true (rewind invalidated)")
	}
	if h.tempPinyinBuffer != "zha" {
		t.Errorf("tempPinyinBuffer = %q, want %q (after deleting 'b')",
			h.tempPinyinBuffer, "zha")
	}
	if h.inputBuffer != "" {
		t.Errorf("inputBuffer should remain empty, got %q", h.inputBuffer)
	}
}
