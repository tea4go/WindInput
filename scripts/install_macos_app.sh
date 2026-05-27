#!/usr/bin/env bash
# install_macos_app.sh — 把 WindInput.app 装到 /Library/Input Methods/.
#
# 需要 sudo. 装完后用户去 系统设置 → 键盘 → 输入法 → + 号 → 中文 → WindInput
# 添加一次, 后续就能在状态栏 IME 切换菜单看到.
#
# 用法:
#   scripts/install_macos_app.sh                  # 装 release build
#   scripts/install_macos_app.sh --debug          # 装 debug build (路径同)
#   scripts/install_macos_app.sh --build          # 先 build 再装
#   scripts/install_macos_app.sh --uninstall      # 卸载
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
MACOS_DIR="$REPO_DIR/wind_macos"
APP_NAME="WindInput"
APP_BUNDLE="$MACOS_DIR/build/$APP_NAME.app"
INSTALL_DIR="/Library/Input Methods"
INSTALL_APP="$INSTALL_DIR/$APP_NAME.app"

DO_BUILD=0
DO_UNINSTALL=0
BUILD_ARGS=()
for arg in "$@"; do
    case "$arg" in
        --build) DO_BUILD=1 ;;
        --debug) BUILD_ARGS+=("--debug") ;;
        --uninstall) DO_UNINSTALL=1 ;;
        *) echo "[错误] 未知参数: $arg" >&2; exit 1 ;;
    esac
done

bold() { printf "\033[1m%s\033[0m\n" "$*"; }
info() { printf "  %s\n" "$*"; }
err()  { printf "\033[31m[错误] %s\033[0m\n" "$*" >&2; }

# -------- uninstall --------
if [[ $DO_UNINSTALL -eq 1 ]]; then
    bold "==> Uninstall $INSTALL_APP"
    sudo killall "$APP_NAME" 2>/dev/null || true
    sudo rm -rf "$INSTALL_APP"
    info "已删除 $INSTALL_APP"
    info "注: 系统设置里需手动移除 WindInput 输入法项 (键盘 → 输入法 → -)"
    exit 0
fi

# -------- build (可选) --------
if [[ $DO_BUILD -eq 1 ]]; then
    "$SCRIPT_DIR/build_macos_app.sh" "${BUILD_ARGS[@]}"
fi

[[ -d "$APP_BUNDLE" ]] || { err "未找到 $APP_BUNDLE, 先跑 scripts/build_macos_app.sh"; exit 1; }

# -------- install --------
bold "==> Install $APP_BUNDLE -> $INSTALL_APP"

# 1. 关掉旧实例 (IMKit 进程通常常驻; 不杀的话 cp 会被持锁)
if pgrep -x "$APP_NAME" >/dev/null; then
    info "停止旧 $APP_NAME 进程"
    sudo killall "$APP_NAME" 2>/dev/null || true
    sleep 0.5
fi

# 2. 复制 .app
sudo rm -rf "$INSTALL_APP"
sudo cp -R "$APP_BUNDLE" "$INSTALL_DIR/"
info "已复制 $INSTALL_APP"

# 3. 重新签名 (cp 后保留签名; 如失败仍 ad-hoc 重签一次, 双保险)
sudo codesign --force --sign - --timestamp=none "$INSTALL_APP" >/dev/null 2>&1 || true

# 4. 让系统重新发现 IME bundle (TextInputSources 缓存)
killall -HUP cfprefsd 2>/dev/null || true

bold "==> Done"
cat <<EOF

  下一步:
    1. 打开 系统设置 → 键盘 → 文本输入 → 编辑 → 添加 (+) → 简体中文 → 选 WindInput
    2. 切到 WindInput (Ctrl+Space 或菜单栏 IME 切换)
    3. 在任意文本框敲一个字母键, 然后:

         tail -F "\$HOME/Library/Caches/WindInput/logs/wind_input.log"
         log stream --predicate 'process == "WindInput"' --info --debug

       应看到:
         Go 端 : "bridge client connected connID=N"
         IME 端: "WindInput[InputController] bridge connected"
                "WindInput[handle] ..." 或 PassThrough/Consumed 路径

  卸载:    sudo scripts/install_macos_app.sh --uninstall

EOF
