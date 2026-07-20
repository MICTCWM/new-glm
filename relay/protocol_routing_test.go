package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/zhipu"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRouteResponsesThroughChatUsesSelectedChannelProtocol(t *testing.T) {
	tests := []struct {
		name     string
		mode     int
		channel  int
		setting  dto.ChannelSettings
		expected bool
	}{
		{
			name:     "chat channel bridges Responses to Chat",
			mode:     relayconstant.RelayModeResponses,
			channel:  constant.ChannelTypeOpenAI,
			expected: true,
		},
		{
			name:    "Responses channel stays Responses",
			mode:    relayconstant.RelayModeResponses,
			channel: constant.ChannelTypeOpenAI,
			setting: dto.ChannelSettings{ResponsesProtocol: true},
		},
		{
			name:     "native Chat adaptor also follows Chat setting",
			mode:     relayconstant.RelayModeResponses,
			channel:  constant.ChannelTypeZhipu,
			expected: true,
		},
		{
			name:    "Anthropic is not treated as OpenAI Chat",
			mode:    relayconstant.RelayModeResponses,
			channel: constant.ChannelTypeAnthropic,
		},
		{
			name:    "Chat input does not use Responses bridge",
			mode:    relayconstant.RelayModeChatCompletions,
			channel: constant.ChannelTypeOpenAI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    tt.channel,
					ChannelSetting: tt.setting,
				},
				RelayMode: tt.mode,
			}
			require.Equal(t, tt.expected, shouldRouteResponsesThroughChat(info))
		})
	}
}

func TestShouldRouteResponsesThroughChatNilInfo(t *testing.T) {
	require.False(t, shouldRouteResponsesThroughChat(nil))
}

func TestNormalizeNativeChatResponseToResponses(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeZhipu,
		},
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "glm-5.2",
	}
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"success":true,"data":{"task_id":"task-1","choices":[{"role":"assistant","content":"hello"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	usage, apiErr := normalizeAdaptorChatResponseToResponses(c, info, &zhipu.Adaptor{}, upstreamResp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"object":"response"`)
	require.Contains(t, recorder.Body.String(), `"hello"`)
}
