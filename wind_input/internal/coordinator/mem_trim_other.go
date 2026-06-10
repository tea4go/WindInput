//go:build !windows

package coordinator

// emptyWorkingSet 非 Windows 平台为 no-op：mmap 页回收交给操作系统的
// 常规页面回收机制。
func emptyWorkingSet() error {
	return nil
}
