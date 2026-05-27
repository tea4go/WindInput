#!/usr/bin/env bash
# setup_signing.sh — 命令行创建自签 Code Signing 证书并 import 到 login keychain.
# 用 openssl + security cli, 完全跳过 Keychain Access GUI.
#
# 输出: 一个名为 "WindInput Dev" 的可用于 codesign 的本机证书.
# 用法: scripts/setup_signing.sh        # 创建
#       scripts/setup_signing.sh check  # 仅检查现状
#       scripts/setup_signing.sh remove  # 删掉证书
set -uo pipefail

CERT_NAME="WindInput Dev"
WORK_DIR="${TMPDIR:-/tmp}/wind_input_cert"
CFG_FILE="$WORK_DIR/openssl.cnf"
KEY_FILE="$WORK_DIR/cert.key"
CRT_FILE="$WORK_DIR/cert.crt"
P12_FILE="$WORK_DIR/cert.p12"
P12_PASS="windinput-dev"

bold() { printf "\n\033[1m==> %s\033[0m\n" "$*"; }
info() { printf "  %s\n" "$*"; }
err()  { printf "\033[31m[错误] %s\033[0m\n" "$*" >&2; }

CMD="${1:-create}"

# ---------------- check ----------------
if [[ "$CMD" == "check" ]]; then
    bold "查询当前 codesigning identity"
    security find-identity -v -p codesigning
    exit 0
fi

# ---------------- remove ----------------
if [[ "$CMD" == "remove" ]]; then
    bold "删 \"$CERT_NAME\" 证书 (所有同名条目)"
    # 循环删, 因为可能有重复
    while security find-certificate -c "$CERT_NAME" >/dev/null 2>&1; do
        security delete-certificate -c "$CERT_NAME" 2>&1 | sed 's/^/  /'
    done
    # 删 trust 设置 (admin trust 与 user trust)
    sudo security remove-trusted-cert -d -p codeSign 2>/dev/null || true
    bold "remove 完成"
    exit 0
fi

# ---------------- create ----------------
command -v openssl  >/dev/null || { err "openssl 未安装"; exit 1; }
command -v security >/dev/null || { err "security cli 未安装"; exit 1; }

# 清理已有同名证书 (踩过的坑: 失败的 import 也会留条目, 重复后 codesign ambiguous)
while security find-certificate -c "$CERT_NAME" >/dev/null 2>&1; do
    bold "发现已有 \"$CERT_NAME\" 证书, 清掉重建"
    security delete-certificate -c "$CERT_NAME" 2>&1 | sed 's/^/  /'
done

mkdir -p "$WORK_DIR"
chmod 700 "$WORK_DIR"

bold "1. 生成 openssl 配置 (X509 extensions for code signing)"
cat > "$CFG_FILE" <<EOF
[ req ]
distinguished_name = req_distinguished_name
prompt             = no
x509_extensions    = v3_self

[ req_distinguished_name ]
CN = $CERT_NAME
O  = WindInput Local
C  = CN

[ v3_self ]
basicConstraints       = critical, CA:false
keyUsage               = critical, digitalSignature
extendedKeyUsage       = critical, codeSigning
subjectKeyIdentifier   = hash
EOF
info "$CFG_FILE"

bold "2. 生成 RSA 2048 私钥 + 自签 X509 证书 (有效期 10 年)"
openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$KEY_FILE" -out "$CRT_FILE" \
    -days 3650 -config "$CFG_FILE" -sha256 2>&1 | tail -3 | sed 's/^/  /'
[[ -f "$CRT_FILE" ]] || { err "openssl 生成失败"; exit 1; }

bold "3. 打成 PKCS12 (.p12, legacy 格式) 以便 security import"
# openssl 3.x 默认 PBES2 (PBKDF2 + AES) macOS security import 不识别, 必须 -legacy
# 让它用老的 PKCS12 RC2-40 + SHA-1 (本地用安全够了)
openssl pkcs12 -export -legacy -inkey "$KEY_FILE" -in "$CRT_FILE" \
    -out "$P12_FILE" -name "$CERT_NAME" -passout pass:"$P12_PASS" 2>&1 | tail -3 | sed 's/^/  /'

bold "4a. unlock login keychain (会弹一次密码框)"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"
security unlock-keychain "$KEYCHAIN" || {
    err "解锁失败. 请手动跑: security unlock-keychain ~/Library/Keychains/login.keychain-db"
    exit 1
}

bold "4b. import 到 login keychain (允许 codesign 直接用)"
# -T /usr/bin/codesign: 把 codesign 加入私钥 ACL, 后续 codesign 不再弹框
# -A: 允许所有应用使用此私钥 (开发期方便, 否则每次 codesign 都要点 Always Allow)
security import "$P12_FILE" -k "$KEYCHAIN" \
    -P "$P12_PASS" -A 2>&1 | sed 's/^/  /'

bold "5. 把证书加为 trusted code-signing root (这一步要 sudo)"
# 没有 trust, codesign 用上后系统仍判 CSSMERR_TP_NOT_TRUSTED 等同 ad-hoc, IME 注册照样拒
# -d: 加到 admin trust domain (System keychain)
# -r trustRoot: 当 root CA trust
# -p codeSign: 仅信任此 cert 的 code signing 用途, 不开成全能 root
sudo security add-trusted-cert -d -r trustRoot -p codeSign \
    -k "/Library/Keychains/System.keychain" "$CRT_FILE" 2>&1 | sed 's/^/  /'

bold "6. 验证 identity 可用 (Valid identities only 段应出现 \"$CERT_NAME\")"
security find-identity -v -p codesigning | sed 's/^/  /'

if security find-identity -v -p codesigning | grep -q "\"$CERT_NAME\""; then
    bold "成功"
    info "现在跑:"
    info "  SIGN_IDENTITY=\"$CERT_NAME\" scripts/build_macos_app.sh"
    info "  sudo scripts/install_macos_app.sh --uninstall"
    info "  sudo SIGN_IDENTITY=\"$CERT_NAME\" scripts/install_macos_app.sh"
    info "  swift scripts/list_input_sources.swift"
else
    err "证书仍未 valid. 看上面 add-trusted-cert 输出"
    exit 1
fi

rm -rf "$WORK_DIR"
