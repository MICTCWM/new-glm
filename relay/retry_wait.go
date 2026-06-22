package relay

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	streamnotice "github.com/QuantumNous/new-api/relay/stream_notice"
	"github.com/gin-gonic/gin"
)

func WaitBeforeRetry(c *gin.Context, info *relaycommon.RelayInfo, delay time.Duration, retryNumber int, label string) {
	if delay <= 0 {
		return
	}
	if label == "" {
		label = "retry"
	}
	logger.LogInfo(c, fmt.Sprintf("%s #%d: waiting %v before next attempt", label, retryNumber, delay))
	SendRetryWaitNotice(c, info)
	time.Sleep(delay)
}

func SendRetryWaitNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	return streamnotice.SendRetryWaitNotice(c, info)
}
