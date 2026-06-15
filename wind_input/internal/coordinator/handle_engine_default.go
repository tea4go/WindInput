// handle_engine_default.go — engine_default 兜底宿主的「正常输入」按键处理。
//
// 由 engineDefaultProcessor.KeyHandlers() 的 engineDefaultKeyHandler.Apply 调用（决策器链分发，
// 与其它宿主同构）。本方法是原 HandleKeyEvent 末尾 switch 的**逐字节搬迁**：导航/光标/编辑/上屏/
// 字母（含 z 混合回退）/数字/选词/拼音分隔符/标点。prevDigitState 经 c.keyPrevDigitState 传入。
package coordinator

import (
	"strings"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/ipc"
	"github.com/huanfeng/wind_input/internal/store"
	"github.com/huanfeng/wind_input/internal/transform"
)

// handleEngineDefaultKey 处理 engine_default 宿主（普通码表/拼音输入态）的按键。
func (c *Coordinator) handleEngineDefaultKey(key string, data *bridge.KeyEventData) *bridge.KeyEventResult {
	// Chinese mode handling
	vk := uint32(data.KeyCode)
	hasShift := data.Modifiers&ModShift != 0

	// 自动配对：方向键、Enter、Escape 等清空配对栈
	if c.pairTracker != nil {
		switch vk {
		case ipc.VK_LEFT, ipc.VK_RIGHT, ipc.VK_UP, ipc.VK_DOWN,
			ipc.VK_HOME, ipc.VK_END, ipc.VK_RETURN, ipc.VK_ESCAPE:
			c.pairTracker.Clear()
		}
	}
	if c.pairTrackerEn != nil {
		switch vk {
		case ipc.VK_LEFT, ipc.VK_RIGHT, ipc.VK_UP, ipc.VK_DOWN,
			ipc.VK_HOME, ipc.VK_END, ipc.VK_RETURN, ipc.VK_ESCAPE:
			c.pairTrackerEn.Clear()
		}
	}

	switch {
	case c.isHighlightUpKey(vk, uint32(data.Modifiers)):
		return c.handleArrowUp()

	case c.isHighlightDownKey(vk, uint32(data.Modifiers)):
		return c.handleArrowDown()

	case vk == ipc.VK_LEFT:
		return c.handleCursorLeft()

	case vk == ipc.VK_RIGHT:
		return c.handleCursorRight()

	case vk == ipc.VK_HOME:
		return c.handleCursorHome()

	case vk == ipc.VK_END:
		return c.handleCursorEnd()

	case vk == ipc.VK_BACK:
		return c.handleBackspace()

	case vk == ipc.VK_DELETE:
		return c.handleDelete()

	case vk == ipc.VK_RETURN:
		return c.handleEnter()

	case vk == ipc.VK_ESCAPE:
		return c.handleEscape()

	case vk == ipc.VK_SPACE:
		return c.handleSpace()

	case !hasShift && c.isSelectCharFirstKey(key, data.KeyCode):
		if result := c.handleSelectCharWithOverflow(0, key, c.keyPrevDigitState, data.PrevChar); result != nil {
			return result
		}
		return nil

	case !hasShift && c.isSelectCharSecondKey(key, data.KeyCode):
		if result := c.handleSelectCharWithOverflow(1, key, c.keyPrevDigitState, data.PrevChar); result != nil {
			return result
		}
		return nil

	case c.isPageUpKey(key, data.KeyCode, uint32(data.Modifiers)):
		if result := c.handlePageUp(); result != nil {
			return result
		}
		// No candidates — fall through to punctuation if applicable
		if len(key) == 1 && c.isPunctuation(rune(key[0])) {
			return c.handlePunctuation(rune(key[0]), c.keyPrevDigitState, data.PrevChar)
		}
		return nil

	case c.isPageDownKey(key, data.KeyCode, uint32(data.Modifiers)):
		if result := c.handlePageDown(); result != nil {
			return result
		}
		// No candidates — fall through to punctuation if applicable
		if len(key) == 1 && c.isPunctuation(rune(key[0])) {
			return c.handlePunctuation(rune(key[0]), c.keyPrevDigitState, data.PrevChar)
		}
		return nil

	case vk == ipc.VK_TAB:
		// Tab 安全网：输入态下始终消费，防止透传给宿主程序导致焦点跳转
		// 如果 Tab 已被 isHighlightDownKey/UpKey 匹配则不会到达此处
		if c.hasPendingInput() {
			return &bridge.KeyEventResult{Type: bridge.ResponseTypeConsumed}
		}
		return nil

	case len(key) == 1 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z')):
		lowerKey := strings.ToLower(key)
		// z 键混合回退：决策器判定（engine_default 宿主裁决，含 z 触发键门禁，见
		// pipeline_engine_default.go）。执行复用 enterTempPinyinFromZBuffer（CompHot 原地切换）。
		if buf, ok := c.decider.judgeZFallback(lowerKey, data); ok {
			res := c.enterTempPinyinFromZBuffer(buf, c.inputBuffer)
			// CompHot 进入 temp_pinyin：对齐受管宿主 host，后续模式内键走 dispatchHostChain。
			c.decider.onTempPinyinEntered()
			return res
		}
		// Chinese mode: convert to lowercase for pinyin
		return c.handleAlphaKey(lowerKey)

	case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
		result := c.handleNumberKey(int(key[0] - '0'))
		if result == nil {
			// 数字直通（无候选词选择），标记用于智能标点
			if c.pairTracker != nil {
				c.pairTracker.Clear()
			}
			if c.pairTrackerEn != nil {
				c.pairTrackerEn.Clear()
			}
			c.lastOutputWasDigit = true
			// 空码状态：有待处理输入但无候选，必须显式清空并上屏数字；
			// 透传（nil）会让应用得到数字但 composition 不会结束，导致状态混乱。
			if c.hasPendingInput() {
				c.clearState()
				c.hideUI()
				digit := key
				if c.fullWidth {
					digit = transform.ToFullWidth(key)
				}
				return &bridge.KeyEventResult{
					Type: bridge.ResponseTypeInsertText,
					Text: digit,
				}
			}
			// 全角模式下输出全角数字
			if c.fullWidth {
				return &bridge.KeyEventResult{
					Type: bridge.ResponseTypeInsertText,
					Text: transform.ToFullWidth(key),
				}
			}
			// 透传路径：result 为 nil，defer fallback 不会触发，需主动记录
			c.recordCommit(key, 0, -1, store.SourcePunctuation)
		}
		return result

	case len(key) == 1 && key[0] == '0':
		result := c.handleNumberKey(10)
		if result == nil {
			if c.pairTracker != nil {
				c.pairTracker.Clear()
			}
			if c.pairTrackerEn != nil {
				c.pairTrackerEn.Clear()
			}
			c.lastOutputWasDigit = true
			// 空码状态：有待处理输入但无候选，必须显式清空并上屏数字；
			// 透传（nil）会让应用得到数字但 composition 不会结束，导致状态混乱。
			if c.hasPendingInput() {
				c.clearState()
				c.hideUI()
				digit := key
				if c.fullWidth {
					digit = transform.ToFullWidth(key)
				}
				return &bridge.KeyEventResult{
					Type: bridge.ResponseTypeInsertText,
					Text: digit,
				}
			}
			// 全角模式下输出全角数字
			if c.fullWidth {
				return &bridge.KeyEventResult{
					Type: bridge.ResponseTypeInsertText,
					Text: transform.ToFullWidth(key),
				}
			}
			// 透传路径：result 为 nil，defer fallback 不会触发，需主动记录
			c.recordCommit(key, 0, -1, store.SourcePunctuation)
		}
		return result

	case !hasShift && c.isSelectKey2(key, data.KeyCode):
		// buffer 非空时的二候选/overflow 已由 routeBufferedTriggerKey 接管，
		// 这里只处理无输入缓冲时的标点回退。
		if len(c.inputBuffer) == 0 && len(key) == 1 && c.isPunctuation(rune(key[0])) {
			return c.handlePunctuation(rune(key[0]), c.keyPrevDigitState, data.PrevChar)
		}
		return nil

	case !hasShift && c.isPinyinSeparator(key, data.KeyCode):
		return c.handlePinyinSeparator()

	case !hasShift && c.isSelectKey3(key, data.KeyCode):
		// buffer 非空时的三候选/overflow 已由 routeBufferedTriggerKey 接管，
		// 这里只处理无输入缓冲时的标点回退。
		if len(c.inputBuffer) == 0 && len(key) == 1 && c.isPunctuation(rune(key[0])) {
			return c.handlePunctuation(rune(key[0]), c.keyPrevDigitState, data.PrevChar)
		}
		return nil

	case len(key) == 1 && c.isPunctuation(rune(key[0])):
		return c.handlePunctuation(rune(key[0]), c.keyPrevDigitState, data.PrevChar)

	default:
		c.logger.Debug("Unhandled key", "key", key, "keycode", data.KeyCode)
		return nil
	}
}
