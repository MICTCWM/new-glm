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

	channel, err := GetChannel("rpm-test", "rpm-model", 0, nil)

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

	channel, err := GetChannel("rpm-test", "rpm-model", 0, nil)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 202, channel.Id)
}

func TestGetChannelWithoutCacheFiltersSpecialUsers(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	priority := int64(1)
	insertChannelWithAbility(t, 301, "special-test", "special-model", priority, 0, 0, `{"special_user_enabled":true,"special_user_ids":[7,9]}`)

	channel, err := GetChannel("special-test", "special-model", 0, nil, 8)
	require.Nil(t, channel)
	require.True(t, errors.Is(err, ErrChannelSpecialUserUnauthorized))

	channel, err = GetChannel("special-test", "special-model", 0, nil, 7)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 301, channel.Id)
}

func TestGetRandomSatisfiedChannelReturnsRpmFullWhenAllowedChannelIsFull(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldCheckChannelRpmFullFunc := CheckChannelRpmFullFunc
	common.MemoryCacheEnabled = true
	CheckChannelRpmFullFunc = func(channelId int) bool {
		return channelId == 402
	}
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		CheckChannelRpmFullFunc = oldCheckChannelRpmFullFunc
		InitChannelCache()
	})

	priority := int64(1)
	insertChannelWithAbility(t, 401, "special-cache-test", "special-cache-model", priority, 0, 0, `{"special_user_enabled":true,"special_user_ids":[9]}`)
	insertChannelWithAbility(t, 402, "special-cache-test", "special-cache-model", priority, 0, 1, `{"special_user_enabled":true,"special_user_ids":[7]}`)
	InitChannelCache()

	channel, err := GetRandomSatisfiedChannel("special-cache-test", "special-cache-model", 0, nil, 7)

	require.Nil(t, channel)
	require.True(t, errors.Is(err, ErrAllChannelsRpmFull))
}

func TestGetStableSatisfiedChannelIsDeterministic(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		InitChannelCache()
	})

	priority := int64(1)
	insertChannelWithAbility(t, 451, "stable-route-test", "stable-route-model", priority, 1, 0)
	insertChannelWithAbility(t, 452, "stable-route-test", "stable-route-model", priority, 1, 0)
	InitChannelCache()

	first, err := GetStableSatisfiedChannel("stable-route-test", "stable-route-model", "same-affinity", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := GetStableSatisfiedChannel("stable-route-test", "stable-route-model", "same-affinity", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.Id, second.Id)
}

func TestGetStableSatisfiedChannelWithoutCacheIsDeterministic(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	priority := int64(1)
	insertChannelWithAbility(t, 461, "stable-db-route-test", "stable-db-route-model", priority, 1, 0)
	insertChannelWithAbility(t, 462, "stable-db-route-test", "stable-db-route-model", priority, 1, 0)

	first, err := GetStableSatisfiedChannel("stable-db-route-test", "stable-db-route-model", "same-affinity", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := GetStableSatisfiedChannel("stable-db-route-test", "stable-db-route-model", "same-affinity", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.Id, second.Id)
}

func TestHasEmergencyChannel(t *testing.T) {
	truncateTables(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		InitChannelCache()
	})

	priority := int64(1)
	insertChannelWithAbility(t, 471, "emergency-route-test", "emergency-route-model", priority, 1, 0)
	insertChannelWithAbility(t, 472, "emergency-route-test", "emergency-route-model", priority, 1, 0, `{"emergency_plan_enabled":true}`)
	InitChannelCache()

	require.True(t, HasEmergencyChannel("emergency-route-test", "emergency-route-model"))
	require.False(t, HasEmergencyChannel("missing-group", "emergency-route-model"))
	selected, err := GetStableSatisfiedChannel("emergency-route-test", "emergency-route-model", "same-affinity", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.True(t, selected.IsEmergencyPlanEnabled())

	common.MemoryCacheEnabled = false
	require.True(t, HasEmergencyChannel("emergency-route-test", "emergency-route-model"))
}

func insertChannelWithAbility(t *testing.T, channelId int, group string, modelName string, priority int64, weight uint, maxRPM int, setting ...string) {
	t.Helper()

	var channelSetting *string
	if len(setting) > 0 {
		channelSetting = &setting[0]
	}
	require.NoError(t, DB.Create(&Channel{
		Id:      channelId,
		Name:    "channel",
		Key:     "sk-test",
		Group:   group,
		Models:  modelName,
		Status:  common.ChannelStatusEnabled,
		MaxRPM:  maxRPM,
		Setting: channelSetting,
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
