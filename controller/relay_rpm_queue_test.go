package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
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

func TestGetChannelPreservesMiddlewareRpmFullForQueue(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldCheckChannelRpmFullFunc := model.CheckChannelRpmFullFunc
	common.MemoryCacheEnabled = false
	model.CheckChannelRpmFullFunc = func(channelId int) bool {
		return channelId == 703
	}
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		model.CheckChannelRpmFullFunc = oldCheckChannelRpmFullFunc
	})

	priority := int64(1)
	require.NoError(t, db.Create(&model.Channel{
		Id:      703,
		Name:    "middleware-rpm-full-channel",
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
		ChannelId: 703,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyRpmQueuePending, true)
	info := &relaycommon.RelayInfo{
		OriginModelName: "rpm-model",
		TokenGroup:      "default",
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

func TestGetChannelUsesSelectedChannelForSpecificChannelRpm(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	selected := &model.Channel{
		Id:      702,
		Name:    "specific-rpm-channel",
		Type:    1,
		MaxRPM:  1,
		AutoBan: common.GetPointer(1),
	}
	common.SetContextKey(ctx, constant.ContextKeySelectedChannel, selected)

	info := &relaycommon.RelayInfo{
		OriginModelName: "rpm-model",
		TokenGroup:      "default",
	}
	retryParam := &service.RetryParam{
		Ctx:            ctx,
		TokenGroup:     "default",
		ModelName:      "rpm-model",
		Retry:          common.GetPointer(0),
		UsedChannelIds: []int{},
	}

	channel, apiErr := getChannel(ctx, info, retryParam)

	require.Nil(t, apiErr)
	require.Same(t, selected, channel)
	require.Equal(t, 1, channel.MaxRPM)
}

func TestSendRpmQueueThinkingNoticeByRelayFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      types.RelayFormat
		mode        int
		path        string
		assertions  []string
		notExpected []string
	}{
		{
			name:   "openai chat",
			format: types.RelayFormatOpenAI,
			mode:   relayconstant.RelayModeChatCompletions,
			path:   "/v1/chat/completions",
			assertions: []string{
				`"object":"chat.completion.chunk"`,
				`"reasoning_content":"` + common.UserMessageRpmQueuedThinking,
			},
		},
		{
			name:   "claude messages",
			format: types.RelayFormatClaude,
			mode:   relayconstant.RelayModeChatCompletions,
			path:   "/v1/messages",
			assertions: []string{
				"event: message_start",
				"event: content_block_start",
				"event: content_block_delta",
				`"type":"thinking_delta"`,
				common.UserMessageRpmQueuedThinking,
			},
		},
		{
			name:   "gemini native",
			format: types.RelayFormatGemini,
			mode:   relayconstant.RelayModeGemini,
			path:   "/v1beta/models/gemini:streamGenerateContent",
			assertions: []string{
				`"thought":true`,
				common.UserMessageRpmQueuedThinking,
			},
		},
		{
			name:   "responses",
			format: types.RelayFormatOpenAI,
			mode:   relayconstant.RelayModeResponses,
			path:   "/v1/responses",
			assertions: []string{
				"event: response.reasoning_summary_part.added",
				"event: response.reasoning_summary_text.delta",
				common.UserMessageRpmQueuedThinking,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			info := &relaycommon.RelayInfo{
				IsStream:        true,
				RelayFormat:     tt.format,
				RelayMode:       tt.mode,
				OriginModelName: "rpm-model",
				ChannelMeta:     &relaycommon.ChannelMeta{},
			}
			info.SetEstimatePromptTokens(123)

			require.True(t, sendRpmQueueThinkingNotice(ctx, info))
			require.True(t, info.RpmQueueThinkingNoticeSent)
			body := recorder.Body.String()
			for _, expected := range tt.assertions {
				require.Truef(t, strings.Contains(body, expected), "body should contain %q, got %s", expected, body)
			}
			for _, unexpected := range tt.notExpected {
				require.Falsef(t, strings.Contains(body, unexpected), "body should not contain %q, got %s", unexpected, body)
			}
		})
	}
}

func TestWaitForRpmQueueSendsStreamNoticeAfterEnqueue(t *testing.T) {
	oldTimeout := common.RpmQueueTimeout
	common.RpmQueueTimeout = 5 * time.Second
	t.Cleanup(func() {
		common.RpmQueueTimeout = oldTimeout
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RequestId:       "rpm-queue-notice-test",
		IsStream:        true,
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "rpm-model",
		TokenGroup:      "default",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	info.SetEstimatePromptTokens(123)

	var queueDeadline time.Time
	queueNoticeSent := false
	done := make(chan bool, 1)
	go func() {
		done <- waitForRpmQueue(ctx, info, &queueDeadline, &queueNoticeSent)
	}()

	require.Eventually(t, func() bool {
		queued := false
		for _, item := range service.GetQueueSnapshot() {
			if item.RequestID == info.RequestId {
				queued = true
				break
			}
		}
		return queued &&
			strings.Contains(recorder.Body.String(), common.UserMessageRpmQueuedThinking) &&
			recorder.Flushed
	}, time.Second, 10*time.Millisecond)

	service.GetRpmQueue().NotifyRpmRelease()
	select {
	case ok := <-done:
		require.True(t, ok)
	case <-time.After(time.Second):
		t.Fatal("expected queued request to wake after RPM release")
	}
	require.True(t, queueNoticeSent)
	require.True(t, info.RpmQueueThinkingNoticeSent)
}
