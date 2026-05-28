package ui

// dpi_neutral.go — 平台无关的 DPI 抽象。
// Win 端 dpi.go (windows tag) 在 init 时把真实 DPI 计算函数注入 dpiScaleProvider;
// darwin 端默认返回 1.0 (logical pixels), 后续可由 IMKit 端发 dpiChanged 事件时
// 调 SetDPIScale 更新。

var dpiScaleProvider = func() float64 { return 1.0 }

// SetDPIScaleProvider 允许平台层 (win/darwin) 注入自己的 DPI 计算逻辑。
// Win 端注入返回 GetEffectiveDPI()/96 的闭包; darwin 端可注入读 NSScreen.backingScaleFactor。
func SetDPIScaleProvider(p func() float64) {
	if p != nil {
		dpiScaleProvider = p
	}
}

// GetDPIScale returns the DPI scale factor (1.0 = 100%, 1.5 = 150%, etc.)
func GetDPIScale() float64 {
	return dpiScaleProvider()
}

// ScaleForDPI scales a value according to the current DPI.
func ScaleForDPI(value float64) float64 {
	return value * GetDPIScale()
}

// ScaleIntForDPI scales an integer value according to the current DPI.
func ScaleIntForDPI(value int) int {
	return int(float64(value) * GetDPIScale())
}
