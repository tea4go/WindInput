//go:build windows || darwin

package e2e

import "testing"

// mustHarness 用真实发布数据装配一个 harness，并登记测试结束时清理。
// 首次会同步生成方案词库缓存（码表 wdb / 拼音 wdat），可能耗时数秒。
func mustHarness(t *testing.T, schemaID string) *Harness {
	t.Helper()
	h, err := BuildHarness(Options{SchemaID: schemaID})
	if err != nil {
		t.Fatalf("BuildHarness(%q): %v", schemaID, err)
	}
	t.Cleanup(h.Close)
	return h
}

// TestPunctuationChineseBasic 验证中文标点模式下、无输入态直接按标点键的转换上屏：
// 逗号/句号/问号/感叹号/分号/冒号/反斜杠 应转为对应中文标点。
// 标点处理在引擎之前、与具体方案无关，这里用码表方案 wubi86 装配（缓存生成较快）。
func TestPunctuationChineseBasic(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type(",").
		Type(".").
		Type("?").
		Type("!").
		Type(";").
		Type(":").
		Type("\\")
	AssertGolden(t, "punct_chinese_basic", rec.Render())
}

// TestPunctuationPairedQuotes 验证成对标点的左右交替状态机：
// 连续双引号应交替输出 “ / ”，单引号交替 ‘ / ’，括号交替 （ / ）。
func TestPunctuationPairedQuotes(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("\"").
		Type("\"").
		Type("'").
		Type("'").
		Type("(").
		Type(")")
	AssertGolden(t, "punct_paired_quotes", rec.Render())
}

// TestPunctuationDigitFollowed 验证「数字后标点」智能转换：中文标点模式下，
// 紧跟数字输入的句号/逗号应保持英文半角（避免 "3.14" 变 "3。14"）。
func TestPunctuationDigitFollowed(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("3").
		Type(".").
		Type("1").
		Type("4")
	AssertGolden(t, "punct_digit_followed", rec.Render())
}
