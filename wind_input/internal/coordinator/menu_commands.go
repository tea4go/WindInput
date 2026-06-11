package coordinator

// menu_commands.go — 托盘菜单 / 全局热键命令码的 Go 侧具名常量。
//
// 这些命令字符串由 C++ 托盘菜单与全局热键经 bridge 发来（跨进程协议），值由 C++ 侧
// 约定、不可单边变更（属 enum-constraint.md 的"真边界"——C++/Go 各持一份字面量是协议
// 本质）。此处提供 Go 接收侧的具名常量，供 HandleMenuCommand / handleGlobalHotkeyCommand
// 两处 switch 及彼此转发调用引用，消除两处之间的拼写漂移。
const (
	// 托盘菜单命令（HandleMenuCommand 接收）
	menuCmdToggleMode     = "toggle_mode"
	menuCmdToggleWidth    = "toggle_width"
	menuCmdTogglePunct    = "toggle_punct"
	menuCmdToggleToolbar  = "toggle_toolbar"
	menuCmdOpenSettings   = "open_settings"
	menuCmdOpenDictionary = "open_dictionary"
	menuCmdAddWord        = "add_word"
	menuCmdShowAbout      = "show_about"
	menuCmdReloadConfig   = "reload_config"
	menuCmdExit           = "exit"

	// 全局热键命令（handleGlobalHotkeyCommand 接收）；toggle_punct / toggle_toolbar /
	// open_settings 与菜单命令同值，复用上面的 menuCmd* 常量。
	menuCmdSwitchEngine    = "switch_engine"
	menuCmdToggleFullWidth = "toggle_full_width"
	menuCmdTakeScreenshot  = "take_screenshot"
	menuCmdActivateIME     = "activate_ime"
)
