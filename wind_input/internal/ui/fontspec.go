package ui

import (
	"path/filepath"
	"strings"
)

// fontspec.go — 平台无关的字体名称归一化辅助。原在 gdi_text.go (windows tag) 里,
// 抽出来让 font_config.go 等跨平台调用方能在 darwin 也用上。
// 表内键 (msyh.ttc/segoeui.ttf 等) 是 Windows 字体文件名, 在 darwin 上不会命中,
// 直接走 fallback "Microsoft YaHei" — darwin 调用方再走 systemfont.ResolveFile 时
// 自然会失败并回退到 darwin 字体, 语义安全。

var knownFontNames = map[string]string{
	"simhei.ttf":   "SimHei",
	"simsun.ttf":   "SimSun",
	"simsun.ttc":   "SimSun",
	"msyh.ttf":     "Microsoft YaHei",
	"msyh.ttc":     "Microsoft YaHei",
	"msyhbd.ttf":   "Microsoft YaHei",
	"msyhbd.ttc":   "Microsoft YaHei",
	"arial.ttf":    "Arial",
	"segoeui.ttf":  "Segoe UI",
	"seguisym.ttf": "Segoe UI Symbol",
	"segmdl2.ttf":  "Segoe MDL2 Assets",
}

// FontSpecToName converts a configured font spec to a GDI/DirectWrite family name.
// The preferred input is a system font family name. File-path handling remains as
// a narrow fallback for internal resolution.
func FontSpecToName(fontSpec string) string {
	fontSpec = strings.TrimSpace(fontSpec)
	if fontSpec == "" {
		return "Microsoft YaHei"
	}
	if !strings.ContainsAny(fontSpec, `\/`) {
		return fontSpec
	}
	base := strings.ToLower(filepath.Base(fontSpec))
	if name, ok := knownFontNames[base]; ok {
		return name
	}
	return "Microsoft YaHei"
}
