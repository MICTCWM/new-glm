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

// WaitBeforeMaxRetry 极限重试模式下，第6次起每次重试前发送 "retry X/total" 提示并等待
func WaitBeforeMaxRetry(c *gin.Context, info *relaycommon.RelayInfo, retryNumber int, total int) {
	delay := common.MaxRetryDelay
	if delay <= 0 {
		return
	}
	msg := fmt.Sprintf("retry %d/%d", retryNumber, total)
	logger.LogInfo(c, fmt.Sprintf("max retry #%d: waiting %v before next attempt (%s)", retryNumber, delay, msg))
	// 发送固定格式提示到 thinking/reasoning_content 通道
	streamnotice.SendRetryMessage(c, info, msg)
	time.Sleep(delay)
}

func SendRetryWaitNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	return streamnotice.SendRetryWaitNotice(c, info)
}

// SendErrorNotice 在已经开始流式输出后，将错误信息作为正文内容（content）流式输出给用户。
// 用于所有重试都失败的场景，因为此时 HTTP 响应头已发送 200，无法再通过状态码传递错误。
func SendErrorNotice(c *gin.Context, info *relaycommon.RelayInfo, errorMsg string) bool {
	return streamnotice.SendErrorNotice(c, info, errorMsg)
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
