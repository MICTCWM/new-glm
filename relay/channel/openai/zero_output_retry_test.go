package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type synchronizedResponseWriter struct {
	mu     sync.Mutex
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *synchronizedResponseWriter) Header() http.Header {
	return w.header
}

func (w *synchronizedResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = status
	}
}

func (w *synchronizedResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *synchronizedResponseWriter) Flush() {}

func (w *synchronizedResponseWriter) Body() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

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

func TestOaiStreamHandlerRetriesZeroOutput(t *testing.T) {
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
	// 缓存机制移除后，空 chunk（role）会立即转发，但 usage-only chunk 不会被转发，
	// 也不应泄露任何正文内容。
	written := recorder.Body.String()
	if strings.Contains(written, `"usage"`) {
		t.Fatalf("usage-only chunk should not be forwarded when include_usage=false: %q", written)
	}
	if strings.Contains(written, `"content"`) {
		t.Fatalf("no content should be written for a zero-output stream: %q", written)
	}
}

func TestOaiStreamHandlerSendsFirstOutputWithoutWaitingForNextChunk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	writer := &synchronizedResponseWriter{header: make(http.Header)}
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamReader, upstreamWriter := io.Pipe()
	defer upstreamReader.Close()
	defer upstreamWriter.Close()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       upstreamReader,
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "test-model",
		},
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		IsStream:    true,
		DisablePing: true,
	}
	info.SetEstimatePromptTokens(10)

	result := make(chan *types.NewAPIError, 1)
	go func() {
		_, apiErr := OaiStreamHandler(ctx, info, resp)
		result <- apiErr
	}()

	firstChunk := `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"first-token"},"finish_reason":null}]}`
	if _, err := io.WriteString(upstreamWriter, firstChunk+"\n"); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(writer.Body(), "first-token") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(writer.Body(), "first-token") {
		t.Fatalf("first output was not forwarded before the next upstream chunk: %s", writer.Body())
	}

	finishChunk := `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`
	if _, err := io.WriteString(upstreamWriter, finishChunk+"\n"); err != nil {
		t.Fatalf("write finish chunk: %v", err)
	}
	if _, err := io.WriteString(upstreamWriter, "data: [DONE]\n"); err != nil {
		t.Fatalf("write done marker: %v", err)
	}

	select {
	case apiErr := <-result:
		if apiErr != nil {
			t.Fatalf("OaiStreamHandler returned an error: %v", apiErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OaiStreamHandler did not finish after the upstream stream ended")
	}
}
