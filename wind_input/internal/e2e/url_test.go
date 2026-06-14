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

// TestUrlModeBackspaceExit 验证退格删空退出：进入后逐字删空前缀应退出 URL 模式。
func TestUrlModeBackspaceExit(t *testing.T) {
	h := urlHarness(t)
	rec := NewRecorder(h).
		Type("ftp.").
		Backspace().
		Backspace().
		Backspace().
		Backspace()
	AssertGolden(t, "mode_url_backspace_exit", rec.Render())
}
