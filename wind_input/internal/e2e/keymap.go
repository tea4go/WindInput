//go:build windows || darwin

// keymap.go — 把测试用的字符 / 键名翻译成 bridge.KeyEventData。
//
// coordinator.HandleKeyEvent 的路由主要依据 data.KeyCode（Win32 VK 码）与 data.Modifiers
// （ModShift/ModCtrl/...），data.Key 作为辅助字符串。这里同时填好三者，与 C++ TSF 侧
// 真实送入的键事件对齐，使 in-process 驱动尽量贴近真机。
package e2e

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/coordinator"
)

// namedKeys 把人类可读的键名（小写）映射到 Win32 VK 码。
// 覆盖 E2E 常用的功能键；选择键 1-9/0 通过 runeToKeyEvent 走数字路径，不在此表。
var namedKeys = map[string]int{
	"space":     0x20, // VK_SPACE
	"enter":     0x0D, // VK_RETURN
	"return":    0x0D,
	"backspace": 0x08, // VK_BACK
	"bs":        0x08,
	"tab":       0x09, // VK_TAB
	"esc":       0x1B, // VK_ESCAPE
	"escape":    0x1B,
	"pageup":    0x21, // VK_PRIOR
	"pgup":      0x21,
	"pagedown":  0x22, // VK_NEXT
	"pgdn":      0x22,
	"home":      0x24, // VK_HOME
	"end":       0x23, // VK_END
	"left":      0x25, // VK_LEFT
	"up":        0x26, // VK_UP
	"right":     0x27, // VK_RIGHT
	"down":      0x28, // VK_DOWN
	"delete":    0x2E, // VK_DELETE
	"del":       0x2E,
	"lshift":    0xA0, // VK_LSHIFT（默认中英切换键之一）
	"rshift":    0xA1, // VK_RSHIFT
	"lctrl":     0xA2, // VK_LCONTROL
	"rctrl":     0xA3, // VK_RCONTROL
}

// symbolVK 把 US 键盘上"无需 Shift 即可输入"的符号映射到对应的 OEM VK 码。
// 标点处理路径可能同时参考 KeyCode 与 Key，填准 VK 让标点用例更贴近真机。
var symbolVK = map[rune]int{
	';':  0xBA, // VK_OEM_1
	'=':  0xBB, // VK_OEM_PLUS
	',':  0xBC, // VK_OEM_COMMA
	'-':  0xBD, // VK_OEM_MINUS
	'.':  0xBE, // VK_OEM_PERIOD
	'/':  0xBF, // VK_OEM_2
	'`':  0xC0, // VK_OEM_3
	'[':  0xDB, // VK_OEM_4
	'\\': 0xDC, // VK_OEM_5
	']':  0xDD, // VK_OEM_6
	'\'': 0xDE, // VK_OEM_7
}

// shiftedToBase 把"需要 Shift 才能输入"的符号还原为 (基础键字符, 需要 Shift)。
// 与 coordinator/handle_key_event.go 的 shiftedKeyMap 互为逆表（US 布局），照此同步。
var shiftedToBase = map[rune]rune{
	'!': '1', '@': '2', '#': '3', '$': '4', '%': '5',
	'^': '6', '&': '7', '*': '8', '(': '9', ')': '0',
	'_': '-', '+': '=',
	'{': '[', '}': ']', '|': '\\',
	':': ';', '"': '\'',
	'<': ',', '>': '.', '?': '/',
	'~': '`',
}

// runeToKeyEvent 把单个字符翻译成一次按键事件。
//   - a-z：KeyCode = 对应大写 ASCII（与 Win32 VK_A..VK_Z 对齐），无 Shift。
//   - A-Z：KeyCode = 该 ASCII，带 ModShift。
//   - 0-9：KeyCode = 该 ASCII。
//   - 空格：走 VK_SPACE。
//   - 免 Shift 符号：KeyCode = OEM VK；需 Shift 符号：还原基础键 + ModShift。
//   - 其余字符：退化为 KeyCode = int(r)，Key = string(r)。
func runeToKeyEvent(r rune) bridge.KeyEventData {
	switch {
	case r >= 'a' && r <= 'z':
		return bridge.KeyEventData{Key: string(r), KeyCode: int(r - 'a' + 'A')}
	case r >= 'A' && r <= 'Z':
		return bridge.KeyEventData{Key: string(r), KeyCode: int(r), Modifiers: coordinator.ModShift}
	case r >= '0' && r <= '9':
		return bridge.KeyEventData{Key: string(r), KeyCode: int(r)}
	case r == ' ':
		return bridge.KeyEventData{Key: " ", KeyCode: 0x20}
	}
	if vk, ok := symbolVK[r]; ok {
		return bridge.KeyEventData{Key: string(r), KeyCode: vk}
	}
	if base, ok := shiftedToBase[r]; ok {
		ev := runeToKeyEvent(base) // 取基础键的 VK
		ev.Key = string(r)         // 实际输入的字符是 shifted 符号
		ev.Modifiers |= coordinator.ModShift
		return ev
	}
	return bridge.KeyEventData{Key: string(r), KeyCode: int(r)}
}

// nameToKeyEvent 把键名翻译成一次功能键事件；未知名返回 ok=false。
// 单字符名（如 "a" / "1" / "."）回退到 runeToKeyEvent。
func nameToKeyEvent(name string) (bridge.KeyEventData, bool) {
	if vk, ok := namedKeys[name]; ok {
		return bridge.KeyEventData{Key: name, KeyCode: vk}, true
	}
	r := []rune(name)
	if len(r) == 1 {
		return runeToKeyEvent(r[0]), true
	}
	return bridge.KeyEventData{}, false
}
