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

// 注：z 键混合回退（zHybridFallback）的「首次退格撤销夺取」由 coordinator 的 z_rewind_test
// 单测覆盖（直接 decider.armRewind + 驱动 HandleKeyEvent，不依赖真实 wubi 前缀结构能否构造出
// fallback 场景）；统一回退机制的端到端已由 mode_url_rewind golden 覆盖。两者共用同一
// decider.armRewind/rewindHijack 路径，故此处不再单设 z fallback 回退 golden。

// TestTempPinyinPaging 覆盖 temp_pinyin 的候选导航——验证 KeyHandler 链分解后导航键经
// 链上 navKeyHandler 分发（决策器开），与旧 handlePinyinModeKey switch（决策器关）逐字节
// 等价（A/B 经 WIND_E2E_DECIDER=1 验证）。backtick 进临时拼音、shi 得多候选；PageDown/PageUp
// 走翻页路径，方向下/上键走高亮移动（页内 selectedIndex 真状态变化）。
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
		PageUp().
		Key("down").
		Key("down").
		Key("up")
	AssertGolden(t, "mode_temp_pinyin_paging", rec.Render())
}
