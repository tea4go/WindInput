//go:build windows || darwin

package e2e

import "testing"

// TestTempPinyinZFallback 覆盖决策器的 z 键混合回退路径（judgeZFallback / CompHot）：
// wubi86 码表 + z 配为临时拼音触发键，打 z 后接拼音；z+码不是 wubi 前缀时回退进临时拼音
// （buffer 去掉首字母 z）。决策器开/关下应逐字节等价（A/B 经 WIND_E2E_DECIDER=1 验证）。
func TestTempPinyinZFallback(t *testing.T) {
	h, err := BuildHarness(Options{
		SchemaID:              "wubi86",
		TempPinyinTriggerKeys: []string{"backtick", "z"},
	})
	if err != nil {
		t.Fatalf("BuildHarness: %v", err)
	}
	defer h.Close()
	rec := NewRecorder(h).
		Type("znihao").
		Space()
	AssertGolden(t, "mode_temp_pinyin_z_fallback", rec.Render())
}

// TestTempPinyinPaging 覆盖 temp_pinyin 的候选翻页——验证 KeyHandler 链分解后导航键经
// 链上 pinyinNavKeyHandler 分发（决策器开），与旧 handlePinyinModeKey switch（决策器关）
// 逐字节等价（A/B 经 WIND_E2E_DECIDER=1 验证）。backtick 进临时拼音，shi 候选跨多页，
// PageDown 进下一页、PageUp 回上一页。
func TestTempPinyinPaging(t *testing.T) {
	h, err := BuildHarness(Options{
		SchemaID:              "wubi86",
		TempPinyinTriggerKeys: []string{"backtick", "z"},
	})
	if err != nil {
		t.Fatalf("BuildHarness: %v", err)
	}
	defer h.Close()
	rec := NewRecorder(h).
		Key("`").
		Type("shi").
		PageDown().
		PageUp()
	AssertGolden(t, "mode_temp_pinyin_paging", rec.Render())
}
