//go:build windows || darwin

// learning_test.go — 选词词频记录管线的回归测试。
//
// 验证"选候选 → FreqHandler.Record → Store 词频桶"这条管线在 harness 内端到端打通：
// 重复选同一候选并 FlushLearning 后，该候选在当前方案的词频记录应累积。
//
// 说明（当前实现现状，非本测试人为限制）：
//   - 选词词频写入是异步批量的（生产靠后台 50 条/30s flush），故测试须显式 FlushLearning。
//   - 首次选择不计入词频（误选保护）：选 N 次后 Count == N-1。
//   - 词频对候选「排序」的重排目前未接通到 wdat / 码表引擎主候选（CompositeDict.Query 会
//     应用 FreqBoost 并重排，但这两个引擎的主候选不经该路径生成）——词频排序模式重构待实施
//     （docs/design/freq-sort-mode.md），故本文件只回归"记录"管线，不断言候选顺序变化。
package e2e

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/schema"
)

// TestLearningFreqRecorded 验证重复选词累积词频：拼音方案下重复选第 1 候选 5 次并
// FlushLearning，词频桶中应存在对应记录且 Count==5（每次选择均记录）。
// 选第 1 候选而非固定文字，避免测试依赖特定词典版本的候选顺序。
// 通过 ConfigureSchema 显式启用 freq，不依赖生产 schema 默认值。
func TestLearningFreqRecorded(t *testing.T) {
	h, err := BuildHarness(Options{
		SchemaID: "pinyin",
		ConfigureSchema: func(s *schema.Schema) {
			if s.Learning.Freq == nil {
				s.Learning.Freq = &schema.FreqSpec{}
			}
			s.Learning.Freq.Enabled = true
		},
	})
	if err != nil {
		t.Fatalf("BuildHarness: %v", err)
	}
	t.Cleanup(h.Close)

	// 先确定 nihao 第 1 候选的文字（由词典决定，但每次查询稳定）
	h.Type("nihao")
	initState := h.Coord.ExportState()
	if len(initState.Candidates) == 0 {
		t.Fatal("nihao 无候选，拼音词典可能未就绪")
	}
	targetText := initState.Candidates[0].Text
	h.SelectCandidate(1)

	const selections = 5
	for range [selections - 1]struct{}{} {
		h.Type("nihao")
		h.SelectCandidate(1)
	}
	h.FlushLearning()

	store := h.DictMgr.GetStore()
	if store == nil {
		t.Fatal("harness store 为 nil")
	}
	schemaID := h.EngineMgr.GetCurrentSchemaID()
	rec, err := store.GetFreq(schemaID, "nihao", targetText)
	if err != nil {
		t.Fatalf("GetFreq(%q): %v", schemaID, err)
	}
	if got, want := int(rec.Count), selections; got != want {
		t.Errorf("选 %d 次后 %s 词频 Count = %d, 期望 %d", selections, targetText, got, want)
	}
}

// TestLearningFreqIsolatedPerHarness 验证每个 harness 的词频库相互隔离（独立临时 db）：
// 新建 harness 在未选词前，目标候选不应有遗留词频记录（Count==0），保证用例间确定性。
func TestLearningFreqIsolatedPerHarness(t *testing.T) {
	h := mustHarness(t, "pinyin")
	store := h.DictMgr.GetStore()
	if store == nil {
		t.Fatal("harness store 为 nil")
	}
	schemaID := h.EngineMgr.GetCurrentSchemaID()
	rec, err := store.GetFreq(schemaID, "nihao", "拟好")
	if err != nil {
		t.Fatalf("GetFreq(%q): %v", schemaID, err)
	}
	if rec.Count != 0 {
		t.Errorf("新建 harness 词频应为空，但 拟好 Count = %d", rec.Count)
	}
}

// TestUserDictCustomWordPinyin 验证用户词库自定义词可被查询为候选（用户层经 CompositeDict
// 参与候选生成）：拼音方案加自定义词 你好呀(nihaoya)，输入该编码后应出现该候选并可选中上屏。
func TestUserDictCustomWordPinyin(t *testing.T) {
	h := mustHarness(t, "pinyin")
	if err := h.DictMgr.AddUserWord("nihaoya", "你好呀", 1200); err != nil {
		t.Fatalf("AddUserWord: %v", err)
	}
	rec := NewRecorder(h).
		Type("nihaoya").
		SelectCandidate(1)
	AssertGolden(t, "user_dict_custom_word_pinyin", rec.Render())
}

// TestUserDictCustomWordWubi 验证码表方案用户词库自定义词：wubi86 加自定义词 测试词(aaaa)，
// 输入编码后融合系统候选与用户词，数字 2 选中用户自定义词上屏。
func TestUserDictCustomWordWubi(t *testing.T) {
	h := mustHarness(t, "wubi86")
	if err := h.DictMgr.AddUserWord("aaaa", "测试词", 1200); err != nil {
		t.Fatalf("AddUserWord: %v", err)
	}
	rec := NewRecorder(h).
		Type("aaaa").
		SelectCandidate(2)
	AssertGolden(t, "user_dict_custom_word_wubi", rec.Render())
}
