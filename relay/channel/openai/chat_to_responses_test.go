package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestChatCompletionsToResponsesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_1","model":"upstream-model","created":1,"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "upstream-model"},
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "display-model",
	}

	usage, apiErr := ChatCompletionsToResponsesHandler(ctx, info, resp)
	if apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, want total=5", usage)
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"object":"response"`) || !strings.Contains(got, `"model":"display-model"`) {
		t.Fatalf("unexpected Responses body: %s", got)
	}
	if strings.Contains(got, `"model":"upstream-model"`) {
		t.Fatalf("upstream model leaked to downstream: %s", got)
	}
}

func TestChatCompletionsToResponsesStreamHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"upstream-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"upstream-model","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"upstream-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "upstream-model"},
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
		OriginModelName: "display-model",
	}

	usage, apiErr := ChatCompletionsToResponsesStreamHandler(ctx, info, resp)
	if apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, want total=5", usage)
	}
	got := recorder.Body.String()
	if !strings.Contains(got, "event: response.output_text.delta") || !strings.Contains(got, `"delta":"hello"`) {
		t.Fatalf("missing converted text event: %s", got)
	}
	if !strings.Contains(got, "event: response.completed") {
		t.Fatalf("missing converted completed event: %s", got)
	}
	if !strings.Contains(got, `"model":"display-model"`) {
		t.Fatalf("downstream model was not overridden: %s", got)
	}
}

func TestChatCompletionsToResponsesBufferedStreamHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"upstream-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"upstream-model","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"upstream-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "upstream-model"},
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        false,
		OriginModelName: "display-model",
	}

	usage, apiErr := ChatCompletionsToResponsesBufferedStreamHandler(ctx, info, resp)
	if apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, want total=5", usage)
	}
	got := recorder.Body.String()
	if strings.Contains(got, "event: response.") || !strings.Contains(got, `"object":"response"`) || !strings.Contains(got, `"hello"`) {
		t.Fatalf("upstream SSE was not buffered to Responses JSON: %s", got)
	}
	if !strings.Contains(got, `"model":"display-model"`) {
		t.Fatalf("downstream model was not overridden: %s", got)
	}
}

func TestChatCompletionsToResponsesStreamFromJSONHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_1","model":"upstream-model","object":"chat.completion","created":1,"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
		)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "upstream-model"},
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
		OriginModelName: "display-model",
	}

	usage, apiErr := ChatCompletionsToResponsesStreamFromJSONHandler(ctx, info, resp)
	if apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, want total=5", usage)
	}
	got := recorder.Body.String()
	if !strings.Contains(got, "event: response.output_text.delta") || !strings.Contains(got, `"delta":"hello"`) || !strings.Contains(got, "event: response.completed") {
		t.Fatalf("upstream JSON was not converted to Responses SSE: %s", got)
	}
	if !strings.Contains(got, `"model":"display-model"`) {
		t.Fatalf("downstream model was not overridden: %s", got)
	}
}
