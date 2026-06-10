package engine

// PinyinOptions 拼音引擎热更新选项——方案 spec（schema.PinyinSpec）到引擎
// 内部配置（pinyin.Config / pinyin.FuzzyConfig）的中介结构。
// 原为 pkg/config 的 PinyinConfig/FuzzyPinyinConfig；它们从不挂在应用 Config
// 树上，随配置重构（docs/design/config-restructure.md §3 归属说明）迁入本包，
// pkg/config 只保留应用级配置。
type PinyinOptions struct {
	ShowCodeHint    bool
	UseSmartCompose bool
	CandidateOrder  string // 候选排序：char_first/phrase_first/smart
	Fuzzy           FuzzyPinyinOptions
	SkipAbbrev      bool // 跳过简拼匹配（混输模式专用）
}

// FuzzyPinyinOptions 模糊拼音开关集（11 个独立开关，可任意组合）。
type FuzzyPinyinOptions struct {
	Enabled bool // 总开关
	ZhZ     bool // zh ↔ z
	ChC     bool // ch ↔ c
	ShS     bool // sh ↔ s
	NL      bool // n ↔ l
	FH      bool // f ↔ h
	RL      bool // r ↔ l
	AnAng   bool // an ↔ ang
	EnEng   bool // en ↔ eng
	InIng   bool // in ↔ ing
	IanIang bool // ian ↔ iang
	UanUang bool // uan ↔ uang
}
