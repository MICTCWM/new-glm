package relay

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAddPromptCacheKeyToRawBodyPreservesRequestFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5","max_tokens":9007199254740993,"messages":[{"role":"user","content":"hello"}],"metadata":{"request_tag":"keep"}}`)

	patched, err := addPromptCacheKeyToRawBody(body, "cache-session-1")
	require.NoError(t, err)
	require.Equal(t, "cache-session-1", gjson.GetBytes(patched, "prompt_cache_key").String())
	require.Equal(t, "gpt-5", gjson.GetBytes(patched, "model").String())
	require.Equal(t, "9007199254740993", gjson.GetBytes(patched, "max_tokens").Raw)
	require.Equal(t, "keep", gjson.GetBytes(patched, "metadata.request_tag").String())
}

func TestEnsureOpenAIPromptCacheKeyDerivesStableKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("token_id", 42)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ApiType:           constant.APITypeOpenAI,
			ChannelId:         7,
			UpstreamModelName: "gpt-5",
		},
	}
	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`)

	first, err := ensureOpenAIPromptCacheKey(ctx, info, body, "")
	require.NoError(t, err)
	second, err := ensureOpenAIPromptCacheKey(ctx, info, body, "")
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.NotEmpty(t, gjson.GetBytes(first, "prompt_cache_key").String())
	require.Equal(t, "gpt-5", gjson.GetBytes(first, "model").String())
}

func TestEnsureOpenAIPromptCacheKeyIgnoresChangingBodyUserIDForAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ApiType:           constant.APITypeOpenAI,
			UpstreamModelName: "gpt-5",
		},
	}

	firstContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	firstContext.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-5","metadata":{"user_id":"request-1"}}`),
	)
	firstContext.Set("id", 42)
	first, err := ensureOpenAIPromptCacheKey(firstContext, info, []byte(`{"model":"gpt-5","metadata":{"user_id":"request-1"}}`), "")
	require.NoError(t, err)

	secondContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	secondContext.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-5","metadata":{"user_id":"request-2"}}`),
	)
	secondContext.Set("id", 42)
	second, err := ensureOpenAIPromptCacheKey(secondContext, info, []byte(`{"model":"gpt-5","metadata":{"user_id":"request-2"}}`), "")
	require.NoError(t, err)

	require.Equal(t, gjson.GetBytes(first, "prompt_cache_key").String(), gjson.GetBytes(second, "prompt_cache_key").String())
}

func TestEnsureOpenAIPromptCacheKeyPreservesExplicitAndSkipsCompatibleProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("token_id", 42)

	openAIInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ApiType:     constant.APITypeOpenAI,
		},
	}
	body := []byte(`{"model":"gpt-5","prompt_cache_key":"client-key"}`)
	patched, err := ensureOpenAIPromptCacheKey(ctx, openAIInfo, body, "server-key")
	require.NoError(t, err)
	require.Equal(t, "client-key", gjson.GetBytes(patched, "prompt_cache_key").String())

	compatibleInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
			ApiType:     constant.APITypeOpenRouter,
		},
	}
	unchanged, err := ensureOpenAIPromptCacheKey(ctx, compatibleInfo, []byte(`{"model":"gpt-5"}`), "server-key")
	require.NoError(t, err)
	require.Equal(t, `{"model":"gpt-5"}`, string(unchanged))
}

func TestOpenAIAdaptorSendsPromptCacheKeyInUpstreamBody(t *testing.T) {
	service.InitHttpClient()

	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	body, err := addPromptCacheKeyToRawBody(
		[]byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`),
		"cache-session-2",
	)
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ChannelBaseUrl:    server.URL,
			ApiKey:            "test-key",
			UpstreamModelName: "gpt-5",
		},
		RequestURLPath: "/v1/chat/completions",
		RelayMode:      relayconstant.RelayModeChatCompletions,
	}
	adaptor := &openaichannel.Adaptor{}
	adaptor.Init(info)

	response, err := adaptor.DoRequest(ctx, info, bytes.NewReader(body))
	require.NoError(t, err)
	require.NotNil(t, response)
	require.NoError(t, response.(*http.Response).Body.Close())
	require.Equal(t, "cache-session-2", gjson.GetBytes(received, "prompt_cache_key").String())
	require.Equal(t, "gpt-5", gjson.GetBytes(received, "model").String())
}

func TestEnsureClaudePromptCacheBreakpointUsesHistoryPrefix(t *testing.T) {
	request := &dto.ClaudeRequest{
		Model: "claude-sonnet",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "first answer"},
			{Role: "user", Content: "current question"},
		},
	}

	require.True(t, ensureClaudePromptCacheBreakpoint(request))
	require.IsType(t, []dto.ClaudeMediaMessage{}, request.Messages[1].Content)
	require.Equal(t, "current question", request.Messages[2].Content)

	history := request.Messages[1].Content.([]dto.ClaudeMediaMessage)
	require.Len(t, history, 1)
	require.JSONEq(t, `{"type":"ephemeral"}`, string(history[0].CacheControl))
}

func TestEnsureClaudePromptCacheBreakpointPreservesExistingBoundary(t *testing.T) {
	cacheControl := json.RawMessage(`{"type":"ephemeral"}`)
	request := &dto.ClaudeRequest{
		Model: "claude-sonnet",
		System: []dto.ClaudeMediaMessage{{
			Type:         dto.ContentTypeText,
			Text:         stringPtr("stable system"),
			CacheControl: cacheControl,
		}},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "question"}},
	}

	require.True(t, ensureClaudePromptCacheBreakpoint(request))
	require.Equal(t, cacheControl, request.ParseSystem()[0].CacheControl)
	require.IsType(t, []dto.ClaudeMediaMessage{}, request.Messages[0].Content)
}

func TestClaudePromptCacheBreakpointCountsToolBoundaries(t *testing.T) {
	cacheControl := map[string]any{"type": "ephemeral"}
	request := &dto.ClaudeRequest{
		Tools: []map[string]any{
			{"name": "tool-1", "cache_control": cacheControl},
			{"name": "tool-2", "cache_control": cacheControl},
			{"name": "tool-3", "cache_control": cacheControl},
			{"name": "tool-4", "cache_control": cacheControl},
		},
		Messages: []dto.ClaudeMessage{{
			Role:    "user",
			Content: "hello",
		}},
	}

	require.Equal(t, 4, claudeCacheControlCount(request))
	require.False(t, ensureClaudePromptCacheBreakpoint(request))
	require.Equal(t, "hello", request.Messages[0].Content)
}

func TestAddClaudePromptCacheBreakpointToRawBodyPreservesUnknownFields(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet",
		"metadata":{"request_tag":"keep-me"},
		"messages":[
			{"role":"user","content":"first question"},
			{"role":"assistant","content":"first answer"},
			{"role":"user","content":"current question"}
		]
	}`)

	patched, err := addClaudePromptCacheBreakpointToRawBody(body)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(patched, &payload))
	require.JSONEq(t, `{"request_tag":"keep-me"}`, string(payload["metadata"]))

	var messages []map[string]any
	require.NoError(t, json.Unmarshal(payload["messages"], &messages))
	require.Len(t, messages, 3)
	historyContent := messages[1]["content"].([]any)
	historyBlock := historyContent[0].(map[string]any)
	require.Equal(t, "ephemeral", historyBlock["cache_control"].(map[string]any)["type"])
	currentContent := messages[2]["content"].(string)
	require.Equal(t, "current question", currentContent)
	_, currentHasCache := messages[2]["cache_control"]
	require.False(t, currentHasCache)
}

func stringPtr(value string) *string {
	return &value
}
