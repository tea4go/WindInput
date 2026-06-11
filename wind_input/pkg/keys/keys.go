// Package keys 提供按键名与修饰键的统一规范化 token。
//
// 该包是项目中所有按键字符串（如 "a"、"semicolon"、"pageup"）和修饰键字符串
// （如 "ctrl"、"shift"）的单一权威来源（single source of truth）。其它包应当
// 使用 ParseKey/Key/Modifier 而不是裸字符串比较，以避免历史上出现过的别名漂移
// （例如 "pageup" 与 "page_up" 并存）。
package keys

import (
	"fmt"
	"sort"
	"strings"
)

// Key 是统一的按键名规范化 token。
type Key string

// 字母 a-z
const (
	KeyA Key = "a"
	KeyB Key = "b"
	KeyC Key = "c"
	KeyD Key = "d"
	KeyE Key = "e"
	KeyF Key = "f"
	KeyG Key = "g"
	KeyH Key = "h"
	KeyI Key = "i"
	KeyJ Key = "j"
	KeyK Key = "k"
	KeyL Key = "l"
	KeyM Key = "m"
	KeyN Key = "n"
	KeyO Key = "o"
	KeyP Key = "p"
	KeyQ Key = "q"
	KeyR Key = "r"
	KeyS Key = "s"
	KeyT Key = "t"
	KeyU Key = "u"
	KeyV Key = "v"
	KeyW Key = "w"
	KeyX Key = "x"
	KeyY Key = "y"
	KeyZ Key = "z"
)

// 数字 0-9
const (
	Key0 Key = "0"
	Key1 Key = "1"
	Key2 Key = "2"
	Key3 Key = "3"
	Key4 Key = "4"
	Key5 Key = "5"
	Key6 Key = "6"
	Key7 Key = "7"
	Key8 Key = "8"
	Key9 Key = "9"
)

// 功能键 F1-F12
const (
	KeyF1  Key = "f1"
	KeyF2  Key = "f2"
	KeyF3  Key = "f3"
	KeyF4  Key = "f4"
	KeyF5  Key = "f5"
	KeyF6  Key = "f6"
	KeyF7  Key = "f7"
	KeyF8  Key = "f8"
	KeyF9  Key = "f9"
	KeyF10 Key = "f10"
	KeyF11 Key = "f11"
	KeyF12 Key = "f12"
)

// 标点
const (
	KeySemicolon Key = "semicolon"
	KeyQuote     Key = "quote"
	KeyComma     Key = "comma"
	KeyPeriod    Key = "period"
	KeyMinus     Key = "minus"
	KeyEqual     Key = "equal"
	KeyLBracket  Key = "lbracket"
	KeyRBracket  Key = "rbracket"
	KeyBackslash Key = "backslash"
	KeySlash     Key = "slash"
	KeyGrave     Key = "grave"
)

// 控制键
const (
	KeySpace     Key = "space"
	KeyTab       Key = "tab"
	KeyEnter     Key = "enter"
	KeyBackspace Key = "backspace"
	KeyEscape    Key = "escape"
	KeyPageUp    Key = "pageup"
	KeyPageDown  Key = "pagedown"
	KeyShiftTab  Key = "shift_tab"
)

// 修饰键（作为独立按键 token 时使用）
const (
	KeyLShift   Key = "lshift"
	KeyRShift   Key = "rshift"
	KeyLCtrl    Key = "lctrl"
	KeyRCtrl    Key = "rctrl"
	KeyCapsLock Key = "capslock"
)

// Modifier 是修饰键 token。
type Modifier string

const (
	ModCtrl  Modifier = "ctrl"
	ModShift Modifier = "shift"
	ModAlt   Modifier = "alt"
	ModWin   Modifier = "win"
)

// aliasToKey 把所有可接受的别名/字符表示映射到规范 Key。
// 这是单一权威表 —— 所有 "page_up"/"pageup"、"`"/"grave"、"."/"period"、
// "esc"/"escape"、"return"/"enter" 等别名都在这里收敛。
var aliasToKey = map[string]Key{
	// 标点字符 -> 规范名
	"`": KeyGrave, "~": KeyGrave, "grave": KeyGrave, "backtick": KeyGrave,
	";": KeySemicolon, "semicolon": KeySemicolon,
	"'": KeyQuote, "quote": KeyQuote,
	",": KeyComma, "comma": KeyComma,
	".": KeyPeriod, "period": KeyPeriod,
	"-": KeyMinus, "minus": KeyMinus,
	"=": KeyEqual, "plus": KeyEqual, "equal": KeyEqual,
	"[": KeyLBracket, "lbracket": KeyLBracket, "open_bracket": KeyLBracket,
	"]": KeyRBracket, "rbracket": KeyRBracket, "close_bracket": KeyRBracket,
	"\\": KeyBackslash, "backslash": KeyBackslash,
	"/": KeySlash, "slash": KeySlash,

	// 控制键别名
	"space":             KeySpace,
	"tab":               KeyTab,
	"enter":             KeyEnter,
	"return":            KeyEnter,
	"backspace":         KeyBackspace,
	"back":              KeyBackspace,
	"escape":            KeyEscape,
	"esc":               KeyEscape,
	"pageup":            KeyPageUp,
	"page_up":           KeyPageUp,
	"prior":             KeyPageUp,
	"pagedown":          KeyPageDown,
	"page_down":         KeyPageDown,
	"next":              KeyPageDown,
	string(KeyShiftTab): KeyShiftTab,

	// 修饰键作为独立按键
	"lshift":   KeyLShift,
	"rshift":   KeyRShift,
	"lctrl":    KeyLCtrl,
	"rctrl":    KeyRCtrl,
	"capslock": KeyCapsLock,
}

func init() {
	// 字母 a-z
	for c := 'a'; c <= 'z'; c++ {
		s := string(c)
		aliasToKey[s] = Key(s)
	}
	// 数字 0-9
	for c := '0'; c <= '9'; c++ {
		s := string(c)
		aliasToKey[s] = Key(s)
	}
	// f1-f12
	for i := 1; i <= 12; i++ {
		s := fmt.Sprintf("f%d", i)
		aliasToKey[s] = Key(s)
	}
}

// ParseKey 把任意输入字符串解析为规范 Key（处理大小写、别名）。
// 返回的 Key 已经是规范化形式，不论调用方传入哪种别名（例如 "page_up" 与
// "pageup" 都会被解析为 KeyPageUp）。
func ParseKey(s string) (Key, bool) {
	k, ok := aliasToKey[strings.ToLower(s)]
	return k, ok
}

// Valid 返回 Key 是否为已知的规范名（即在 aliasToKey 中作为规范值出现）。
func (k Key) Valid() bool {
	if v, ok := aliasToKey[string(k)]; ok && v == k {
		return true
	}
	return false
}

// CanonicalKeys 返回全部规范 Key（去重、按字典序排序）。
// 用途：导出给设置前端做 enums.ts 的 Key 值一致性校验——前端按键 token 的值
// 必须 ∈ 本清单，杜绝历史上出现过的别名漂移（如前端用 "open_bracket" 而 Go
// 规范是 "lbracket"）。由 keys_export_test.go 的 -update 写出 keys.json。
func CanonicalKeys() []Key {
	seen := make(map[Key]bool, len(aliasToKey))
	out := make([]Key, 0, len(aliasToKey))
	for _, v := range aliasToKey {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CanonicalModifiers 返回全部规范 Modifier（按字典序排序）。
func CanonicalModifiers() []Modifier {
	out := []Modifier{ModAlt, ModCtrl, ModShift, ModWin}
	return out
}

// Aliases 返回别名 → 规范 Key 的全量映射副本（含规范名自映射）。
// 前端可用它把存量配置中的别名（如 "backtick"）归一化为规范名后再做控件回显匹配。
func Aliases() map[string]Key {
	m := make(map[string]Key, len(aliasToKey))
	for k, v := range aliasToKey {
		m[k] = v
	}
	return m
}

// ParseModifier 把任意输入字符串解析为规范 Modifier（处理大小写）。
func ParseModifier(s string) (Modifier, bool) {
	m := Modifier(strings.ToLower(s))
	if m.Valid() {
		return m, true
	}
	return "", false
}

// Valid 返回 Modifier 是否为已知的修饰键 token。
func (m Modifier) Valid() bool {
	switch m {
	case ModCtrl, ModShift, ModAlt, ModWin:
		return true
	}
	return false
}
