package pinyin

import "os"

// pinyinTraceEnabled 控制 lattice / viterbi 在 pinyin 路径上的逐节点 / 逐 DP 步追踪日志。
//
// 这类日志原本用于算法开发期的逐边比对, 正常运行不需要。
// 历史问题: viterbi.go / lattice.go 里曾经 hardcoded `traceEnabled := n >= 8`,
// 意为「长输入时自动详细追踪」, 看似只在 debug 时打开但实际上在 DP 热循环里
// 每个节点都跑一次 logger.Debug. 即使输出被 logger level 过滤, key/value 的
// boxing + format 也有显著开销, 在 buf >= 8 时把 pinyin engine.ConvertEx 拖慢
// 一个量级。release binary 同样受影响因为 slog 仍会按 level 检查。
//
// 现在改为环境变量 WIND_INPUT_PINYIN_TRACE=1 显式打开 (默认关闭),
// 让正常运行路径完全不进入这些 logger.Debug 调用。
var pinyinTraceEnabled = os.Getenv("WIND_INPUT_PINYIN_TRACE") != ""
