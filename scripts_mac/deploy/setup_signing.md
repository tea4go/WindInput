# macOS 本机自签 Code Signing 证书 — 一次性 5 分钟

为什么需要这个: macOS 26 (Tahoe) 对第三方输入法的代码签名要求收紧, ad-hoc 签名 (`Signature=adhoc, TeamIdentifier=not set`) 会被 TextInputSources 注册逻辑静默拒绝, 表现为 .app 装在 `/Library/Input Methods/` 但系统设置 → 键盘 → 输入法 列表里看不到。Apple 不留 user-visible 日志, 必须给 .app 一个有真实 Authority 的签名才能让 TIS 收录。

本机自签证书 = 免费 + 5 分钟, 缺点是只在本机有效 (其他机器不认), 适合开发期。正式发布要走 Apple Developer Program ($99/年) + Notarization。

## 步骤

### 1. 打开钥匙串访问

```bash
open -a "Keychain Access"
```

### 2. 创建证书

菜单栏 **钥匙串访问 (Keychain Access)** → **证书助理 (Certificate Assistant)** → **创建证书... (Create a Certificate...)**

填入:

| 字段 | 值 |
|---|---|
| 名称 (Name) | `WindInput Dev` |
| 身份类型 (Identity Type) | `自签名根 (Self Signed Root)` |
| 证书类型 (Certificate Type) | **`代码签名 (Code Signing)`** ← 必选这个 |
| 让我覆盖默认值 (Let me override defaults) | 不勾 |

点 **创建 (Create)** → 弹出"此根证书不被信任"提示 → 点 **继续**.

### 3. 验证证书已就位

```bash
security find-identity -v -p codesigning
```

期望看到:
```
1) XXXXXXXXXX...  "WindInput Dev"
   1 valid identities found
```

如果列表里有 `"WindInput Dev"`, 完事.

### 4. 用证书重新构建并安装

```bash
# 构建
SIGN_IDENTITY="WindInput Dev" scripts_mac/build/app.sh

# 卸旧的 + 重装
sudo scripts_mac/deploy/install_app.sh --uninstall
sudo SIGN_IDENTITY="WindInput Dev" scripts_mac/deploy/install_app.sh
```

### 5. 验证签名 + TIS 注册

```bash
# 签名 Authority 现在应该指向 WindInput Dev (不再是 adhoc)
codesign -dv --verbose=4 "/Library/Input Methods/WindInput.app" 2>&1 | grep -E "Authority|Signature|TeamIdentifier"

# TIS 应该终于收录我们
swift scripts_mac/test/list_input_sources.swift
```

期望:
- `Authority=WindInput Dev`  (不是 adhoc)
- `Signature size=...` (不是 `Signature=adhoc`)
- TIS list 里出现 `to.feng.wind_input.mode`

然后去 系统设置 → 键盘 → 文本输入 → 编辑 → + → 简体中文 加 **清风输入法**.

## 删除证书 (后续不想要时)

钥匙串访问 → 登录 → 我的证书 → 找到 `WindInput Dev` → 右键 → 删除. 删完跑 `scripts_mac/deploy/install_app.sh --uninstall` 卸掉签了它的 .app.
