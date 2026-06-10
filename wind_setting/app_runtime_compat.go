package main

import (
	"fmt"
	"log"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// 本文件提供 wailsRuntime 日志/事件调用的 Web 安全包装。
//
// 背景：Wails runtime（LogInfof/LogErrorf/EventsEmit/...）从 ctx 取 logger/frontend/
// events，缺失时调 log.Fatalf 直接终止进程（os.Exit，recover 拦不住）。Web 形态下
// a.ctx 是占位 context.Background()，故任何被调到的方法若直接用 wailsRuntime.*(a.ctx,...)
// 都会让进程退出。所有此类调用须改走下面的包装：Web 模式降级为标准库 log / no-op，
// 桌面模式行为不变。

// logInfof 记录信息日志。Web 模式走标准库 log（输出到 web.log），桌面模式走 Wails。
func (a *App) logInfof(format string, args ...any) {
	if a.webMode {
		log.Printf(format, args...)
		return
	}
	wailsRuntime.LogInfof(a.ctx, format, args...)
}

// logErrorf 记录错误日志。Web 模式走标准库 log，桌面模式走 Wails。
func (a *App) logErrorf(format string, args ...any) {
	if a.webMode {
		log.Printf(format, args...)
		return
	}
	wailsRuntime.LogErrorf(a.ctx, format, args...)
}

// emitEvent 向前端发事件。桌面模式走 wailsRuntime.EventsEmit；Web 模式经 webEmit
// （= webServer.broadcast）把事件经 SSE 投到浏览器，使下载进度 update:* 等正常工作。
func (a *App) emitEvent(name string, data ...any) {
	if a.webMode {
		if a.webEmit != nil {
			a.webEmit(name, data...)
		}
		return
	}
	wailsRuntime.EventsEmit(a.ctx, name, data...)
}

// openFileDialog Web 安全的文件选择对话框。
// Web 模式下文件对话框依赖 Wails ctx，不可用；返回 error 由调用方向前端报错。
func (a *App) openFileDialog(opts wailsRuntime.OpenDialogOptions) (string, error) {
	if a.webMode {
		return "", fmt.Errorf("文件选择对话框在 Web 模式下不可用")
	}
	return wailsRuntime.OpenFileDialog(a.ctx, opts)
}

// saveFileDialog Web 安全的文件保存对话框。
func (a *App) saveFileDialog(opts wailsRuntime.SaveDialogOptions) (string, error) {
	if a.webMode {
		return "", fmt.Errorf("文件保存对话框在 Web 模式下不可用")
	}
	return wailsRuntime.SaveFileDialog(a.ctx, opts)
}

// openDirectoryDialog Web 安全的目录选择对话框。
func (a *App) openDirectoryDialog(opts wailsRuntime.OpenDialogOptions) (string, error) {
	if a.webMode {
		return "", fmt.Errorf("目录选择对话框在 Web 模式下不可用")
	}
	return wailsRuntime.OpenDirectoryDialog(a.ctx, opts)
}
