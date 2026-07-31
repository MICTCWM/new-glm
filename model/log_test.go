package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSumTodayConsumeQuotaUsesLocalCalendarDay(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	marker := "daily-usage-limit-test"

	removeTestLogs := func() {
		LOG_DB.Where("content = ?", marker).Delete(&Log{})
	}
	removeTestLogs()
	t.Cleanup(func() {
		removeTestLogs()
	})

	require.NoError(t, LOG_DB.Create(&Log{Content: marker, CreatedAt: start.Add(-time.Second).Unix(), Type: LogTypeConsume, Quota: 100}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Content: marker, CreatedAt: start.Unix(), Type: LogTypeConsume, Quota: 200}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Content: marker, CreatedAt: now.Unix(), Type: LogTypeConsume, Quota: 300}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Content: marker, CreatedAt: start.AddDate(0, 0, 1).Unix(), Type: LogTypeConsume, Quota: 400}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Content: marker, CreatedAt: now.Unix(), Type: LogTypeTopup, Quota: 500}).Error)

	quota, err := SumTodayConsumeQuota(now)
	require.NoError(t, err)
	require.Equal(t, 500, quota)
}
