package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecreaseUserGptQuotaPersistsSmallDelta(t *testing.T) {
	truncateTables(t)

	userID := 91001
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "gpt_quota_small_delta",
		Status:   common.UserStatusEnabled,
		GptQuota: 0.003,
	}).Error)

	require.NoError(t, DecreaseUserGptQuota(userID, GptQuotaFromBaseQuota(500)))

	var got float64
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userID).Select("gpt_quota").Scan(&got).Error)
	assert.InDelta(t, 0.002, got, 1e-12)

	baseQuota, err := TransferGptQuotaToQuota(userID, got)
	require.NoError(t, err)
	assert.Equal(t, 333333, baseQuota)

	var reloaded User
	require.NoError(t, DB.First(&reloaded, userID).Error)
	assert.Equal(t, 333333, reloaded.Quota)
	assert.InDelta(t, 0, reloaded.GptQuota, 1e-12)
}

func TestGptQuotaFromBaseQuotaMatchesBillingValue(t *testing.T) {
	assert.InDelta(t, 0.003338, GptQuotaFromBaseQuota(1669), 1e-12)
	assert.InDelta(t, 1.0, GptQuotaFromBaseQuota(500000), 1e-12)
}

func TestTransferQuotaToGptQuotaUsesWalletExchangeRate(t *testing.T) {
	truncateTables(t)

	userID := 91005
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "gpt_quota_wallet_exchange",
		Status:   common.UserStatusEnabled,
		Quota:    250000000,
	}).Error)

	gptQuota, err := TransferQuotaToGptQuota(userID, 250000000)
	require.NoError(t, err)
	assert.InDelta(t, 1.5, gptQuota, 1e-12)

	var reloaded User
	require.NoError(t, DB.First(&reloaded, userID).Error)
	assert.Zero(t, reloaded.Quota)
	assert.InDelta(t, 1.5, reloaded.GptQuota, 1e-12)

	baseQuota, err := TransferGptQuotaToQuota(userID, reloaded.GptQuota)
	require.NoError(t, err)
	assert.Equal(t, 250000000, baseQuota)

	require.NoError(t, DB.First(&reloaded, userID).Error)
	assert.Equal(t, 250000000, reloaded.Quota)
	assert.InDelta(t, 0, reloaded.GptQuota, 1e-12)
}

func TestDecreaseUserGptQuotaRejectsInsufficientBalance(t *testing.T) {
	truncateTables(t)

	userID := 91002
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "gpt_quota_insufficient",
		Status:   common.UserStatusEnabled,
		GptQuota: GptQuotaFromBaseQuota(1),
	}).Error)

	err := DecreaseUserGptQuota(userID, GptQuotaFromBaseQuota(2))
	require.ErrorContains(t, err, "余额不足")

	var got float64
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userID).Select("gpt_quota").Scan(&got).Error)
	assert.InDelta(t, GptQuotaFromBaseQuota(1), got, 1e-12)
}

func TestTransferGptQuotaToQuotaDoesNotRoundUpTinyFraction(t *testing.T) {
	truncateTables(t)

	userID := 91003
	tinyGptQuota := 0.000000000000000001
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "gpt_quota_floor",
		Status:   common.UserStatusEnabled,
		GptQuota: tinyGptQuota,
	}).Error)

	_, err := TransferGptQuotaToQuota(userID, tinyGptQuota)
	require.ErrorContains(t, err, "转换金额过小")

	var reloaded User
	require.NoError(t, DB.First(&reloaded, userID).Error)
	assert.Zero(t, reloaded.Quota)
	assert.InDelta(t, tinyGptQuota, reloaded.GptQuota, 1e-24)
}

func TestForceDecreaseUserGptQuotaAllowsNegativeBalance(t *testing.T) {
	truncateTables(t)

	userID := 91004
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "gpt_quota_negative",
		Status:   common.UserStatusEnabled,
		GptQuota: GptQuotaFromBaseQuota(1),
	}).Error)

	require.NoError(t, ForceDecreaseUserGptQuota(userID, GptQuotaFromBaseQuota(2)))

	var got float64
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userID).Select("gpt_quota").Scan(&got).Error)
	assert.InDelta(t, -GptQuotaFromBaseQuota(1), got, 1e-12)
}
