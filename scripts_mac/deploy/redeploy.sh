#!/usr/bin/env bash
# redeploy.sh — 一键重新签名 + 卸载 + 安装 + 验证 TIS 注册.
#
# 用法:
#   sudo scripts_mac/deploy/redeploy.sh                     # 默认用 "WindInput Dev" 证书
#   sudo SIGN_IDENTITY="My Cert" scripts_mac/deploy/redeploy.sh
#
# 完整流程:
#   1. (drop sudo) 用 SIGN_IDENTITY 重 build .app (注入 hardened runtime + entitlements + 真证书)
#   2. uninstall 旧版
#   3. install 新版 (用户域 ~/Library, 无 sudo; cp + lsregister + TISRegisterInputSource)
#   4. 验证 .app 签名 / spctl / TIS list
set -uo pipefail

REPO_DIR=$(cd "$(dirname "$0")/../.." && pwd)
SIGN_IDENTITY="${SIGN_IDENTITY:-WindInput Dev}"

bold() { printf "\n\033[1m==> %s\033[0m\n" "$*"; }
info() { printf "  %s\n" "$*"; }
err()  { printf "\033[31m[错误] %s\033[0m\n" "$*" >&2; }

# 设计上必须以普通用户跑 (codesign 要访问 user login keychain 里的私钥;
# sudo -u user 子 shell 拿不到 user session 的 keychain unlock 状态, 报
# errSecInternalComponent). 内部对需要 root 的步骤显式 invoke sudo.
if [[ $EUID -eq 0 ]]; then
    err "请直接以普通用户跑 (不要 sudo). 我会在需要 root 时自动 invoke sudo."
    err "  用法: scripts_mac/deploy/redeploy.sh"
    exit 1
fi

export SIGN_IDENTITY
export PATH=/opt/homebrew/bin:/usr/local/bin:$PATH

bold "1. 验证 codesign identity 可用 (user keychain)"
# 用 -F (固定字符串) 避免证书名里的 ( ) . @ 等被当 regex 解析
IDENT_LINE=$(security find-identity -v -p codesigning | grep -F "\"$SIGN_IDENTITY\"" | head -1)
if [[ -z "$IDENT_LINE" ]]; then
    err "找不到 valid identity \"$SIGN_IDENTITY\"."
    err "  当前 keychain 里的 codesigning identity:"
    security find-identity -v -p codesigning | sed 's/^/    /'
    exit 1
fi
info "$IDENT_LINE"

bold "2. 解锁 login keychain + 设私钥 partition list (允许 codesign 用)"
# codesign 报 errSecInternalComponent 多数是因为 partition list 限制 codesign
# 访问私钥. 必须一次性允许 apple-tool/apple/codesign 走 partition list.
# (会弹一次 keychain 密码框)
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
security unlock-keychain "$KEYCHAIN" 2>&1 | sed 's/^/  /' || true
security set-key-partition-list -S apple-tool:,apple:,codesign: -s "$KEYCHAIN" \
    >/dev/null 2>&1 || info "set-key-partition-list 已是允许状态 (或弹密码框被取消)"

bold "3. 重 build .app (release + hardened runtime + \"$SIGN_IDENTITY\" 签名)"
"$REPO_DIR/scripts_mac/build/app.sh" 2>&1 | tail -20 | sed 's/^/  /'

# 立刻验证 build 出的 .app 真用了 SIGN_IDENTITY 签
BUILT_APP="$REPO_DIR/wind_macos/build/WindInput.app"
BUILT_AUTH=$(codesign -dv --verbose=4 "$BUILT_APP" 2>&1 | grep "^Authority=" | head -1)
if [[ "$BUILT_AUTH" != "Authority=$SIGN_IDENTITY" ]]; then
    err "build 后的 .app Authority 不是 \"$SIGN_IDENTITY\" 而是: $BUILT_AUTH"
    err "codesign 可能因 keychain ACL 拒绝, 跑以下命令解 partition list:"
    err "  security unlock-keychain ~/Library/Keychains/login.keychain-db"
    err "  security set-key-partition-list -S apple-tool:,apple:,codesign: -s ~/Library/Keychains/login.keychain-db"
    exit 1
fi
info "build OK, $BUILT_AUTH"

bold "4. uninstall 旧版 (用户域, 无 sudo)"
"$REPO_DIR/scripts_mac/deploy/install_app.sh" --uninstall 2>&1 | tail -10 | sed 's/^/  /'

bold "5. 装新版 (用户域, 含 lsregister + --register-input-source)"
"$REPO_DIR/scripts_mac/deploy/install_app.sh" 2>&1 | sed 's/^/  /'

bold "6. 验证安装后 .app 签名"
codesign -dv --verbose=4 "$HOME/Library/Input Methods/WindInput.app" 2>&1 \
    | grep -E "Authority|Signature|flags|TeamIdentifier|Runtime|Format" | sed 's/^/  /'

bold "7. TIS list 看本 IME 是否终于收录"
/usr/bin/swift "$REPO_DIR/scripts_mac/test/list_input_sources.swift" 2>&1 | head -30 | sed 's/^/  /'

bold "8. 结果"
if /usr/bin/swift "$REPO_DIR/scripts_mac/test/list_input_sources.swift" 2>&1 \
        | grep -q "to.feng.inputmethod.WindInput"; then
    info "✓ TIS 收录成功. 现在去 系统设置 → 键盘 → 文本输入 → 编辑 → +"
    info "  → 简体中文 → 选 \"清风输入法\" 添加, 然后切到 WindInput 测试"
else
    info "✗ TIS 仍未收录. 复制上面 5/6 段输出 + install 日志贴回"
fi
