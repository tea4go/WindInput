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

import "testing"

// TestLearningFreqRecorded 验证重复选词累积词频：拼音方案下选第二候选（拟好）5 次并
// FlushLearning，词频桶中应存在 ("nihao","拟好") 记录且 Count==4（首次不计，误选保护）。
func TestLearningFreqRecorded(t *testing.T) {
	h := mustHarness(t, "pinyin")

	const selections = 5
	for range [selections]struct{}{} {
		h.Type("nihao")
		h.SelectCandidate(2) // 拟好
	}
	h.FlushLearning()

	store := h.DictMgr.GetStore()
	if store == nil {
		t.Fatal("harness store 为 nil")
	}
	schemaID := h.EngineMgr.GetCurrentSchemaID()
	rec, err := store.GetFreq(schemaID, "nihao", "拟好")
	if err != nil {
		t.Fatalf("GetFreq(%q): %v", schemaID, err)
	}
	if got, want := int(rec.Count), selections-1; got != want {
		t.Errorf("选 %d 次后 拟好 词频 Count = %d, 期望 %d（首次选择不计入）", selections, got, want)
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
