// z_first_trigger_test.go — z 首次触发仲裁收编进决策器的回归测试。
//
// 验证 P5 收尾：z 首触发不再走 handle_key_event.go 的 getTempPinyinTriggerKey 旁路，
// 而是经 tempPinyinProcessor.Judge（judgeZFirstTrigger）作为 registry 激活裁决产出。
// judgeZFirstTrigger 直接复用旧 getTempPinyinTriggerKey 的 z 渐进仲裁，保证与旧路径等价：
//   - z 无码表前缀（死前缀）且无重复历史 → 进临时拼音；
//   - z 仍有码表前缀 → 不进（渐进，留给正常码表输入）；
//   - 开启 z-repeat 且有上屏历史 → 不进（z 留作重复上屏）。
package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
)

const vkZ = 0x5A // VK 'Z'

// TestJudgeZFirstTrigger 表驱动验证 z 首触发渐进仲裁（judgeZFirstTrigger 委托旧逻辑）。
func TestJudgeZFirstTrigger(t *testing.T) {
	type entry struct{ code, text string }
	cases := []struct {
		name    string
		zRepeat bool
		entries []entry
		history string // 非空则预置一条上屏历史
		key     string
		want    bool
	}{
		{
			// z 是死前缀（码表里没有 z* 条目）、无重复历史 → 进临时拼音
			name:    "dead_z_prefix_no_repeat",
			zRepeat: false,
			entries: []entry{{"abc", "X"}},
			key:     "z",
			want:    true,
		},
		{
			// z 仍有码表前缀（zhang）→ 渐进决策不进，留给正常码表
			name:    "z_has_codetable_prefix",
			zRepeat: false,
			entries: []entry{{"zhang", "张"}},
			key:     "z",
			want:    false,
		},
		{
			// 开启 z-repeat 且有上屏历史 → z 留作重复上屏，不进临时拼音
			name:    "z_repeat_with_history",
			zRepeat: true,
			entries: []entry{{"abc", "X"}},
			history: "你好",
			key:     "z",
			want:    false,
		},
		{
			// 开启 z-repeat 但无历史 → repeat 分支跳过，死前缀仍进临时拼音
			name:    "z_repeat_no_history",
			zRepeat: true,
			entries: []entry{{"abc", "X"}},
			key:     "z",
			want:    true,
		},
		{
			// 非 z 键 → 不命中 z 首触发
			name:    "non_z_key",
			zRepeat: false,
			entries: []entry{{"abc", "X"}},
			key:     "a",
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engineOpts := make([]engineOption, 0, len(tc.entries))
			for _, e := range tc.entries {
				engineOpts = append(engineOpts, withCodetableEntry(e.code, e.text))
			}
			h := newTestCoordinator(t, withEngineMgr(engineOpts...), withZHybridSchema(tc.zRepeat))
			if tc.history != "" {
				h.inputHistory.Record(tc.history, "", "", 0)
			}

			if got := h.judgeZFirstTrigger(tc.key, vkZ); got != tc.want {
				t.Errorf("judgeZFirstTrigger(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

// TestTempPinyinProcessorJudgeZActivate 验证 z 首触发经决策器 registry 产出 Activate("z")——
// 这是 z 收编进决策器的核心：tryActivateFromEmpty 遍历到 tempPinyinProcessor 时，其 Judge
// 对死前缀 z 返回 VerdictActivate，等价旧 getTempPinyinTriggerKey 返回 "z"。
func TestTempPinyinProcessorJudgeZActivate(t *testing.T) {
	h := newTestCoordinator(t,
		withEngineMgr(withCodetableEntry("abc", "X")), // 无 z 前缀 → z 为死前缀
		withZHybridSchema(false),
	)
	tpp := newTempPinyinProcessor(h.Coordinator)
	// ctx host=engine_default，对齐 tryActivateFromEmpty 在 buffer 空时的真实上下文。
	ctx := newDecisionCtx(h.Coordinator, newEngineDefaultProcessor(h.Coordinator))

	dec := tpp.Judge(ctx, "z", &bridge.KeyEventData{Key: "z", KeyCode: vkZ})
	if dec.Verdict != VerdictActivate {
		t.Fatalf("Judge(z) verdict = %v, want Activate", dec.Verdict)
	}
	if dec.TriggerKey != "z" {
		t.Errorf("Judge(z) TriggerKey = %q, want \"z\"", dec.TriggerKey)
	}
}

// TestTempPinyinProcessorJudgeZPrefixPass 验证 z 仍有码表前缀时 Judge 不激活（Pass），
// 渐进决策把 z 留给正常码表输入。
func TestTempPinyinProcessorJudgeZPrefixPass(t *testing.T) {
	h := newTestCoordinator(t,
		withEngineMgr(withCodetableEntry("zhang", "张")), // z 有前缀
		withZHybridSchema(false),
	)
	tpp := newTempPinyinProcessor(h.Coordinator)
	ctx := newDecisionCtx(h.Coordinator, newEngineDefaultProcessor(h.Coordinator))

	if dec := tpp.Judge(ctx, "z", &bridge.KeyEventData{Key: "z", KeyCode: vkZ}); dec.Verdict != VerdictPass {
		t.Errorf("Judge(z) with codetable prefix = %v, want Pass", dec.Verdict)
	}
}

// TestTempPinyinProcessorJudgeZBufferGuard 验证 buffer 非空时 Judge 不激活 z 首触发
// （ctx.BufferLen()==0 门禁）——已在输入中不应被 z 首触发打断。
func TestTempPinyinProcessorJudgeZBufferGuard(t *testing.T) {
	h := newTestCoordinator(t,
		withEngineMgr(withCodetableEntry("abc", "X")),
		withZHybridSchema(false),
	)
	h.inputBuffer = "ab" // engine_default host 的 buffer 非空
	tpp := newTempPinyinProcessor(h.Coordinator)
	ctx := newDecisionCtx(h.Coordinator, newEngineDefaultProcessor(h.Coordinator))

	if dec := tpp.Judge(ctx, "z", &bridge.KeyEventData{Key: "z", KeyCode: vkZ}); dec.Verdict != VerdictPass {
		t.Errorf("Judge(z) with non-empty buffer = %v, want Pass", dec.Verdict)
	}
}
