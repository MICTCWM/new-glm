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

// ForceDisableChannelFor429OrInvalidToken 强制禁用渠道并标记为配额耗尽
// 当用户请求遇到 429 或 Invalid token 错误时，直接禁用渠道，状态标记为配额耗尽。
// 设计思路：429 和 InvalidToken 是明确的上游容量/鉴权问题，无需延迟确认。
// 直接禁用比原有的延迟禁用+三次重试更高效，减少对上游的无效请求，
// 且能更快切换到可用渠道，提升用户响应速度。
func ForceDisableChannelFor429OrInvalidToken(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生 429/InvalidToken 错误，强制禁用并标记配额耗尽，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过强制禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, model.ChannelStatusReasonQuotaExhausted)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被强制禁用（配额耗尽）", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被强制禁用，原因：检测到 429/InvalidToken 错误，已标记为配额耗尽", channelError.ChannelName, channelError.ChannelId)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
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

func ShouldForceDisableFor429OrInvalidToken(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	return is429OrInvalidTokenError(err)
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
