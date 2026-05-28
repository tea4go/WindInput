package bridge

import "encoding/binary"

// unified_menu.go — 统一菜单的桥接类型与编码 (平台无关)。
//
// darwin 上 IMKit `.app` 右键候选框空白处时, 经 bridge 请求统一菜单树, Go 端把
// Coordinator 构建的菜单 (ui.BuildUnifiedMenuItems) 转成 bridge.MenuItem 并编码下发,
// .app 据此构建原生 NSMenu; 点击后回发菜单项 ID 由 Coordinator 派发。
//
// 用 bridge 本地 MenuItem 类型 (而非 ui/uicmd 的) 避免 bridge ← ui 反向依赖;
// Coordinator (已依赖 bridge) 负责把 ui.MenuItem 转为本类型。

// MenuItem 桥接菜单项 (树形, Children 表达子菜单)。
type MenuItem struct {
	ID        int32
	Label     string
	Separator bool
	Checked   bool
	Disabled  bool
	Children  []MenuItem
}

// unifiedMenuHandler 可选扩展接口: darwin bridge 请求统一菜单 / 派发菜单动作。
// Coordinator 实现, DeferredHandler 转发。
type unifiedMenuHandler interface {
	UnifiedMenuItems() []MenuItem
	HandleUnifiedMenuAction(id int)
}

// encodeUnifiedMenuPayload 把菜单树编码为 payload (不含帧头)。
// 布局: count(u32) + count×item; item = id(i32) + flags(u8) + labelLen(u32) + label
//   - childCount(u32) + children(递归)。flags: 0x01 separator, 0x02 checked, 0x04 disabled。
func encodeUnifiedMenuPayload(items []MenuItem) []byte {
	var buf []byte
	buf = appendU32(buf, uint32(len(items)))
	for _, it := range items {
		buf = appendMenuItem(buf, it)
	}
	return buf
}

func appendMenuItem(buf []byte, it MenuItem) []byte {
	buf = appendU32(buf, uint32(it.ID))
	var flags uint8
	if it.Separator {
		flags |= 0x01
	}
	if it.Checked {
		flags |= 0x02
	}
	if it.Disabled {
		flags |= 0x04
	}
	buf = append(buf, flags)
	lb := []byte(it.Label)
	buf = appendU32(buf, uint32(len(lb)))
	buf = append(buf, lb...)
	buf = appendU32(buf, uint32(len(it.Children)))
	for _, c := range it.Children {
		buf = appendMenuItem(buf, c)
	}
	return buf
}

func appendU32(buf []byte, v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return append(buf, b[:]...)
}
