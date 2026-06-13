// pipeline_comp_phase_test.go — CompositionPhase 推导与 Capability diff 纯函数的表驱动单测。
package coordinator

import "testing"

func TestComputeCapabilityDiff(t *testing.T) {
	cases := []struct {
		name             string
		old, new         Capability
		wantAdd, wantRem Capability
	}{
		{"空→拼音层(挂载)", 0, CapPinyinLayer, CapPinyinLayer, 0},
		{"拼音层→空(卸载)", CapPinyinLayer, 0, 0, CapPinyinLayer},
		{"拼音层→拼音层(去抖,不动)", CapPinyinLayer, CapPinyinLayer, 0, 0},
		{"空→空", 0, 0, 0, 0},
		{"拼音→英文(换层)", CapPinyinLayer, CapEnglishDict, CapEnglishDict, CapPinyinLayer},
		{"拼音→拼音+英文(仅挂英文)", CapPinyinLayer, CapPinyinLayer | CapEnglishDict, CapEnglishDict, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			add, rem := computeCapabilityDiff(tc.old, tc.new)
			if add != tc.wantAdd || rem != tc.wantRem {
				t.Errorf("computeCapabilityDiff(%d,%d) = (add=%d,rem=%d), want (add=%d,rem=%d)",
					tc.old, tc.new, add, rem, tc.wantAdd, tc.wantRem)
			}
		})
	}
}

func TestDeriveCompositionPhase(t *testing.T) {
	ed := newEngineDefaultProcessor(&Coordinator{})
	tp := newTempPinyinProcessor(&Coordinator{})
	qi := newQuickInputProcessor(&Coordinator{})

	cases := []struct {
		name     string
		from, to Processor
		want     CompositionPhase
	}{
		{"engine_default→temp_pinyin(冷启)", ed, tp, CompCold},
		{"temp_pinyin→engine_default(结束)", tp, ed, CompEnd},
		{"模式A→模式B(热切)", tp, qi, CompHot},
		{"engine_default→engine_default(退化冷)", ed, ed, CompCold},
		{"模式→自身(热切)", tp, tp, CompHot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveCompositionPhase(tc.from, tc.to, ed); got != tc.want {
				t.Errorf("deriveCompositionPhase(%s→%s) = %s, want %s",
					tc.from.Name(), tc.to.Name(), got, tc.want)
			}
		})
	}
}
