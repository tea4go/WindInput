package main

import "github.com/huanfeng/wind_input/pkg/rpcapi"

// GetStatsSummary 获取统计概览
func (a *App) GetStatsSummary() (*rpcapi.StatsSummaryReply, error) {
	return a.rpcClient.StatsGetSummary()
}

// GetDailyStats 获取日期范围内的每日统计
func (a *App) GetDailyStats(from, to string) (*rpcapi.StatsGetDailyReply, error) {
	return a.rpcClient.StatsGetDaily(from, to)
}

// ClearStats 清空统计数据
func (a *App) ClearStats() error {
	return a.rpcClient.StatsClear()
}

// ClearStatsBefore 清理指定天数之前的统计数据
func (a *App) ClearStatsBefore(days int) (*rpcapi.StatsPruneReply, error) {
	return a.rpcClient.StatsPrune(days)
}
