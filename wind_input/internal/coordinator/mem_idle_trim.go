package coordinator

import (
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"
)

const (
	// idleTrimThreshold 持续无按键活动达到该时长后，执行一次内存修剪。
	// 取 10 分钟：明显长于正常输入间隙，短于"用户离开工位"的典型时长，
	// 修剪后首次按键的重新缺页代价只在真正长时间空闲后才会发生。
	idleTrimThreshold = 10 * time.Minute
	// idleTrimCheckEvery 空闲检测周期。
	idleTrimCheckEvery = time.Minute
)

// idleMemTrimmer 空闲内存修剪器。
//
// 输入法常驻进程的 Working Set 大头是 mmap 词库的已触页（方案预热 + 正常
// 查询拉入），它们是 file-backed 页，Go 的 FreeOSMemory 管不到，OS 也只在
// 全局内存压力下才回收。本修剪器在持续空闲后调用 EmptyWorkingSet 主动把
// 整个 Working Set 还给 OS：mmap 页直接丢弃（重读文件即可恢复），堆页先经
// GC+FreeOSMemory 释放后剩余部分进备用列表。用户恢复输入后按需软缺页拉回。
//
// 每个空闲期最多修剪一次（trimmed 标记），任何按键活动重置标记。
type idleMemTrimmer struct {
	lastActivity atomic.Int64 // 最近按键活动时间（UnixNano）
	trimmed      atomic.Bool  // 本次空闲期是否已修剪
}

// noteActivity 记录一次用户活动。按键热路径调用，仅两次原子写。
func (t *idleMemTrimmer) noteActivity() {
	t.lastActivity.Store(time.Now().UnixNano())
	t.trimmed.Store(false)
}

// startIdleMemoryTrim 启动空闲内存修剪后台 goroutine。
// 与 goroutine watchdog 一样随 Coordinator 创建启动，进程生命周期内常驻。
func (c *Coordinator) startIdleMemoryTrim() {
	c.memTrim.noteActivity()
	go func() {
		ticker := time.NewTicker(idleTrimCheckEvery)
		defer ticker.Stop()
		for range ticker.C {
			t := &c.memTrim
			if t.trimmed.Load() {
				continue
			}
			idle := time.Duration(time.Now().UnixNano() - t.lastActivity.Load())
			if idle < idleTrimThreshold {
				continue
			}
			if !t.trimmed.CompareAndSwap(false, true) {
				continue
			}
			start := time.Now()
			runtime.GC()
			debug.FreeOSMemory()
			if err := emptyWorkingSet(); err != nil {
				c.logger.Warn("空闲内存修剪: EmptyWorkingSet 失败", "err", err)
				continue
			}
			c.logger.Info("空闲内存修剪完成",
				"idle", idle.Round(time.Second),
				"elapsed", time.Since(start).Round(time.Millisecond))
		}
	}()
}
