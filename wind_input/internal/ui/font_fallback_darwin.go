//go:build darwin

package ui

import "github.com/huanfeng/wind_input/pkg/systemfont"

// platformTextFallbackFonts 返回 darwin 上的字形级回退字体路径链 (按优先级)。
//
// 单一主字体 (如 PingFang/Hiragino) 覆盖不全:
//   - 候选窗 UI 标记 (✓ ▸ 箭头等符号) 主字体可能缺 → 退到 Apple Symbols / Helvetica
//   - emoji (五笔 emoji 词库层) 需要 Apple Color Emoji (sbix 彩色位图)
//   - 主字体缺的 CJK 字 → 退到其他中文字体
//
// 经 systemfont.ResolveFile 解析家族名到真实文件 (允许 .ttc); 解析不到的跳过,
// 这样在缺字体的精简镜像 (tart VM) 上也不会塞死路径。
func platformTextFallbackFonts() []string {
	// 顺序: 符号/拉丁 (便宜且 UI 标记常用) → CJK 备选 → emoji。
	candidates := []string{
		"Apple Symbols",     // ✓ ▸ ◂ 等符号
		"Helvetica",         // 拉丁 + 基础符号 (换主字体后回归的那批符号靠它兜底)
		"PingFang SC",       // 全功能 macOS 主力中文 (主字体已是它时被去重)
		"Hiragino Sans GB",  // 冬青黑简
		"STHeiti",           // 华文黑体
		"Songti SC",         // 宋体
		"Apple Color Emoji", // emoji 彩色字形 (sbix)
	}
	out := make([]string, 0, len(candidates))
	for _, fam := range candidates {
		if p := systemfont.ResolveFile(fam, false); p != "" {
			out = append(out, p)
		}
	}
	return out
}
