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

func TestOaiResponsesStreamHandlerForwardsOutputEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":{"city":"Hong Kong"}}}`,
		`data: {"type":"response.completed","response":{"model":"upstream-model","created_at":1,"usage":{"input_tokens":5,"output_tokens":7,"total_tokens":12}}}`,
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
			UpstreamModelName: "upstream-model",
		},
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		IsStream:        true,
		OriginModelName: "display-model",
	}

	usage, apiErr := OaiResponsesStreamHandler(ctx, info, resp)
	if apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	if usage == nil {
		t.Fatal("usage = nil, want non-nil")
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 7 || usage.TotalTokens != 12 {
		t.Fatalf("usage = %#v, want prompt=5 completion=7 total=12", usage)
	}

	got := recorder.Body.String()
	if !strings.Contains(got, "event: response.output_text.delta") {
		t.Fatalf("missing output_text.delta event, body = %q", got)
	}
	if !strings.Contains(got, `data: {"type":"response.output_text.delta","delta":"hello"}`) {
		t.Fatalf("delta chunk was not forwarded verbatim, body = %q", got)
	}
	if !strings.Contains(got, "event: response.output_item.done") {
		t.Fatalf("missing output_item.done event, body = %q", got)
	}
	if !strings.Contains(got, `data: {"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":{"city":"Hong Kong"}}}`) {
		t.Fatalf("item.done chunk was not forwarded verbatim, body = %q", got)
	}
	if !strings.Contains(got, "event: response.completed") {
		t.Fatalf("missing completed event, body = %q", got)
	}
	if !strings.Contains(got, `"model":"display-model"`) {
		t.Fatalf("completed event model was not overridden, body = %q", got)
	}
	if strings.Contains(got, `"model":"upstream-model"`) {
		t.Fatalf("upstream model leaked to downstream, body = %q", got)
	}
}

func TestOpenAIAdaptorUsesConfiguredChatOrResponsesPath(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://upstream.example",
			ChannelType:    constant.ChannelTypeOpenAI,
		},
		RequestURLPath: "/v1/chat/completions",
		RelayMode:      relayconstant.RelayModeChatCompletions,
	}

	chatURL, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("chat URL error = %v", err)
	}
	if chatURL != "https://upstream.example/v1/chat/completions" {
		t.Fatalf("chat URL = %q", chatURL)
	}

	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	responsesURL, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("responses URL error = %v", err)
	}
	if responsesURL != "https://upstream.example/v1/responses" {
		t.Fatalf("responses URL = %q", responsesURL)
	}
}
