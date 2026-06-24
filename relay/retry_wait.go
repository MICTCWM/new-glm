package relay

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
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

// ApplyRetryDelay applies retry delay logic based on common.RetryDelays configuration.
// Returns true if a delay was applied, false otherwise.
// This is a helper to eliminate duplicated retry delay code across handlers.
func ApplyRetryDelay(c *gin.Context, info *relaycommon.RelayInfo, attempt int, label string) bool {
	var delay time.Duration
	if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
		delay = common.RetryDelays[attempt]
	}
	if delay > 0 {
		WaitBeforeRetry(c, info, delay, attempt+1, label)
		return true
	}
	return false
}
