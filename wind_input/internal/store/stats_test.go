package store

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func newTestCollector(t *testing.T) *StatCollector {
	t.Helper()
	s := openTestStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc := NewStatCollector(s, logger)
	t.Cleanup(func() { sc.Close() })
	return sc
}

func TestStatCollectorRecord(t *testing.T) {
	sc := newTestCollector(t)
	now := time.Now()
	sc.Record(StatEvent{
		Timestamp:    now,
		RuneCount:    5,
		ChineseCount: 5,
		CodeLen:      2,
		CandidatePos: 0,
		Source:       SourceCandidate,
		SchemaID:     "wubi86",
	})

	today := sc.GetTodayStat()
	if today.TotalChars != 5 {
		t.Fatalf("TotalChars = %d, want 5", today.TotalChars)
	}
	if today.CommitCount != 1 {
		t.Fatalf("CommitCount = %d, want 1", today.CommitCount)
	}
	if today.CodeLenSum != 2 || today.CodeLenCount != 1 {
		t.Fatalf("CodeLen sum=%d count=%d, want 2/1", today.CodeLenSum, today.CodeLenCount)
	}
	if today.CandPosDist[0] != 1 {
		t.Fatalf("CandPosDist[0] = %d, want 1", today.CandPosDist[0])
	}
	if today.BySource[SourceCandidate] != 5 {
		t.Fatalf("BySource[Candidate] = %d, want 5", today.BySource[SourceCandidate])
	}
	ss := today.BySchema["wubi86"]
	if ss == nil || ss.TotalChars != 5 || ss.CommitCount != 1 {
		t.Fatalf("BySchema[wubi86] = %+v, want TotalChars=5 CommitCount=1", ss)
	}
	if meta := sc.GetMeta(); meta.TotalChars != 5 {
		t.Fatalf("meta.TotalChars = %d, want 5", meta.TotalChars)
	}
}

func TestStatCollectorCrossDay(t *testing.T) {
	sc := newTestCollector(t)

	// 模拟旧日：直接修改私有字段（同包可访问）
	sc.mu.Lock()
	sc.today.Date = "2026-01-01"
	sc.today.TotalChars = 100
	sc.dirty = true
	sc.mu.Unlock()

	// Record 触发跨天检测，应 flush 旧日然后开启新日
	sc.Record(StatEvent{
		Timestamp:    time.Now(),
		RuneCount:    3,
		ChineseCount: 3,
		Source:       SourceCandidate,
	})

	// 旧日数据应已写入 DB
	old, err := sc.store.GetDailyStat("2026-01-01")
	if err != nil || old == nil {
		t.Fatalf("old day not flushed to DB: %v", err)
	}
	if old.TotalChars != 100 {
		t.Fatalf("old.TotalChars = %d, want 100", old.TotalChars)
	}

	// 新日只含新记录
	today := sc.GetTodayStat()
	if today.Date == "2026-01-01" {
		t.Fatal("sc.today.Date not updated after cross-day")
	}
	if today.TotalChars != 3 {
		t.Fatalf("new day TotalChars = %d, want 3", today.TotalChars)
	}

	// 旧日 flush 应同时更新 streak
	meta := sc.GetMeta()
	if meta.StreakLastDay != "2026-01-01" {
		t.Fatalf("StreakLastDay = %q, want 2026-01-01", meta.StreakLastDay)
	}
	if meta.StreakCurrent < 1 {
		t.Fatalf("StreakCurrent = %d, want >= 1", meta.StreakCurrent)
	}
}

func TestStatCollectorActiveTime(t *testing.T) {
	sc := newTestCollector(t)
	base := time.Now()

	sc.Record(StatEvent{Timestamp: base, RuneCount: 1, Source: SourceCandidate})
	// 10s 间隔 < 15s 阈值，应累加
	sc.Record(StatEvent{Timestamp: base.Add(10 * time.Second), RuneCount: 1, Source: SourceCandidate})
	// 90s 间隔 > 15s 阈值，应跳过
	sc.Record(StatEvent{Timestamp: base.Add(100 * time.Second), RuneCount: 1, Source: SourceCandidate})

	today := sc.GetTodayStat()
	if today.ActiveSeconds != 10 {
		t.Fatalf("ActiveSeconds = %d, want 10", today.ActiveSeconds)
	}
}

func TestStatCollectorStreak(t *testing.T) {
	sc := newTestCollector(t)

	// Day1 flush
	sc.mu.Lock()
	sc.today.Date = "2026-04-01"
	sc.today.TotalChars = 10
	sc.dirty = true
	sc.flushLocked()
	sc.mu.Unlock()

	meta := sc.GetMeta()
	if meta.StreakCurrent != 1 || meta.StreakLastDay != "2026-04-01" {
		t.Fatalf("day1: streak=%d last=%q, want 1/2026-04-01", meta.StreakCurrent, meta.StreakLastDay)
	}

	// Day2 连续 → streak +1
	sc.mu.Lock()
	sc.today.Date = "2026-04-02"
	sc.today.TotalChars = 10
	sc.dirty = true
	sc.flushLocked()
	sc.mu.Unlock()

	meta = sc.GetMeta()
	if meta.StreakCurrent != 2 || meta.StreakMax != 2 {
		t.Fatalf("day2: current=%d max=%d, want 2/2", meta.StreakCurrent, meta.StreakMax)
	}

	// Day4 有间隔 → streak 重置为 1，max 保留 2
	sc.mu.Lock()
	sc.today.Date = "2026-04-04"
	sc.today.TotalChars = 10
	sc.dirty = true
	sc.flushLocked()
	sc.mu.Unlock()

	meta = sc.GetMeta()
	if meta.StreakCurrent != 1 || meta.StreakMax != 2 {
		t.Fatalf("day4: current=%d max=%d, want 1/2", meta.StreakCurrent, meta.StreakMax)
	}
}

func TestSpeedPerMinute(t *testing.T) {
	// 正常时间段：直接计算
	if got := SpeedPerMinute(252, 6); got != 2520 {
		t.Fatalf("SpeedPerMinute(252, 6) = %d, want 2520", got)
	}
	if got := SpeedPerMinute(252, 120); got != 126 {
		t.Fatalf("SpeedPerMinute(252, 120) = %d, want 126", got)
	}
	// 极短时间触发 5s 下限
	if got := SpeedPerMinute(60, 3); got != 720 {
		t.Fatalf("SpeedPerMinute(60, 3) = %d, want 720", got)
	}
	// 零值边界
	if got := SpeedPerMinute(0, 60); got != 0 {
		t.Fatalf("SpeedPerMinute(0, 60) = %d, want 0", got)
	}
	if got := SpeedPerMinute(100, 0); got != 0 {
		t.Fatalf("SpeedPerMinute(100, 0) = %d, want 0", got)
	}
}

func TestRecalculateStatsMetaAfterPrune(t *testing.T) {
	s := openTestStore(t)

	stats := []*DailyStat{
		{Date: "2026-04-20", TotalChars: 100, ActiveSeconds: 60},
		{Date: "2026-04-21", TotalChars: 200, ActiveSeconds: 120},
		{Date: "2026-04-23", TotalChars: 300, ActiveSeconds: 60},
	}
	for _, stat := range stats {
		stat.BySchema = map[string]*SchemaStats{}
		if err := s.PutDailyStat(stat); err != nil {
			t.Fatalf("PutDailyStat(%s): %v", stat.Date, err)
		}
	}

	deleted, err := s.PruneStats("2026-04-21")
	if err != nil {
		t.Fatalf("PruneStats: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("PruneStats deleted %d, want 1", deleted)
	}

	meta, err := s.RecalculateStatsMeta()
	if err != nil {
		t.Fatalf("RecalculateStatsMeta: %v", err)
	}
	if meta.TotalChars != 500 {
		t.Fatalf("TotalChars = %d, want 500", meta.TotalChars)
	}
	if meta.FirstDay != "2026-04-21" {
		t.Fatalf("FirstDay = %q, want 2026-04-21", meta.FirstDay)
	}
	if meta.StreakCurrent != 1 || meta.StreakMax != 1 || meta.StreakLastDay != "2026-04-23" {
		t.Fatalf("streak = current %d max %d last %q, want 1/1/2026-04-23",
			meta.StreakCurrent, meta.StreakMax, meta.StreakLastDay)
	}
	if meta.MaxSpeed != 300 {
		t.Fatalf("MaxSpeed = %d, want 300", meta.MaxSpeed)
	}
}
