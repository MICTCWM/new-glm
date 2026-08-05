package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

const ChannelProbeErrorKeyword = "Service temporarily unavailable"

// IsChannelProbeError identifies the upstream response that should take a
// probe-managed channel out of service immediately.
func IsChannelProbeError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(ChannelProbeErrorKeyword))
}

// HandleChannelProbeError returns true when the channel is probe-managed. The
// caller should skip every normal automatic-disable rule in that case.
// Matching the probe error also moves the channel to the dedicated probe
// disabled status, regardless of the global auto-ban switch or channel flag.
func HandleChannelProbeError(channelError types.ChannelError, err *types.NewAPIError) bool {
	channel, getErr := model.CacheGetChannel(channelError.ChannelId)
	if getErr != nil || channel == nil || !channel.IsProbeEnabled() {
		return false
	}
	if IsChannelProbeError(err) {
		DisableChannelForProbe(channelError, err.ErrorWithStatusCode())
	}
	return true
}

// DisableChannelForProbe moves a channel to the probe-specific disabled state.
// It intentionally does not consult AutoBan or the global automatic-disable
// switch. The hourly probe can later restore only this state.
func DisableChannelForProbe(channelError types.ChannelError, reason string) bool {
	if strings.TrimSpace(reason) == "" {
		reason = "channel probe failed"
	}
	success := model.UpdateChannelProbeStatus(channelError.ChannelId, common.ChannelStatusProbeDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被探针禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）因探针检测失败已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusProbeDisabled), subject, content)
	}
	return success
}
