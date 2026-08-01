package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRefreshRelayChannelPricingUsesSelectedChannelOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalModelPrices := ratio_setting.GetModelPriceCopy()
	t.Cleanup(func() {
		data, err := common.Marshal(originalModelPrices)
		if err == nil {
			_ = ratio_setting.UpdateModelPriceByJSONString(string(data))
		}
	})

	prices := ratio_setting.GetModelPriceCopy()
	prices["channel-priority-model"] = 1.5
	data, err := common.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(data)))

	limit := int64(272000)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("group", "default")
	common.SetContextKey(c, constant.ContextKeyChannelId, 903)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SpecialBilling: true,
		SpecialBillingPrices: map[string][]dto.SpecialBillingPrice{
			"channel-priority-model": {
				{MaxInputTokens: &limit, Price: 1},
			},
		},
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "channel-priority-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	if priceErr := refreshRelayChannelPricing(c, info, 272000, &types.TokenCountMeta{}); priceErr != nil {
		t.Fatalf("refreshRelayChannelPricing() error = %#v, underlying = %v", priceErr, priceErr.Err)
	}
	require.Equal(t, 903, info.ChannelId)
	require.True(t, info.PriceData.UsePrice)
	require.Equal(t, 1.0, info.PriceData.ModelPrice,
		"the selected channel price must override the global 1.5 price")
}
