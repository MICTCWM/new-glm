package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannelFallbackKeepsRequestModelPricing(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	originalModelPrices := ratio_setting.GetModelPriceCopy()
	t.Cleanup(func() {
		data, err := common.Marshal(originalModelPrices)
		if err == nil {
			_ = ratio_setting.UpdateModelPriceByJSONString(string(data))
		}
	})
	prices := ratio_setting.GetModelPriceCopy()
	prices["fallback-billing-model"] = 100
	data, err := common.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(data)))

	fallbackChannel := &model.Channel{
		Id:     901,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-fallback",
		Name:   "fallback-billing-channel",
		Status: common.ChannelStatusEnabled,
		Models: "fallback-billing-model",
		Group:  "default",
	}
	fallbackChannel.SetSetting(dto.ChannelSettings{
		FallbackModelEnabled: true,
		FallbackModel:        "fallback-billing-model",
	})
	require.NoError(t, db.Create(fallbackChannel).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("source_channel_supports_fallback", true)
	ctx.Set("fallback_force_next", true)
	ctx.Set("group", "default")
	info := &relaycommon.RelayInfo{
		OriginModelName: "requested-billing-model",
		ChannelMeta:     &relaycommon.ChannelMeta{},
		UserGroup:       "default",
		UsingGroup:      "default",
		PriceData: types.PriceData{
			ModelPrice: 1,
			UsePrice:   true,
		},
	}
	retryParam := &service.RetryParam{
		Ctx:            ctx,
		TokenGroup:     "default",
		ModelName:      "requested-billing-model",
		Retry:          common.GetPointer(1),
		UsedChannelIds: []int{},
	}

	channel, apiErr := getChannel(ctx, info, retryParam)

	require.Nil(t, apiErr)
	require.NotNil(t, channel)
	require.Equal(t, fallbackChannel.Id, channel.Id)
	require.Equal(t, "fallback-billing-model", info.UpstreamModelName)
	require.Equal(t, 1.0, info.PriceData.ModelPrice,
		"fallback routing must not replace the request model's billing price")
}

func TestChannelAffinityFailureStillAllowsFallback(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	fallbackChannel := &model.Channel{
		Id:     903,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-affinity-fallback",
		Name:   "affinity-fallback-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
	}
	fallbackChannel.SetSetting(dto.ChannelSettings{
		FallbackModelEnabled: true,
		FallbackModel:        "fallback-model",
	})
	require.NoError(t, db.Create(fallbackChannel).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_affinity_failure", true)
	ctx.Set("source_channel_supports_fallback", true)
	ctx.Set("fallback_force_next", true)
	retryParam := &service.RetryParam{
		Ctx:            ctx,
		TokenGroup:     "default",
		ModelName:      "requested-model",
		Retry:          common.GetPointer(1),
		UsedChannelIds: []int{},
	}

	require.True(t, canRelayFallback(ctx, retryParam),
		"affinity retry suppression must not disable configured fallback routing")

	info := &relaycommon.RelayInfo{
		OriginModelName: "requested-model",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	channel, apiErr := getChannel(ctx, info, retryParam)
	require.Nil(t, apiErr)
	require.NotNil(t, channel)
	require.Equal(t, fallbackChannel.Id, channel.Id)
}

func TestAffinityRpmTimeoutPreservesSourceFallbackPolicy(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_affinity_hit", true)
	channel := &model.Channel{}
	channel.SetSetting(dto.ChannelSettings{SupportFallback: true})

	require.True(t, setRelayFallbackSource(ctx, channel))
	require.True(t, ctx.GetBool("source_channel_supports_fallback"))
}

func TestGetChannelSelectsFallbackChannelsInOrder(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	for _, channel := range []*model.Channel{
		{
			Id:       911,
			Type:     constant.ChannelTypeOpenAI,
			Key:      "sk-fallback-a",
			Name:     "fallback-a",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer(int64(10)),
		},
		{
			Id:       912,
			Type:     constant.ChannelTypeOpenAI,
			Key:      "sk-fallback-b",
			Name:     "fallback-b",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer(int64(40)),
		},
		{
			Id:       913,
			Type:     constant.ChannelTypeOpenAI,
			Key:      "sk-fallback-c",
			Name:     "fallback-c",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer(int64(30)),
		},
		{
			Id:       914,
			Type:     constant.ChannelTypeOpenAI,
			Key:      "sk-fallback-d",
			Name:     "fallback-d",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer(int64(20)),
		},
	} {
		channel.SetSetting(dto.ChannelSettings{
			FallbackModelEnabled: true,
			FallbackModel:        "fallback-model",
		})
		require.NoError(t, db.Create(channel).Error)
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("source_channel_supports_fallback", true)
	ctx.Set("fallback_force_next", true)
	info := &relaycommon.RelayInfo{
		OriginModelName: "requested-model",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{
		Ctx:            ctx,
		TokenGroup:     "default",
		ModelName:      "requested-model",
		Retry:          common.GetPointer(0),
		UsedChannelIds: []int{},
	}

	for _, expectedID := range []int{912, 913, 914, 911} {
		channel, apiErr := getChannel(ctx, info, retryParam)
		require.Nil(t, apiErr)
		require.Equal(t, expectedID, channel.Id)
		retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, channel.Id)
		ctx.Set("fallback_triggered", false)
		ctx.Set("fallback_force_next", true)
	}
}

func TestGetChannelSkipsFallbackPreselectionForNormalRequest(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	normalChannel := &model.Channel{
		Id:      915,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "sk-normal",
		Name:    "normal-channel",
		Status:  common.ChannelStatusEnabled,
		Models:  "normal-model",
		Group:   "default",
		AutoBan: common.GetPointer(1),
	}
	fallbackChannel := &model.Channel{
		Id:      916,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "sk-fallback",
		Name:    "fallback-channel",
		Status:  common.ChannelStatusEnabled,
		Models:  "normal-model",
		Group:   "default",
		AutoBan: common.GetPointer(1),
	}
	fallbackChannel.SetSetting(dto.ChannelSettings{
		FallbackModelEnabled: true,
		FallbackModel:        "fallback-model",
	})
	require.NoError(t, db.Create(normalChannel).Error)
	require.NoError(t, db.Create(fallbackChannel).Error)
	priority := int64(1)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "normal-model",
		ChannelId: normalChannel.Id,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeySelectedChannel, fallbackChannel)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, fallbackChannel.Id)
	info := &relaycommon.RelayInfo{OriginModelName: "normal-model", TokenGroup: "default"}
	retryParam := &service.RetryParam{
		Ctx:        ctx,
		TokenGroup: "default",
		ModelName:  "normal-model",
		Retry:      common.GetPointer(0),
	}

	channel, apiErr := getChannel(ctx, info, retryParam)

	require.Nil(t, apiErr)
	require.NotNil(t, channel)
	require.Equal(t, normalChannel.Id, channel.Id)
}

func TestGetChannelDailyUsageLimitForcesFallback(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	fallbackChannel := &model.Channel{
		Id:     902,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-daily-fallback",
		Name:   "daily-fallback-channel",
		Status: common.ChannelStatusEnabled,
		Models: "daily-fallback-model",
		Group:  "default",
	}
	fallbackChannel.SetSetting(dto.ChannelSettings{
		FallbackModelEnabled: true,
		FallbackModel:        "daily-fallback-model",
	})
	require.NoError(t, db.Create(fallbackChannel).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("daily_usage_limit_triggered", true)
	info := &relaycommon.RelayInfo{
		OriginModelName: "requested-daily-model",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{
		Ctx:            ctx,
		TokenGroup:     "default",
		ModelName:      "requested-daily-model",
		Retry:          common.GetPointer(0),
		UsedChannelIds: []int{},
	}

	channel, apiErr := getChannel(ctx, info, retryParam)

	require.Nil(t, apiErr)
	require.NotNil(t, channel)
	require.Equal(t, fallbackChannel.Id, channel.Id)
}

func TestDailyUsageLimitSkipsExcludedChannels(t *testing.T) {
	oldDailyUsageLimit := common.DailyUsageLimit
	common.DailyUsageLimit = 1
	t.Cleanup(func() {
		common.DailyUsageLimit = oldDailyUsageLimit
	})

	tests := []struct {
		name    string
		setting dto.ChannelSettings
	}{
		{name: "gpt", setting: dto.ChannelSettings{GptModeRequired: true}},
		{name: "emergency", setting: dto.ChannelSettings{EmergencyPlanEnabled: true}},
		{name: "fallback", setting: dto.ChannelSettings{FallbackModelEnabled: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			channel := &model.Channel{}
			channel.SetSetting(test.setting)
			common.SetContextKey(ctx, constant.ContextKeySelectedChannel, channel)

			require.False(t, dailyUsageLimitExceeded(ctx))
		})
	}
}
