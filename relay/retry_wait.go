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

func WaitBeforeRetry(c *gin.Context, info *relaycommon.RelayInfo, delay time.Duration, retryNumber int, label string) bool {
	if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	if delay <= 0 {
		return true
	}
	if label == "" {
		label = "retry"
	}
	logger.LogInfo(c, fmt.Sprintf("%s #%d: waiting %v before next attempt", label, retryNumber, delay))
	SendRetryWaitNotice(c, info)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	if c == nil || c.Request == nil {
		<-timer.C
		return true
	}
	select {
	case <-timer.C:
		return true
	case <-c.Request.Context().Done():
		return false
	}
}

// WaitBeforeMaxRetry 极限重试模式下，第6次起每次重试前发送 "retry X/total" 提示并等待
func WaitBeforeMaxRetry(c *gin.Context, info *relaycommon.RelayInfo, retryNumber int, total int) bool {
	delay := common.MaxRetryDelay
	if delay <= 0 {
		return true
	}
	if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	msg := fmt.Sprintf("retry %d/%d", retryNumber, total)
	logger.LogInfo(c, fmt.Sprintf("max retry #%d: waiting %v before next attempt (%s)", retryNumber, delay, msg))
	// 发送固定格式提示到 thinking/reasoning_content 通道
	streamnotice.SendRetryMessage(c, info, msg)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	if c == nil || c.Request == nil {
		<-timer.C
		return true
	}
	select {
	case <-timer.C:
		return true
	case <-c.Request.Context().Done():
		return false
	}
}

func SendRetryWaitNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	return streamnotice.SendRetryWaitNotice(c, info)
}

// SendErrorNotice 在已经开始流式输出后，将错误信息以标准错误 chunk
// （OpenAI/Claude/Gemini/Responses 各自的 error 事件）而非正文 content 输出给用户。
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
	return WaitBeforeRetry(c, info, delay, attempt+1, label)
}
