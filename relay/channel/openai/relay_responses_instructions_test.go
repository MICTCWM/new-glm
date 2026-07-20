package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestOaiResponsesHandlerRedactsInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	const secret = "internal session prompt"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp-1","object":"response","status":"completed","instructions":"` + secret + `","model":"grok-4.5","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)),
	}
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{UpstreamModelName: "grok-4.5"},
		RelayFormat: types.RelayFormatOpenAIResponses,
	}

	if _, apiErr := OaiResponsesHandler(ctx, info, resp); apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	got := recorder.Body.String()
	if strings.Contains(got, secret) || strings.Contains(got, `"instructions"`) {
		t.Fatalf("instructions leaked from non-stream Responses response: %s", got)
	}
}

func TestOaiResponsesStreamHandlerRedactsInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	const secret = "internal session prompt"
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp-1","instructions":"` + secret + `","model":"grok-4.5"}}`,
		`data: {"type":"response.in_progress","response":{"id":"resp-1","instructions":"` + secret + `","model":"grok-4.5"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"id":"resp-1","status":"completed","instructions":"` + secret + `","model":"grok-4.5","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{UpstreamModelName: "grok-4.5"},
		RelayFormat: types.RelayFormatOpenAIResponses,
		IsStream:    true,
	}

	if _, apiErr := OaiResponsesStreamHandler(ctx, info, resp); apiErr != nil {
		t.Fatalf("apiErr = %v, want nil", apiErr)
	}
	got := recorder.Body.String()
	if strings.Contains(got, secret) || strings.Contains(got, `"instructions"`) {
		t.Fatalf("instructions leaked from streaming Responses response: %s", got)
	}
	if !strings.Contains(got, `"delta":"hello"`) {
		t.Fatalf("converted response text is missing: %s", got)
	}
}
