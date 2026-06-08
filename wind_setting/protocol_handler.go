package main

import (
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ProtocolImportPayload 是投递给前端的协议导入负载（含解析成功/失败）。
type ProtocolImportPayload struct {
	OK      bool             `json:"ok"`
	Error   string           `json:"error,omitempty"`
	Request *ProtocolRequest `json:"request,omitempty"`
}

// ProtocolRegStatus 协议注册状态（供设置页展示）。
type ProtocolRegStatus struct {
	Registered bool   `json:"registered"`
	Command    string `json:"command"`
	Managed    bool   `json:"managed"` // true=系统托管(macOS)，前端只读
}

func buildProtocolPayload(raw string) *ProtocolImportPayload {
	req, err := ParseProtocolURL(raw)
	if err != nil {
		return &ProtocolImportPayload{OK: false, Error: err.Error()}
	}
	return &ProtocolImportPayload{OK: true, Request: req}
}

// handleProtocolURL 解析协议链接，缓存为 pending；若前端就绪则 emit 一个【无负载信号】
// 通知前端来拉取。负载一律经 ConsumePendingProtocol 拉取（取出即清空），不随事件下发。
//
// 为什么不把 payload 直接 emit：emit(push) 与 ConsumePendingProtocol(pull) 是两条通道，
// 冷启动时若 emit 恰好晚于前端 EventsOn 注册，两条通道会各触发一次导入对话框（重复弹框/
// 重复下载）。改为「单一权威数据 + 幂等取出」后，无论 push/pull 谁先到、触发几次，第二次
// 取出必得 nil，导入只发生一次。
func (a *App) handleProtocolURL(raw string) {
	payload := buildProtocolPayload(raw)
	a.pendingMu.Lock()
	a.pendingProtocol = payload
	a.pendingMu.Unlock()
	if a.ctx != nil {
		// 仅发信号（不带 payload），前端收到后调 ConsumePendingProtocol 拉取
		wailsRuntime.EventsEmit(a.ctx, "protocol-import")
	}
}

// ConsumePendingProtocol 拉取并清空缓存的协议请求（Wails 导出）。
// 取出即清空，是协议导入负载的唯一权威入口，保证多次拉取只生效一次。
func (a *App) ConsumePendingProtocol() *ProtocolImportPayload {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	p := a.pendingProtocol
	a.pendingProtocol = nil
	return p
}

// GetProtocolStatus 返回协议注册状态（Wails 导出）。
func (a *App) GetProtocolStatus() ProtocolRegStatus {
	reg, cmd := ProtocolStatus()
	return ProtocolRegStatus{Registered: reg, Command: cmd, Managed: protocolManagedBySystem}
}

// SetProtocolRegistered 注册/注销协议（Wails 导出，macOS 上为 no-op）。
func (a *App) SetProtocolRegistered(enabled bool) error {
	if enabled {
		return RegisterProtocol()
	}
	return UnregisterProtocol()
}
