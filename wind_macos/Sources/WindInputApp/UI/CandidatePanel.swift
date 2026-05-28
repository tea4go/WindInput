import Cocoa

// CandidatePanel — IMKit `.app` 候选框浮窗 (PR-A.5 Phase 1).
//
// 设计要点 (与 rime/squirrel SquirrelPanel.m 对齐):
//   - styleMask: [.borderless, .nonactivatingPanel] — 无标题栏, 不抢键盘焦点
//   - level = .popUpMenu — 浮在普通窗上, 不抢全屏窗
//   - collectionBehavior 含 .canJoinAllSpaces — 跟随用户切 Space
//   - isOpaque=false + backgroundColor=clear — 让候选框 RGBA alpha 走起
//   - hidesOnDeactivate=false — 切到别的 .app 时不消失 (IME 全局)
//
// 用法: panel.show(image: nsImage, atScreenPoint: NSPoint(wireTopLeft x, y))
//   wireTopLeft 是协议侧坐标 (top-left), 内部转 Cocoa bottom-left。

final class CandidatePanel: NSPanel {
    private let imageView = NSImageView()

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

        imageView.imageScaling = .scaleNone
        let host = NSView(frame: NSRect(x: 0, y: 0, width: 200, height: 60))
        host.addSubview(imageView)
        self.contentView = host
    }

    /// 显示候选框: image 为 BGRA 转成的 CGImage 包裹, atScreenPoint 是 wire top-left。
    func show(image: NSImage, atScreenPoint p: NSPoint) {
        imageView.image = image
        imageView.frame = NSRect(origin: .zero, size: image.size)
        self.contentView?.frame = imageView.frame
        self.setContentSize(image.size)

        guard let screen = NSScreen.main ?? NSScreen.screens.first else {
            self.orderFrontRegardless()
            return
        }
        // wire 是 top-left, Cocoa 是 bottom-left, 转换 (考虑 panel 自身高度)
        let cocoaY = screen.frame.height - p.y - image.size.height
        self.setFrameOrigin(NSPoint(x: p.x, y: cocoaY))
        self.orderFrontRegardless()
    }

    func hidePanel() {
        self.orderOut(nil)
    }
}
