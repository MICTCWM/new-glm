package model

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareSpecialUsageTables(t *testing.T) {
	t.Helper()
	clear := DB.Session(&gorm.Session{AllowGlobalUpdate: true})
	require.NoError(t, clear.Delete(&SpecialUsageHourly{}).Error)
	require.NoError(t, clear.Delete(&SpecialUsageRecord{}).Error)
	t.Cleanup(func() {
		_ = clear.Delete(&SpecialUsageHourly{}).Error
		_ = clear.Delete(&SpecialUsageRecord{}).Error
	})
}

func TestMergeSpecialUsageRecordPrefersFinalUsage(t *testing.T) {
	existing := SpecialUsageRecord{
		InputTokens:     10,
		OutputTokens:    2,
		UpstreamCostUSD: 0.01,
		UserChargeUSD:   0.02,
		Status:          SpecialUsageStatusFailed,
		RequestTime:     100,
	}
	incoming := SpecialUsageRecord{
		UserID:           8,
		ChannelName:      "final-channel",
		GroupName:        "final-group",
		ModelName:        "final-model",
		InputTokens:      1,
		OutputTokens:     2,
		UpstreamCostUSD:  0.001,
		UserChargeUSD:    0.002,
		InputPriceUSD:    0.003,
		OutputPriceUSD:   0.004,
		Multiplier:       0.5,
		UsedSpecialPrice: true,
		Status:           SpecialUsageStatusSuccess,
		RequestTime:     200,
	}

	merged, improved := mergeSpecialUsageRecord(existing, incoming)
	require.True(t, improved)
	require.Equal(t, SpecialUsageStatusSuccess, merged.Status)
	require.Equal(t, incoming.UserID, merged.UserID)
	require.Equal(t, incoming.ChannelName, merged.ChannelName)
	require.Equal(t, incoming.GroupName, merged.GroupName)
	require.Equal(t, incoming.ModelName, merged.ModelName)
	require.Equal(t, incoming.InputTokens, merged.InputTokens)
	require.Equal(t, incoming.OutputTokens, merged.OutputTokens)
	require.Equal(t, incoming.UpstreamCostUSD, merged.UpstreamCostUSD)
	require.Equal(t, incoming.UserChargeUSD, merged.UserChargeUSD)
	require.Equal(t, incoming.InputPriceUSD, merged.InputPriceUSD)
	require.Equal(t, incoming.OutputPriceUSD, merged.OutputPriceUSD)
	require.Equal(t, incoming.Multiplier, merged.Multiplier)
	require.Equal(t, incoming.UsedSpecialPrice, merged.UsedSpecialPrice)
	require.Equal(t, existing.RequestTime, merged.RequestTime)
}

func TestMergeSpecialUsageRecordDoesNotDowngradeFinalUsage(t *testing.T) {
	existing := SpecialUsageRecord{
		InputTokens:     100,
		OutputTokens:    20,
		UpstreamCostUSD: 0.2,
		UserChargeUSD:   0.3,
		Status:          SpecialUsageStatusSuccess,
	}
	incoming := SpecialUsageRecord{
		InputTokens:     0,
		OutputTokens:    0,
		UpstreamCostUSD: 0,
		UserChargeUSD:   0,
		Status:          SpecialUsageStatusFailed,
	}

	merged, improved := mergeSpecialUsageRecord(existing, incoming)
	require.False(t, improved)
	require.Equal(t, existing, merged)
}

func TestSpecialUsageModelCandidatesIncludesCompactAlias(t *testing.T) {
	candidates := specialUsageModelCandidates("special-model-openai-compact")
	require.Contains(t, candidates, "special-model-openai-compact")
	require.Contains(t, candidates, "special-model")
}

func TestUpdateSpecialUsageHourlyBackfillsMissingBucket(t *testing.T) {
	prepareSpecialUsageTables(t)
	requestTime := int64(10*3600 + 100)
	records := []SpecialUsageRecord{
		{RequestID: "backfill-existing", ChannelID: 1, GroupName: "group", ModelName: "model", RequestTime: requestTime, InputTokens: 10, OutputTokens: 2, UpstreamCostUSD: 0.1, UserChargeUSD: 0.2},
		{RequestID: "backfill-current", ChannelID: 1, GroupName: "group", ModelName: "model", RequestTime: requestTime + 10, InputTokens: 20, OutputTokens: 4, UpstreamCostUSD: 0.3, UserChargeUSD: 0.4},
	}
	require.NoError(t, DB.Create(&records).Error)

	tx := DB.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, updateSpecialUsageHourly(tx, nil, &records[1]))
	require.NoError(t, tx.Commit().Error)

	var hourly SpecialUsageHourly
	require.NoError(t, DB.Where("bucket_time = ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketHour(requestTime), "group", 1, "model").First(&hourly).Error)
	require.Equal(t, int64(2), hourly.RequestCount)
	require.Equal(t, int64(30), hourly.InputTokens)
	require.Equal(t, int64(6), hourly.OutputTokens)
	require.InDelta(t, 0.4, hourly.UpstreamCostUSD, 1e-9)
	require.InDelta(t, 0.6, hourly.UserChargeUSD, 1e-9)
}

func TestUpdateSpecialUsageHourlyBackfillsZeroDeltaRetry(t *testing.T) {
	prepareSpecialUsageTables(t)
	record := SpecialUsageRecord{
		RequestID: "zero-delta-retry", ChannelID: 2, GroupName: "group", ModelName: "model", RequestTime: 11*3600 + 100,
		InputTokens: 12, OutputTokens: 3, UpstreamCostUSD: 0.12, UserChargeUSD: 0.24,
	}
	require.NoError(t, DB.Create(&record).Error)

	tx := DB.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, updateSpecialUsageHourly(tx, &record, &record))
	require.NoError(t, tx.Commit().Error)

	var hourly SpecialUsageHourly
	require.NoError(t, DB.Where("bucket_time = ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketHour(record.RequestTime), record.GroupName, record.ChannelID, record.ModelName).First(&hourly).Error)
	require.Equal(t, int64(1), hourly.RequestCount)
	require.Equal(t, record.InputTokens, hourly.InputTokens)
	require.Equal(t, record.OutputTokens, hourly.OutputTokens)
	require.InDelta(t, record.UpstreamCostUSD, hourly.UpstreamCostUSD, 1e-9)
	require.InDelta(t, record.UserChargeUSD, hourly.UserChargeUSD, 1e-9)
}
