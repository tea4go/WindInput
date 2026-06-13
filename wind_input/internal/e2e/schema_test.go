//go:build windows || darwin

// schema_test.go — 双拼(shuangpin) 与 混输(wubi86_pinyin) 方案的核心转换 golden。
//
// 补齐 pinyin/wubi86 之外两个主力方案的转换路径覆盖：
//   - shuangpin：每音节 2 键，preedit 体现双拼分段（与全拼逐字母分段不同）。
//   - wubi86_pinyin（mixed）：码表候选与拼音候选融合（engine_name=mixed）。
//
// 用真实发布数据，候选集/顺序由系统词库决定，结果确定；weight 默认 mask。
package e2e

import "testing"

// TestShuangpinSingleSyllable 验证双拼单音节转换：输入 "wo"（w+o）得候选 我，数字 1 选首选上屏。
func TestShuangpinSingleSyllable(t *testing.T) {
	h := mustHarness(t, "shuangpin")
	rec := NewRecorder(h).
		Type("wo").
		SelectCandidate(1)
	AssertGolden(t, "shuangpin_single_syllable", rec.Render())
}

// TestShuangpinTwoSyllable 验证双拼双音节分段与分步确认：输入 "nihk"（ni+hk）preedit 分段
// 为 "ni hk"（双拼每音节固定 2 键），空格确认首音节段（你）、余 "hk" 继续，体现双拼下空格
// 走分步确认而非整词上屏。
func TestShuangpinTwoSyllable(t *testing.T) {
	h := mustHarness(t, "shuangpin")
	rec := NewRecorder(h).
		Type("nihk").
		Space()
	AssertGolden(t, "shuangpin_two_syllable", rec.Render())
}

// TestMixedFusionConvert 验证混输方案码表+拼音融合：输入 "wgkg" 得融合候选（码表字 +
// 拼音字），engine_name=mixed，数字 2 选第二候选上屏。
func TestMixedFusionConvert(t *testing.T) {
	h := mustHarness(t, "wubi86_pinyin")
	rec := NewRecorder(h).
		Type("wgkg").
		SelectCandidate(2)
	AssertGolden(t, "mixed_fusion_convert", rec.Render())
}

// TestMixedSpaceCommit 验证混输方案空格上屏首选：输入 "wgkg" 后空格上屏融合候选首选。
func TestMixedSpaceCommit(t *testing.T) {
	h := mustHarness(t, "wubi86_pinyin")
	rec := NewRecorder(h).
		Type("wgkg").
		Space()
	AssertGolden(t, "mixed_space_commit", rec.Render())
}
