package main

import (
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
