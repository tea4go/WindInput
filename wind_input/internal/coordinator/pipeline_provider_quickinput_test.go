package coordinator

import (
	"slices"
	"testing"
)

// localDedup 复刻旧 updateQuickInputCandidates 的去重（保序保首现），用于字节对拍基准。
func localDedup(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		if _, ok := seen[it]; !ok {
			seen[it] = struct{}{}
			out = append(out, it)
		}
	}
	return out
}

// oldQuickInputTexts 复刻旧 inline 候选逻辑（date→yearMonth→calc→number 拼接 + dedup），
// 作为「抽 Provider 后字节级等价」的对拍基准。两条路径调用完全相同的 is*/generate* 函数，
// 故差异只可能来自合并段位顺序或去重语义——正是本测试要锁住的不变量。
func oldQuickInputTexts(buf string, decimals int) []string {
	var all []string
	if isDateExpression(buf) {
		all = append(all, generateDateCandidates(buf)...)
	}
	if isYearMonthExpression(buf) {
		all = append(all, generateYearMonthCandidates(buf)...)
	}
	if isCalcExpression(buf) {
		all = append(all, generateCalcCandidates(buf, decimals)...)
	}
	if isDecimalNumber(buf) {
		all = append(all, generateNumberCandidates(buf)...)
	}
	return localDedup(all)
}

func TestQuickInputProvidersByteIdentical(t *testing.T) {
	c := &Coordinator{} // config 为 nil → calc 小数位数取默认 6

	buffers := []string{
		"2024.1.1", "2024.12.31", "2.3", "2024.12",
		"1+1", "1+2*3", "(1+2)*3", "100/3",
		"123", "12.5", "0.5", "1000000",
		"2024.1.1+1", // date 与 calc 可能同时命中，验证分段顺序 + 跨段去重
		"abc",        // 全不匹配
		"",           // 空 buffer
	}
	for _, buf := range buffers {
		want := oldQuickInputTexts(buf, 6)
		got := textsOf(mergeProviderCandidates(buf, c.quickInputBaseProviders()))
		if !slices.Equal(got, want) {
			t.Errorf("buf=%q 字节不等价\n  got  = %v\n  want = %v", buf, got, want)
		}
	}
}

// TestQuickInputProvidersMeta 锁定 provider 的 ID/Rank 段位顺序（date<calc<number）。
func TestQuickInputProvidersMeta(t *testing.T) {
	c := &Coordinator{}
	ps := c.quickInputBaseProviders()
	if len(ps) != 3 {
		t.Fatalf("want 3 base providers, got %d", len(ps))
	}
	want := []struct {
		id   ProviderID
		rank int
	}{
		{ProviderDate, 10},
		{ProviderCalc, 20},
		{ProviderNumber, 30},
	}
	for i, w := range want {
		if ps[i].ID() != w.id || ps[i].Rank() != w.rank {
			t.Errorf("provider[%d]: got id=%q rank=%d, want id=%q rank=%d",
				i, ps[i].ID(), ps[i].Rank(), w.id, w.rank)
		}
	}
}

// TestQuickInputProvidersSourceEmpty 验证 date/calc/number 候选 Source 空、ConsumedLength 0
// （= 整条上屏路径，与拼音的分段上屏天然区分）。
func TestQuickInputProvidersSourceEmpty(t *testing.T) {
	c := &Coordinator{}
	got := mergeProviderCandidates("2024.1.1", c.quickInputBaseProviders())
	if len(got) == 0 {
		t.Fatal("expected date candidates, got none")
	}
	for _, cand := range got {
		if cand.Source != "" || cand.ConsumedLength != 0 {
			t.Errorf("结构化候选应整条上屏：Source=%q ConsumedLength=%d", cand.Source, cand.ConsumedLength)
		}
	}
}
