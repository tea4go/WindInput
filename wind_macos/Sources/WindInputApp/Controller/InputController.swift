import Cocoa
import InputMethodKit
import WindInputKit

// InputController — IMKit 为每个文本框/会话实例化一个本类对象 (PR-1 设计 方案 A).
//
// M2.1 范围 (本提交):
//   - init 时连 bridge.sock (BridgeClient.connect, 失败仅 log 不抛)
//   - handle(_:client:) 把 NSEvent 翻译成 KeyEvent 帧, 同步发送, 等响应,
//     按 Consumed/PassThrough 决定是否吃键
//   - Composition / Commit / Candidates push 等 M2.2+ 再做; 收到非 PassThrough/
//     Consumed 的响应一律打 NSLog 后吃键 (避免重复出字符)
//
// 线程模型: IMKit 在主线程调用 handle, BridgeClient 是阻塞 socket I/O.
//   M2.1 用同步发送 + 同步读响应; 一次 round-trip 实测 < 1ms (UDS), 用户感知不到.
//   未来如出现卡顿, M2.2+ 改成 async + barrier seq 匹配.
@objc(WindInputController)
public class InputController: IMKInputController {

    private var bridge: BridgeClient?
    private var keySeq: UInt16 = 0

    public override init!(server: IMKServer!, delegate: Any!, client inputClient: Any!) {
        super.init(server: server, delegate: delegate, client: inputClient)

        let path = BridgeEndpoints.requestSocket
        do {
            bridge = try BridgeClient(socketPath: path)
            NSLog("WindInput[InputController] bridge connected path=\(path)")
        } catch {
            NSLog("WindInput[InputController] bridge connect FAILED path=\(path) err=\(error)")
            bridge = nil
        }
    }

    deinit {
        bridge?.close()
    }

    // MARK: - IMKit hook

    public override func handle(_ event: NSEvent!, client sender: Any!) -> Bool {
        guard let event = event else { return false }
        // 只关心 keyDown; keyUp / flagsChanged 在 M2.1 不处理.
        guard event.type == .keyDown else { return false }
        guard let bridge = bridge, bridge.isConnected else {
            NSLog("WindInput[handle] bridge not connected, pass through")
            return false
        }

        keySeq &+= 1
        guard let frame = KeyHandler.encodeKeyEvent(event, seq: keySeq) else {
            // 未映射的键 (空 VK), 让系统自己处理
            return false
        }

        do {
            try bridge.send(frame)
            let resp = try bridge.readFrame()
            return applyResponse(resp, sender: sender)
        } catch {
            NSLog("WindInput[handle] bridge io error: \(error)")
            // I/O 错误: 尝试重连一次, 这次按键 PassThrough
            reconnect()
            return false
        }
    }

    // MARK: - Response routing

    private func applyResponse(_ frame: Frame, sender: Any?) -> Bool {
        switch frame.cmd {
        case DownstreamCmd.passThrough:
            return false
        case DownstreamCmd.consumed:
            return true
        case DownstreamCmd.ack:
            // M2.1: Ack 单独到 handle 路径不常见; 当作消费处理避免重复出字符
            return true
        case DownstreamCmd.commitText:
            // M2.2 才做完整解码 + insertText; 这一阶段仅 log 文本字节数
            NSLog("WindInput[handle] commitText payload=\(frame.payload.count) bytes (M2.2 待实装 insertText)")
            return true
        case DownstreamCmd.updateComposition:
            NSLog("WindInput[handle] updateComposition payload=\(frame.payload.count) bytes (M2.2 待实装 setMarkedText)")
            return true
        case DownstreamCmd.clearComposition:
            NSLog("WindInput[handle] clearComposition")
            return true
        default:
            NSLog(String(format: "WindInput[handle] unhandled cmd=0x%04x len=%d", frame.cmd, frame.payload.count))
            return true
        }
    }

    // MARK: - Reconnect

    private func reconnect() {
        bridge?.close()
        bridge = nil
        do {
            bridge = try BridgeClient(socketPath: BridgeEndpoints.requestSocket)
            NSLog("WindInput[reconnect] bridge reconnected")
        } catch {
            NSLog("WindInput[reconnect] still down: \(error)")
        }
    }
}
