package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}

	if is429OrInvalidTokenError(err) || operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}
	lowerError := strings.ToLower(err.Error())
	for _, keyword := range operation_setting.AutomaticDisableKeywords {
		if keyword != "" && strings.Contains(lowerError, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// is429OrInvalidTokenError 判断错误是否为 429 Too Many Requests 或 Invalid Token
// 这两种错误明确表示上游容量不足或鉴权失败，无需延迟确认即可直接禁用
func is429OrInvalidTokenError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if err.StatusCode == http.StatusTooManyRequests {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "invalid token")
}

// ShouldHardDisableChannel 判断是否需要硬禁用渠道（401/429），不依赖总开关
func ShouldHardDisableChannel(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return err.StatusCode == 401 || err.StatusCode == 429 || is429OrInvalidTokenError(err)
}

// HardDisableChannel 硬禁用渠道，根据错误码设置对应的禁用原因
func HardDisableChannel(channelError types.ChannelError, err *types.NewAPIError) {
	reason := model.ChannelStatusReasonRateLimit
	if err.StatusCode == 401 {
		reason = model.ChannelStatusReasonAuthError
	}
	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被强制禁用（HTTP %d）", channelError.ChannelName, channelError.ChannelId, err.StatusCode)
		content := fmt.Sprintf("通道「%s」（#%d）因上游返回 HTTP %d 错误被强制禁用。\n错误详情：%s", channelError.ChannelName, channelError.ChannelId, err.StatusCode, err.ErrorWithStatusCode())
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func ShouldDelayDisableChannel(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return is429OrInvalidTokenError(err)
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
