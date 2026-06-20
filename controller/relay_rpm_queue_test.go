package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetChannelPreservesRpmFullForQueue(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldCheckChannelRpmFullFunc := model.CheckChannelRpmFullFunc
	common.MemoryCacheEnabled = false
	model.CheckChannelRpmFullFunc = func(channelId int) bool {
		return channelId == 701
	}
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		model.CheckChannelRpmFullFunc = oldCheckChannelRpmFullFunc
	})

	priority := int64(1)
	require.NoError(t, db.Create(&model.Channel{
		Id:      701,
		Name:    "rpm-full-channel",
		Key:     "sk-test",
		Group:   "default",
		Models:  "rpm-model",
		Status:  common.ChannelStatusEnabled,
		MaxRPM:  1,
		AutoBan: common.GetPointer(1),
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "rpm-model",
		ChannelId: 701,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "rpm-model",
		TokenGroup:      "default",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{
		Ctx:            ctx,
		TokenGroup:     "default",
		ModelName:      "rpm-model",
		Retry:          common.GetPointer(0),
		UsedChannelIds: []int{},
	}

	channel, apiErr := getChannel(ctx, info, retryParam)

	require.Nil(t, channel)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.True(t, errors.Is(apiErr.Err, service.ErrAllChannelsRpmFull))
}
