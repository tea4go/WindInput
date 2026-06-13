// pipeline_decider_test.go — 第 0 批地基的表驱动单测。
// 重点验证最复杂的 z 键混合回退判定（纯函数，脱离 Coordinator/引擎环境）。
package coordinator

import (
	"io"
	"log/slog"
	"testing"

	"github.com/huanfeng/wind_input/internal/bridge"
)

// discardLogger 返回丢弃输出的 logger，供需要 logSwitch（切换遥测）的宿主状态机测试使用。
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDecideEngineDefaultZFallback(t *testing.T) {
	cases := []struct {
		name             string
		buffer           string
		key              string
		hasPrefixWithKey bool
		isCodeTable      bool
		zIsTrigger       bool
		wantResidual     string
		wantRelease      bool
	}{
		{"z后续失配回退", "z", "q", false, true, true, "q", true},
		{"z后续仍匹配不回退", "z", "h", true, true, true, "", false},
		{"多字符z前缀失配回退", "zhf", "x", false, true, true, "hfx", true},
		{"非z开头不回退", "ji", "a", false, true, true, "", false},
		{"非码表引擎不回退", "z", "q", false, false, true, "", false},
		{"z非临时拼音触发键不回退(门禁)", "z", "q", false, true, false, "", false},
		{"空buffer不回退", "", "a", false, true, true, "", false},
		{"非字母键不回退", "z", "1", false, true, true, "", false},
		{"大写字母不回退(需上游归一)", "z", "Q", false, true, true, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResidual, gotRelease := decideEngineDefaultZFallback(
				tc.buffer, tc.key, tc.hasPrefixWithKey, tc.isCodeTable, tc.zIsTrigger)
			if gotRelease != tc.wantRelease || gotResidual != tc.wantResidual {
				t.Errorf("decideEngineDefaultZFallback(%q,%q,%v,%v,%v) = (%q,%v), want (%q,%v)",
					tc.buffer, tc.key, tc.hasPrefixWithKey, tc.isCodeTable, tc.zIsTrigger,
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

// TestRegistryProcessorsJudgeNilEnginePass 验证所有 registry 宿主在引擎/配置缺失时安全 Pass。
func TestRegistryProcessorsJudgeNilEnginePass(t *testing.T) {
	c := &Coordinator{}
	procs := []Processor{
		newQuickInputProcessor(c),
		newTempPinyinProcessor(c),
		newTempEnglishProcessor(c),
	}
	for _, p := range procs {
		ctx := newDecisionCtx(c, p)
		if d := p.Judge(ctx, ";", &bridge.KeyEventData{Key: ";"}); d.Verdict != VerdictPass {
			t.Errorf("%s.Judge with nil engine/config should Pass, got %v", p.Name(), d.Verdict)
		}
	}
}

// TestTryActivateFromEmptyNoMatch 验证 nil 配置下无触发键匹配时不接管（交旧路径）。
func TestTryActivateFromEmptyNoMatch(t *testing.T) {
	c := &Coordinator{}
	d := newDecider(c)
	if res, ok := d.tryActivateFromEmpty(";", &bridge.KeyEventData{Key: ";"}); ok || res != nil {
		t.Errorf("tryActivateFromEmpty should not activate with nil config, got (%v,%v)", res, ok)
	}
}

// TestDeciderManagedHosts 验证受管宿主集合：temp_pinyin + quick_input + temp_english + special
// 均被 decide() 全接管（special 模式内键受管但触发不在 registry）；engine_default 不算受管。
func TestDeciderManagedHosts(t *testing.T) {
	d := newDecider(&Coordinator{})
	managed := map[string]bool{"temp_pinyin": true, "quick_input": true, "temp_english": true, "special": true}
	for _, p := range []Processor{d.engineDefault, d.tempPinyin, d.quickInput, d.tempEnglish, d.special} {
		if got := d.isManaged(p); got != managed[p.Name()] {
			t.Errorf("isManaged(%s) = %v, want %v", p.Name(), got, managed[p.Name()])
		}
	}
}

// TestDeciderRegistrySharesManagedSingletons 验证 registry 中的受管宿主与单例字段同一——
// 激活后 d.host 与 registry 实例一致，模式内键链能命中。
func TestDeciderRegistrySharesManagedSingletons(t *testing.T) {
	d := newDecider(&Coordinator{})
	want := map[string]Processor{"temp_pinyin": d.tempPinyin, "quick_input": d.quickInput, "temp_english": d.tempEnglish}
	for name, singleton := range want {
		var found Processor
		for _, p := range d.registry {
			if p.Name() == name {
				found = p
			}
		}
		if found == nil {
			t.Fatalf("registry missing %s", name)
		}
		if found != singleton {
			t.Errorf("registry %s must be the managed singleton field", name)
		}
	}
	// special 受管但**不在 registry**（触发走旧 2 步动态匹配，host 经 onSpecialEntered 对齐）。
	for _, p := range d.registry {
		if p == d.special {
			t.Error("special must NOT be in registry (trigger stays on old 2-step path)")
		}
	}
}

// TestReconcileHost 验证退出回落：受管宿主对应模式标志为 false 时 host 回落 engine_default，
// 仍为 true 时 host 保持。temp_pinyin / quick_input 各验一遍。
func TestReconcileHost(t *testing.T) {
	cases := []struct {
		name     string
		host     func(d *decider) Processor
		setMode  func(c *Coordinator, on bool)
		hostName string
	}{
		{"temp_pinyin", func(d *decider) Processor { return d.tempPinyin }, func(c *Coordinator, on bool) { c.tempPinyinMode = on }, "temp_pinyin"},
		{"quick_input", func(d *decider) Processor { return d.quickInput }, func(c *Coordinator, on bool) { c.quickInputMode = on }, "quick_input"},
		{"temp_english", func(d *decider) Processor { return d.tempEnglish }, func(c *Coordinator, on bool) { c.tempEnglishMode = on }, "temp_english"},
		{"special", func(d *decider) Processor { return d.special }, func(c *Coordinator, on bool) { c.specialMode = on }, "special"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Coordinator{logger: discardLogger()}
			d := newDecider(c)

			tc.setMode(c, true)
			d.host = tc.host(d)
			d.reconcileHost()
			if d.host != tc.host(d) {
				t.Errorf("reconcile while mode=true: host=%s, want %s", d.host.Name(), tc.hostName)
			}

			tc.setMode(c, false)
			d.reconcileHost()
			if d.host != d.engineDefault {
				t.Errorf("reconcile after exit: host=%s, want engine_default", d.host.Name())
			}
		})
	}
}

// TestMarkEntered 验证进入对齐：仅当模式真置位时设 host，模式未置位（如引擎加载失败）时不误切。
// 经具名包装 onTempPinyinEntered / onQuickInputEntered 各验一遍。
func TestMarkEntered(t *testing.T) {
	cases := []struct {
		name    string
		enter   func(d *decider)
		target  func(d *decider) Processor
		setMode func(c *Coordinator, on bool)
	}{
		{"temp_pinyin", func(d *decider) { d.onTempPinyinEntered() }, func(d *decider) Processor { return d.tempPinyin }, func(c *Coordinator, on bool) { c.tempPinyinMode = on }},
		{"quick_input", func(d *decider) { d.onQuickInputEntered() }, func(d *decider) Processor { return d.quickInput }, func(c *Coordinator, on bool) { c.quickInputMode = on }},
		{"temp_english", func(d *decider) { d.onTempEnglishEntered() }, func(d *decider) Processor { return d.tempEnglish }, func(c *Coordinator, on bool) { c.tempEnglishMode = on }},
		{"special", func(d *decider) { d.onSpecialEntered() }, func(d *decider) Processor { return d.special }, func(c *Coordinator, on bool) { c.specialMode = on }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Coordinator{logger: discardLogger()}
			d := newDecider(c)

			// 模式未置位：不切（防 setup 失败误切）。
			tc.setMode(c, false)
			tc.enter(d)
			if d.host != d.engineDefault {
				t.Errorf("entered with mode=false: host=%s, want engine_default (no switch)", d.host.Name())
			}

			// 模式已置位：对齐受管单例。
			tc.setMode(c, true)
			tc.enter(d)
			if d.host != tc.target(d) {
				t.Errorf("entered with mode=true: host=%s, want %s", d.host.Name(), tc.name)
			}
		})
	}
}
