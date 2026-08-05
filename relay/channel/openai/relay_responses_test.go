package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
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

func TestOaiResponsesToChatStreamHandlerNeverLeaksResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"chatcmpl-1","object":"response","created_at":0,"status":"in_progress","instructions":null,"max_output_tokens":0,"model":"grok-4.5","output":null,"parallel_tool_calls":false,"previous_response_id":null,"reasoning":null,"store":false,"temperature":0,"tool_choice":null,"tools":null,"top_p":0,"truncation":null,"usage":null,"user":null,"metadata":null}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"id":"chatcmpl-1","object":"response","created_at":0,"status":"completed","model":"grok-4.5","output":null,"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-4.5"},
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		IsStream:    true,
	}

	if _, apiErr := OaiResponsesToChatStreamHandler(ctx, info, resp); apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	got := recorder.Body.String()
	for _, leaked := range []string{"response.created", "response.output_text.delta", "response.completed"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("Responses event %q leaked into Chat response: %s", leaked, got)
		}
	}
	if !strings.Contains(got, `"object":"chat.completion.chunk"`) || !strings.Contains(got, `"choices"`) {
		t.Fatalf("converted Chat chunks are missing: %s", got)
	}
	if !strings.Contains(got, `"content":"hello"`) {
		t.Fatalf("converted text delta is missing: %s", got)
	}
}

func TestOaiResponsesToChatStreamHandlerConvertsReasoningSummarySSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := strings.Join([]string{
		`data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_1","summary_index":0,"delta":"plan"}`,
		`data: {"type":"response.reasoning_summary_text.done","item_id":"rs_1","summary_index":0,"text":"plan"}`,
		`data: {"type":"response.output_text.delta","delta":"answer"}`,
		`data: {"type":"response.completed","response":{"id":"resp-1","model":"gpt-test","status":"completed","usage":{"input_tokens":2,"output_tokens":2,"total_tokens":4}}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
			ChannelSetting: dto.ChannelSettings{
				ThinkingToContent: true,
			},
		},
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		IsStream:    true,
	}

	if _, apiErr := OaiResponsesToChatStreamHandler(ctx, info, resp); apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"reasoning_content":"plan"`) {
		t.Fatalf("reasoning summary was not converted to Chat reasoning_content: %s", got)
	}
	if strings.Count(got, `"reasoning_content":"plan"`) != 1 {
		t.Fatalf("reasoning summary was duplicated: %s", got)
	}
	if !strings.Contains(got, `"content":"answer"`) {
		t.Fatalf("output text was not converted: %s", got)
	}
}

func TestOaiResponsesToChatBufferedStreamHandlerReturnsChatJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp-1","model":"grok-4.5"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"id":"resp-1","model":"grok-4.5","status":"completed","usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-4.5"},
		RelayFormat: types.RelayFormatOpenAI,
	}

	if _, apiErr := OaiResponsesToChatBufferedStreamHandler(ctx, info, resp); apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	got := recorder.Body.String()
	if strings.Contains(got, "response.created") || strings.Contains(got, "response.completed") {
		t.Fatalf("Responses events leaked into buffered Chat response: %s", got)
	}
	if !strings.Contains(got, `"object":"chat.completion"`) || !strings.Contains(got, `"content":"hello"`) {
		t.Fatalf("unexpected buffered Chat response: %s", got)
	}
}

func TestOaiResponsesToChatBufferedStreamHandlerDoesNotDuplicateCompletedOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{\"city\":\"Paris\"}"}}`,
		`data: {"type":"response.completed","response":{"id":"resp-1","model":"grok-4.5","status":"completed","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{\"city\":\"Paris\"}"}],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-4.5"},
		RelayFormat: types.RelayFormatOpenAI,
	}

	if _, apiErr := OaiResponsesToChatBufferedStreamHandler(ctx, info, resp); apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	if strings.Count(recorder.Body.String(), `"id":"call_1"`) != 1 {
		t.Fatalf("function call was duplicated: %s", recorder.Body.String())
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
