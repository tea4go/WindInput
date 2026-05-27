import Cocoa
import InputMethodKit

// WindInput macOS IMKit 入口.
//
// 系统通过 Info.plist 中 InputMethodConnectionName 定位本 .app, 启动它,
// 然后通过 InputMethodServerControllerClass 指定的类创建 IMKInputController 实例.
//
// top-level code 自动运行在 main actor, 因此可以直接同步调用 NSApp/IMKServer API,
// 无需 @MainActor 注解与 unsafe 闭包包装.

// 与 Info.plist 中 InputMethodConnectionName 必须一致.
let connectionName = "WindInput_1_Connection"

guard let bundleID = Bundle.main.bundleIdentifier else {
    fatalError("WindInputApp: Info.plist 缺 CFBundleIdentifier")
}

NSLog("WindInputApp boot bundleID=\(bundleID) connection=\(connectionName)")

// IMKServer 必须在 NSApp.run 前创建并持有引用; ARC 不能释放它,
// 所以放到顶层 (top-level let) 让进程生命周期持有.
let imkServer = IMKServer(name: connectionName, bundleIdentifier: bundleID)
NSLog("WindInputApp IMKServer ready name=\(connectionName) instance=\(String(describing: imkServer))")

let app = NSApplication.shared
// accessory 策略: 不在 Dock 显示, 也不抢菜单焦点. IMKit `.app` 标准做法.
app.setActivationPolicy(.accessory)
app.run()
