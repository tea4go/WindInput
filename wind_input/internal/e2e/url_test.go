//go:build windows || darwin

// url_test.go — URL 临时输入模式的 golden 回归。
//
// URL 模式默认关闭，经 Options.Configure 开启（前缀用 DefaultConfig 的 www./http/https/ftp.）。
// 用 pinyin 方案：正常输入下 inputBuffer 累积前缀字符，恰好等于某前缀时夺取进入。
package e2e

import (
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
)

func urlHarness(t *testing.T) *Harness {
	t.Helper()
	h, err := BuildHarness(Options{
		SchemaID: "pinyin",
		Configure: func(cfg *config.Config) {
			cfg.Input.UrlInput.Enabled = true // 前缀默认 www./http/https/ftp.
		},
	})
	if err != nil {
		t.Fatalf("BuildHarness(url): %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

// TestUrlModeHttp 验证字母结尾前缀（http）夺取：打 "http"（第 4 键 p 完成前缀）进入 URL 模式、
// buffer="http"，续打 "://a.com" 自由输入不上屏，空格上屏完整 URL。
func TestUrlModeHttp(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("http").
		Type("://a.com").
		Space()
	AssertGolden(t, "mode_url_http", rec.Render())
}

// TestUrlModeWww 验证点结尾前缀（www.）夺取：打 "www."（'.' 完成前缀，须在标点处理前拦截）
// 进入 URL 模式，续打 "baidu.com"，回车上屏。
func TestUrlModeWww(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("www.").
		Type("baidu.com").
		Enter()
	AssertGolden(t, "mode_url_www", rec.Render())
}

// TestUrlModeEscExit 验证 ESC 放弃退出（不上屏，回正常态）。
func TestUrlModeEscExit(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("http").
		Key("esc")
	AssertGolden(t, "mode_url_esc", rec.Render())
}

// TestUrlModeRewind 验证「夺取后首次退格撤销夺取」（统一回退机制）：打 "http" 夺取进 URL 模式后，
// 未编辑时第一次退格撤销夺取、回到正常输入流 "htt"（而非在 URL buffer 内删字符）。
func TestUrlModeRewind(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("http").
		Backspace()
	AssertGolden(t, "mode_url_rewind", rec.Render())
}

// TestUrlModeRewindBackToPrefix 验证多键夺取的回退（编辑后退格删回前缀边界仍回退）：打 "http"
// 夺取后续打 "x"（编辑——URL 是多键夺取，**不**作废回退登记），退格删回 "http"（前缀边界），再退格
// → 撤销夺取、回正常输入流 "htt"（前缀本是被夺取的正常输入，删进它即撤销；而非在 URL 删到空）。
func TestUrlModeRewindBackToPrefix(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("http"). // 进 URL，buffer=http
		Type("x").    // 编辑 → httpx
		Backspace().  // → http（删回前缀边界，尚未回退）
		Backspace()   // → 撤销夺取，回正常输入 htt
	AssertGolden(t, "mode_url_rewind_back_to_prefix", rec.Render())
}

// TestUrlModeRewindAfterUrlContent 复现真机场景：打 "http" 后续打 "://"（完整 http://），退格逐字
// 删回 "http" 前缀边界，再一退格撤销夺取回正常输入流（覆盖「续打网址内容后仍能退回五笔」）。
func TestUrlModeRewindAfterUrlContent(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("http"). // 进 URL，buffer=http
		Type("://").  // → http://
		Backspace().  // → http:/
		Backspace().  // → http:
		Backspace().  // → http（前缀边界）
		Backspace()   // → 撤销夺取，回正常输入 htt
	AssertGolden(t, "mode_url_rewind_after_content", rec.Render())
}
