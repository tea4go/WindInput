package foreground

import "testing"

// TestAppTitleCallable 仅验证 App / Title 能调通且类型正确; 不断言返回值
// 内容 (CI 环境可能没有前台窗口, 返回空属正常)。
func TestAppTitleCallable(t *testing.T) {
	_ = App()
	_ = Title()
}
