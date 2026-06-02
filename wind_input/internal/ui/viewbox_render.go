package ui

// 盒模型引擎的生产形态入口：构建 View 树 → 布局 → 绘制 → 提取命中矩形。
// 盒模型 View 引擎是候选窗唯一渲染路径（旧固定化渲染器已退役）。

import (
	"image"

	"github.com/gogpu/gg"
)

// newSharedDrawContext 创建 dc 与 img 实时共享同一像素缓冲的绘制上下文（独立窗口用，
// 每次新建 buffer，不复用 scratch；候选窗高频路径用 (*Renderer).acquireDrawContext）。
// 注意：gogpu/gg 的 NewContext().Image() 返回快照而非实时视图，PaintTree 要求 dc 绘制
// 实时反映到 img，故必须经 pixmap 的 ImageView 共享缓冲。
func newSharedDrawContext(w, h int) (*gg.Context, *image.RGBA) {
	pm := gg.NewPixmapFromBuffer(make([]byte, w*h*4), w, h)
	return gg.NewContextForPixmap(pm), pm.ImageView()
}

// renderHorizontalV2 用盒模型引擎渲染横排候选窗。
func (r *Renderer) renderHorizontalV2(
	candidates []Candidate,
	input string,
	cursorPos, page, totalPages, hoverIndex int,
	hoverPageBtn string,
	selectedIndex int,
) (*image.RGBA, *RenderResult) {
	r.refreshResolvedViews()
	tree := r.buildHorizontalCandidateTree(candidates, input, cursorPos, page, totalPages, selectedIndex, hoverIndex, hoverPageBtn)
	return r.renderTree(tree)
}

// renderVerticalV2 用盒模型引擎渲染竖排候选窗。
func (r *Renderer) renderVerticalV2(
	candidates []Candidate,
	input string,
	cursorPos, page, totalPages, hoverIndex int,
	hoverPageBtn string,
	selectedIndex int,
) (*image.RGBA, *RenderResult) {
	r.refreshResolvedViews()
	tree := r.buildVerticalCandidateTree(candidates, input, cursorPos, page, totalPages, selectedIndex, hoverIndex, hoverPageBtn)
	return r.renderTree(tree)
}

// renderTree 对已构建的候选窗 View 树执行布局 → 绘制 → 命中矩形提取。
// 窗口根铺满画布，仅为投影偏移在右下留出空间（画布即窗口尺寸 + 投影 2px）。
func (r *Renderer) renderTree(tree *candWindowTree) (*image.RGBA, *RenderResult) {
	td := r.textDrawer
	root := tree.root
	Layout(root, 0, 0, td)

	ext := 0
	if root.Shadow != nil {
		ext = maxInt(root.Shadow.OffsetX, root.Shadow.OffsetY)
	}
	w := root.Rect().Dx() + ext
	h := root.Rect().Dy() + ext
	dc, img := r.acquireDrawContext(w, h)
	PaintTree(root, dc, img, td)
	DrawDebugBanner(img)

	res := &RenderResult{Rects: make([]CandidateRect, len(tree.items))}
	for i, it := range tree.items {
		res.Rects[i] = rectOf(i, it)
	}
	if tree.pagerUp != nil {
		rc := rectOf(0, tree.pagerUp)
		res.PageUpRect = &rc
	}
	if tree.pagerDown != nil {
		rc := rectOf(0, tree.pagerDown)
		res.PageDownRect = &rc
	}
	return img, res
}

func rectOf(index int, v *View) CandidateRect {
	r := v.Rect()
	return CandidateRect{
		Index: index,
		X:     float64(r.Min.X),
		Y:     float64(r.Min.Y),
		W:     float64(r.Dx()),
		H:     float64(r.Dy()),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
