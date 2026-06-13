// pipeline_provider_quickinput.go — 快捷输入基础候选 Provider（date/calc/number）。
//
// 把旧 updateQuickInputCandidates 内 inline 的三路结构化候选（日期/计算/数字）抽成 Provider，
// 经 mergeProviderCandidates 分段合并。Rank 段位 = 旧拼接顺序（date 段含年月日+年月两子段 →
// calc → number），dedup 语义一致，故合并输出与旧 `dedup(allTexts)` 逐条等价（字节级）。
//
// 三者均产出整条上屏候选：Source 空（SourceNone）、ConsumedLength 0，选中走旧
// selectQuickInputCandidate 的整条 commit 路径，与拼音候选（Source=SourcePinyin、
// ConsumedLength>0 分段上屏）天然区分。
package coordinator

import "github.com/huanfeng/wind_input/internal/candidate"

// dateProvider：年月日（三段）+ 年月（两段）日期候选。结构化、Rank 最低（最先呈现）。
// 无状态（Query 只调包级纯函数），故不持 *Coordinator。
type dateProvider struct{}

func (dateProvider) ID() ProviderID { return ProviderDate }
func (dateProvider) Rank() int      { return 10 }

func (dateProvider) Query(buffer string) []candidate.Candidate {
	var texts []string
	if isDateExpression(buffer) {
		texts = append(texts, generateDateCandidates(buffer)...)
	}
	if isYearMonthExpression(buffer) {
		texts = append(texts, generateYearMonthCandidates(buffer)...)
	}
	return textsToCandidates(texts)
}

// calcProvider：计算表达式候选（小数位数取配置 Features.QuickInput.DecimalPlaces）。
type calcProvider struct{ c *Coordinator }

func (calcProvider) ID() ProviderID { return ProviderCalc }
func (calcProvider) Rank() int      { return 20 }

func (p calcProvider) Query(buffer string) []candidate.Candidate {
	if !isCalcExpression(buffer) {
		return nil
	}
	decimalPlaces := 6 // config 缺失时的回落值，须与 pkg/config 默认 DecimalPlaces 一致
	if p.c.config != nil {
		decimalPlaces = p.c.config.Features.QuickInput.DecimalPlaces
	}
	return textsToCandidates(generateCalcCandidates(buffer, decimalPlaces))
}

// numberProvider：数字/小数候选。无状态（同 dateProvider），不持 *Coordinator。
type numberProvider struct{}

func (numberProvider) ID() ProviderID { return ProviderNumber }
func (numberProvider) Rank() int      { return 30 }

func (numberProvider) Query(buffer string) []candidate.Candidate {
	if !isDecimalNumber(buffer) {
		return nil
	}
	return textsToCandidates(generateNumberCandidates(buffer))
}

// textsToCandidates 把纯文本候选包成 candidate.Candidate（Source 空 = 整条上屏）。
// 空输入返回 nil，由合并器跳过。
func textsToCandidates(texts []string) []candidate.Candidate {
	if len(texts) == 0 {
		return nil
	}
	out := make([]candidate.Candidate, len(texts))
	for i, t := range texts {
		out[i] = candidate.Candidate{Text: t}
	}
	return out
}

// quickInputBaseProviders 返回快捷输入「结构化」候选源（date/calc/number），用于非拼音上下文。
// 拼音上下文（quickInputPinyinActive）走共享的 updatePinyinModeCandidates（内部经 pinyinProvider
// 取源），与结构化候选 XOR——二者由 buffer 内容互斥，不在同一列表合并。
func (c *Coordinator) quickInputBaseProviders() []CandidateProvider {
	return []CandidateProvider{
		dateProvider{},
		calcProvider{c: c},
		numberProvider{},
	}
}
