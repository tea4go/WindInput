//go:build windows || darwin

package e2e

import (
	"log/slog"
	"os"
	"testing"
)

// TestMain 在测试进程启动时把全局默认 logger 静音。
//
// 装配链路里部分组件（如 ui 的 DirectWrite 渲染器）通过全局 slog.Default() 打 INFO 日志，
// 不走 harness 注入的 discard logger；这些日志直接写进程 stderr，go test 不缓冲、无论
// 加不加 -v 都会原样输出，既污染结果又浪费阅读成本。这里统一静音，让测试输出只剩
// PASS/FAIL 摘要（harness 自身日志已默认 DiscardHandler）。
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}
