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
