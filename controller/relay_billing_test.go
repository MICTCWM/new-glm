package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRefundRelayBillingKeepsRefundForUnmarkedCancelledRequest(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.True(t, shouldRefundRelayBilling(c))
}

func TestShouldRefundRelayBillingSkipsMarkedCancelledRequest(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("request_context_cancelled", true)

	require.False(t, shouldRefundRelayBilling(c))
}

func TestShouldRefundRelayBillingKeepsRefundForRealFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.True(t, shouldRefundRelayBilling(c))
}

func TestShouldRefundRelayBillingKeepsRefundForUnmarkedContextCancellation(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.True(t, shouldRefundRelayBilling(c))
}
