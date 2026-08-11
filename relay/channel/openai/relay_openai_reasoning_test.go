package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSendStreamDataPreservesReasoningWhenConversionIsDisabled(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "reasoning-model"}

	err := sendStreamData(c, info, `{"id":"chunk-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":"private reasoning"}}]}`, false, false)

	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"reasoning_content":"private reasoning"`)
	require.NotContains(t, recorder.Body.String(), `"content":"private reasoning"`)
}
