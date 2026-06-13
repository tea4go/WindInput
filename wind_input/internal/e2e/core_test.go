//go:build windows || darwin

// core_test.go — 核心转换路径的 golden 回归。
//
// 覆盖 IME 心脏路径：拼音转换/选词/空格上屏/编辑重转/分页/分步确认，以及五笔码表
// 转换上屏。每个用例用 mustHarness 起独立的临时用户 db（无学习历史），候选集与顺序
// 由系统词库决定，结果确定；weight 由 golden 层默认 mask，词频微调不影响断言。
//
// 改引擎/coordinator 后跑 `go test ./internal/e2e`，golden 不一致即报 diff；人工确认
// 变化符合预期后 `go test ./internal/e2e -update` 刷新。
package e2e

import "testing"

// TestPinyinBasicConvert 验证拼音基本转换 + 数字键选词上屏：
// 输入 "nihao" 得到候选（首选 你好），按数字键 1 选首选并上屏，缓冲清空。
func TestPinyinBasicConvert(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		SelectCandidate(1)
	AssertGolden(t, "pinyin_basic_convert", rec.Render())
}

// TestPinyinSpaceCommit 验证空格上屏首选：输入 "nihao" 后按空格，应上屏首选 你好，
// 等价于 SelectCandidate(1) 的结果（确认空格键走"选首选"路径）。
func TestPinyinSpaceCommit(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Space()
	AssertGolden(t, "pinyin_space_commit", rec.Render())
}

// TestPinyinBackspaceEdit 验证编辑中退格重转：输入 "nihao" 后连退两格到 "nih"，
// preedit 与候选应按截短后的输入重新派生（而非保留旧候选）。
func TestPinyinBackspaceEdit(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		Backspace().
		Backspace()
	AssertGolden(t, "pinyin_backspace_edit", rec.Render())
}

// TestPinyinPaging 验证候选分页：输入 "jian"（候选跨 2 页），PageDown 进第 2 页、
// PageUp 回第 1 页，current_page/total_pages 随之变化，selected_index 翻页归零。
func TestPinyinPaging(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("jian").
		PageDown().
		PageUp()
	AssertGolden(t, "pinyin_paging", rec.Render())
}

// TestPinyinSegmentConfirm 验证分步确认：输入 "nihao" 后选第 3 个候选（单字 你），
// 已确认段 你 进入 confirmed_segments，input_buffer 收缩为剩余 "hao" 并重转候选。
func TestPinyinSegmentConfirm(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("nihao").
		SelectCandidate(3)
	AssertGolden(t, "pinyin_segment_confirm", rec.Render())
}

// TestWubiCodeCommit 验证五笔码表转换 + 空格上屏：输入编码 "wgkg" 得到候选（首选 鸽），
// 按空格上屏首选，缓冲清空。覆盖码表方案（与拼音不同的引擎路径）。
func TestWubiCodeCommit(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("wgkg").
		Space()
	AssertGolden(t, "wubi_code_commit", rec.Render())
}
