package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBillingSessionDoesNotTrustGptWallet(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("token_quota", common.GetTrustQuota()+1)

	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{
			UserId:       1,
			UserGptQuota: 999,
		},
		funding: &GptWalletFunding{userId: 1},
	}

	assert.False(t, session.shouldTrust(c))
}

func TestDeferredResponseFlushesLater(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	EnableDeferredResponse(c)
	IOCopyBytesGracefully(c, &http.Response{
		StatusCode: http.StatusAccepted,
		Header: http.Header{
			"X-Test":         []string{"ok"},
			"Content-Length": []string{"999"},
		},
	}, []byte("hello"))

	assert.Empty(t, recorder.Body.String())
	assert.Empty(t, recorder.Header().Get("X-Test"))

	FlushDeferredResponse(c)

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.Equal(t, "hello", recorder.Body.String())
	assert.Equal(t, "ok", recorder.Header().Get("X-Test"))
	assert.Equal(t, "5", recorder.Header().Get("Content-Length"))
}
