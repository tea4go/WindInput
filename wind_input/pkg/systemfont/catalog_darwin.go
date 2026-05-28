//go:build darwin

package systemfont

import (
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// catalog_darwin.go — darwin 版字体目录: 扫描系统/用户字体目录, 复用 nametable.go
// 解 sfnt name table 拿 family 名。无 DirectWrite/Registry, 因此实现远比 Win 版简单:
//   - 没有"localized 异步解析"两阶段, 一次扫描完成
//   - 没有 dwFamilyMap (CoreText 直接吃 nameID=1, 同 ResolveFile 命中的 key)
//   - ResolveDWFamily 返回 displayNames 即可

type FontInfo struct {
	Family      string `json:"family"`
	DisplayName string `json:"display_name"`
}

type catalog struct {
	fonts        []FontInfo
	families     map[string][]string // normalizeKey(family) -> []paths
	displayNames map[string]string   // normalizeKey -> family display name
}

var (
	catalogOnce sync.Once
	cached      catalog
	cachedErr   error
)

// macOS 字体安装位置 (按优先级低→高扫描, 后扫的会覆盖 displayName 不覆盖路径列表):
//
//	/System/Library/Fonts             — Apple 内置只读
//	/System/Library/Fonts/Supplemental — Apple 附赠 (Arial / Songti SC 等)
//	/Library/Fonts                    — 机器范围用户安装
//	~/Library/Fonts                   — 当前用户安装
var fontDirs = []string{
	"/System/Library/Fonts",
	"/System/Library/Fonts/Supplemental",
	"/Library/Fonts",
}

// assetsV2Root: macOS Tahoe (26+) 把大量系统字体 (PingFang/SF Pro/Apple Color Emoji
// 等) 从 /System/Library/Fonts 迁到这里, 按 SHA hash 散在 .asset/AssetData/*.ttc 下。
// 递归扫描这一根目录, 限深度避免 walk 整个 AssetsV2 (里面还有非字体 asset)。
const assetsV2Root = "/System/Library/AssetsV2"

func userFontDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Fonts")
	}
	return ""
}

// fallbackFamilies — 当扫描结果为空时的兜底 (Sandbox / 权限异常等极端情况)。
// PingFang 是 macOS 10.11+ 默认中文字体, .AppleSystemUIFont 是 SF Pro 系统字体别名。
var fallbackFamilies = []FontInfo{
	{Family: "PingFang SC", DisplayName: "PingFang SC"},
	{Family: "PingFang TC", DisplayName: "PingFang TC"},
	{Family: "Helvetica", DisplayName: "Helvetica"},
	{Family: "Apple Color Emoji", DisplayName: "Apple Color Emoji"},
	{Family: ".AppleSystemUIFont", DisplayName: "Apple System"},
}

func normalizeKey(v string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(v)), " "))
}

func appendUniquePath(paths []string, path string) []string {
	key := normalizeKey(path)
	for _, e := range paths {
		if normalizeKey(e) == key {
			return paths
		}
	}
	return append(paths, path)
}

func isFontFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

func isSingleFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf":
		return true
	}
	return false
}

func ingestFontFile(path string, cat *catalog) {
	tables := readAllNameTables(path)
	if len(tables) == 0 {
		return
	}
	for _, data := range tables {
		names := parseFamilyNamesAnyPlatform(data)
		if len(names) == 0 {
			continue
		}
		display := names[0]
		if zh := parseChineseFamilyName(data); zh != "" {
			display = zh
		}
		for _, name := range names {
			key := normalizeKey(name)
			cat.families[key] = appendUniquePath(cat.families[key], path)
			if _, ok := cat.displayNames[key]; !ok {
				cat.displayNames[key] = name
			}
		}
		dkey := normalizeKey(display)
		if _, ok := cat.displayNames[dkey]; !ok {
			cat.displayNames[dkey] = display
			cat.families[dkey] = appendUniquePath(cat.families[dkey], path)
		}
	}
}

func walkFontDir(dir string, cat *catalog) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !isFontFile(e.Name()) {
			continue
		}
		ingestFontFile(filepath.Join(dir, e.Name()), cat)
	}
}

// walkAssetsV2 递归扫描 Tahoe AssetsV2 字体目录。WalkDir 在不存在目录上返回 nil
// (无 entries), 因此 Big Sur/Monterey/Sequoia 等旧系统调用此函数也安全 no-op。
func walkAssetsV2(cat *catalog) {
	_ = filepath.WalkDir(assetsV2Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// 单条 entry 错误 (权限/坏符号链接) 不打断整树遍历
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isFontFile(d.Name()) {
			ingestFontFile(path, cat)
		}
		return nil
	})
}

// readAllNameTables 读一份字体文件的所有 name table 字节。
// 单字体 (TTF/OTF) 返回 1 元素切片; TTC/OTC 返回 N 元素 (每个子字体各一份)。
// 与 nametable.go 的 readNameTableData (只取 TTC 第一个子字体) 区别在于此函数枚举所有子字体,
// 这对 macOS PingFang/Helvetica/Apple Color Emoji 等 TTC 集合至关重要 (主家族在 [1..n-1])。
func readAllNameTables(fontPath string) [][]byte {
	f, err := os.Open(fontPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var tag [4]byte
	if _, err := f.Read(tag[:]); err != nil {
		return nil
	}

	if string(tag[:]) == "ttcf" {
		// TTC: ttcf(4) + major(2) + minor(2) + numFonts(4) + offsets[numFonts]
		var hdr [8]byte
		if _, err := f.Read(hdr[:]); err != nil {
			return nil
		}
		numFonts := binary.BigEndian.Uint32(hdr[4:8])
		if numFonts == 0 || numFonts > 256 {
			return nil
		}
		offsets := make([]uint32, numFonts)
		for i := range offsets {
			var b [4]byte
			if _, err := f.Read(b[:]); err != nil {
				return nil
			}
			offsets[i] = binary.BigEndian.Uint32(b[:])
		}
		out := make([][]byte, 0, numFonts)
		for _, off := range offsets {
			if _, err := f.Seek(int64(off), 0); err != nil {
				continue
			}
			if data := readSFNTNameTable(f); data != nil {
				out = append(out, data)
			}
		}
		return out
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}
	if data := readSFNTNameTable(f); data != nil {
		return [][]byte{data}
	}
	return nil
}

// readSFNTNameTable 从 *os.File 当前位置开始解 SFNT (TTF/OTF) header, 返回 name table 字节。
func readSFNTNameTable(f *os.File) []byte {
	var hdr [12]byte
	if _, err := f.Read(hdr[:]); err != nil {
		return nil
	}
	numTables := binary.BigEndian.Uint16(hdr[4:6])
	var nameOff, nameLen uint32
	for i := 0; i < int(numTables); i++ {
		var rec [16]byte
		if _, err := f.Read(rec[:]); err != nil {
			return nil
		}
		if string(rec[:4]) == "name" {
			nameOff = binary.BigEndian.Uint32(rec[8:12])
			nameLen = binary.BigEndian.Uint32(rec[12:16])
			break
		}
	}
	if nameOff == 0 || nameLen == 0 {
		return nil
	}
	if nameLen > 64*1024 {
		nameLen = 64 * 1024
	}
	if _, err := f.Seek(int64(nameOff), 0); err != nil {
		return nil
	}
	data := make([]byte, nameLen)
	if _, err := f.Read(data); err != nil {
		return nil
	}
	return data
}

// parseFamilyNamesAnyPlatform 是 darwin 版 parseAllFamilyNames: 同时接受
// platformID=3 (Windows Unicode BMP), 0 (Apple Unicode), 1 (Macintosh Roman)。
// macOS 系统字体三种 platform 都可能出现:
//   - Helvetica/Times/Courier 等老 Apple 字体: 仅 platformID=1 + Mac Roman
//   - SF Pro / Apple Color Emoji 等新字体: platformID=0 (Apple Unicode)
//   - 第三方/字体厂商打包字体: platformID=3 (Windows)
func parseFamilyNamesAnyPlatform(data []byte) []string {
	if len(data) < 6 {
		return nil
	}
	count := binary.BigEndian.Uint16(data[2:4])
	stringOffset := binary.BigEndian.Uint16(data[4:6])

	seen := make(map[string]struct{})
	var names []string

	for i := 0; i < int(count); i++ {
		off := 6 + i*12
		if off+12 > len(data) {
			break
		}
		platformID := binary.BigEndian.Uint16(data[off:])
		encodingID := binary.BigEndian.Uint16(data[off+2:])
		nameID := binary.BigEndian.Uint16(data[off+6:])
		length := binary.BigEndian.Uint16(data[off+8:])
		strOff := binary.BigEndian.Uint16(data[off+10:])

		if nameID != 1 {
			continue
		}
		start := int(stringOffset) + int(strOff)
		end := start + int(length)
		if end > len(data) || length == 0 {
			continue
		}
		raw := data[start:end]

		var s string
		switch {
		case platformID == 3 && encodingID == 1:
			s = decodeUTF16BE(raw)
		case platformID == 0:
			s = decodeUTF16BE(raw)
		case platformID == 1 && encodingID == 0:
			// Mac Roman — Latin family names (Helvetica, Times) 走 ASCII 子集即可。
			// 非 ASCII Mac Roman 字符 (rare for family names) 暂忽略。
			s = decodeMacRomanASCII(raw)
		default:
			continue
		}
		if s == "" {
			continue
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			names = append(names, s)
		}
	}
	return names
}

// decodeMacRomanASCII 将 Mac Roman 字节按 ASCII 子集解码 (字节 < 0x80 直接当 ASCII,
// >= 0x80 跳过)。font family 名 99% 在 ASCII 范围内, 不在的极少数 Mac Roman 字符也无 IME
// 用户场景, 简化处理即可。
func decodeMacRomanASCII(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c < 0x80 {
			out = append(out, c)
		}
	}
	return string(out)
}

func ensureCatalog() error {
	catalogOnce.Do(func() {
		cached = catalog{
			families:     make(map[string][]string),
			displayNames: make(map[string]string),
		}
		for _, dir := range fontDirs {
			walkFontDir(dir, &cached)
		}
		if user := userFontDir(); user != "" {
			walkFontDir(user, &cached)
		}
		walkAssetsV2(&cached)

		seen := make(map[string]struct{})
		for key, paths := range cached.families {
			if len(paths) == 0 {
				continue
			}
			name := cached.displayNames[key]
			if name == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			cached.fonts = append(cached.fonts, FontInfo{Family: name, DisplayName: name})
		}

		if len(cached.fonts) == 0 {
			for _, f := range fallbackFamilies {
				cached.displayNames[normalizeKey(f.Family)] = f.Family
				cached.fonts = append(cached.fonts, f)
			}
		}

		sort.Slice(cached.fonts, func(i, j int) bool {
			return strings.ToLower(cached.fonts[i].DisplayName) < strings.ToLower(cached.fonts[j].DisplayName)
		})
	})
	return cachedErr
}

// List returns installed system font families.
func List() ([]FontInfo, error) {
	err := ensureCatalog()
	out := make([]FontInfo, len(cached.fonts))
	copy(out, cached.fonts)
	return out, err
}

// HasFamily reports whether the family exists in the catalog.
func HasFamily(family string) bool {
	_ = ensureCatalog()
	_, ok := cached.families[normalizeKey(family)]
	return ok
}

func firstAvailable(paths []string, singleFontOnly bool) string {
	for _, p := range paths {
		if singleFontOnly && !isSingleFontFile(p) {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ResolveFile returns a font file path for the given family.
// singleFontOnly=true 跳过 .ttc/.otc 集合, 仅返回 .ttf/.otf 单字体文件。
func ResolveFile(family string, singleFontOnly bool) string {
	_ = ensureCatalog()
	return firstAvailable(cached.families[normalizeKey(family)], singleFontOnly)
}

// ResolveDWFamily 在 darwin 上没有 DirectWrite 概念, CoreText 直接吃 sfnt nameID=1,
// 这里返回扫描时记录的标准 display name (与 ResolveFile 命中的 key 同源)。
// 名字未注册返回空字符串, 调用方据此走 fallback 路径, 与 Win 版语义一致。
func ResolveDWFamily(family string) string {
	_ = ensureCatalog()
	return cached.displayNames[normalizeKey(family)]
}
