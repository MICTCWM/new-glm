package openaicompat

import (
	"strings"

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

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return ShouldChatCompletionsUseResponsesPolicy(
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy,
		channelID, channelType, model,
	)
}

// IsResponsesOnlyChannelType identifies channels that cannot receive Chat
// Completions requests directly. Codex currently exposes only the Responses
// protocol, so Chat requests must be converted before reaching it.
func IsResponsesOnlyChannelType(channelType int) bool {
	return channelType == constant.ChannelTypeCodex
}

// ResponsesProtocolRequiredForChannel reports whether conversion is mandatory
// even when pass-through mode is enabled. This prevents a Responses-only
// fallback channel from receiving an incompatible Chat request body.
func ResponsesProtocolRequiredForChannel(settings dto.ChannelSettings, channelType int) bool {
	if IsResponsesOnlyChannelType(channelType) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(settings.UpstreamProtocol)) {
	case "responses", "response", "re":
		return true
	default:
		return false
	}
}

// ShouldChatCompletionsUseResponsesForChannel evaluates the protocol for the
// channel selected for the current retry/fallback attempt. Explicit channel
// configuration and Responses-only channel types take precedence over the
// legacy global policy.
func ShouldChatCompletionsUseResponsesForChannel(settings dto.ChannelSettings, channelID int, channelType int, model string) bool {
	if IsResponsesOnlyChannelType(channelType) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(settings.UpstreamProtocol)) {
	case "responses", "response", "re":
		return true
	case "chat", "chat_completions", "chat-completions":
		return false
	}
	return ShouldChatCompletionsUseResponsesGlobal(channelID, channelType, model)
}
