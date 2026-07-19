package openaicompat

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	if !policy.IsChannelEnabled(channelID, channelType) {
		return false
	}
	return matchAnyRegex(policy.ModelPatterns, model)
}

// ShouldChatCompletionsUseResponsesGlobal is retained for source compatibility
// with integrations that used the old global policy. Global automatic routing
// is intentionally disabled; use ChannelSettings.ResponsesProtocol instead.
func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return false
}

// ShouldChatCompletionsUseResponsesForChannel returns the protocol explicitly
// configured on the selected channel. It intentionally ignores the global
// model policy and channel type: a fallback channel may use a different
// upstream protocol, so conversion must be a per-channel opt-in.
func ShouldChatCompletionsUseResponsesForChannel(settings dto.ChannelSettings, channelID int, channelType int, model string) bool {
	if channelType == constant.ChannelTypeAnthropic {
		// An Anthropic channel always receives Claude Messages requests.
		return false
	}
	return settings.ResponsesProtocol
}

// ResponsesProtocolRequiredForChannel reports whether the selected channel
// must receive a Responses request even when pass-through is enabled.
func ResponsesProtocolRequiredForChannel(settings dto.ChannelSettings, channelType int) bool {
	return channelType != constant.ChannelTypeAnthropic && settings.ResponsesProtocol
}
