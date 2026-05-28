//go:build darwin

package updater

import "os/exec"

// InstallRelease 在 macOS 上用 `open` 打开下载的安装包 (.dmg/.pkg/.app),
// 交给系统安装器/Finder 处理。silent 在 macOS 无对应语义, 忽略。
func InstallRelease(installerPath string, silent bool) error {
	_ = silent
	return exec.Command("open", installerPath).Start()
}
