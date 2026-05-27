// swift-tools-version:5.9
//
// WindInput macOS — SwiftPM 工程 (PR-A M1 + M2).
//
// 目标:
//   - WindInputKit   : 二进制协议 codec + UDS BridgeClient (Pure Swift, 跨 target 共享)
//   - WindInputSmoke : 命令行 smoke 工具, 连真实 bridge 收发帧 (用于无 IMKit 时的协议验证)
//   - WindInputApp   : IMKit 输入法主体, swift build 出来的二进制由 scripts/build_macos_app.sh
//                       打成 .app bundle (含 Info.plist + Resources), 系统通过 .app 加载
//
// 注: SwiftPM 自身不支持直接产出 .app bundle, build script 负责后处理.
import PackageDescription

let package = Package(
    name: "WindInput",
    platforms: [.macOS(.v12)],
    products: [
        .library(name: "WindInputKit", targets: ["WindInputKit"]),
        .executable(name: "wind-smoke", targets: ["WindInputSmoke"]),
        .executable(name: "wind-input-app", targets: ["WindInputApp"]),
    ],
    targets: [
        .target(
            name: "WindInputKit",
            path: "Sources/WindInputKit"
        ),
        .executableTarget(
            name: "WindInputSmoke",
            dependencies: ["WindInputKit"],
            path: "Sources/WindInputSmoke"
        ),
        .executableTarget(
            name: "WindInputApp",
            dependencies: ["WindInputKit"],
            path: "Sources/WindInputApp",
            // Info.plist / entitlements 不参与 swift build, 由 build_macos_app.sh
            // 在拼 .app bundle 时复制. exclude 避免 SwiftPM 把它们当成源文件.
            exclude: [
                "Resources/Info.plist",
                "Resources/WindInput.entitlements",
            ],
            linkerSettings: [
                .linkedFramework("InputMethodKit"),
                .linkedFramework("Cocoa"),
            ]
        ),
        .testTarget(
            name: "WindInputKitTests",
            dependencies: ["WindInputKit"],
            path: "Tests/WindInputKitTests"
        ),
    ]
)
