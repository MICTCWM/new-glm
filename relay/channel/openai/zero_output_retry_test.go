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

func TestOpenaiHandlerRetriesZeroOutputBeforeWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	body := `{"id":"chatcmpl-test","object":"chat.completion","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":9673,"completion_tokens":0,"total_tokens":9673}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "test-model",
		},
		RelayFormat: types.RelayFormatOpenAI,
	}
	info.SetEstimatePromptTokens(9673)

	usage, apiErr := OpenaiHandler(ctx, info, resp)
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
	if apiErr == nil {
		t.Fatal("expected zero output retry error")
	}
	if apiErr.GetErrorCode() != types.ErrorCodeChannelZeroOutputTokens {
		t.Fatalf("error code = %s, want %s", apiErr.GetErrorCode(), types.ErrorCodeChannelZeroOutputTokens)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("response body was written: %q", recorder.Body.String())
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("response status = %d, want default %d", recorder.Code, http.StatusOK)
	}
}

func TestOaiStreamHandlerRetriesZeroOutputBeforeWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "test-model",
		},
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		IsStream:    true,
	}
	info.SetEstimatePromptTokens(10)

	usage, apiErr := OaiStreamHandler(ctx, info, resp)
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
	if apiErr == nil {
		t.Fatal("expected zero output retry error")
	}
	if apiErr.GetErrorCode() != types.ErrorCodeChannelZeroOutputTokens {
		t.Fatalf("error code = %s, want %s", apiErr.GetErrorCode(), types.ErrorCodeChannelZeroOutputTokens)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("response body was written: %q", recorder.Body.String())
	}
}
