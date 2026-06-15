//go:build windows || darwin

// engine_layer_test.go — I3 引擎层对称挂卸（applyEngineDiff）的显式不变量断言。
//
// 与 8 个 *_pinyin_* golden 的 pinyin_layer_mounted 字段互为印证：golden 防字节回归，
// 本测试把「进拼音上下文→挂载、退出→卸载、零泄漏」作为一等命名断言，直接读 ExportState。
package e2e

import "testing"

func TestEngineLayerSymmetry(t *testing.T) {
	mounted := func(h *Harness) bool { return h.Coord.ExportState().PinyinLayerMounted }

	// 临时拼音：进入挂载，ESC 退出卸载（无泄漏）。
	t.Run("temp_pinyin_esc", func(t *testing.T) {
		h := mustHarness(t, "wubi86")
		if mounted(h) {
			t.Fatal("layer mounted before entry")
		}
		h.Type("wg")
		h.Key("`") // 顶屏 wg 候选 + 进临时拼音
		if !mounted(h) {
			t.Fatal("layer not mounted after entering temp pinyin")
		}
		h.Type("hao")
		if !mounted(h) {
			t.Fatal("layer unmounted while typing in temp pinyin")
		}
		h.Key("esc")
		if mounted(h) {
			t.Fatal("layer leaked after esc exit")
		}
	})

	// 临时拼音：空格上屏也对称卸载。
	t.Run("temp_pinyin_commit", func(t *testing.T) {
		h := mustHarness(t, "wubi86")
		h.Type("wg")
		h.Key("`")
		h.Type("hao")
		h.Space()
		if mounted(h) {
			t.Fatal("layer leaked after space commit")
		}
	})

	// 快捷输入拼音子上下文：基础态不挂载、字母进拼音挂载、上屏卸载。
	t.Run("quick_input_pinyin", func(t *testing.T) {
		h := mustHarness(t, "wubi86")
		h.Key(";") // 快捷输入基础态
		if mounted(h) {
			t.Fatal("quick input base context should not mount pinyin layer")
		}
		h.Type("rq") // 字母 → 拼音子上下文
		if !mounted(h) {
			t.Fatal("layer not mounted in quick input pinyin sub-context")
		}
		h.Space()
		if mounted(h) {
			t.Fatal("layer leaked after quick input pinyin commit")
		}
	})
}
