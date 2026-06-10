//go:build windows

package coordinator

import (
	"golang.org/x/sys/windows"
)

var (
	modpsapi            = windows.NewLazySystemDLL("psapi.dll")
	procEmptyWorkingSet = modpsapi.NewProc("EmptyWorkingSet")
)

// emptyWorkingSet 把当前进程 Working Set 中的页全部移出物理内存。
// file-backed 页（mmap 词库）被直接丢弃，私有页进修改/备用列表；
// 后续访问按需软缺页拉回，长时间空闲后才值得调用（见 idleMemTrimmer）。
func emptyWorkingSet() error {
	r1, _, err := procEmptyWorkingSet.Call(uintptr(windows.CurrentProcess()))
	if r1 == 0 {
		return err
	}
	return nil
}
