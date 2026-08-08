package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSkipGlobalRpmOverloadTransferByChannelMode(t *testing.T) {
	tests := []struct {
		name     string
		setting  dto.ChannelSettings
		excluded bool
	}{
		{
			name:     "ordinary channel",
			excluded: false,
		},
		{
			name: "gpt mode channel",
			setting: dto.ChannelSettings{
				GptModeRequired: true,
			},
			excluded: true,
		},
		{
			name: "fallback channel",
			setting: dto.ChannelSettings{
				FallbackModelEnabled: true,
			},
			excluded: true,
		},
		{
			name: "emergency channel",
			setting: dto.ChannelSettings{
				EmergencyPlanEnabled: true,
			},
			excluded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			channel := &model.Channel{Id: 1}
			channel.SetSetting(tt.setting)
			common.SetContextKey(ctx, constant.ContextKeySelectedChannel, channel)

			require.Equal(t, tt.excluded, skipGlobalRpmOverloadTransfer(ctx))
			require.Equal(t, tt.excluded, channel.IsExcludedFromRpmOverloadTransfer())
		})
	}
}

func TestSkipGlobalRpmOverloadTransferWithoutSelectedChannel(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.False(t, skipGlobalRpmOverloadTransfer(ctx))
	require.False(t, (*model.Channel)(nil).IsExcludedFromRpmOverloadTransfer())
}

func TestShouldApplyRpmOverloadProtectionOnlyForSelectedChannels(t *testing.T) {
	original := common.OverloadProtectionChannelIDsJSONString()
	require.NoError(t, common.UpdateOverloadProtectionChannelIDs("[1]"))
	t.Cleanup(func() {
		require.NoError(t, common.UpdateOverloadProtectionChannelIDs(original))
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	selected := &model.Channel{Id: 1}
	common.SetContextKey(ctx, constant.ContextKeySelectedChannel, selected)

	require.True(t, shouldApplyRpmOverloadProtection(ctx, selected))
	require.False(t, shouldApplyRpmOverloadProtection(ctx, &model.Channel{Id: 2}))

	excluded := &model.Channel{Id: 1}
	excluded.SetSetting(dto.ChannelSettings{FallbackModelEnabled: true})
	require.False(t, shouldApplyRpmOverloadProtection(ctx, excluded))
}
