//go:build !darwin

package coordinator

// englishModeAutoPairInGo: 非 darwin 平台 (Windows/Linux) 为 false。
// Windows 英文模式的自动配对由 C++ TSF 层处理, Go 端透传不重复配对; 见
// english_pair_darwin.go 的完整说明与 getAutoPairTracker 注释。
const englishModeAutoPairInGo = false
