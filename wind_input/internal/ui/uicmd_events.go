//go:build windows

package ui

import "github.com/huanfeng/wind_input/internal/uicmd"

// uicmd_events.go 提供反向通道工具: 把 ui 内部各窗口的 callback 触发,
// 同时镜像一份 uicmd.Event 推到 m.eventCh, 供 macOS forwarder 订阅。
//
// 设计要点:
//   - Win 端 callback 链路不变, 业务消费走 callback 闭包 (现有 coordinator 代码无感)。
//   - eventCh 是并行流, 满时丢弃 (与 cmdCh 一致, 防止 UI 线程被业务慢消费拖垮)。
//   - 仅覆盖"用户主动触发"的事件; 系统事件 (OnForegroundFullscreenChange,
//     OnHoverChange 等高频) 视情况选择是否推送, 避免事件流被淹没。

// Events 返回 UI 反向事件通道, coordinator 或 macOS forwarder 可订阅。
func (m *Manager) Events() <-chan uicmd.Event {
	return m.eventCh
}

// postEvent 非阻塞推送一个 uicmd.Event。channel 满则丢弃 (debug 日志记录)。
func (m *Manager) postEvent(evt uicmd.Event) {
	select {
	case m.eventCh <- evt:
	default:
		m.logger.Debug("UI event channel full, dropping", "type", evt.Type.String())
	}
}

// wrapCandidateCallbacks 在用户传入的 CandidateCallback 外层包一层, 每个 callback
// 调用同时推一份 uicmd.Event。返回值替换原 callback 注册到 window 上。
// 入参可为 nil, 此时返回 nil (caller 自行决定是否注册)。
func (m *Manager) wrapCandidateCallbacks(cb *CandidateCallback) *CandidateCallback {
	if cb == nil {
		return nil
	}
	wrap := *cb // 浅拷贝, 不修改调用方的 struct

	origSelect := cb.OnSelect
	wrap.OnSelect = func(index int) {
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateSelect,
			uicmd.CandidateSelectPayload{Index: int32(index)}))
		if origSelect != nil {
			origSelect(index)
		}
	}

	origHover := cb.OnHoverChange
	wrap.OnHoverChange = func(index, tipX, tipBelowY, tipAboveY int) {
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateHover, uicmd.CandidateHoverPayload{
			Index:         int32(index),
			TooltipX:      int32(tipX),
			TooltipBelowY: int32(tipBelowY),
			TooltipAboveY: int32(tipAboveY),
		}))
		if origHover != nil {
			origHover(index, tipX, tipBelowY, tipAboveY)
		}
	}

	origPageUp := cb.OnPageUp
	wrap.OnPageUp = func() {
		m.postEvent(uicmd.NewEvent(uicmd.EvtPageUp, uicmd.PageUpPayload{}))
		if origPageUp != nil {
			origPageUp()
		}
	}

	origPageDown := cb.OnPageDown
	wrap.OnPageDown = func() {
		m.postEvent(uicmd.NewEvent(uicmd.EvtPageDown, uicmd.PageDownPayload{}))
		if origPageDown != nil {
			origPageDown()
		}
	}

	wrap.OnMoveUp = wrapCandidateAction(cb.OnMoveUp, m, uicmd.CandidateActionMoveUp)
	wrap.OnMoveDown = wrapCandidateAction(cb.OnMoveDown, m, uicmd.CandidateActionMoveDown)
	wrap.OnMoveTop = wrapCandidateAction(cb.OnMoveTop, m, uicmd.CandidateActionMoveTop)
	wrap.OnDelete = wrapCandidateAction(cb.OnDelete, m, uicmd.CandidateActionDelete)
	wrap.OnResetDefault = wrapCandidateAction(cb.OnResetDefault, m, uicmd.CandidateActionResetDefault)
	wrap.OnCopy = wrapCandidateAction(cb.OnCopy, m, uicmd.CandidateActionCopy)

	origCopyDebug := cb.OnCopyDebugBatch
	wrap.OnCopyDebugBatch = func(maxPages int) {
		// Debug action: maxPages 复用 Index 字段
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateContextMenu, uicmd.CandidateContextMenuPayload{
			Index:  int32(maxPages),
			Action: uicmd.CandidateActionCopyDebugBatch,
		}))
		if origCopyDebug != nil {
			origCopyDebug(maxPages)
		}
	}

	origOpenSettings := cb.OnOpenSettings
	wrap.OnOpenSettings = func() {
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateContextMenu, uicmd.CandidateContextMenuPayload{
			Action: uicmd.CandidateActionOpenSettings,
		}))
		if origOpenSettings != nil {
			origOpenSettings()
		}
	}

	origAbout := cb.OnAbout
	wrap.OnAbout = func() {
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateContextMenu, uicmd.CandidateContextMenuPayload{
			Action: uicmd.CandidateActionAbout,
		}))
		if origAbout != nil {
			origAbout()
		}
	}

	origShowMenu := cb.OnShowUnifiedMenu
	wrap.OnShowUnifiedMenu = func(screenX, screenY int) {
		// 空白处右键: 用 EvtCandidateContextMenu 携带 X/Y? Payload 没有 X/Y 字段;
		// 这里只推 action 信号, coordinator 自行用现有 ShowUnifiedMenu 流程获取坐标。
		// macOS 侧 IMKit 本地处理菜单, 不需要这个事件; Win 侧本来就走 callback。
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateContextMenu, uicmd.CandidateContextMenuPayload{
			Action: uicmd.CandidateActionShowMenu,
		}))
		if origShowMenu != nil {
			origShowMenu(screenX, screenY)
		}
	}

	origDragEnd := cb.OnDragEnd
	wrap.OnDragEnd = func(x, y int) {
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateDragEnd,
			uicmd.CandidateDragEndPayload{X: int32(x), Y: int32(y)}))
		if origDragEnd != nil {
			origDragEnd(x, y)
		}
	}

	return &wrap
}

// wrapCandidateAction 是 MoveUp/MoveDown/MoveTop/Delete/ResetDefault/Copy 等
// 共用形式 (func(int) 签名) 的小工厂, 避免重复 6 段几乎一样的代码。
func wrapCandidateAction(orig func(int), m *Manager, action uicmd.CandidateContextMenuAction) func(int) {
	return func(index int) {
		m.postEvent(uicmd.NewEvent(uicmd.EvtCandidateContextMenu, uicmd.CandidateContextMenuPayload{
			Index:  int32(index),
			Action: action,
		}))
		if orig != nil {
			orig(index)
		}
	}
}

// wrapToolbarCallbacks 包装 ToolbarCallback, 行为同 wrapCandidateCallbacks。
// OnForegroundFullscreenChange 不推送 uicmd.Event (系统 shell 事件, 非用户操作)。
func (m *Manager) wrapToolbarCallbacks(cb *ToolbarCallback) *ToolbarCallback {
	if cb == nil {
		return nil
	}
	wrap := *cb

	wrap.OnToggleMode = wrapToolbarSimple(cb.OnToggleMode, m, uicmd.ToolbarActionToggleMode)
	wrap.OnToggleWidth = wrapToolbarSimple(cb.OnToggleWidth, m, uicmd.ToolbarActionToggleWidth)
	wrap.OnTogglePunct = wrapToolbarSimple(cb.OnTogglePunct, m, uicmd.ToolbarActionTogglePunct)
	wrap.OnOpenSettings = wrapToolbarSimple(cb.OnOpenSettings, m, uicmd.ToolbarActionOpenSettings)

	origPos := cb.OnPositionChanged
	wrap.OnPositionChanged = func(x, y int) {
		m.postEvent(uicmd.NewEvent(uicmd.EvtToolbarClick, uicmd.ToolbarClickPayload{
			Action: uicmd.ToolbarActionDragEnd,
			X:      int32(x),
			Y:      int32(y),
		}))
		if origPos != nil {
			origPos(x, y)
		}
	}

	origCtx := cb.OnContextMenu
	wrap.OnContextMenu = func(action ToolbarContextMenuAction) {
		var wireAction uicmd.ToolbarClickAction
		switch action {
		case ToolbarMenuSettings:
			wireAction = uicmd.ToolbarActionContextSettings
		case ToolbarMenuRestartService:
			wireAction = uicmd.ToolbarActionContextRestart
		case ToolbarMenuAbout:
			wireAction = uicmd.ToolbarActionContextAbout
		default:
			wireAction = uicmd.ToolbarClickAction("context_unknown")
		}
		m.postEvent(uicmd.NewEvent(uicmd.EvtToolbarClick, uicmd.ToolbarClickPayload{Action: wireAction}))
		if origCtx != nil {
			origCtx(action)
		}
	}

	origShowMenu := cb.OnShowMenu
	wrap.OnShowMenu = func(screenX, screenY, flipRefY int) {
		// flipRefY 不进 wire (PR-4 暂忽略, macOS IMKit 自己算翻转)。
		m.postEvent(uicmd.NewEvent(uicmd.EvtToolbarClick, uicmd.ToolbarClickPayload{
			Action: uicmd.ToolbarActionOpenMenu,
			X:      int32(screenX),
			Y:      int32(screenY),
		}))
		if origShowMenu != nil {
			origShowMenu(screenX, screenY, flipRefY)
		}
	}

	// OnForegroundFullscreenChange 不包装: 系统 shell 事件, 不属于用户 UI 交互;
	// macOS 上由 IMKit `.app` 自己监听 NSApplicationDidChangeOcclusionState 等。

	return &wrap
}

// wrapToolbarSimple 是无参 callback (toggle / open settings) 的共用包装工厂。
func wrapToolbarSimple(orig func(), m *Manager, action uicmd.ToolbarClickAction) func() {
	return func() {
		m.postEvent(uicmd.NewEvent(uicmd.EvtToolbarClick, uicmd.ToolbarClickPayload{Action: action}))
		if orig != nil {
			orig()
		}
	}
}

// wrapHotkeyCallback 包装全局快捷键 callback, 触发时同时推 EvtHotkeyTriggered。
func (m *Manager) wrapHotkeyCallback(orig func(command string)) func(command string) {
	return func(command string) {
		m.postEvent(uicmd.NewEvent(uicmd.EvtHotkeyTriggered,
			uicmd.HotkeyTriggeredPayload{Command: command}))
		if orig != nil {
			orig(command)
		}
	}
}
