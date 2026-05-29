#!/usr/bin/env bash
# dev_mac.sh — macOS 开发助手 (仓库根), 对位 dev.ps1.
# 菜单见 usage(); 调试版统一加前缀 d (如 d1 / d2 / dr / di).
set -euo pipefail

REPO_DIR=$(cd "$(dirname "$0")" && pwd)
MAC="$REPO_DIR/scripts_mac"
PID_FILE="$HOME/Library/Application Support/WindInput/wind_input.pid"

usage() {
    cat <<'EOF'
WindInput - Dev Menu (macOS)     调试版加前缀 d (如 d1 / d2 / dr / di / du)

  -- 构建 --
  1      构建全部 (Go 服务 + 词库 + IME .app)
  2      仅构建 Go 服务 (跳过词库下载)
  app    仅构建 IME .app bundle
  clean  清 build/ 与 build_debug/

  -- 本机安装 / 卸载 --
  i         本机安装 (Go 服务 LaunchAgent + IME .app)
  redeploy  IME .app 重签 + 重装 + TIS 验证 (需 SIGN_IDENTITY, macOS 26 主入口)
  u         本机卸载 (Go 服务 + IME .app)

  -- 远程 VM (余参透传给 scripts_mac/vm/deploy.sh) --
  deploy    host→VM 一键部署 (服务 + .app)
  undeploy  host→VM 远程卸载 + 验证清除
            (附加 --service-only / --app-only / 目标 admin@ip 均透传)

  -- 运行 / 诊断 (macOS IME 专用) --
  r      前台运行 Go 服务 (debug 日志)
  stop   停止前台 Go 服务 (按 pid 文件)
  smoke  协议验证 (swift run wind-smoke [秒])
  tis    显示 TIS 内 WindInput 注册状态

用法: ./dev_mac.sh [菜单代码] [附加参数]
EOF
}

CHOICE="${1:-}"
if [[ -z "$CHOICE" ]]; then
    usage
    printf "请选择: "
    read -r CHOICE
fi
[[ -z "$CHOICE" ]] && { usage; exit 1; }

# 调试版前缀 d: 白名单显式列出, 避免误剥离 deploy 的首字母 d.
VARIANT=""
case "$CHOICE" in
    d1)   VARIANT="--debug"; CHOICE="1" ;;
    d2)   VARIANT="--debug"; CHOICE="2" ;;
    dapp) VARIANT="--debug"; CHOICE="app" ;;
    di)   VARIANT="--debug"; CHOICE="i" ;;
    du)   VARIANT="--debug"; CHOICE="u" ;;
    dr)   VARIANT="--debug"; CHOICE="r" ;;
esac

# ---- 构建 ----
do_build_all() { "$MAC/build/build.sh" all ${VARIANT:+$VARIANT}; "$MAC/build/app.sh" ${VARIANT:+$VARIANT}; }
do_build_svc() { "$MAC/build/build.sh" service ${VARIANT:+$VARIANT}; }
do_build_app() { "$MAC/build/app.sh" ${VARIANT:+$VARIANT}; }
do_clean()     { "$MAC/build/build.sh" clean; }

# ---- 安装 / 部署 ----
# install_service.sh 是 per-user (不要 sudo); install_app.sh 装到 /Library/ 需 sudo.
do_install() {
    "$MAC/deploy/install_service.sh" ${VARIANT:+$VARIANT}
    sudo bash "$MAC/deploy/install_app.sh"
}
do_redeploy() { bash "$MAC/deploy/redeploy.sh"; }
do_deploy()   { bash "$MAC/vm/deploy.sh" "$@"; }

# ---- 卸载 ----
do_uninstall() {
    "$MAC/deploy/install_service.sh" --uninstall
    sudo bash "$MAC/deploy/install_app.sh" --uninstall
}

# ---- 运行 / 诊断 ----
do_run() {
    local exe
    if [[ -n "$VARIANT" ]]; then exe="$REPO_DIR/build_debug/wind_input_debug"; else exe="$REPO_DIR/build/wind_input"; fi
    [[ -x "$exe" ]] || { echo "[错误] 未找到 $exe, 先构建 (1 / 2 / d1 / d2)" >&2; exit 1; }
    cd "$(dirname "$exe")"
    echo "==> 启动 $exe (Ctrl+C 退出)"
    WIND_INPUT_LOG_LEVEL=debug ./"$(basename "$exe")"
}
do_stop() {
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            echo "已发送 SIGTERM 到 pid=$pid"
        else
            echo "pid $pid 已不在运行, 清理 pid 文件"
            rm -f "$PID_FILE"
        fi
    else
        echo "无 pid 文件, 未发现运行中的服务"
    fi
}
do_smoke() {
    cd "$REPO_DIR/wind_macos"
    swift run wind-smoke "${1:-10}"
}
do_tis() {
    local swift_tool="$MAC/test/list_input_sources.swift"
    [[ -f "$swift_tool" ]] || { echo "[错误] 未找到 $swift_tool" >&2; exit 1; }
    echo "==> TIS 内 WindInput / 相关条目 (huanfeng / wind / qingg / imkit)"
    swift "$swift_tool" 2>/dev/null \
        | grep -iE "huanfeng|wind|qingg|aodaren|imkit" | sed 's/^/  /' \
        || echo "  (无)"
}

case "$CHOICE" in
    1)         do_build_all ;;
    2)         do_build_svc ;;
    app)       do_build_app ;;
    clean)     do_clean ;;
    i)         do_install ;;
    redeploy)  do_redeploy ;;
    u)         do_uninstall ;;
    deploy)    do_deploy "${@:2}" ;;
    undeploy)  do_deploy --uninstall "${@:2}" ;;
    r)         do_run ;;
    stop)      do_stop ;;
    smoke)     do_smoke "${2:-10}" ;;
    tis)       do_tis ;;
    *)         echo "[错误] 未知选项: $CHOICE" >&2; usage; exit 1 ;;
esac
