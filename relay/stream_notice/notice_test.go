package stream_notice

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSendRetryWaitNoticeByRelayFormat(t *testing.T) {
	tests := []struct {
		name       string
		format     types.RelayFormat
		mode       int
		path       string
		assertions []string
	}{
		{
			name:   "openai chat",
			format: types.RelayFormatOpenAI,
			mode:   relayconstant.RelayModeChatCompletions,
			path:   "/v1/chat/completions",
			assertions: []string{
				`"object":"chat.completion.chunk"`,
				`"reasoning_content":`,
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
			},
		},
		{
			name:   "gemini native",
			format: types.RelayFormatGemini,
			mode:   relayconstant.RelayModeGemini,
			path:   "/v1beta/models/gemini:streamGenerateContent",
			assertions: []string{
				`"thought":true`,
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
				OriginModelName: "retry-model",
				ChannelMeta:     &relaycommon.ChannelMeta{},
			}
			info.SetEstimatePromptTokens(123)

			require.True(t, SendRetryWaitNotice(ctx, info))
			require.True(t, info.RpmQueueThinkingNoticeSent)
			require.True(t, recorder.Flushed)

			body := recorder.Body.String()
			for _, expected := range tt.assertions {
				require.Truef(t, strings.Contains(body, expected), "body should contain %q, got %s", expected, body)
			}
		})
	}
}

func TestSendRetryWaitNoticeNoopsForNonStream(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		IsStream:        false,
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "retry-model",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}

	require.False(t, SendRetryWaitNotice(ctx, info))
	require.Empty(t, recorder.Body.String())
	require.False(t, recorder.Flushed)
}
