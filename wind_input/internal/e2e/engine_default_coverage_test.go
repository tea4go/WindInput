//go:build windows || darwin

// engine_default_coverage_test.go — engine_default 兜底宿主「正常输入」路径的 golden 覆盖。
//
// 专为「engine_default 正常输入进决策器」重构补齐护栏：覆盖此前无专门 golden 的 switch 分支
// ——光标移动（左右 / Home / End）、Delete、正常拼音态方向键高亮、Escape、拼音分隔符。
// 搬迁是逐字节等价的纯重构（switch 原样进 engineDefaultProcessor 链），这些 golden 用于捕捉
// 搬迁中可能的分支遗漏 / 顺序错误。golden 记录的是**当前行为**，与"是否理想"无关。
package e2e

import "testing"

// TestEngineDefaultCursorMove 光标在编辑缓冲内移动：左移两格、Home、End、右移。
func TestEngineDefaultCursorMove(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Key("left").
		Key("left").
		Key("home").
		Key("end").
		Key("right")
	AssertGolden(t, "engine_default_cursor_move", rec.Render())
}

// TestEngineDefaultDelete 光标左移后按 Delete 删除光标处字符（区别于退格删光标前字符）。
func TestEngineDefaultDelete(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Key("left").
		Key("left").
		Key("delete")
	AssertGolden(t, "engine_default_delete", rec.Render())
}

// TestEngineDefaultHighlightNav 正常拼音输入态方向键移动候选高亮：下、下、上（selected_index 变化）。
func TestEngineDefaultHighlightNav(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Key("down").
		Key("down").
		Key("up")
	AssertGolden(t, "engine_default_highlight_nav", rec.Render())
}

// TestEngineDefaultEscape 输入态按 Escape 清空 composition（缓冲与候选清零）。
func TestEngineDefaultEscape(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Key("esc")
	AssertGolden(t, "engine_default_escape", rec.Render())
}

// TestEngineDefaultEnterRaw 正常输入态按 Enter：上屏原始编码缓冲（非候选）并清空。
func TestEngineDefaultEnterRaw(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Enter()
	AssertGolden(t, "engine_default_enter_raw", rec.Render())
}

// TestEngineDefaultPinyinSeparator 拼音分隔符 ' 手动分割音节（xian'an → xi'an/西安 等）。
func TestEngineDefaultPinyinSeparator(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("xian").
		Type("'").
		Type("an")
	AssertGolden(t, "engine_default_pinyin_separator", rec.Render())
}
