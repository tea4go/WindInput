//go:build windows || darwin

// golden.go — golden 快照的录制、渲染与断言。
//
// 一个用例 = 若干步驱动操作，每步录一份 StepSnapshot，串成稳定文本写入
// testdata/<name>.golden。改功能后跑 `go test ./internal/e2e`，golden 不一致即报 diff；
// 人工确认变化符合预期后 `go test ./internal/e2e -update` 刷新。
package e2e

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/huanfeng/wind_input/internal/coordinator"
)

var updateGolden = flag.Bool("update", false, "更新 testdata 下的 .golden 快照文件")

// Recorder 包装 Harness 的驱动方法，每次操作后自动录一份快照。链式调用。
// 默认对候选做 weight mask + 截断到当前页，减少词频抖动 / 长列表造成的 golden 噪声。
type Recorder struct {
	h     *Harness
	steps []StepSnapshot
	mask  bool
}

// NewRecorder 创建录制器（默认开启 mask）。
func NewRecorder(h *Harness) *Recorder {
	return &Recorder{h: h, mask: true}
}

// WithRawWeights 关闭 weight mask 与候选截断（做真实词频 / 全量候选断言时用）。
func (r *Recorder) WithRawWeights() *Recorder {
	r.mask = false
	return r
}

func (r *Recorder) snap() { r.steps = append(r.steps, r.h.Snapshot()) }

func (r *Recorder) Type(s string) *Recorder         { r.h.Type(s); r.snap(); return r }
func (r *Recorder) Key(name string) *Recorder       { r.h.Key(name); r.snap(); return r }
func (r *Recorder) SelectCandidate(n int) *Recorder { r.h.SelectCandidate(n); r.snap(); return r }
func (r *Recorder) Space() *Recorder                { r.h.Space(); r.snap(); return r }
func (r *Recorder) Enter() *Recorder                { r.h.Enter(); r.snap(); return r }
func (r *Recorder) Backspace() *Recorder            { r.h.Backspace(); r.snap(); return r }
func (r *Recorder) PageDown() *Recorder             { r.h.PageDown(); r.snap(); return r }
func (r *Recorder) PageUp() *Recorder               { r.h.PageUp(); r.snap(); return r }

// FlushLearning 同步 flush 词频增量（不产生快照步）：让前面选词记录的频次对随后
// Type/Key 的候选查询立即可见，用于词频重排回归。链式调用。
func (r *Recorder) FlushLearning() *Recorder { r.h.FlushLearning(); return r }

// renderStep 是 golden 中每步序列化的结构（command 走 ">>> " 行，不进 JSON）。
type renderStep struct {
	ResultType string            `json:"result_type"`
	CommitText string            `json:"commit_text,omitempty"`
	State      coordinator.State `json:"state"`
}

// Render 把录制的步骤序列渲染成稳定的 golden 文本。
func (r *Recorder) Render() []byte {
	var buf bytes.Buffer
	for _, st := range r.steps {
		fmt.Fprintf(&buf, ">>> %s\n", st.Command)
		rs := renderStep{
			ResultType: st.ResultType,
			CommitText: st.CommitText,
			State:      normalizeState(st.State, r.mask),
		}
		b, err := json.MarshalIndent(rs, "", "  ")
		if err != nil {
			fmt.Fprintf(&buf, "<<marshal error: %v>>\n", err)
			continue
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// normalizeState 在 mask 模式下把候选 weight 清零并截断到「当前页」窗口，降低 golden 脆弱性。
// 截断按 current_page 取窗口 [(page-1)*perPage : page*perPage]：单页用例 start=0，等价于取
// 前 perPage 条；多页用例（如翻页）则随 current_page 显示该页实际候选，使分页 golden 能体现
// 候选窗滑动，而非每页重复同一批。
// 截断前先按 (weight↓, text↑) 稳定排序，使同权重候选的顺序不随词典版本变化而漂移。
func normalizeState(s coordinator.State, mask bool) coordinator.State {
	if !mask {
		return s
	}
	sorted := make([]coordinator.CandidateView, len(s.Candidates))
	copy(sorted, s.Candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Weight != sorted[j].Weight {
			return sorted[i].Weight > sorted[j].Weight
		}
		return sorted[i].Text < sorted[j].Text
	})
	cands := sorted
	if limit := s.CandidatesPerPage; limit > 0 && len(cands) > limit {
		start := 0
		if s.CurrentPage > 1 {
			start = min((s.CurrentPage-1)*limit, len(cands))
		}
		end := min(start+limit, len(cands))
		cands = cands[start:end]
	}
	masked := make([]coordinator.CandidateView, len(cands))
	for i, c := range cands {
		c.Weight = 0
		masked[i] = c
	}
	s.Candidates = masked
	return s
}

// AssertGolden 比对 got 与 testdata/<name>.golden；-update 时改为写入。
func AssertGolden(t testingTB, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("e2e: 创建 testdata 目录失败: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("e2e: 写入 golden %s 失败: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("e2e: 读取 golden %s 失败（首次请运行 -update 生成）: %v", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("e2e: golden 不一致 %s\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

// testingTB 是 *testing.T 用到的最小子集，便于渲染逻辑与断言解耦。
type testingTB interface {
	Helper()
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
}
