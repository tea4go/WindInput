//go:build windows || darwin

// modes_test.go — 四态接管模式的生命周期 golden 回归。
//
// 覆盖触发键/快捷键激活的子模式：快捷输入(quick_input)、临时英文(temp_english)、
// 临时拼音(temp_pinyin)。锚定「进入 → 模式内操作 → 退出/上屏」全链路，重点防回归：
//   - 进入时模式标志位正确置位、preedit/候选切到子模式来源；
//   - 退出（Esc）或上屏后模式标志位清零、缓冲清空（today 的"层清理"回归点）。
//
// 触发条件（均由 config.DefaultConfig 决定，harness 默认装配）：
//   - quick_input：空缓冲按 ';'（Features.QuickInput.TriggerKeys=["semicolon"]）。
//   - temp_english：空缓冲 Shift+大写字母（ShiftTempEnglish.Enabled=true，behavior=temp_english）。
//   - temp_pinyin：码表方案下按 '`'（Input.TempPinyin.TriggerKeys=["backtick"]）；buffer 非空时
//     先顶屏高亮候选再进模式，故用 wubi86 装配。
//
// special 模式未覆盖：shipped schema 未配置 special 实例、且需自定义码表词库 fixture，
// 待补测试数据后另行补充（见 docs/design 与 project_special_mode_codetable 记录）。
package e2e

import "testing"

// TestQuickInputLifecycle 验证快捷输入模式生命周期：拼音方案空缓冲按 ';' 进入
// （quick_input_mode 置位），Esc 退出后模式清零、缓冲清空、回到普通中文态。
func TestQuickInputLifecycle(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Key(";").
		Key("esc")
	AssertGolden(t, "mode_quick_input_lifecycle", rec.Render())
}

// TestTempEnglishLifecycle 验证临时英文模式生命周期：拼音方案空缓冲打大写 "Hello"
// 进入临时英文（temp_english_mode 置位，候选为英文大小写变体），空格上屏首选。
func TestTempEnglishLifecycle(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("Hello").
		Space()
	AssertGolden(t, "mode_temp_english_lifecycle", rec.Render())
}

// TestTempPinyinLifecycle 验证临时拼音模式生命周期：五笔方案输入码 "wg"（有候选），
// 按 '`' 顶屏高亮候选并进入临时拼音（temp_pinyin_mode 置位），打 "hao" 得拼音候选，
// 空格上屏。覆盖"buffer 非空进模式先顶屏"与码表→临时拼音的词库层切换。
func TestTempPinyinLifecycle(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("wg").
		Key("`").
		Type("hao").
		Space()
	AssertGolden(t, "mode_temp_pinyin_lifecycle", rec.Render())
}
