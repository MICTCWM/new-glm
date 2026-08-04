package model

import (
	"archive/zip"
	"bytes"
	"io"
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
		RequestTime:      200,
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

func TestNormalizeSpecialUsageConfigInfersExplicitChannelSelection(t *testing.T) {
	config := normalizeSpecialUsageConfig(SpecialUsageConfig{
		ChannelIDs:         []int{9, 3, 9},
		ChannelMultipliers: map[string]float64{"3": 2, "9": 0},
	})

	if !config.ChannelIDsSet {
		t.Fatal("expected a non-empty legacy channel list to become explicit")
	}
	if len(config.ChannelIDs) != 2 || config.ChannelIDs[0] != 3 || config.ChannelIDs[1] != 9 {
		t.Fatalf("unexpected normalized channel ids: %#v", config.ChannelIDs)
	}
	if config.ChannelMultipliers["9"] != 1 {
		t.Fatalf("invalid multiplier should fall back to 1, got %v", config.ChannelMultipliers["9"])
	}
}

func TestSpecialUsageChannelSelectionDistinguishesEmptyExplicitList(t *testing.T) {
	channel := &Channel{Id: 7, Group: "default,paid", Models: "gpt-4o,gpt-4o-mini"}

	base := SpecialUsageConfig{
		Enabled:    true,
		GroupNames: []string{"paid"},
		ModelNames: []string{"gpt-4o"},
	}
	if !specialUsageChannelSelected(base, channel, "gpt-4o") {
		t.Fatal("expected an unset channel selection to match for compatibility")
	}
	if !specialUsageChannelSelected(base, channel, "") {
		t.Fatal("expected an empty model name to match the channel model intersection")
	}

	base.ChannelIDsSet = true
	if specialUsageChannelSelected(base, channel, "gpt-4o") {
		t.Fatal("an explicit empty channel selection must match no channels")
	}

	base.ChannelIDs = []int{7}
	if !specialUsageChannelSelected(base, channel, "gpt-4o") {
		t.Fatal("expected the explicitly selected channel to match")
	}
}

func TestMarkSpecialUsageAnomaliesUsesStrictThirtyPercent(t *testing.T) {
	overview := SpecialUsageOverview{
		Channels: []SpecialUsageChannelStat{
			{ChannelID: 1, RequestCount: 1, UpstreamCostUSD: 7, AverageCostUSD: 7},
			{ChannelID: 2, RequestCount: 1, UpstreamCostUSD: 13, AverageCostUSD: 13},
		},
	}
	markSpecialUsageAnomalies(&overview)
	require.InDelta(t, 10.0, overview.Channels[0].BaselineCostUSD, 1e-9)
	require.False(t, overview.Channels[0].Anomaly)
	require.False(t, overview.Channels[1].Anomaly)

	overview.Channels[0].AverageCostUSD = 6.9
	overview.Channels[0].UpstreamCostUSD = 6.9
	markSpecialUsageAnomalies(&overview)
	require.True(t, overview.Channels[0].Anomaly)
	require.NotEmpty(t, overview.Channels[0].AnomalyReason)
}

func TestWriteSpecialUsageXLSX(t *testing.T) {
	var output bytes.Buffer
	records := []SpecialUsageRecord{{
		RequestID:       "request-1",
		ChannelID:       7,
		ChannelName:     "Channel & One",
		GroupName:       "paid",
		ModelName:       "model",
		InputTokens:     12,
		OutputTokens:    3,
		UpstreamCostUSD: 0.125,
		UserChargeUSD:   0.2,
		Status:          SpecialUsageStatusSuccess,
		RequestTime:     1_700_000_000,
	}}
	require.NoError(t, WriteSpecialUsageXLSX(&output, records))

	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	require.NoError(t, err)
	files := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		handle, openErr := file.Open()
		require.NoError(t, openErr)
		content, readErr := io.ReadAll(handle)
		require.NoError(t, handle.Close())
		require.NoError(t, readErr)
		files[file.Name] = string(content)
	}
	require.Contains(t, files, "[Content_Types].xml")
	require.Contains(t, files, "xl/workbook.xml")
	require.Contains(t, files, "xl/worksheets/sheet1.xml")
	require.Contains(t, files["xl/worksheets/sheet1.xml"], `r="B2"><v>7</v>`)
	require.Contains(t, files["xl/worksheets/sheet1.xml"], "Channel &amp; One")
}
