package ui

import (
	"syscall"
	"unsafe"
)

// registerShellHook 将工具栏窗口注册为 Shell Hook 接收方，让我们能在前台窗口
// 进/出全屏时被动收到通知（HSHELL_WINDOWENTERFULLSCREEN=53 /
// HSHELL_WINDOWEXITFULLSCREEN=54）。
//
// 这条通道是 Windows 任务栏「全屏时自动隐藏」依赖的同一套机制 —— 浏览器 F11、
// 视频播放器无边框全屏、PPT 放映、D3D 全屏等典型场景都会触发，且**完全与
// 输入流程解耦**（不依赖按键事件，不引入定时器）。
//
// `WM_SHELLHOOKMESSAGE` 不是固定值，必须通过 RegisterWindowMessageW("SHELLHOOK")
// 动态注册；结果保存到包级 shellHookMsg，供 toolbarWndProc 拦截匹配。
//
// 常量 53/54 在 SDK 头文件里没有，但任务栏、Explorer 等多年来稳定依赖，
// 实际是事实标准。RegisterShellHookWindow 调用失败时静默降级 —— 此时工具栏
// 仅在「显示决策点」走 IsForegroundFullscreen() 一次判定，不会主动跟随变化。
func (w *ToolbarWindow) registerShellHook() {
	if w.hwnd == 0 {
		return
	}

	if shellHookMsg == 0 {
		name, err := syscall.UTF16PtrFromString("SHELLHOOK")
		if err != nil {
			w.logger.Warn("UTF16PtrFromString(SHELLHOOK) failed", "error", err)
			return
		}
		msgID, _, callErr := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
		if msgID == 0 {
			w.logger.Warn("RegisterWindowMessageW(SHELLHOOK) failed", "error", callErr)
			return
		}
		shellHookMsg = uint32(msgID)
	}

	ret, _, err := procRegisterShellHookWindow.Call(uintptr(w.hwnd))
	if ret == 0 {
		w.logger.Warn("RegisterShellHookWindow failed", "error", err)
		return
	}
	w.logger.Info("Shell hook registered for fullscreen detection",
		"shellHookMsg", shellHookMsg, "hwnd", w.hwnd)
}

// handleShellHook 处理 RegisterShellHookWindow 投递的通知。
// wParam 低 16 位是 HSHELL_* 消息码；lParam 通常是触发事件的 HWND（这里未使用）。
//
// 仅关心 53/54 两个全屏相关码，其它消息（窗口创建/激活/任务变更等）直接忽略。
func (w *ToolbarWindow) handleShellHook(wParam, _ uintptr) uintptr {
	code := int(wParam & 0xFFFF)
	switch code {
	case hshellWindowEnterFullscreen:
		if w.callback != nil && w.callback.OnForegroundFullscreenChange != nil {
			w.callback.OnForegroundFullscreenChange(true)
		}
	case hshellWindowExitFullscreen:
		if w.callback != nil && w.callback.OnForegroundFullscreenChange != nil {
			w.callback.OnForegroundFullscreenChange(false)
		}
	}
	return 0
}
