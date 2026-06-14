// handle_url.go — URL 临时输入模式。
//
// 正常输入下 inputBuffer 加上本键字符恰好构成某配置前缀（www./http/https/ftp. 等，悲观全
// 匹配）时夺取进入：前缀作初始 buffer，模式内自由输入网址字符（字母/数字/标点都不触发上屏），
// 仅空格/回车上屏，退格删空退出。无候选，纯 preedit。详见 docs/design/input-processor-pipeline.md。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/ipc"
	"github.com/huanfeng/wind_input/internal/store"
	"github.com/huanfeng/wind_input/internal/ui"
)

// urlEnabled URL 临时输入模式是否启用（总开关 + 至少一个前缀）。
func (c *Coordinator) urlEnabled() bool {
	return c.config != nil && c.config.Input.UrlInput.Enabled &&
		len(c.config.Input.UrlInput.Prefixes) > 0
}

// urlActivationResidual 判定「正常输入下本键是否恰好完成某 URL 前缀」（悲观全匹配）。
// 返回 (residual, true) 时 residual = inputBuffer+key（完整前缀），应夺取进入 URL 模式。
// 前缀 >=3 字符，故空 buffer 不会误触；大写键（Shift）不匹配小写前缀，自然排除。
func (c *Coordinator) urlActivationResidual(key string) (string, bool) {
	if !c.urlEnabled() || len(key) != 1 {
		return "", false
	}
	cand := c.inputBuffer + key
	for _, p := range c.config.Input.UrlInput.Prefixes {
		if cand == p {
			return cand, true
		}
	}
	return "", false
}

// enterUrlMode 从正常输入夺取进入 URL 模式，residual 为完整前缀（作初始 buffer）。
// 清理当前正常输入状态（前缀字符不单独上屏，转为 URL buffer），进入后原地显示。
func (c *Coordinator) enterUrlMode(residual string) *bridge.KeyEventResult {
	c.clearState()
	c.urlMode = true
	c.urlBuffer = residual
	c.urlCursorPos = len(residual)

	c.logger.Debug("Entered URL mode", "prefixLen", len(residual))

	// 首次进入触发 C++ 端 StartComposition + pendingFirstShow（与其它模式一致）。
	c.armPendingFirstShow()
	c.showUrlUI()
	return c.urlCompositionResult()
}

// handleUrlKey URL 模式按键处理：自由输入网址字符（不上屏），仅空格/回车上屏、退格删空退出。
func (c *Coordinator) handleUrlKey(key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	vk := uint32(data.KeyCode)

	switch {
	// 空格 / 回车：上屏当前 URL buffer 原文
	case vk == ipc.VK_SPACE || vk == ipc.VK_RETURN:
		if len(c.urlBuffer) > 0 {
			return c.exitUrlMode(true, c.urlBuffer)
		}
		return c.exitUrlMode(false, "")

	// ESC：放弃退出
	case vk == ipc.VK_ESCAPE:
		return c.exitUrlMode(false, "")

	// 退格：删光标前一字符；删空则退出
	case vk == ipc.VK_BACK:
		if c.urlCursorPos > 0 && len(c.urlBuffer) > 0 {
			c.urlBuffer = c.urlBuffer[:c.urlCursorPos-1] + c.urlBuffer[c.urlCursorPos:]
			c.urlCursorPos--
			if len(c.urlBuffer) == 0 {
				return c.exitUrlMode(false, "")
			}
			c.showUrlUI()
			return c.urlCompositionResult()
		}
		return c.exitUrlMode(false, "")

	// 左右光标移动
	case vk == ipc.VK_LEFT:
		if c.urlCursorPos > 0 {
			c.urlCursorPos--
			c.showUrlUI()
			return c.urlCompositionResult()
		}
		return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}

	case vk == ipc.VK_RIGHT:
		if c.urlCursorPos < len(c.urlBuffer) {
			c.urlCursorPos++
			c.showUrlUI()
			return c.urlCompositionResult()
		}
		return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}

	// 任意 ASCII 可见字符（字母/数字/标点 . / : - _ ~ ? & = 等）→ 在光标位置追加，不上屏
	case len(key) == 1 && key[0] >= '!' && key[0] <= '~':
		c.urlBuffer = c.urlBuffer[:c.urlCursorPos] + key + c.urlBuffer[c.urlCursorPos:]
		c.urlCursorPos += len(key)
		c.showUrlUI()
		return c.urlCompositionResult()

	default:
		return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}
	}
}

// exitUrlMode 退出 URL 模式（commit 时上屏 text）。
func (c *Coordinator) exitUrlMode(commit bool, text string) *bridge.KeyEventResult {
	c.urlMode = false
	c.urlBuffer = ""
	c.urlCursorPos = 0
	c.preeditDisplay = ""
	c.clearHostUIState()
	c.hideUI()

	c.logger.Debug("Exited URL mode", "commit", commit, "textLen", len(text))

	if commit && len(text) > 0 {
		// URL 文本即原文上屏，归类 SourceRawInput（不另设枚举）。
		c.recordCommit(text, 0, -1, store.SourceRawInput)
		return &bridge.KeyEventResult{Type: bridge.ResponseTypeInsertText, Text: text}
	}
	return &bridge.KeyEventResult{Type: bridge.ResponseTypeClearComposition}
}

// urlCompositionResult 构建 URL 模式编辑区更新（preedit = buffer，光标在 cursorPos）。
// 同步 c.preeditDisplay，使状态快照/UI 反映正在输入的网址。
func (c *Coordinator) urlCompositionResult() *bridge.KeyEventResult {
	c.preeditDisplay = c.urlBuffer
	return c.modeCompositionResult(c.urlBuffer, c.urlCursorPos)
}

// showUrlUI 显示 URL 模式 UI（无候选，仅 preedit + 模式徽标）。
func (c *Coordinator) showUrlUI() {
	if c.uiManager == nil || !c.uiManager.IsReady() {
		return
	}

	caretX := c.caretX
	caretY := c.caretY
	caretHeight := c.caretHeight
	if c.config != nil && c.config.UI.Candidate.InlinePreedit && c.compositionStartValid {
		caretX = c.compositionStartX
		caretY = c.compositionStartY
	}

	const maxCoord = 32000
	if (c.caretX == 0 && c.caretY == 0) || caretX > maxCoord || caretX < -maxCoord || caretY > maxCoord || caretY < -maxCoord {
		if c.lastValidX != 0 || c.lastValidY != 0 {
			caretX = c.lastValidX
			caretY = c.lastValidY
			caretHeight = 20
		} else {
			caretX = 400
			caretY = 300
			caretHeight = 20
		}
	}

	preedit := c.urlBuffer
	caretPosUI := c.urlCursorPos
	// 嵌入编码下刚进入（buffer 为空，理论上不会——前缀非空——防御性置空）。
	if c.isInlinePreedit() && len(c.urlBuffer) == 0 {
		preedit = ""
		caretPosUI = 0
	}

	c.uiManager.SetModeLabel("网址输入")
	c.uiManager.SetModeAccentColor(c.modeAccentColor("url"))
	c.uiManager.ShowCandidates(
		[]ui.Candidate{}, // 无候选
		preedit,
		caretPosUI,
		caretX,
		caretY,
		caretHeight,
		1, // currentPage
		1, // totalPages
		0, // totalCandidates
		c.candidatesPerPage,
		0, // selectedIndex
	)
}
