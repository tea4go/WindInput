package mixed

import "testing"

// TestSuppressNonPinyinPreedit verifies overflow pinyin segmentation suppression.
// Uses built-in syllable trie via NewEngine(nil,...); no prebuilt dict required.
func TestSuppressNonPinyinPreedit(t *testing.T) {
	engine := NewEngine(nil, nil, nil, nil)

	tests := []struct {
		input    string
		wantKept bool
		desc     string
	}{
		{"nihao", true, "ni+hao full coverage"},
		{"woaini", true, "wo+ai+ni full coverage"},
		{"nihaoz", true, "ni+hao + legal tail prefix z"},
		{"nihaozk", false, "ni+hao + illegal tail zk"},
		{"abcde", false, "leading single-letter a, fragmented"},
		{"gggge", false, "no leading contiguous syllable"},
	}

	for _, tt := range tests {
		result := &ConvertResult{
			IsPinyinFallback:   true,
			PreeditDisplay:     "x x y",
			CompletedSyllables: []string{"x", "x"},
			PartialSyllable:    "y",
			HasPartial:         true,
		}
		engine.suppressNonPinyinPreedit(tt.input, result)

		kept := result.PreeditDisplay != ""
		if kept != tt.wantKept {
			t.Errorf("input=%q: PreeditDisplay=%q (kept=%v), want kept=%v",
				tt.input, result.PreeditDisplay, kept, tt.wantKept)
		}
		if !tt.wantKept {
			if len(result.CompletedSyllables) != 0 || result.PartialSyllable != "" || result.HasPartial {
				t.Errorf("input=%q: residual syllable fields after suppress: completed=%v partial=%q hasPartial=%v",
					tt.input, result.CompletedSyllables, result.PartialSyllable, result.HasPartial)
			}
		}
	}
}
