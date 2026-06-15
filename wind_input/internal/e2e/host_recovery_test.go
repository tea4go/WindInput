//go:build windows || darwin

// host_recovery_test.go — 决策器 host 状态机的回归：链外 clearState 后 d.host 不得滞留陈旧。
package e2e

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/coordinator"
	"github.com/huanfeng/wind_input/pkg/config"
)

// TestHostRecoveryAfterOutOfBandClearState 回归 [收口审查 HIGH]：临时拼音中经**链外** clearState
// （输入态 Ctrl/Alt 透传、失焦等不走 dispatchHostChain 的路径）清模式标志后，d.host 滞留陈旧
// （仍=tempPinyin）。下一个字母键须经 dispatchHostChain 的"组链前 reconcileHost"回落 engine_default
// 走正常输入，绝不能用陈旧 host 误把字母写进 tempPinyinBuffer（且拼音层已卸载）。
func TestHostRecoveryAfterOutOfBandClearState(t *testing.T) {
	h, err := BuildHarness(coordinator_e2eOptions())
	if err != nil {
		t.Fatalf("BuildHarness: %v", err)
	}
	t.Cleanup(h.Close)

	// 进入临时拼音并打入拼音 buffer。
	h.Type("wg") // 五笔 buffer
	h.Key("`")   // 顶屏高亮候选 + 进临时拼音
	h.Type("ni") // 临时拼音 buffer = "ni"
	if !h.Coord.ExportState().TempPinyinMode {
		t.Fatalf("setup: 期望临时拼音激活")
	}

	// Ctrl+S（非热键的 Ctrl 组合）→ 命中"输入态 Ctrl/Alt 透传"分支 → clearState（清模式标志，
	// 不经 dispatchHostChain，d.host 滞留 tempPinyin）。
	h.Coord.HandleKeyEvent(bridge.KeyEventData{Key: "s", KeyCode: 0x53, Modifiers: coordinator.ModCtrl})
	if h.Coord.ExportState().TempPinyinMode {
		t.Fatalf("Ctrl+S 应已清临时拼音模式")
	}

	// 下一个字母 'a' → 必须走正常五笔输入（inputBuffer="a"），不得回到临时拼音。
	h.Type("a")
	st := h.Coord.ExportState()
	if st.TempPinyinMode {
		t.Errorf("HIGH 回归：'a' 误重激活临时拼音")
	}
	if st.InputBuffer != "a" {
		t.Errorf("HIGH 回归：'a' 应进正常输入流，got inputBuffer=%q（陈旧 host 会把 'a' 写进 tempPinyinBuffer 致 inputBuffer 为空）", st.InputBuffer)
	}
}

func coordinator_e2eOptions() Options {
	return Options{
		SchemaID:              "wubi86",
		TempPinyinTriggerKeys: []string{"backtick"},
		Configure:             func(cfg *config.Config) {},
	}
}
