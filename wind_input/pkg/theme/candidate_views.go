package theme

import (
	"image/color"
	"strings"
)

// resolveCandidateViewColor 解析候选窗 views 颜色字段：${name}→palette 候选窗语义色 /
// hex(#RRGGBB[AA]) 直解 / 空或未知 token → nil（调用方据此保留 palette 默认）。
// 从 ui 侧 resolveViewColor 下沉（P6 2b），token 映射表与之一致。
func resolveCandidateViewColor(s string, pal ResolvedPalette) color.Color {
	if s == "" {
		return nil
	}
	cw := pal.CandidateWindow
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		switch s[2 : len(s)-1] {
		case "background":
			return cw.Background
		case "border":
			return cw.Border
		case "text":
			return cw.Text
		case "index_bg":
			return cw.IndexBg
		case "index_text":
			return cw.IndexText
		case "hover_bg":
			return cw.HoverBg
		case "selected_bg":
			return cw.SelectedBg
		case "preedit_bg":
			return cw.PreeditBg
		case "preedit_text":
			return cw.PreeditText
		case "comment":
			return cw.Comment
		case "accent":
			return cw.AccentBar
		case "shadow":
			return pal.Shadow
		}
		return nil
	}
	if c, err := ParseHexColor(s); err == nil {
		return c
	}
	return nil
}

// ResolveCandidateViews 把候选窗 Views（已 merge defaultViews 基线，含 Metrics）+ palette
// 解析为渲染消费的 ResolvedViews（几何=逻辑像素、颜色=color.Color）。
// 颜色 = palette 默认 ⊕ views token 覆盖（views 颜色非空才覆盖）。
// 不设字号（Text/Index/PreeditBar.FontSize）、ItemHeight、VerticalMaxWidth——这些是运行时值，由 ui 回填。
func ResolveCandidateViews(views Views, pal ResolvedPalette) ResolvedViews {
	build := func(n ViewNode, defBg, defBorder, defText color.Color) RVNode {
		out := RVNode{
			MarginTop:    edgeOr(n.Margin.Top, 0),
			MarginRight:  edgeOr(n.Margin.Right, 0),
			MarginBottom: edgeOr(n.Margin.Bottom, 0),
			MarginLeft:   edgeOr(n.Margin.Left, 0),
			PadTop:       edgeOr(n.Padding.Top, 0),
			PadRight:     edgeOr(n.Padding.Right, 0),
			PadBottom:    edgeOr(n.Padding.Bottom, 0),
			PadLeft:      edgeOr(n.Padding.Left, 0),
			BorderRadius: edgeOr(n.Border.Radius, 0),
			BorderWidth:  edgeOr(n.Border.Width, 0),
			BgColor:      defBg,
			BorderColor:  defBorder,
			TextColor:    defText,
		}
		if c := resolveCandidateViewColor(n.Background.Color, pal); c != nil {
			out.BgColor = c
		}
		if c := resolveCandidateViewColor(n.Border.Color, pal); c != nil {
			out.BorderColor = c
		}
		if c := resolveCandidateViewColor(n.Color, pal); c != nil {
			out.TextColor = c
		}
		return out
	}
	cw := pal.CandidateWindow
	rv := ResolvedViews{
		Window:        build(views.Window, cw.Background, cw.Border, nil),
		PreeditBar:    build(views.PreeditBar, cw.PreeditBg, nil, cw.PreeditText),
		CandidateList: build(views.CandidateList, nil, nil, nil),
		Item:          build(views.Item, nil, nil, nil),
		Index:         build(views.Index, cw.IndexBg, nil, cw.IndexText),
		Text:          build(views.Text, nil, nil, cw.Text),
		Comment:       build(views.Comment, nil, nil, cw.Comment),
		AccentBar:     build(views.AccentBar, cw.AccentBar, nil, nil),
		FooterBar:     build(views.FooterBar, nil, nil, nil),
		ShadowColor:   pal.Shadow,
	}
	rv.Item.SelectedBg = cw.SelectedBg
	rv.Item.HoverBg = cw.HoverBg
	if views.Item.Selected != nil {
		if c := resolveCandidateViewColor(views.Item.Selected.Background.Color, pal); c != nil {
			rv.Item.SelectedBg = c
		}
	}
	if views.Item.Hover != nil {
		if c := resolveCandidateViewColor(views.Item.Hover.Background.Color, pal); c != nil {
			rv.Item.HoverBg = c
		}
	}
	if m := views.Metrics; m != nil {
		rv.ItemSpacing = edgeOr(m.ItemSpacing, 0)
		rv.WindowGap = edgeOr(m.BandGap, 0)
		rv.ShadowOffset = edgeOr(m.ShadowOffset, 0)
		if m.AccentBar != nil {
			rv.AccentBarWidth = edgeOr(m.AccentBar.Width, 0)
			rv.AccentBarOffset = edgeOr(m.AccentBar.Offset, 0)
			if m.AccentBar.HeightRatio != nil {
				rv.AccentBarHRatio = *m.AccentBar.HeightRatio
			}
		}
	}
	return rv
}
