#!/usr/bin/env bash
# build_macos_app.sh — 拼装 WindInput.app bundle (PR-A M2).
#
# SwiftPM 不直接产 .app, 这里:
#   1. swift build --product wind-input-app  (release, arm64)
#   2. 按标准 macOS .app 结构拼 Contents/{MacOS, Resources, Info.plist}
#   3. codesign --force --sign - (ad-hoc 签名, 让本机能加载; 上架走 PR-A.5 M6)
#
# 输出: wind_macos/build/WindInput.app
#
# 用法:
#   scripts/build_macos_app.sh            # release build + ad-hoc 签名
#   scripts/build_macos_app.sh --debug    # debug build (swift build -c debug)
#   scripts/build_macos_app.sh --no-sign  # 不 codesign (调试用)
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
MACOS_DIR="$REPO_DIR/wind_macos"
APP_NAME="WindInput"
APP_BUNDLE="$MACOS_DIR/build/$APP_NAME.app"

SWIFT_CONFIG="release"
DO_SIGN=1
for arg in "$@"; do
    case "$arg" in
        --debug)   SWIFT_CONFIG="debug" ;;
        --no-sign) DO_SIGN=0 ;;
        *) echo "[错误] 未知参数: $arg" >&2; exit 1 ;;
    esac
done

bold() { printf "\033[1m%s\033[0m\n" "$*"; }
info() { printf "  %s\n" "$*"; }
err()  { printf "\033[31m[错误] %s\033[0m\n" "$*" >&2; }

command -v swift    >/dev/null || { err "swift 未安装 (装 Xcode CLT)"; exit 1; }
command -v codesign >/dev/null || { err "codesign 未安装 (装 Xcode CLT)"; exit 1; }

bold "==> Build wind-input-app ($SWIFT_CONFIG)"
cd "$MACOS_DIR"
swift build -c "$SWIFT_CONFIG" --product wind-input-app

# SwiftPM 把二进制放在 .build/<config>/wind-input-app
BIN_PATH="$MACOS_DIR/.build/$SWIFT_CONFIG/wind-input-app"
[[ -x "$BIN_PATH" ]] || { err "二进制未找到: $BIN_PATH"; exit 1; }
info "binary: $BIN_PATH ($(stat -f%z "$BIN_PATH") bytes)"

bold "==> Assemble $APP_BUNDLE"
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS" "$APP_BUNDLE/Contents/Resources"

# 二进制 → Contents/MacOS/WindInput (与 Info.plist 的 CFBundleExecutable 对齐)
cp "$BIN_PATH" "$APP_BUNDLE/Contents/MacOS/$APP_NAME"
chmod +x "$APP_BUNDLE/Contents/MacOS/$APP_NAME"

# Info.plist
cp "$MACOS_DIR/Sources/WindInputApp/Resources/Info.plist" "$APP_BUNDLE/Contents/Info.plist"

# (可选) 资源, 当前无图标; 后续 M4 加 menu icon 时把 .icns 放 Resources/
# cp "$MACOS_DIR/Resources/AppIcon.icns" "$APP_BUNDLE/Contents/Resources/" 2>/dev/null || true

# 写一个空的 PkgInfo (传统 macOS 期望)
printf "APPL????" > "$APP_BUNDLE/Contents/PkgInfo"

# 校验 Info.plist
plutil -lint "$APP_BUNDLE/Contents/Info.plist" >/dev/null

# Ad-hoc 签名 (本机加载够用)
if [[ $DO_SIGN -eq 1 ]]; then
    bold "==> codesign ad-hoc"
    ENTS="$MACOS_DIR/Sources/WindInputApp/Resources/WindInput.entitlements"
    if [[ -f "$ENTS" ]]; then
        codesign --force --sign - --entitlements "$ENTS" --timestamp=none "$APP_BUNDLE"
    else
        codesign --force --sign - --timestamp=none "$APP_BUNDLE"
    fi
    codesign -dv --verbose=2 "$APP_BUNDLE" 2>&1 | sed 's/^/    /' | head -10
fi

bold "==> Done"
info "Bundle: $APP_BUNDLE"
info "下一步: sudo scripts/install_macos_app.sh"
info "       (会把 .app 复制到 /Library/Input Methods/ 并 killall 旧实例)"
