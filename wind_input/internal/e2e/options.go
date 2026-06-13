//go:build windows || darwin

// Package e2e 提供 WindInput 服务端的 in-process 端到端测试 harness：
// 用真实发布的方案/词库文件（build_debug/data）按 cmd/service/main.go 的序列装配
// 引擎 + coordinator（UI 走 headless，不弹窗、不碰 TSF DLL），驱动按键序列并导出
// 完整核心状态快照。既供 go test 的 golden 回归，也供 cmd/e2e-repl 手动验证。
package e2e

import (
	"log/slog"

	"github.com/huanfeng/wind_input/pkg/config"
)

// Options 控制 harness 的装配。零值可用：自动探测 build_debug/data、临时用户目录、
// 丢弃日志、默认 pinyin 方案、不开 user_data.db。
type Options struct {
	// SchemaID 要激活的方案 ID（"pinyin"/"wubi86"/"shuangpin"/"wubi86_pinyin"）。空 = "pinyin"。
	SchemaID string
	// DataRoot 含 schemas/ 的运行时数据目录（对应 main.go 的 dataRoot）。
	// 空 = 自动向上探测 build_debug/data。
	DataRoot string
	// DataDir 用户数据目录（user_data.db / 运行时状态写入处）。空 = 新建临时目录。
	DataDir string
	// Logger 日志器。nil = slog.New(slog.DiscardHandler)。
	Logger *slog.Logger
	// SpecialModes 注入引导键特殊模式实例（自定义码表）。非空时 BuildHarness 在装配后
	// 用 SpecialSchemasDir 解析码表并重建特殊模式注册表。
	SpecialModes []config.SpecialModeConfig
	// SpecialSchemasDir 特殊模式码表 fixture 的搜索目录（含 SpecialModes[].Table 文件）。
	// 仅 SpecialModes 非空时生效；空 = "testdata"。
	SpecialSchemasDir string
}

func (o Options) schemaID() string {
	if o.SchemaID == "" {
		return "pinyin"
	}
	return o.SchemaID
}

func (o Options) logger() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slog.New(slog.DiscardHandler)
}
