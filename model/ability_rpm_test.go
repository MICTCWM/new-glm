package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetChannelWithoutCacheReturnsRpmFullWhenAllCandidatesFull(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldCheckChannelRpmFullFunc := CheckChannelRpmFullFunc
	common.MemoryCacheEnabled = false
	CheckChannelRpmFullFunc = func(channelId int) bool {
		return channelId == 101 || channelId == 102
	}
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		CheckChannelRpmFullFunc = oldCheckChannelRpmFullFunc
	})

	priority := int64(1)
	insertChannelWithAbility(t, 101, "rpm-test", "rpm-model", priority, 0, 1)
	insertChannelWithAbility(t, 102, "rpm-test", "rpm-model", priority, 0, 1)

	channel, err := GetChannel("rpm-test", "rpm-model", 0)

	require.Nil(t, channel)
	require.True(t, errors.Is(err, ErrAllChannelsRpmFull))
}

func TestGetChannelWithoutCacheSkipsRpmFullCandidates(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldCheckChannelRpmFullFunc := CheckChannelRpmFullFunc
	common.MemoryCacheEnabled = false
	CheckChannelRpmFullFunc = func(channelId int) bool {
		return channelId == 201
	}
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		CheckChannelRpmFullFunc = oldCheckChannelRpmFullFunc
	})

	priority := int64(1)
	insertChannelWithAbility(t, 201, "rpm-test", "rpm-model", priority, 0, 1)
	insertChannelWithAbility(t, 202, "rpm-test", "rpm-model", priority, 0, 1)

	channel, err := GetChannel("rpm-test", "rpm-model", 0)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 202, channel.Id)
}

func insertChannelWithAbility(t *testing.T, channelId int, group string, modelName string, priority int64, weight uint, maxRPM int) {
	t.Helper()

	require.NoError(t, DB.Create(&Channel{
		Id:     channelId,
		Name:   "channel",
		Key:    "sk-test",
		Group:  group,
		Models: modelName,
		Status: common.ChannelStatusEnabled,
		MaxRPM: maxRPM,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channelId,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}
