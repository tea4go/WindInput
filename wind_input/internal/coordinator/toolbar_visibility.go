package coordinator

import (
	"github.com/huanfeng/wind_input/internal/foreground"
	"github.com/huanfeng/wind_input/internal/ui"
)

// showToolbarRespectingFullscreen 显示工具栏的统一入口：
//
// 当 config.Toolbar.HideInFullscreen 启用（默认）且前台应用处于全屏状态时，
// 隐藏工具栏而非显示；否则正常以 (posX, posY) + 当前 ToolbarState 显示。
//
// 同时把抑制状态记到 c.toolbarSuppressedByFullscreen，供 ShellHook 通知路径比较翻转。
//
// 仅在调用方已确认「用户期望显示工具栏」时调用（即 toolbarVisible && imeActivated
// 等前置条件已成立）。所有原先直接调用 uiManager.ShowToolbarWithState 的位置应
// 改为调用本方法，以保证全屏检测语义一致。
//
// 调用方必须持有 c.mu。
func (c *Coordinator) showToolbarRespectingFullscreen(posX, posY int) {
	if c.uiManager == nil {
		return
	}
	suppress := c.shouldHideToolbarDueToFullscreen()
	c.toolbarSuppressedByFullscreen = suppress
	if suppress {
		c.logger.Debug("Toolbar suppressed: foreground app is fullscreen")
		c.uiManager.SetToolbarVisible(false)
		return
	}
	c.uiManager.ShowToolbarWithState(posX, posY, c.buildToolbarState())
}

// shouldHideToolbarDueToFullscreen 返回是否应因「前台全屏」抑制工具栏显示。
// 调用方必须持有 c.mu（用于安全访问 c.config）。
func (c *Coordinator) shouldHideToolbarDueToFullscreen() bool {
	if c.config == nil || !c.config.Toolbar.IsHideInFullscreen() {
		return false
	}
	return foreground.IsForegroundFullscreen()
}

// OnShellFullscreenChange 由 UI 层在收到系统 Shell 全屏通知
// (HSHELL_WINDOWENTERFULLSCREEN=53 / HSHELL_WINDOWEXITFULLSCREEN=54) 时调用。
//
// 这是 Windows 任务栏自身用来「全屏时自动隐藏」的同一套通道，浏览器 F11、
// 视频播放器全屏、PPT 放映、D3D 全屏均会触发，且**与按键事件解耦** —— 无论
// 用户在不在打字，全屏切换发生时都会立即收到通知，不给输入流程引入任何延迟。
//
// enter=true 表示有窗口进入全屏；enter=false 表示有窗口退出全屏。
// 仅在「用户期望显示工具栏 && IME 已激活 && 配置启用全屏隐藏」时才实际下发 UI 命令，
// 且只在抑制状态翻转时下发，避免重复刷新。
func (c *Coordinator) OnShellFullscreenChange(enter bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.uiManager == nil || !c.toolbarVisible || !c.imeActivated {
		return
	}
	if c.config == nil || !c.config.Toolbar.IsHideInFullscreen() {
		return
	}
	if enter == c.toolbarSuppressedByFullscreen {
		return
	}
	c.toolbarSuppressedByFullscreen = enter
	if enter {
		c.logger.Debug("ShellHook: foreground entered fullscreen, hiding toolbar")
		c.uiManager.SetToolbarVisible(false)
	} else {
		c.logger.Debug("ShellHook: foreground exited fullscreen, restoring toolbar")
		posX, posY := c.computeToolbarPositionLocked()
		c.uiManager.ShowToolbarWithState(posX, posY, c.buildToolbarState())
	}
}

// computeToolbarPositionLocked 按当前 caret 位置（或默认位置）计算工具栏坐标，
// 并复用用户曾在该显示器上的拖拽位置。
// 调用方必须持有 c.mu。
func (c *Coordinator) computeToolbarPositionLocked() (int, int) {
	const toolbarWidth, toolbarHeight = 140, 30
	scaledW := ui.ScaleIntForDPI(toolbarWidth)
	scaledH := ui.ScaleIntForDPI(toolbarHeight)

	var posX, posY int
	if c.caretValid {
		posX, posY = ui.GetToolbarPositionForCaret(c.caretX, c.caretY, scaledW, scaledH)
	} else {
		posX, posY = ui.GetDefaultToolbarPosition(scaledW, scaledH)
	}

	_, _, monRight, monBottom := ui.GetMonitorWorkAreaFromPoint(posX, posY)
	key := ui.MonitorKeyStr(monRight, monBottom)
	if saved, ok := c.toolbarUserPos[key]; ok {
		posX, posY = saved.X, saved.Y
	}
	return posX, posY
}
