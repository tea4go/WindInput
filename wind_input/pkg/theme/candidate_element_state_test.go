package theme

import (
	"image/color"
	"testing"
)

// TestResolveCandidate_ElementStates 守护架构统一：候选文字/注释/序号各自的选中态
// 背景/边框/文字色/字重都被解析进 rv.X.Selected（供渲染统一经 effectiveNode 消费）。
func TestResolveCandidate_ElementStates(t *testing.T) {
	views := Views{
		Text: ViewNode{
			Selected: &ViewNode{
				Background: ViewFill{Color: NewLightDark("#112233")},
				Border:     ViewBorder{Color: NewLightDark("#445566"), Width: dimp(2)},
				Color:      NewLightDark("#778899"),
				FontWeight: intp(700),
			},
		},
		Comment: ViewNode{Selected: &ViewNode{Background: ViewFill{Color: NewLightDark("#AABBCC")}}},
		Index:   ViewNode{Selected: &ViewNode{Border: ViewBorder{Color: NewLightDark("#DDEEFF")}}},
	}
	rv := ResolveCandidateViews(views, ResolvedPalette{Tokens: map[string]color.Color{}})

	if rv.Text.Selected == nil {
		t.Fatal("text.selected 应解析为非 nil")
	}
	if rv.Text.Selected.BgColor == nil || rv.Text.Selected.BorderColor == nil || rv.Text.Selected.TextColor == nil {
		t.Errorf("text.selected 的 bg/border/color 应全部解析: bg=%v border=%v text=%v",
			rv.Text.Selected.BgColor, rv.Text.Selected.BorderColor, rv.Text.Selected.TextColor)
	}
	if rv.Text.Selected.BorderWidth != Dp(2) {
		t.Errorf("text.selected border width 应=2dp, got %v", rv.Text.Selected.BorderWidth)
	}
	if rv.Text.Selected.FontWeight != 700 {
		t.Errorf("text.selected 字重应=700, got %d", rv.Text.Selected.FontWeight)
	}
	if rv.Comment.Selected == nil || rv.Comment.Selected.BgColor == nil {
		t.Error("comment.selected 背景应解析")
	}
	if rv.Index.Selected == nil || rv.Index.Selected.BorderColor == nil {
		t.Error("index.selected 边框应解析")
	}
}
