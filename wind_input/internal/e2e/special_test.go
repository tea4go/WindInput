//go:build windows || darwin

// special_test.go — 引导键特殊模式（自定义码表）的多场景 golden 回归。
//
// 特殊模式由 schema/config 配 special 实例触发，shipped 方案默认无配置，故这里经
// Options.SpecialModes 注入 fixture 实例 + testdata/e2e_special.dict.yaml 码表
// （ConfigureSpecialModes 重建注册表）。码表内容（by_order）：
//
//	a → ★ / ☆ / ●     （三候选，且有更长码 aa → 数字/空格选词场景）
//	aa → ◆            （唯一且无更长码 → prefix_free 可自动上屏）
//	b → ▲             （唯一且无更长码 → prefix_free 可自动上屏）
//
// 用 pinyin 方案装配：grave('`') 在拼音下不与 temp_pinyin（仅码表引擎）触发键冲突。
// 多数用例用 AutoCommit=manual 以完全控制每步；另设 prefix_free 用例验证自动上屏。
package e2e

import (
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
)

// specialModeManual 返回 manual 自动上屏档的 fixture 实例（grave 触发）。
func specialModeManual() config.SpecialModeConfig {
	return config.SpecialModeConfig{
		ID:          "e2e_sym",
		Name:        "测试快符",
		TriggerKeys: []string{"grave"},
		Table:       "e2e_special.dict.yaml",
		AutoCommit:  config.SpecialAutoCommitManual,
	}
}

// specialModePrefixFree 返回 prefix_free 自动上屏档的 fixture 实例（grave 触发）。
func specialModePrefixFree() config.SpecialModeConfig {
	cfg := specialModeManual()
	cfg.ID = "e2e_pf"
	cfg.AutoCommit = config.SpecialAutoCommitPrefixFree
	return cfg
}

// mustSpecialHarness 用 pinyin 方案 + 注入的 special 实例装配 harness。
func mustSpecialHarness(t *testing.T, mode config.SpecialModeConfig) *Harness {
	t.Helper()
	h, err := BuildHarness(Options{
		SchemaID:     "pinyin",
		SpecialModes: []config.SpecialModeConfig{mode},
	})
	if err != nil {
		t.Fatalf("BuildHarness(special): %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

// TestSpecialModeDigitSelect 验证字母输入 + 数字键选词：grave 进入特殊模式，打 "a"
// 得三候选 ★/☆/●，按数字 2 选 ☆ 上屏并退出模式。
func TestSpecialModeDigitSelect(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		SelectCandidate(2)
	AssertGolden(t, "special_digit_select", rec.Render())
}

// TestSpecialModeSpaceCommit 验证空格选高亮：grave 进入，打 "a"，空格上屏当前高亮
// 首候选 ★ 并退出。
func TestSpecialModeSpaceCommit(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		Space()
	AssertGolden(t, "special_space_commit", rec.Render())
}

// TestSpecialModeEnterRaw 验证回车上屏原文：grave 进入，打 "a"，回车上屏编码原文
// "a"（而非候选 ★），退出模式。
func TestSpecialModeEnterRaw(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		Enter()
	AssertGolden(t, "special_enter_raw", rec.Render())
}

// TestSpecialModeSymbolCommit 验证符号顶屏：grave 进入，打 "a"，按逗号 ',' 顶屏高亮
// 候选 ★ 并追加转换后的中文标点（"，"），退出模式。
func TestSpecialModeSymbolCommit(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		Type(",")
	AssertGolden(t, "special_symbol_commit", rec.Render())
}

// TestSpecialModeEscExit 验证 ESC 退出：grave 进入，打 "a"，ESC 清空退出、不上屏。
func TestSpecialModeEscExit(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		Key("esc")
	AssertGolden(t, "special_esc_exit", rec.Render())
}

// TestSpecialModeBackspace 验证退格编辑：grave 进入，打 "aa"（候选 ◆），退格回 "a"
// （候选 ★/☆/●），再退格到空，第三次退格退出模式。
func TestSpecialModeBackspace(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("aa").
		Backspace().
		Backspace().
		Backspace()
	AssertGolden(t, "special_backspace", rec.Render())
}

// TestSpecialModeContinuous 验证连续输入：grave 进入打 "a" 选 1（★ 上屏退出），
// 再次 grave 进入打 "b" 空格上屏 ▲。覆盖上屏后再进入的状态复位。
func TestSpecialModeContinuous(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		SelectCandidate(1).
		Key("`").
		Type("b").
		Space()
	AssertGolden(t, "special_continuous", rec.Render())
}

// TestSpecialModeAutoCommitPrefixFree 验证 prefix_free 自动上屏：grave 进入，打 "b"
// （唯一候选 ▲ 且无更长码）应立即自动上屏并退出，无需空格/回车。
func TestSpecialModeAutoCommitPrefixFree(t *testing.T) {
	h := mustSpecialHarness(t, specialModePrefixFree())
	rec := NewRecorder(h).
		Key("`").
		Type("b")
	AssertGolden(t, "special_autocommit_prefix_free", rec.Render())
}

// TestSpecialModeHighlightNav 验证 special 的候选高亮导航——KeyHandler 链分解后导航键经链上
// 复用的 navKeyHandler 分发（决策器开），与旧 handleSpecialModeKey switch（决策器关）逐字节
// 等价（A/B 经 WIND_E2E_DECIDER=1 验证）。grave 进入打 "a" 得三候选 ★/☆/●（selectedIndex=0），
// 方向下键高亮下移到 1、2，方向上键回到 1（页内移动，真状态变化）。
func TestSpecialModeHighlightNav(t *testing.T) {
	h := mustSpecialHarness(t, specialModeManual())
	rec := NewRecorder(h).
		Key("`").
		Type("a").
		Key("down").
		Key("down").
		Key("up")
	AssertGolden(t, "special_highlight_nav", rec.Render())
}
