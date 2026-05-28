import Cocoa
import WindInputKit

// CandidatePanel — IMKit `.app` 候选框浮窗 (PR-A.5 Phase 1 + M5 鼠标点选).
//
// 设计要点 (与 rime/squirrel SquirrelPanel.m 对齐):
//   - styleMask: [.borderless, .nonactivatingPanel] — 无标题栏, 不抢键盘焦点
//   - level = .popUpMenu — 浮在普通窗上, 不抢全屏窗
//   - collectionBehavior 含 .canJoinAllSpaces — 跟随用户切 Space
//   - isOpaque=false + backgroundColor=clear — 让候选框 RGBA alpha 走起
//   - hidesOnDeactivate=false — 切到别的 .app 时不消失 (IME 全局)
//
// 鼠标点选: contentView 自绘 bitmap + 持候选命中矩形 (panel-local, top-left),
//   mouseDown 命中 → onSelect(pageLocalIndex)。nonactivating panel 仍收 mouseDown,
//   acceptsFirstMouse=true 让首次点击 (panel 非 key 时) 也生效。

/// 自绘候选框 bitmap + 处理鼠标命中的内容视图。
final class CandidateContentView: NSView {
    private var image: NSImage?
    private var hitRects: [CandidateHitRect] = []
    private var lastHover: Int = -1
    var onSelect: ((Int) -> Void)?
    var onHover: ((Int) -> Void)?

    override var isFlipped: Bool { true } // top-left 原点, 与 wire/rects 坐标系一致

    func update(image: NSImage, rects: [CandidateHitRect]) {
        self.image = image
        self.hitRects = rects
        needsDisplay = true
    }

    /// 仅更新命中矩形 (rects 帧晚于 render 帧到达时用)。
    func setRects(_ rects: [CandidateHitRect]) {
        self.hitRects = rects
    }

    override func draw(_ dirtyRect: NSRect) {
        image?.draw(in: bounds)
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        trackingAreas.forEach { removeTrackingArea($0) }
        addTrackingArea(NSTrackingArea(rect: .zero,
                                       options: [.activeAlways, .mouseMoved, .mouseEnteredAndExited, .inVisibleRect],
                                       owner: self, userInfo: nil))
    }

    /// 命中候选 (index>=0) / 翻页按钮 (index<0) / 空白 (nil)。
    private func hitIndex(_ event: NSEvent) -> Int? {
        let p = convert(event.locationInWindow, from: nil) // isFlipped, top-left
        for r in hitRects where r.contains(px: p.x, py: p.y) { return Int(r.index) }
        return nil
    }

    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    override func mouseDown(with event: NSEvent) {
        if let idx = hitIndex(event) { onSelect?(idx) }
    }

    override func mouseMoved(with event: NSEvent) {
        // 仅对候选 (index>=0) 报悬停; 翻页按钮 (index<0) 与空白都视为无悬停。
        let idx = hitIndex(event) ?? -1
        let report = idx >= 0 ? idx : -1
        if report != lastHover { lastHover = report; onHover?(report) }
    }

    override func mouseExited(with event: NSEvent) {
        if lastHover != -1 { lastHover = -1; onHover?(-1) }
    }
}

final class CandidatePanel: NSPanel {
    private let content = CandidateContentView()

    /// 鼠标点击命中候选时回调 (pageLocalIndex; <0 = 翻页按钮 -1=上 -2=下)。
    var onSelect: ((Int) -> Void)? {
        get { content.onSelect }
        set { content.onSelect = newValue }
    }
    /// 鼠标悬停候选变化时回调 (pageLocalIndex; -1=离开)。
    var onHover: ((Int) -> Void)? {
        get { content.onHover }
        set { content.onHover = newValue }
    }

    init() {
        super.init(contentRect: NSRect(x: 0, y: 0, width: 200, height: 60),
                   styleMask: [.borderless, .nonactivatingPanel],
                   backing: .buffered,
                   defer: false)
        self.isOpaque = false
        self.backgroundColor = .clear
        self.hasShadow = true
        self.level = .popUpMenu
        self.isFloatingPanel = true
        self.collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle]
        self.hidesOnDeactivate = false
        self.becomesKeyOnlyIfNeeded = true
        self.contentView = content
    }

    /// 显示候选框: image=BGRA→CGImage 包裹, atScreenPoint=wire top-left, rects=命中矩形。
    func show(image: NSImage, atScreenPoint p: NSPoint, rects: [CandidateHitRect]) {
        content.frame = NSRect(origin: .zero, size: image.size)
        content.update(image: image, rects: rects)
        self.setContentSize(image.size)

        guard let screen = NSScreen.main ?? NSScreen.screens.first else {
            self.orderFrontRegardless()
            return
        }
        // wire top-left → Cocoa bottom-left (窗口原点, 考虑 panel 自身高度)
        let cocoaY = screen.frame.height - p.y - image.size.height
        self.setFrameOrigin(NSPoint(x: p.x, y: cocoaY))
        self.orderFrontRegardless()
    }

    /// 更新命中矩形 (CmdCandidateRects 帧晚于 render 帧到达)。
    func updateRects(_ rects: [CandidateHitRect]) {
        content.setRects(rects)
    }

    func hidePanel() {
        self.orderOut(nil)
    }
}
