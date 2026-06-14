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

// ── 基础中英文切换（核心模式，非接管子模式）────────────────────────────────────

// TestModeChineseEnglishToggle 验证 lshift 切换中英文：拼音方案下按 lshift 切到英文
// （chinese_mode=false，字母透传不消费），再按 lshift 切回中文，打 "ni" 恢复候选输入。
func TestModeChineseEnglishToggle(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Key("lshift").
		Type("a").
		Key("lshift").
		Type("ni")
	AssertGolden(t, "mode_chinese_english_toggle", rec.Render())
}

// ── quick_input 补充场景 ──────────────────────────────────────────────────────

// TestQuickInputCalculator 验证快捷输入计算器：';' 进入，打算式 "1+2"，空格上屏首选
// 计算结果 "1+2=3"。
func TestQuickInputCalculator(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Key(";").
		Type("1+2").
		Space()
	AssertGolden(t, "mode_quick_input_calculator", rec.Render())
}

// TestQuickInputPinyinSubmode 验证快捷输入拼音子模式：';' 进入后打字母 "rq" 进入拼音
// 子模式得候选，空格上屏首选 "人群"。
func TestQuickInputPinyinSubmode(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Key(";").
		Type("rq").
		Space()
	AssertGolden(t, "mode_quick_input_pinyin_submode", rec.Render())
}

// TestQuickInputPinyinHighlightNav 验证快捷输入拼音上下文的高亮导航——KeyHandler 链分解后
// 导航键经链上 navKeyHandler（quick_input.pinyin.nav，标准翻页谓词 + showPinyinModeUI）分发，
// 与旧 handlePinyinModeKey switch 逐字节等价（A/B 经 WIND_E2E_DECIDER=1 验证）。';' 进入打
// "shi" 得拼音候选，方向下键高亮下移、上键回移。
func TestQuickInputPinyinHighlightNav(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Key(";").
		Type("shi").
		Key("down").
		Key("down").
		Key("up")
	AssertGolden(t, "mode_quick_input_pinyin_highlight", rec.Render())
}

// TestQuickInputBaseHighlightNav 验证快捷输入基础上下文的高亮导航——导航键经链上 navKeyHandler
// （quick_input.base.nav，专用翻页谓词 isQuickInputPageUpKey + showQuickInputUI）分发，与旧
// handleQuickInputKey switch 逐字节等价。';' 进入打 "123" 得数字读法多候选，方向键高亮移动。
func TestQuickInputBaseHighlightNav(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Key(";").
		Type("123").
		Key("down").
		Key("up")
	AssertGolden(t, "mode_quick_input_base_highlight", rec.Render())
}

// ── temp_english 补充场景 ─────────────────────────────────────────────────────

// TestTempEnglishDigitSelect 验证临时英文数字选词：打 "Hello" 进入，数字 2 选第二候选
// （小写变体 "hello"）上屏。
func TestTempEnglishDigitSelect(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("Hello").
		SelectCandidate(2)
	AssertGolden(t, "mode_temp_english_digit_select", rec.Render())
}

// TestTempEnglishEnterRaw 验证临时英文回车上屏原文：打 "Hello" 进入，回车上屏缓冲原文
// "Hello"（而非候选）。
func TestTempEnglishEnterRaw(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("Hello").
		Enter()
	AssertGolden(t, "mode_temp_english_enter_raw", rec.Render())
}

// TestTempEnglishBackspace 验证临时英文退格编辑：打 "Hello" 进入，退格回 "Hell"，候选
// 按截短后的输入重新派生，仍在临时英文模式。
func TestTempEnglishBackspace(t *testing.T) {
	h := mustHarness(t, "pinyin")
	rec := NewRecorder(h).
		Type("Hello").
		Backspace()
	AssertGolden(t, "mode_temp_english_backspace", rec.Render())
}

// ── temp_pinyin 补充场景 ──────────────────────────────────────────────────────

// TestTempPinyinEnterRaw 验证临时拼音回车上屏原文：wg+'`' 进入后打 "hao"，回车上屏拼音
// 原文 "hao"（而非候选 好）。
func TestTempPinyinEnterRaw(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("wg").
		Key("`").
		Type("hao").
		Enter()
	AssertGolden(t, "mode_temp_pinyin_enter_raw", rec.Render())
}

// TestTempPinyinDigitSelect 验证临时拼音数字选词：wg+'`' 进入后打 "de"（多候选），数字 2
// 选第二候选上屏。
func TestTempPinyinDigitSelect(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("wg").
		Key("`").
		Type("de").
		SelectCandidate(2)
	AssertGolden(t, "mode_temp_pinyin_digit_select", rec.Render())
}

// TestTempPinyinEscExit 验证临时拼音 ESC 退出：wg+'`' 进入后打 "hao"，ESC 清空退出、
// 不上屏，回到普通中文态。
func TestTempPinyinEscExit(t *testing.T) {
	h := mustHarness(t, "wubi86")
	rec := NewRecorder(h).
		Type("wg").
		Key("`").
		Type("hao").
		Key("esc")
	AssertGolden(t, "mode_temp_pinyin_esc_exit", rec.Render())
}
