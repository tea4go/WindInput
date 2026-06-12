// pipeline_decider_test.go — 第 0 批地基的表驱动单测。
// 重点验证最复杂的 z 键混合回退判定（纯函数，脱离 Coordinator/引擎环境）。
package coordinator

import (
	"io"
	"log/slog"
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
)

func TestDecideEngineDefaultZFallback(t *testing.T) {
	cases := []struct {
		name             string
		buffer           string
		key              string
		hasPrefixWithKey bool
		isCodeTable      bool
		wantResidual     string
		wantRelease      bool
	}{
		{"z后续失配回退", "z", "q", false, true, "q", true},
		{"z后续仍匹配不回退", "z", "h", true, true, "", false},
		{"多字符z前缀失配回退", "zhf", "x", false, true, "hfx", true},
		{"非z开头不回退", "ji", "a", false, true, "", false},
		{"非码表引擎不回退", "z", "q", false, false, "", false},
		{"空buffer不回退", "", "a", false, true, "", false},
		{"非字母键不回退", "z", "1", false, true, "", false},
		{"大写字母不回退(需上游归一)", "z", "Q", false, true, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResidual, gotRelease := decideEngineDefaultZFallback(
				tc.buffer, tc.key, tc.hasPrefixWithKey, tc.isCodeTable)
			if gotRelease != tc.wantRelease || gotResidual != tc.wantResidual {
				t.Errorf("decideEngineDefaultZFallback(%q,%q,%v,%v) = (%q,%v), want (%q,%v)",
					tc.buffer, tc.key, tc.hasPrefixWithKey, tc.isCodeTable,
					gotResidual, gotRelease, tc.wantResidual, tc.wantRelease)
			}
		})
	}
}

func TestVerdictString(t *testing.T) {
	cases := map[Verdict]string{
		VerdictPass:     "Pass",
		VerdictHandle:   "Handle",
		VerdictActivate: "Activate",
		VerdictRelease:  "Release",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("Verdict(%d).String() = %q, want %q", v, got, want)
		}
	}
}

func TestCompositionPhaseString(t *testing.T) {
	cases := map[CompositionPhase]string{
		CompCold:   "Cold",
		CompHot:    "Hot",
		CompCommit: "Commit",
		CompEnd:    "End",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("CompositionPhase(%d).String() = %q, want %q", p, got, want)
		}
	}
}

func TestDecisionConstructors(t *testing.T) {
	if d := decActivate("semicolon", 3); d.Verdict != VerdictActivate || d.TriggerKey != "semicolon" || d.CommitIdx != 3 {
		t.Errorf("decActivate mismatch: %+v", d)
	}
	if d := decRelease("hfx"); d.Verdict != VerdictRelease || d.Residual != "hfx" {
		t.Errorf("decRelease mismatch: %+v", d)
	}
	if decPass().Verdict != VerdictPass || decHandle().Verdict != VerdictHandle {
		t.Errorf("decPass/decHandle mismatch")
	}
}

// TestDeciderSkeleton 验证第 0 批决策器骨架：host 永不为空（默认 engine_default），
// 且在尚无 KeyHandler 的情况下不接管任何按键（交旧路径，handled=false）。
func TestDeciderSkeleton(t *testing.T) {
	c := &Coordinator{}
	d := newDecider(c)

	if d.host == nil {
		t.Fatal("host must never be nil (I1)")
	}
	if d.host.Name() != "engine_default" {
		t.Fatalf("default host = %q, want engine_default", d.host.Name())
	}
	if len(d.keyHandlerChain()) != 0 {
		t.Errorf("phase-0 chain should be empty, got %d handlers", len(d.keyHandlerChain()))
	}

	// 第 0 批：链为空，普通字母不被接管，交旧路径。
	res, handled := d.decide("a", &bridge.KeyEventData{Key: "a"})
	if handled || res != nil {
		t.Errorf("phase-0 decide should not handle, got (%v, %v)", res, handled)
	}
}

// TestShadowLogSmoke 验证第 0b 影子运行入口纯只读、不 panic（host.Judge + DEBUG 日志）。
func TestShadowLogSmoke(t *testing.T) {
	c := &Coordinator{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	d := newDecider(c)
	// 普通字母 / z 键 / 非字母，均不应 panic 且不改变状态。
	d.shadowLog("a", &bridge.KeyEventData{Key: "a"})
	d.shadowLog("z", &bridge.KeyEventData{Key: "z"})
	d.shadowLog(";", &bridge.KeyEventData{Key: ";"})
	if c.inputBuffer != "" || len(c.candidates) != 0 {
		t.Errorf("shadowLog must be read-only, but state changed: buffer=%q cands=%d", c.inputBuffer, len(c.candidates))
	}
}

// TestDecisionCtxBufferByHost 验证 buffer 访问器按 host 路由（修正第 0b 影子暴露的失真）：
// engine_default 读 inputBuffer、temp_pinyin 读 tempPinyinBuffer，而非固定读 inputBuffer。
func TestDecisionCtxBufferByHost(t *testing.T) {
	c := &Coordinator{inputBuffer: "abc", tempModeState: tempModeState{tempPinyinBuffer: "nihao"}}

	edCtx := newDecisionCtx(c, newEngineDefaultProcessor(c))
	if edCtx.BufferText() != "abc" || edCtx.BufferLen() != 3 {
		t.Errorf("engine_default host: BufferText=%q Len=%d, want abc/3", edCtx.BufferText(), edCtx.BufferLen())
	}

	tpCtx := newDecisionCtx(c, newTempPinyinProcessor(c))
	if tpCtx.BufferText() != "nihao" || tpCtx.BufferLen() != 5 {
		t.Errorf("temp_pinyin host: BufferText=%q Len=%d, want nihao/5", tpCtx.BufferText(), tpCtx.BufferLen())
	}
}

// TestTempPinyinJudgeNilEngine 验证 temp_pinyin.Judge 在引擎/配置缺失时安全回落 Pass
// （matchTempPinyinTrigger 的 engineMgr==nil / config==nil 门禁）。激活路径需真引擎，留集成测试。
func TestTempPinyinJudgeNilEngine(t *testing.T) {
	c := &Coordinator{}
	tpp := newTempPinyinProcessor(c)
	ctx := newDecisionCtx(c, tpp)
	if d := tpp.Judge(ctx, "`", &bridge.KeyEventData{Key: "`"}); d.Verdict != VerdictPass {
		t.Errorf("nil engine/config should Pass, got %v", d.Verdict)
	}
}
