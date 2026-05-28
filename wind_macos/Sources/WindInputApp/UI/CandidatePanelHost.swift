import Cocoa
import WindInputKit

// CandidatePanelHost — IMKit `.app` 内的候选框承载层 (PR-A.5 Phase 1).
//
// 职责:
//   1. 启动时 try open /WindInput_SHM (Go 服务侧 host_render_darwin.go 创建)
//   2. 启 PushClient 订阅 bridge_push.sock, 收 CmdHostRenderFrame 通知
//   3. 收通知后 snapshot SHM, BGRA → CGImage → 贴到 borderless NSPanel
//
// 单例: 整个 .app 进程一个 panel + 一个 SHM reader + 一个 PushClient,
// 跨所有 IMKInputController 实例 (IMKit 给每个文本框 new 一个 controller,
// 但全局候选框只能有一个)。

public final class CandidatePanelHost {
    public static let shared = CandidatePanelHost()

    private let panel: CandidatePanel
    private var reader: SharedMemoryReader?
    private var push: PushClient?
    private let lock = NSLock()

    private init() {
        // 主线程构造 NSPanel
        if Thread.isMainThread {
            panel = CandidatePanel()
        } else {
            var p: CandidatePanel?
            DispatchQueue.main.sync { p = CandidatePanel() }
            panel = p!
        }
    }

    /// 启动 SHM 订阅 + PushClient。多次调用幂等 (二次启动 no-op)。
    /// SHM 打开失败不致命 — Go 服务可能晚启动, 本端 lazy retry。
    public func start() {
        lock.lock(); defer { lock.unlock() }
        if push != nil { return }

        openSHMIfNeeded()

        let pc = PushClient(socketPath: BridgeEndpoints.pushSocket)
        pc.onFrame = { [weak self] frame in self?.handlePushFrame(frame) }
        pc.onError = { err in NSLog("CandidatePanelHost: push error: \(err)") }
        do {
            try pc.start()
            push = pc
            NSLog("CandidatePanelHost: push subscribed \(BridgeEndpoints.pushSocket)")
        } catch {
            NSLog("CandidatePanelHost: push start failed: \(error)")
        }
    }

    public func stop() {
        lock.lock(); defer { lock.unlock() }
        push?.stop()
        push = nil
        reader?.closeReader()
        reader = nil
        DispatchQueue.main.async { [weak self] in self?.panel.hidePanel() }
    }

    private func openSHMIfNeeded() {
        if reader != nil { return }
        do {
            reader = try SharedMemoryReader(name: "/WindInput_SHM", size: 4 * 1024 * 1024)
            NSLog("CandidatePanelHost: SHM opened /WindInput_SHM")
        } catch {
            // 不是致命错: Go 服务还没起 SHM, push 帧来时再 retry
            NSLog("CandidatePanelHost: SHM open deferred (\(error))")
        }
    }

    // MARK: - Push 路由

    private func handlePushFrame(_ frame: Frame) {
        switch frame.cmd {
        case DownstreamCmd.hostRenderFrame:
            guard let payload = try? BinaryCodec.decodeHostRenderFramePayload(frame.payload) else {
                NSLog("CandidatePanelHost: bad HostRenderFrame payload size=\(frame.payload.count)")
                return
            }
            applyHostRenderFrame(payload)
        default:
            // 其它 push 帧 (commit/composition/syncConfig 等) 由 InputController 路由,
            // 这里不消费, 与 panel 无关。
            break
        }
    }

    private func applyHostRenderFrame(_ p: HostRenderFramePayload) {
        // flags=0 → hide
        let visible = (p.flags & 0x1) != 0
        if !visible || p.width == 0 || p.height == 0 {
            DispatchQueue.main.async { [weak self] in self?.panel.hidePanel() }
            return
        }

        // SHM 没开过? lazy open
        if reader == nil {
            lock.lock(); openSHMIfNeeded(); lock.unlock()
        }
        guard let r = reader, let frame = r.snapshot() else {
            NSLog("CandidatePanelHost: SHM snapshot nil")
            return
        }
        // seq 对不上: push 早于 SHM 写完 (理论上 Go 端先 WriteFrame 再 push, 不应发生)
        if frame.sequence != p.seq {
            NSLog("CandidatePanelHost: seq mismatch push=\(p.seq) shm=\(frame.sequence)")
        }
        guard let img = makeNSImage(from: frame) else {
            NSLog("CandidatePanelHost: makeNSImage failed")
            return
        }
        let pt = NSPoint(x: CGFloat(p.x), y: CGFloat(p.y))
        NSLog("CandidatePanelHost: showing seq=\(p.seq) \(p.width)x\(p.height) at (\(p.x),\(p.y))")
        DispatchQueue.main.async { [weak self] in
            self?.panel.show(image: img, atScreenPoint: pt)
        }
    }

    private func makeNSImage(from f: SharedFrame) -> NSImage? {
        guard let provider = CGDataProvider(data: f.bgra as CFData) else { return nil }
        let bitmapInfo: CGBitmapInfo = [
            CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedFirst.rawValue),
            CGBitmapInfo.byteOrder32Little,
        ]
        let cs = CGColorSpaceCreateDeviceRGB()
        guard let cg = CGImage(
            width: f.width, height: f.height,
            bitsPerComponent: 8, bitsPerPixel: 32,
            bytesPerRow: f.stride,
            space: cs, bitmapInfo: bitmapInfo,
            provider: provider, decode: nil,
            shouldInterpolate: false, intent: .defaultIntent
        ) else { return nil }
        return NSImage(cgImage: cg, size: NSSize(width: f.width, height: f.height))
    }
}
