package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/candidate"
)

// fakeProvider 是测试用的桩 provider：固定 id/rank，按 buffer 返回预置候选。
type fakeProvider struct {
	id    ProviderID
	rank  int
	byBuf map[string][]candidate.Candidate
}

func (f fakeProvider) ID() ProviderID { return f.id }
func (f fakeProvider) Rank() int      { return f.rank }
func (f fakeProvider) Query(buffer string) []candidate.Candidate {
	return f.byBuf[buffer]
}

func cands(texts ...string) []candidate.Candidate {
	out := make([]candidate.Candidate, len(texts))
	for i, t := range texts {
		out[i] = candidate.Candidate{Text: t}
	}
	return out
}

func textsOf(cs []candidate.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Text
	}
	return out
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMergeProviderCandidates(t *testing.T) {
	high := fakeProvider{id: ProviderPinyin, rank: 40, byBuf: map[string][]candidate.Candidate{
		"ni": cands("你", "尼", "泥"),
	}}
	low := fakeProvider{id: ProviderDate, rank: 10, byBuf: map[string][]candidate.Candidate{
		"ni": cands("尼"), // 与 high 的 "尼" 重复，低 rank 先到留存
	}}

	cases := []struct {
		name      string
		buffer    string
		providers []CandidateProvider
		want      []string
	}{
		{"空buffer返回nil", "", []CandidateProvider{low, high}, nil},
		{"无provider返回nil", "ni", nil, nil},
		{"单provider直通", "ni", []CandidateProvider{high}, []string{"你", "尼", "泥"}},
		{
			"按rank分段拼接+去重保留首现",
			"ni",
			[]CandidateProvider{high, low}, // 乱序传入，按 rank 排序后 low(10) 在前
			[]string{"尼", "你", "泥"},        // low 的"尼"先到 → high 的"尼"被去重
		},
		{"全不匹配返回空", "zzz", []CandidateProvider{low, high}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeProviderCandidates(tc.buffer, tc.providers)
			if !eqStrings(textsOf(got), tc.want) {
				t.Errorf("%s: got %v, want %v", tc.name, textsOf(got), tc.want)
			}
		})
	}
}

// TestMergePreservesLineage 验证合并不丢失候选血缘（Source/ConsumedLength）——
// 这是分段上屏 commit dispatch 的依据，不能被合并步骤清零。
func TestMergePreservesLineage(t *testing.T) {
	py := candidate.Candidate{Text: "你", Source: candidate.SourcePinyin, ConsumedLength: 2}
	p := fakeProvider{id: ProviderPinyin, rank: 40, byBuf: map[string][]candidate.Candidate{
		"ni": {py},
	}}
	got := mergeProviderCandidates("ni", []CandidateProvider{p})
	if len(got) != 1 {
		t.Fatalf("want 1 candidate, got %d", len(got))
	}
	if got[0].Source != candidate.SourcePinyin || got[0].ConsumedLength != 2 {
		t.Errorf("lineage lost: Source=%q ConsumedLength=%d", got[0].Source, got[0].ConsumedLength)
	}
}
