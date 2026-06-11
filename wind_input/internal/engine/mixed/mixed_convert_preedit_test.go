package mixed

import (
	"strings"
	"testing"

	"github.com/huanfeng/wind_input/internal/dict"
	"github.com/huanfeng/wind_input/internal/engine/pinyin"
)

// newEmptyDictMixedEngine 构建一个 codetableEngine=nil、拼音引擎挂空词库的混输引擎。
//
// 关键点：拼音引擎的音节切分（Composition/PreeditDisplay）由硬编码 SyllableTrie 完成，
// 不依赖词库候选（见 engine_ex.go convertCore：Composition 在词库查询前即算出），
// 因此无需构建词库即可验证 convertMixed 的拼音预编辑分段逻辑。
// codetableEngine=nil 时 maxCodeLen 取默认值 4，故 2~4 码走 convertMixed 路径。
func newEmptyDictMixedEngine(t *testing.T) *Engine {
	t.Helper()

	pinyinDict := dict.NewPinyinDict(nil)
	pinyinComposite := dict.NewCompositeDict()
	pinyinComposite.AddLayer(dict.NewPinyinDictLayer("pinyin-system", dict.LayerTypeSystem, pinyinDict))

	pinyinEngine := pinyin.NewEngineWithConfig(pinyinComposite, &pinyin.Config{
		FilterMode: "smart",
		SkipAbbrev: true, // 混输模式默认关闭简拼
	}, nil)

	return NewEngine(nil, pinyinEngine, &Config{
		MinPinyinLength:      2,
		CodetableWeightBoost: 10000000,
	}, nil)
}

// TestConvertMixedPinyinPreedit 验证 input ≤ maxCodeLen 时合法多音节拼音会启用音节分段，
// 而单音节/非拼音输入不会被误标为拼音降级。复现并守护用户反馈：
// "设置码长较长时（如 anweishi ≤ maxCodeLen），拼音不再分段，整体连写"。
func TestConvertMixedPinyinPreedit(t *testing.T) {
	engine := newEmptyDictMixedEngine(t)

	// maxCodeLen 默认 4，故仅能测 2~4 码的多音节拼音（anweishi/nihao 等 >4 码走 overflow 路径，
	// 那条路径已由 convertMixedOverflow + suppress 覆盖）。
	t.Run("multi-syllable segments", func(t *testing.T) {
		// 用无歧义双音节：避开首音节单元音（如 "anan" 可被切成 a+nan，首音节单元音
		// 会被 isPossiblePinyinSequence 的守护正确排除——那是 by design，非本用例要测的）。
		cases := []string{"woai", "niai", "mama"}
		for _, input := range cases {
			result := engine.ConvertEx(input, 50)
			if !result.IsPinyinFallback {
				t.Errorf("input=%q: 期望 IsPinyinFallback=true（多音节应分段）", input)
				continue
			}
			if len(result.CompletedSyllables) < 2 {
				t.Errorf("input=%q: CompletedSyllables=%v，期望 ≥2 音节", input, result.CompletedSyllables)
			}
			if result.PreeditDisplay == "" || result.PreeditDisplay == input {
				t.Errorf("input=%q: PreeditDisplay=%q，期望分段后与原始连写不同", input, result.PreeditDisplay)
			}
			// 分段串去掉分隔符后应等于原始输入（不丢字、不加字）。
			stripped := strings.NewReplacer(" ", "", "'", "").Replace(result.PreeditDisplay)
			if stripped != input {
				t.Errorf("input=%q: PreeditDisplay=%q 去分隔后=%q，应等于原始输入", input, result.PreeditDisplay, stripped)
			}
		}
	})

	// 单音节（含同时是合法五笔码长的歧义码）不应被标为拼音降级——单音节无可视分段，
	// 强行标记会污染下游 IsPinyinFallback 并抢占码表编码显示。
	t.Run("single-syllable not flagged", func(t *testing.T) {
		cases := []string{"wo", "an", "shi"}
		for _, input := range cases {
			result := engine.ConvertEx(input, 50)
			if result.IsPinyinFallback {
				t.Errorf("input=%q: 期望 IsPinyinFallback=false（单音节不分段），实际被标记，PreeditDisplay=%q",
					input, result.PreeditDisplay)
			}
		}
	})
}
