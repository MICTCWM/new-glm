package relay

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func cancelledTestContext() (*gin.Context, context.CancelFunc) {
	requestContext, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(requestContext)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	return context, cancel
}

func TestWaitBeforeRetryStopsWhenRequestIsCancelled(t *testing.T) {
	c, cancel := cancelledTestContext()
	cancel()

	started := time.Now()
	retry := WaitBeforeRetry(c, nil, time.Second, 1, "test retry")

	require.False(t, retry)
	require.Less(t, time.Since(started), 500*time.Millisecond)
}

func TestWaitBeforeRetryChecksCancellationWithoutDelay(t *testing.T) {
	c, cancel := cancelledTestContext()
	cancel()

	require.False(t, WaitBeforeRetry(c, nil, 0, 1, "test retry"))
}
