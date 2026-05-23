package dict

import (
	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// decodePhraseEscapes 对纯字面短语文本解码转义序列 (\n \r \t \\)。
//
// 含 cmdbar / 字符组 marker ($CC( $CC1( $SS( $AA() 的文本原样返回:
// marker 内字符串字面量的转义由各自 parser (cmdbar lexer / aa_marker)
// 处理, 不能在此处提前解码, 否则会破坏 marker 语法。
//
// 详见 docs/design/command-bar-escape-support.md §3.3。
func decodePhraseEscapes(text string) string {
	// 守卫顺序: 与 store_layer.go 等包内既有 marker 守卫保持一致写法,
	// 用前缀谓词识别顶层 marker 短语。
	if HasAAMarker(text) || HasSSMarker(text) || HasCmdbarMarker(text) {
		return text
	}
	return cmdbar.DecodeEscapes(text)
}
