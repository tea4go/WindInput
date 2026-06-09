package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type runMode int

const (
	modeGUI runMode = iota
	modeWeb
)

// resolveRunMode 按命令行参数决定运行形态。仅在显式 --web 时进入 Web 形态；否则一律 GUI。
//
// 设计决策：不再按「是否安装 WebView2」自动降级。缺 WebView2 时交给 Wails 自带的
// 安装引导处理——wails build 默认 -webview2=download，运行时若缺 Runtime 会弹原生
// 对话框引导用户下载安装，装完即正常进 GUI。自动降级会抢在该引导之前，把用户静默
// 困在功能受限的 Web 模式，且 Web 进程常驻会锁定 exe（重新构建时覆盖失败）。
func resolveRunMode(args []string) runMode {
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--web":
			return modeWeb
		case "--gui":
			return modeGUI
		}
	}
	return modeGUI
}

// runWebMode 启动 Web 形态：补必要初始化、起 HTTP 服务、自动开浏览器、阻塞保活。
// 仅在显式 --web 时调用。
//
// Web 形态不走 wails.Run，故手动补 app.ctx（占位 ctx，供读 a.ctx 的方法使用）。
// rpcClient 已在 NewApp() 初始化（懒连接管道），可直接用。
// 有意省略：startIPCListener（单例页面切换，Web 无意义）、runStartupUpdateCheck
// （更新功能依赖 Wails 事件，已禁用）、SelfHealProtocol/协议处理（Web 不接）。
// 事件订阅在 /api/events 首次连接时按需启动，不在此常驻。
//
// 刻意不弹任何原生对话框：模态 MessageBox 会阻塞当前 goroutine，且 Web 进程常驻
// 会让模态框长期挂着、exe 被占用。地址改为打印到 stdout（控制台/dev 构建可见）并
// 自动开浏览器（GUI 子系统无控制台时这是唯一可见反馈）；关键事件写入日志文件以便诊断。
func runWebMode(app *App, assets embed.FS) {
	setupWebLog()

	distFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Printf("[web] 无法定位前端资源: %v", err)
		return
	}

	app.ctx = context.Background()
	app.webMode = true // 使 wailsRuntime 日志/事件包装走 Web 安全分支，避免无效 ctx 终止进程

	ws := &webServer{app: app}
	app.webEmit = ws.broadcast // 事件经 SSE 投递到浏览器（下载进度 update:* 等）

	if err := ws.Start(distFS); err != nil {
		log.Printf("[web] 启动 Web 设置服务失败: %v", err)
		return
	}
	defer ws.Stop()

	u := ws.url()
	log.Printf("[web] Web 设置服务已启动: %s", u)
	fmt.Println("清风输入法设置 Web 模式已启动:", u)
	_ = shellOpen(u) // 自动开浏览器，失败不致命

	// 等待空闲退出信号：心跳超时或收到关闭信标后，ws 关闭 done，进程随之退出。
	// 不再 select{} 永久常驻，避免 exe 被长期占用、重新构建时覆盖失败。
	<-ws.Wait()
	log.Printf("[web] 退出 Web 设置服务")
}

// setupWebLog 把日志输出重定向到 %TEMP%/wind_setting/web.log。
// Web 形态由 wails build 产出的 GUI 子系统 exe 启动时无控制台，stderr 被丢弃，
// 故关键事件（启动/心跳/退出）写文件以便诊断。打不开文件时退化为默认 stderr。
func setupWebLog() {
	dir := filepath.Join(os.TempDir(), "wind_setting")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "web.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
}
