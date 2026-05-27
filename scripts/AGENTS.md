<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-08 | Updated: 2026-05-27 -->

# scripts/ - 构建辅助和工具脚本

## Purpose

项目构建辅助脚本和诊断工具目录。这些脚本不参与主构建流程，供开发者手动调用，用于版本管理、系统诊断、macOS 端构建/打包/签名/部署/冒烟等任务。

仓库根另有 `dev.ps1`（Win 开发菜单）与 `dev_mac.sh`（macOS 开发菜单），后者通过 `BUILD_SCRIPT="$REPO_DIR/scripts/build_macos.sh"` 调用本目录的 macOS 构建脚本。

## Key Files

| File | Description |
|------|-------------|
| `bump-version.ps1` | 版本号管理脚本：读取 VERSION 文件，按 major/minor/patch/prerelease 规则递增版本号，同步更新所有版本号引用文件（VERSION、go.mod、CMakeLists.txt 等） |
| `check_band.ps1` | DWM Window Band 诊断工具：枚举系统窗口并显示各窗口的 Band 等级，用于调试 Win11 开始菜单候选框 z-order 问题和验证 HostWindow 机制 |
| `probe_ime_mode.ps1` | IME 中/英文模式外部探针（IMM32 视角）：模拟 KBLSwitch 等第三方工具的探测路径，用 `WM_IME_CONTROL/IMC_GETCONVERSIONMODE` 跨线程查询前台窗口的 IMM32 桥接状态。`NO-IMEWND` 表示前台是 TSF-only 客户端（Win11 新版记事本 / Edge / 部分 UWP），CUAS 未建 IMM HIMC，物理上无法外部读取，需要靠功能行为验证 |
| `lint_agents_md.ps1` | 检查 AGENTS.md 文档引用是否悬空 |
| `build_macos.sh` | macOS 端 Go 服务构建（对位 `build_all.ps1` 的可裁剪子集）：下载 rime-frost / rime-wubi / OpenCC 词库源到 `.cache/`，跑 `gen_unigram` + `dictgen` + `gen_opencc_dict`，产出 `build/{wind_input, data/}`（debug variant 产出 `build_debug/{wind_input_debug, data/}`）。子命令：`all` / `service` / `data` / `clean`，开关：`--debug` |
| `smoke_bridge.py` | bridge IPC 冒烟脚本（macOS 端开发期）：连 `bridge.sock` 发一帧 `CmdKeyEvent` 验证 roundtrip，再订阅 `bridge_push.sock` 打印推送帧；用于在 Swift IMKit 客户端落地前快速验证 Go 服务协议通路。`scripts/smoke_bridge.py [push_wait_seconds]`，默认 5 秒 |
| `build_macos_app.sh` | macOS IMKit `.app` 打包：`swift build --product wind-input-app` + 拼 `Contents/{MacOS,Resources/lproj,Info.plist,PkgInfo}` + `codesign --options runtime` (默认 ad-hoc；`SIGN_IDENTITY="cert name"` 切真证书)。输出 `wind_macos/build/WindInput.app` |
| `install_macos_app.sh` | 把 `WindInput.app` 装到 `/Library/Input Methods/`：cp + 不重签（保留 build 阶段签名）+ `lsregister -f -R` 刷 LS DB + `WindInput --register-input-source` / `--enable-input-source` 让 IME 自身进程调 TIS API。`--uninstall` 卸载，`--build` 先 build |
| `redeploy.sh` | 一键 build + uninstall + install + 自动验证签名链 + 跑 TIS list 看是否被收录。必须以普通用户跑（codesign 要 user login keychain），内部对需 root 步骤显式 invoke sudo。强制用 `SIGN_IDENTITY` 真证书签名（默认 "WindInput Dev"）。开发期主入口 |
| `setup_signing.sh` | 命令行创建本机 self-signed Code Signing 证书：openssl 生成 RSA + X509 (codeSigning EKU) → PKCS12 (`-legacy` 因为 macOS security import 不识别 openssl 3.x 默认格式) → import 到 login keychain → `add-trusted-cert` 加 trust。子命令: 默认创建 / `check` 看现状 / `remove` 删 |
| `setup_signing.md` | self-signed 证书的手动 (Keychain Access GUI) 创建步骤备份；macOS 26 Tahoe 上路径菜单与老版本不同时可作参考 |
| `list_input_sources.swift` | 调 Apple `TISCreateInputSourceList` API 列系统已注册的输入源；用于诊断 IME 是否真被 macOS 收录到 TIS 数据库。默认只列非 `com.apple.*`（第三方），`--all` 列全部，`<keyword>` 模糊搜 |

## Usage

### bump-version.ps1

```powershell
# 升级补丁版本（0.1.0 → 0.1.1）
scripts\bump-version.ps1 -Version patch

# 升级次版本（0.1.0 → 0.2.0）
scripts\bump-version.ps1 -Version minor

# 升级主版本（0.1.0 → 1.0.0）
scripts\bump-version.ps1 -Version major

# 设置预发布标识（0.1.0 → 0.1.0-alpha）
scripts\bump-version.ps1 -Version patch -Preid alpha
```

### check_band.ps1

```powershell
# 倒计时 8 秒后扫描（先打开开始菜单再运行）
scripts\check_band.ps1

# 立即扫描
scripts\check_band.ps1 -Now

# 持续监控（Ctrl+C 停止）
scripts\check_band.ps1 -Loop

# 显示所有窗口（含 Band=0/1 的普通窗口）
scripts\check_band.ps1 -All
```

### build_macos.sh

```bash
# 全量：下载词库 + 构建 Go 服务 + 准备 data
scripts/build_macos.sh

# 仅 Go 服务（不动词库）
scripts/build_macos.sh service

# 仅词库下载 + 准备 data
scripts/build_macos.sh data

# debug variant（产出 build_debug/wind_input_debug）
scripts/build_macos.sh --debug

# 清 build/ 与 build_debug/
scripts/build_macos.sh clean
```

输出：

- release：`build/{wind_input, data/}`
- debug ：`build_debug/{wind_input_debug, data/}`

Win 端 `build_all.ps1` 还会构建 TSF DLL / Wails 设置端 / 便携启动器，macOS 端这些尚未实现（IMKit `.app` 走 PR-A 后续里程碑）。

### smoke_bridge.py

```bash
# 默认监听 push 5 秒
scripts/smoke_bridge.py

# 自定义 push 监听时长
scripts/smoke_bridge.py 10

# 也可改运行时目录
WIND_INPUT_RUNTIME_DIR=/tmp/wind_test scripts/smoke_bridge.py
```

预期输出（Go 服务运行中）：

```
[smoke] === KeyEvent roundtrip ===
[smoke] -> KeyEvent bytes=26 hex=0110010112000000...
[smoke] <- cmd=0x0401 ver=0x1001 len=0
[smoke] === Push channel (5s) ===
[smoke] push cmd=0x0206 len=15 body=0c000000...
[smoke] done
```

### build_macos_app.sh / install_macos_app.sh / redeploy.sh / setup_signing.sh / list_input_sources.swift (macOS IMKit `.app` 工具链)

完整流水线（开发期一次性 setup + 之后每次 build/install 一行）：

```bash
# 一次性: 建本机自签证书 (会弹 2 次密码框: keychain 解锁 + sudo add-trusted-cert)
scripts/setup_signing.sh

# 之后每次改了 wind_macos/ 代码:
SIGN_IDENTITY="WindInput Dev" scripts/redeploy.sh
```

`redeploy.sh` 8 步骤:
1. 验证 codesign identity 可用 (user login keychain)
2. 解锁 login keychain + 设私钥 partition list (允许 codesign 直接用)
3. `build_macos_app.sh` → 出 `wind_macos/build/WindInput.app`
4. `sudo install_macos_app.sh --uninstall`
5. `sudo install_macos_app.sh` → cp 到 `/Library/Input Methods/` + lsregister + IME 自身 `--register-input-source` + `--enable-input-source`
6. 验证安装后 `.app` 签名 (Authority 链 / hardened runtime / TeamIdentifier)
7. `list_input_sources.swift` 看本 IME 是否在 TIS list
8. ✓/✗ 总结

单独跑各步：
```bash
# 仅 build (出 wind_macos/build/WindInput.app)
SIGN_IDENTITY="WindInput Dev" scripts/build_macos_app.sh

# 仅 install
sudo SIGN_IDENTITY="WindInput Dev" scripts/install_macos_app.sh
sudo scripts/install_macos_app.sh --uninstall

# 看 TIS list (是否被 macOS 收录)
swift scripts/list_input_sources.swift             # 第三方 IME (com.apple.* 之外)
swift scripts/list_input_sources.swift --all       # 全部 320 项
swift scripts/list_input_sources.swift wind        # 模糊搜
```

**已知限制 (macOS 26 Tahoe)**: self-signed cert + Personal Team Apple Development 都过不了 macOS 26 IME 注册校验，`TISRegisterInputSource` 返回 `OSStatus=0` 但 silent 不入库。需要 Apple Developer Distribution + Notarization 才能让 IME 出现在 系统设置 → 键盘 → 输入法 列表里。完整踩坑过程见 `../docs/design/macos-imkit-plan.md` "踩坑记录".

### probe_ime_mode.ps1

```powershell
# 默认 200ms 轮询，状态变化时输出
pwsh -File scripts\probe_ime_mode.ps1

# 自定义轮询间隔
pwsh -File scripts\probe_ime_mode.ps1 -IntervalMs 100
```

输出形如：

```
13:45:01.234  CN        open=1 conv=0x0001 imeWnd=0x000A0188 pid=12345  proc=notepad++          win=[xxx.txt - Notepad++]
13:45:02.451  EN        open=1 conv=0x0000 imeWnd=0x000A0188 pid=12345  proc=notepad++          win=[xxx.txt - Notepad++]
13:45:05.012  NO-IMEWND open=0 conv=0x0000 imeWnd=0x0       pid=23456  proc=Notepad             win=[文档 1 - 记事本]
```

- `Mode` 取值：
  - `CN`：IME_CMODE_NATIVE 置位（中文）
  - `EN`：NATIVE 清零（英文）
  - `OFF`：IME 未打开
  - `NO-IMEWND`：`ImmGetDefaultIMEWnd` 返回 0，前台是 TSF-only 客户端
- 验证方法：
  - 传统 IMM32 应用（cmd / Notepad++ / WPS / Chrome）：切换中英文时 `Mode` 应立即翻转，外部第三方工具（KBLSwitch）也能正确读到。
  - TSF-only 应用（Win11 新版记事本 / Edge / 部分 UWP）：通常显示 `NO-IMEWND`，**任何外部 probe 都读不到**（compartment 是进程内状态，CUAS 也没建 IMM HIMC），这种应用 KBLSwitch 的锁定功能受系统限制无法工作 —— 此时只能靠功能行为（实际锁定是否生效）验证。

## For AI Agents

### Working In This Directory

- `bump-version.ps1` 会修改多个文件中的版本号，运行前确认当前工作区干净
- `check_band.ps1` 是只读诊断工具，不修改系统状态，可随时运行
- PowerShell 脚本均不需要管理员权限
- 新增 PowerShell 脚本时保持与现有文件相同的编码风格（UTF-8 with BOM，`$ErrorActionPreference = "Stop"`）
- 新增 bash 脚本（macOS）以 `set -euo pipefail` 开头；调用 `go run` 前用 `cd "$REPO_DIR/wind_input"` 进入 module 根（仓库根本身不是 Go module）
- `build_macos.sh` 词库下载到 `.cache/`（已被 `.gitignore` 排除），dictgen / unigram 输出到 `build/data/schemas/`，仓库 `data/` 目录里的预制文件除 `AGENTS.md` 外全部复制到 `build/data/`

## Dependencies

### Internal
- `bump-version.ps1` 依赖项目根目录的 `VERSION` 文件
- `check_band.ps1` 通过 P/Invoke 调用 `user32.dll` 的 `GetWindowBand`（与 HostWindow 使用相同的非公开 API）
- `build_macos.sh` 依赖 `wind_input/cmd/{gen_unigram,gen_opencc_dict}`、`wind_input/tools/dictgen`、仓库根 `data/`、`VERSION`
- `smoke_bridge.py` 依赖正在运行的 Go 服务（`bridge.sock` + `bridge_push.sock`，路径见 `internal/bridge/endpoint_darwin.go`）
- `build_macos_app.sh` / `install_macos_app.sh` / `redeploy.sh` 依赖 `wind_macos/Package.swift`（含 `wind-input-app` target）+ `wind_macos/Sources/WindInputApp/Resources/{Info.plist, WindInput.entitlements, *.lproj/}`
- `setup_signing.sh` 操作 `~/Library/Keychains/login.keychain-db` 与 `/Library/Keychains/System.keychain`（后者需 sudo）
- `list_input_sources.swift` 用 Apple Carbon `TextInputSources` API，无外部依赖

### External
- PowerShell 5.1+ 或 PowerShell 7+（PowerShell 脚本）
- `check_band.ps1` 需要 Windows 10/11（GetWindowBand API 仅在 Win10+ 存在）
- `build_macos.sh` 需要 macOS + Go 1.24+ + `curl`
- `smoke_bridge.py` 需要 Python 3（仅用 stdlib：`socket`/`struct`/`threading`）
- `build_macos_app.sh` / `install_macos_app.sh` / `redeploy.sh` / `list_input_sources.swift` 需要 Xcode (含 `swift 5.9+`、`codesign`、`/usr/libexec/PlistBuddy`)
- `setup_signing.sh` 需要 `openssl 3+`（系统 libressl 不支持 `-legacy` 标志，建议 `brew install openssl`）+ 用户 login keychain 已解锁

<!-- MANUAL: -->
