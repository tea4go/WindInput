//go:build darwin

package main

import "os/exec"

// shellOpen 用 macOS `open` 打开文件、目录或 URL (走 LaunchServices, 用默认应用)。
func shellOpen(path string) error {
	return exec.Command("open", path).Start()
}
