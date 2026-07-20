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

// ShouldChatCompletionsUseResponsesGlobal is retained for source compatibility
// with integrations that used the old global policy. Global automatic routing
// is intentionally disabled; use ChannelSettings.ResponsesProtocol instead.
func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return false
}

// ShouldChatCompletionsUseResponsesForChannel returns the protocol configured
// on the selected channel. Most OpenAI-compatible channels require an
// explicit ResponsesProtocol opt-in because they can serve either endpoint.
// Codex is the exception: its adaptor only supports the Responses API, so it
// must be treated as a native Responses channel even though the UI does not
// expose the generic protocol switch for it.
func ShouldChatCompletionsUseResponsesForChannel(settings dto.ChannelSettings, channelID int, channelType int, model string) bool {
	if channelType == constant.ChannelTypeAnthropic {
		// An Anthropic channel always receives Claude Messages requests.
		return false
	}
	if channelType == constant.ChannelTypeCodex {
		return true
	}
	if settings.ResponsesProtocol {
		return true
	}

	// Keep old channel records working. Before the boolean switch was added,
	// the same setting was stored as upstream_protocol: "responses" (or "re").
	// Without this compatibility path an existing RE fallback is silently
	// treated as Chat and receives the wrong request protocol.
	switch strings.ToLower(strings.TrimSpace(settings.UpstreamProtocol)) {
	case "responses", "response", "re":
		return true
	default:
		return false
	}
}

// ResponsesProtocolRequiredForChannel reports whether the selected channel
// must receive a Responses request even when pass-through is enabled.
func ResponsesProtocolRequiredForChannel(settings dto.ChannelSettings, channelType int) bool {
	return ShouldChatCompletionsUseResponsesForChannel(settings, 0, channelType, "")
}
