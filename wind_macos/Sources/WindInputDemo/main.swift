import Cocoa
import WindInputKit

// WindInputDemo — M3 候选框开发用 AppKit demo, 绕开 IMKit 注册墙。
//
// 工作流:
//   1. swift run wind-input-demo   启动 demo, 弹一个无边框浮窗
//   2. 另起 Go shmwriter (或 wind_input 服务) 往 /WindInput_SHM 写帧
//   3. demo 每 50ms poll SHM, seq 变化时把 BGRA 解成 CGImage 贴到浮窗
//   4. 关掉 demo 窗或 Cmd+Q 退出
//
// 不依赖 IMKit, 不需要 sudo / signing / TIS 注册, 真正快速迭代候选框 UI 用。

final class CandidatePanel: NSPanel {
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
    }

    func show(image: NSImage, atScreenPoint p: NSPoint) {
        let iv = (self.contentView as? NSImageView) ?? NSImageView()
        iv.image = image
        iv.imageScaling = .scaleNone
        iv.frame = NSRect(origin: .zero, size: image.size)
        if iv.superview == nil {
            let host = NSView(frame: iv.frame)
            host.addSubview(iv)
            self.contentView = host
        }
        self.setContentSize(image.size)
        // 屏幕坐标系: AppKit 是 bottom-left, wire/SHM 是 top-left。转换。
        guard let screen = NSScreen.main ?? NSScreen.screens.first else { return }
        let cocoaY = screen.frame.height - p.y - image.size.height
        self.setFrameOrigin(NSPoint(x: p.x, y: cocoaY))
        self.orderFrontRegardless()
    }

    func hidePanel() {
        self.orderOut(nil)
    }
}

final class DemoController: NSObject {
    let panel = CandidatePanel()
    var reader: SharedMemoryReader?
    var timer: Timer?
    var lastSeq: UInt32 = 0

    func start() {
        connectSHM()
        timer = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { [weak self] _ in
            self?.tick()
        }
        // 也立刻 tick 一次, 避免等 50ms
        tick()
    }

    func connectSHM() {
        do {
            reader = try SharedMemoryReader(name: "/WindInput_SHM",
                                            size: 4 * 1024 * 1024)
            NSLog("WindInputDemo: SHM connected /WindInput_SHM")
        } catch {
            NSLog("WindInputDemo: SHM open failed: \(error) (运行一下 go run ./cmd/shmwriter 写一帧)")
            // 5s 后重试
            DispatchQueue.main.asyncAfter(deadline: .now() + 5) { [weak self] in
                self?.connectSHM()
            }
        }
    }

    func tick() {
        guard let r = reader else { return }
        guard let f = r.snapshotIfNew() else { return }
        if !f.hasContent || !f.isVisible || f.width == 0 || f.height == 0 {
            panel.hidePanel()
            return
        }
        guard let img = makeImage(from: f) else { return }
        panel.show(image: img,
                   atScreenPoint: NSPoint(x: CGFloat(f.screenX), y: CGFloat(f.screenY)))
        NSLog("WindInputDemo: frame seq=\(f.sequence) \(f.width)x\(f.height) at (\(f.screenX),\(f.screenY))")
    }

    /// BGRA bytes → NSImage. CoreGraphics 直接吃 BGRA + premultiplied first 通道顺序。
    func makeImage(from f: SharedFrame) -> NSImage? {
        let bytesPerPixel = 4
        let bytesPerRow = f.stride
        guard let provider = CGDataProvider(data: f.bgra as CFData) else { return nil }
        let bitmapInfo: CGBitmapInfo = [
            CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedFirst.rawValue),
            CGBitmapInfo.byteOrder32Little,
        ]
        let cs = CGColorSpaceCreateDeviceRGB()
        guard let cg = CGImage(width: f.width,
                               height: f.height,
                               bitsPerComponent: 8,
                               bitsPerPixel: bytesPerPixel * 8,
                               bytesPerRow: bytesPerRow,
                               space: cs,
                               bitmapInfo: bitmapInfo,
                               provider: provider,
                               decode: nil,
                               shouldInterpolate: false,
                               intent: .defaultIntent) else { return nil }
        return NSImage(cgImage: cg, size: NSSize(width: f.width, height: f.height))
    }
}

// Top-level (main 主线程)
let app = NSApplication.shared
app.setActivationPolicy(.accessory)   // 不出现在 Dock, 像 menu bar app 一样

let ctl = DemoController()
ctl.start()

// 一个简单的 status bar item 提示 demo 在跑 + Quit
let bar = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
bar.button?.title = "WindDemo"
let menu = NSMenu()
menu.addItem(withTitle: "Quit", action: #selector(NSApp.terminate(_:)), keyEquivalent: "q")
bar.menu = menu

app.run()
