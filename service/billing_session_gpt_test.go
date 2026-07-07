package service

import (
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
