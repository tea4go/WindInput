package pinyin

// IsShuangpinFinalKey 判断给定键（单字节字符）在当前双拼方案的韵母映射表（FinalMap）中是否有映射。
// 若引擎处于全拼模式（无双拼转换器），始终返回 false。
// 此方法供 coordinator 层在候选选词热键检测前做 guard 判断：
// 当有未上屏编码、当前为双拼模式且该键为韵母键时，应优先作为编码输入，而非候选选词热键。
func (e *Engine) IsShuangpinFinalKey(key byte) bool {
	if e.spConverter == nil {
		return false
	}
	scheme := e.spConverter.GetScheme()
	if scheme == nil {
		return false
	}
	_, ok := scheme.FinalMap[key]
	return ok
}
