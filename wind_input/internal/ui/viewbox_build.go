package ui

// 从 RenderConfig + 候选数据构建候选窗的盒模型 View 树（v2.6 P1）。
//
// 本文件是"固定骨架 + 统一 View"思路的落地：旧渲染器里逐元素硬编码的 magic number，
// 在这里被翻译成各 View 的 margin/padding/border/fixed-size。引擎（viewbox.go/_paint.go）
// 只负责 measure/arrange/paint，对"候选窗"语义无感知。
//
// 当前覆盖横排核心：window / preedit_bar / candidate_list / item / index / text / comment
// 以及 selected/hover 背景、accent bar、pager、preedit 光标、ModeLabel、accent-glow。
// 暂未覆盖（后续迭代）：embedded preedit、竖排长候选省略号截断。

import (
	"image/color"

	"github.com/huanfeng/wind_input/pkg/config"
)

// buildEmbeddedPreedit 构建内嵌预编辑（PreeditEmbedded 模式）：编码 + ModeLabel 内嵌到候选行首，
// 与首个候选间留 16*scale 分隔；含内嵌光标。无内容返回 nil。
func (r *Renderer) buildEmbeddedPreedit(input string, cursorPos, rowH int, scale float64, sc func(float64) int) *View {
	cfg := &r.config
	if input == "" && cfg.ModeLabel == "" {
		return nil
	}
	children := make([]*View, 0, 2)
	if input != "" {
		children = append(children, &View{Text: input, TextStyle: TextStyle{FontSize: cfg.FontSize, Color: r.resolvedViews.PreeditBar.TextColor}})
	}
	if cfg.ModeLabel != "" {
		ml := &View{Text: cfg.ModeLabel, TextStyle: TextStyle{FontSize: cfg.IndexFontSize, Color: r.getCommentColor()}}
		if len(children) > 0 {
			ml.Margin = Edges{Left: sc(4 * scale)}
		}
		children = append(children, ml)
	}
	inline := &View{
		Layout: LayoutRow, CrossAlign: AlignCenter, FixedH: rowH,
		Margin:   Edges{Right: sc(16 * scale)},
		Children: children,
	}
	if input != "" && cursorPos >= 0 && cursorPos <= len(input) {
		cw := r.textDrawer.MeasureString(input[:cursorPos], cfg.FontSize)
		inline.Layers = append(inline.Layers, ImageLayer{
			Color: r.resolvedViews.PreeditBar.TextColor, Z: 1, Anchor: "left",
			OffsetX: int(cw + 0.5), W: maxInt(1, sc(1.5*scale)), H: int(float64(rowH) * 0.7),
		})
	}
	return inline
}

// buildPreeditBand 构建预编辑条（横竖排共用）：输入文本 + 光标 + 右对齐 ModeLabel + accent-glow 底色。
// inputH 为条高（横/竖排不同）。
func (r *Renderer) buildPreeditBand(input string, cursorPos, inputH int, scale float64, sc func(float64) int) *View {
	cfg := &r.config
	bgColor := r.resolvedViews.PreeditBar.BgColor
	if cfg.ModeAccentColor != nil {
		bgColor = blendColor(r.resolvedViews.PreeditBar.BgColor, cfg.ModeAccentColor, 35) // 临时拼音等模式：input 区半透 accent 叠加
	}
	children := []*View{{
		Text:      input,
		TextStyle: TextStyle{FontSize: cfg.FontSize, Color: r.resolvedViews.PreeditBar.TextColor},
	}}
	if cfg.ModeLabel != "" {
		children = append(children,
			&View{Grow: true}, // 弹性占位把标签推到右侧
			&View{Text: cfg.ModeLabel, TextStyle: TextStyle{FontSize: cfg.IndexFontSize, Color: r.getCommentColor()}},
		)
	}
	pb := &r.resolvedViews.PreeditBar
	band := &View{
		Layout: LayoutRow, CrossAlign: AlignCenter, Stretch: true, FixedH: inputH,
		Padding:    Edges{Left: sc(float64(pb.PadLeft)), Right: sc(float64(pb.PadRight))},
		Background: Fill{Color: bgColor},
		Border:     Border{Radius: sc(float64(pb.BorderRadius))},
		Children:   children,
	}
	if input != "" && cursorPos >= 0 && cursorPos <= len(input) {
		cw := r.textDrawer.MeasureString(input[:cursorPos], cfg.FontSize)
		band.Layers = append(band.Layers, ImageLayer{
			Color: r.resolvedViews.PreeditBar.TextColor, Z: 1, Anchor: "left",
			OffsetX: sc(float64(pb.PadLeft)) + int(cw+0.5), W: maxInt(1, sc(1.5*scale)), H: int(cfg.FontSize + 0.5),
		})
	}
	return band
}

// windowBorder 返回窗口边框：accent 模式(ModeAccentColor 非空)用更宽的 accent 色 glow 边框。
func (r *Renderer) windowBorder(radius int, sc func(float64) int, scale float64) Border {
	cfg := &r.config
	if cfg.ModeAccentColor != nil {
		return Border{Width: maxInt(1, sc(2.5*scale)), Color: cfg.ModeAccentColor, Radius: radius}
	}
	return Border{Width: 1, Color: r.resolvedViews.Window.BorderColor, Radius: radius}
}

// truncateToWidth 把 text 截断到不超过 avail 像素宽，超出时尾部加省略号。
func (r *Renderer) truncateToWidth(text string, fontSize, avail float64) string {
	if avail <= 0 || r.textDrawer.MeasureString(text, fontSize) <= avail {
		return text
	}
	const ell = "…"
	ellW := r.textDrawer.MeasureString(ell, fontSize)
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if r.textDrawer.MeasureString(string(runes), fontSize)+ellW <= avail {
			return string(runes) + ell
		}
	}
	return ell
}

// effectiveIndexLabels 返回生效的序号标签：用户全局覆盖（config）优先于主题 labels。
// 运行时 per-候选 IndexLabel 优先级更高，由 indexLabel() 内部处理（构成四层优先级）。
func (r *Renderer) effectiveIndexLabels() string {
	if r.config.GlobalIndexLabels != "" {
		return r.config.GlobalIndexLabels
	}
	return r.config.IndexLabels
}

// blendColor 把 over 以 overAlpha/255 透明度叠加到 base 上，返回不透明结果。
func blendColor(base, over color.Color, overAlpha uint32) color.Color {
	br, bg, bb, _ := base.RGBA()
	or, og, ob, _ := over.RGBA()
	inv := 255 - overAlpha
	mix := func(b, o uint32) uint8 { return uint8(((o>>8)*overAlpha + (b>>8)*inv) / 255) }
	return color.RGBA{mix(br, or), mix(bg, og), mix(bb, ob), 255}
}

// buildAccentRail 构建强调条占位元素：作为候选项行的前导 View，FixedW=railW 在**所有行**
// 占位以保持序号/文字列对齐，仅 selected 行绘制强调条（z<0 纯色层，竖直居中、左缘偏移）。
// 主题无强调条（HasAccentBar=false 或无颜色）时返回 nil，调用方据此不加 rail（沿用原内边距/无留白）。
// 替代旧的 item.Layers 覆盖层写法：强调条从此参与盒模型布局，内容自然排在其右，不再依赖左内边距兜位。
func (r *Renderer) buildAccentRail(railW float64, selected bool, rowH int, sc func(float64) int) *View {
	rv := &r.resolvedViews
	if !r.config.HasAccentBar || rv.AccentBar.BgColor == nil {
		return nil
	}
	rail := &View{FixedW: sc(railW), FixedH: rowH}
	if selected {
		barW := sc(float64(rv.AccentBarWidth))
		rail.Layers = []ImageLayer{{
			Color: rv.AccentBar.BgColor, Z: -1, Anchor: "left",
			OffsetX: sc(float64(rv.AccentBarOffset)), W: barW,
			H: int(rv.ItemHeight*rv.AccentBarHRatio + 0.5), Radius: barW / 2,
		}}
	}
	return rail
}

// buildHorizontalCandidateTree 构建横排候选窗 View 树。
// candWindowTree 是构建结果：窗口根 + 命中测试所需的关键 View。
type candWindowTree struct {
	root      *View
	items     []*View // 与 candidates 一一对应
	pagerUp   *View   // nil = 无翻页上键 / 不可用
	pagerDown *View
}

// (x,y) 由调用方在 Layout 时给定；本函数只描述结构与样式。
func (r *Renderer) buildHorizontalCandidateTree(
	candidates []Candidate,
	input string,
	cursorPos int,
	page, totalPages, selectedIndex, hoverIndex int,
	hoverPageBtn string,
) *candWindowTree {
	cfg := &r.config
	scale := GetDPIScale()
	sc := func(v float64) int { return int(v*scale + 0.5) }

	isTextIndex := cfg.IndexStyle == "text"
	isEmbedded := cfg.PreeditMode == config.PreeditEmbedded && !cfg.HidePreedit

	// 外观取值改走 ResolvedViews（逻辑像素，single-scale；后续 sc() 乘一次 scale）。
	rv := &r.resolvedViews
	padX := float64(rv.Window.PadLeft)
	padY := float64(rv.Window.PadTop)
	bgPadL := float64(rv.Item.PadLeft)
	bgPadR := float64(rv.Item.PadRight)
	indexMarginRight := float64(rv.Text.MarginLeft)
	commentMarginLeft := float64(rv.Comment.MarginLeft)

	itemSpacing := float64(rv.ItemSpacing)
	if isTextIndex {
		itemSpacing += 4 // 文本序号模式间距 +4（原合成桥 12/16，现下沉到 build，使 ResolveCandidateViews 保持运行时无感知）
	}
	commentSize := rv.Index.FontSize
	if isTextIndex {
		commentSize = rv.Index.FontSize + 2*scale
	}
	indexSize := maxF(18*scale, rv.Index.FontSize+4*scale)
	rowH := int(rv.ItemHeight + 0.5)
	commentColor := rv.Comment.TextColor

	// ---- 候选项 ----
	items := make([]*View, 0, len(candidates))
	for i, cand := range candidates {
		children := make([]*View, 0, 3)

		if cand.Index >= 0 {
			label := indexLabel(r.effectiveIndexLabels(), cand.Index, cand.IndexLabel)
			if isTextIndex {
				children = append(children, &View{
					Text:      label,
					TextStyle: TextStyle{FontSize: rv.Index.FontSize, Weight: rv.Index.FontWeight, Color: rv.Index.TextColor},
				})
			} else {
				d := int(indexSize + 0.5)
				children = append(children, &View{
					FixedW:     d,
					FixedH:     d,
					Background: Fill{Color: rv.Index.BgColor},
					Border:     Border{Radius: d / 2},
					Layout:     LayoutStack,
					Children: []*View{{
						FixedW:    d,
						FixedH:    d,
						Text:      label,
						TextStyle: TextStyle{FontSize: rv.Index.FontSize, Weight: rv.Index.FontWeight, Color: rv.Index.TextColor, Align: AlignCenter},
					}},
				})
			}
		}

		// 候选文字
		textChild := &View{
			Text:      candidateDisplayText(cand, cfg.CmdbarPrefix),
			TextStyle: TextStyle{FontSize: rv.Text.FontSize, Color: rv.Text.TextColor},
		}
		if len(children) > 0 {
			textChild.Margin = Edges{Left: sc(indexMarginRight)}
		}
		children = append(children, textChild)

		// 注释
		if cand.Comment != "" {
			children = append(children, &View{
				Text:      cand.Comment,
				TextStyle: TextStyle{FontSize: commentSize, Color: commentColor},
				Margin:    Edges{Left: sc(commentMarginLeft)},
			})
		}

		// 强调条占位元素：rail 存在时占据原左内边距宽度（内容位置不变），并承载强调条；
		// 无强调条主题沿用左内边距。
		itemChildren := children
		itemPadLeft := bgPadL
		if rail := r.buildAccentRail(bgPadL, i == selectedIndex, rowH, sc); rail != nil {
			itemPadLeft = 0
			itemChildren = append([]*View{rail}, children...)
		}
		item := &View{
			Layout:     LayoutRow,
			CrossAlign: AlignCenter,
			Padding:    Edges{Left: sc(itemPadLeft), Right: sc(bgPadR)},
			FixedH:     rowH,
			Border:     Border{Radius: sc(float64(rv.Item.BorderRadius))},
			Children:   itemChildren,
		}
		if i == selectedIndex {
			item.Background = Fill{Color: rv.Item.SelectedBg}
		} else if i == hoverIndex {
			item.Background = Fill{Color: rv.Item.HoverBg}
		}
		items = append(items, item)
	}

	// ---- 候选列表行：[内嵌预编辑?] + 候选项 + [翻页区?] ----
	pagerChildren, pagerUp, pagerDown := r.buildPager(scale, sc, isTextIndex, page, totalPages, hoverPageBtn, rowH)
	listChildren := make([]*View, 0, len(items)+4)
	if isEmbedded {
		if inline := r.buildEmbeddedPreedit(input, cursorPos, rowH, scale, sc); inline != nil {
			listChildren = append(listChildren, inline)
		}
	}
	listChildren = append(listChildren, items...)
	if len(pagerChildren) > 0 {
		pagerChildren[0].Margin = Edges{Left: sc(8 * scale)} // 与候选列表的分隔
		listChildren = append(listChildren, pagerChildren...)
	}

	// 候选框间隙：旧渲染器 effectiveSpacing=max(padL+padR, itemSpacing)，扣掉左右内边距后
	// 即相邻框之间的真实间隙（通常为 0，框相邻）。
	boxGap := maxF(itemSpacing-bgPadL-bgPadR, 0)
	list := &View{
		Layout:     LayoutRow,
		CrossAlign: AlignCenter, // 页码文本/箭头按钮在行内垂直居中
		Gap:        sc(boxGap),
		Children:   listChildren,
	}

	// ---- band 列表（preedit + 候选列表）----
	bands := make([]*View, 0, 2)
	if (input != "" || cfg.ModeLabel != "") && !cfg.HidePreedit && !isEmbedded {
		inputH := int(maxF(24*scale, rv.PreeditBar.FontSize*1.3) + 0.5)
		bands = append(bands, r.buildPreeditBand(input, cursorPos, inputH, scale, sc))
	}
	bands = append(bands, list)

	window := &View{
		Layout:     LayoutColumn,
		Gap:        sc(float64(rv.WindowGap)),
		Padding:    Edges{Top: sc(padY), Right: sc(padX), Bottom: sc(padY), Left: sc(padX)},
		Background: Fill{Color: rv.Window.BgColor, Image: cfg.BackgroundImage, Mode: cfg.BackgroundMode, Slice: cfg.BackgroundSlice, Opacity: cfg.BackgroundOpacity},
		Border:     r.windowBorder(sc(float64(rv.Window.BorderRadius)), sc, scale),
		Shadow:     &ViewShadow{OffsetX: sc(float64(rv.ShadowOffset)), OffsetY: sc(float64(rv.ShadowOffset)), Color: rv.ShadowColor},
		Children:   bands,
	}
	return &candWindowTree{root: window, items: items, pagerUp: pagerUp, pagerDown: pagerDown}
}

func pickF(primary, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
