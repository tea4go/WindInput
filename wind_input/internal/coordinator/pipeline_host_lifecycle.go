// pipeline_host_lifecycle.go — 宿主卸载的统一 UI/行为状态清理（设计文档七·3）。
//
// 修复各 exitXxxMode 的清理不一致：exitTempEnglishMode 漏 SetModeAccentColor(nil)（accent
// 残留到下一模式）、四个 exit 都漏 pairTracker.Clear（仅 clearState 清 → 经 commit/ESC 显式
// 退出时配对栈残留干扰新输入）。把这组模式无关的卸载收成单一契约，供 exitXxxMode 复用、与
// clearState 的同组清理保持一致。
//
// 不含布局恢复：各模式用不同 saved 字段（quick_input=savedLayout、special=specialSavedLayout）
// 且 ForceVertical 时序是模式特有的，仍留各自 exit。也不含 lastOutputWasDigit——它在
// HandleKeyEvent 开头按键级重置、clearState 亦不清，非 exit 漏清项。
package coordinator

// clearHostUIState 统一卸载宿主的 UI 标签/光效/快捷输入标志 + 配对栈。
// 幂等：未进入相关模式时各 Set/Clear 均为无副作用的归零。
func (c *Coordinator) clearHostUIState() {
	if c.uiManager != nil {
		c.uiManager.SetModeLabel("")
		c.uiManager.SetModeAccentColor(nil)
		c.uiManager.SetQuickInputMode(false)
	}
	if c.pairTracker != nil {
		c.pairTracker.Clear()
	}
	if c.pairTrackerEn != nil {
		c.pairTrackerEn.Clear()
	}
}
