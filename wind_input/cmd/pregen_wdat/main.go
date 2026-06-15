// Command pregen_wdat 同步预生成拼音 wdat 缓存。
//
// 用途：在 build/CI 数据准备阶段提前生成 wdat，使后续 go test 运行时
// NeedsRegenerate 返回 false，避免首次 E2E 测试因后台异步生成 wdat 竞态
// 而导致 mixed 方案拼音补充缺失或测试超时。
//
// 用法：
//
//	go run ./cmd/pregen_wdat -dict schemas/pinyin/rime_frost.dict.yaml
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/huanfeng/wind_input/internal/dict/dictcache"
)

func main() {
	dictPath := flag.String("dict", "", "拼音主词库路径（rime_frost.dict.yaml）")
	out := flag.String("out", "", "输出 wdat 路径（空=使用默认缓存路径）")
	flag.Parse()

	if *dictPath == "" {
		slog.Error("缺少 -dict 参数")
		os.Exit(1)
	}

	outPath := *out
	if outPath == "" {
		outPath = dictcache.WdatCachePath("pinyin")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	srcPaths := dictcache.RimePinyinSourcePaths(*dictPath)
	if !dictcache.NeedsRegenerate(srcPaths, outPath) {
		logger.Info("wdat 已是最新，跳过生成", "path", outPath)
		return
	}

	logger.Info("开始生成 wdat", "dict", *dictPath, "out", outPath)
	if err := dictcache.ConvertPinyinToWdat(*dictPath, outPath, logger); err != nil {
		logger.Error("生成 wdat 失败", "err", err)
		os.Exit(1)
	}
	logger.Info("wdat 生成完成", "path", outPath)
}
