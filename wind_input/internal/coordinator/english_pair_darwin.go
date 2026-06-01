//go:build darwin

package coordinator

// englishModeAutoPairInGo 控制「IME 英文模式」下的成对标点是否由 Go 协调器接管。
//
// Windows 上英文模式的自动配对由 C++ TSF 层处理 (见 getAutoPairTracker 注释),
// Go 端透传即可; 故非 darwin 平台此常量为 false, 避免与 C++ 重复配对。
// macOS 没有等价的 C++ 层, 英文模式按键经 Go 后只会透传 → 需要 Go 自己接管,
// 故此常量为 true。
const englishModeAutoPairInGo = true
