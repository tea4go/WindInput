// export_state.go — 导出 Coordinator 完整核心状态的只读快照，供 in-process E2E 测试
// 框架（internal/e2e）与 REPL 读取。生产路径不依赖此文件。
//
// 设计要点：
//   - State 是公开类型，字段全部带 json tag，序列化后即「统一输入输出报告」的状态部分。
//   - ExportState 持 c.mu 期间**只读私有字段 + 调用 NoLock 派生方法**，禁止调用
//     GetChineseMode/GetFullWidth 等自带 c.mu.Lock() 的 getter（c.mu 非可重入，会死锁）。
//   - 候选完整导出，不截断/不 mask；截断与 weight mask 由 golden 序列化层按需处理，
//     这样 REPL 能看到全部候选。
package coordinator

// CandidateView 是候选的可序列化精简视图（仅保留 E2E 断言关心的字段）。
type CandidateView struct {
	Text   string `json:"text"`
	Code   string `json:"code"`
	Weight int    `json:"weight"`
}

// ConfirmedSegmentView 是拼音分步确认段的可序列化视图。
type ConfirmedSegmentView struct {
	Text         string `json:"text"`
	ConsumedCode string `json:"consumed_code"`
}

// State 是 Coordinator 在某一时刻的完整核心状态快照。
type State struct {
	// 模式状态
	ChineseMode        bool   `json:"chinese_mode"`
	FullWidth          bool   `json:"full_width"`
	ChinesePunctuation bool   `json:"chinese_punctuation"`
	EffectiveMode      string `json:"effective_mode"`
	EngineName         string `json:"engine_name"`

	// 输入缓冲与 preedit
	InputBuffer    string `json:"input_buffer"`
	InputCursorPos int    `json:"input_cursor_pos"`
	PreeditDisplay string `json:"preedit_display"`

	// 候选与分页
	Candidates        []CandidateView `json:"candidates"`
	CurrentPage       int             `json:"current_page"`
	TotalPages        int             `json:"total_pages"`
	SelectedIndex     int             `json:"selected_index"`
	CandidatesPerPage int             `json:"candidates_per_page"`

	// 拼音分步确认
	ConfirmedSegments []ConfirmedSegmentView `json:"confirmed_segments,omitempty"`

	// 子模式（非默认态才输出，减少 golden 噪声）
	TempEnglishMode bool `json:"temp_english_mode,omitempty"`
	TempPinyinMode  bool `json:"temp_pinyin_mode,omitempty"`
	QuickInputMode  bool `json:"quick_input_mode,omitempty"`
	SpecialMode     bool `json:"special_mode,omitempty"`
	AddWordActive   bool `json:"add_word_active,omitempty"`
	AddWordLen      int  `json:"add_word_len,omitempty"`
}

// ExportState 返回当前 Coordinator 核心状态的快照。线程安全。
func (c *Coordinator) ExportState() State {
	c.mu.Lock()
	defer c.mu.Unlock()

	candidates := make([]CandidateView, 0, len(c.candidates))
	for _, cand := range c.candidates {
		candidates = append(candidates, CandidateView{
			Text:   cand.Text,
			Code:   cand.Code,
			Weight: cand.Weight,
		})
	}

	var confirmed []ConfirmedSegmentView
	if len(c.confirmedSegments) > 0 {
		confirmed = make([]ConfirmedSegmentView, 0, len(c.confirmedSegments))
		for _, seg := range c.confirmedSegments {
			confirmed = append(confirmed, ConfirmedSegmentView{
				Text:         seg.Text,
				ConsumedCode: seg.ConsumedCode,
			})
		}
	}

	return State{
		ChineseMode:        c.chineseMode,
		FullWidth:          c.fullWidth,
		ChinesePunctuation: c.chinesePunctuation,
		EffectiveMode:      effectiveModeName(c.getEffectiveModeNoLock()),
		EngineName:         c.getCurrentEngineNameNoLock(),
		InputBuffer:        c.inputBuffer,
		InputCursorPos:     c.inputCursorPos,
		PreeditDisplay:     c.preeditDisplay,
		Candidates:         candidates,
		CurrentPage:        c.currentPage,
		TotalPages:         c.totalPages,
		SelectedIndex:      c.selectedIndex,
		CandidatesPerPage:  c.candidatesPerPage,
		ConfirmedSegments:  confirmed,
		TempEnglishMode:    c.tempEnglishMode,
		TempPinyinMode:     c.tempPinyinMode,
		QuickInputMode:     c.quickInputMode,
		SpecialMode:        c.specialMode,
		AddWordActive:      c.addWordActive,
		AddWordLen:         c.addWordLen,
	}
}

// effectiveModeName 把 EffectiveMode 枚举转为稳定的字符串名（golden 可读、跨版本稳定）。
func effectiveModeName(m EffectiveMode) string {
	switch m {
	case ModeChinese:
		return "chinese"
	case ModeEnglishLower:
		return "english_lower"
	case ModeEnglishUpper:
		return "english_upper"
	default:
		return "unknown"
	}
}
