package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func TestShouldChatCompletionsUseResponsesForChannelRequiresExplicitOptInForSwitchableChannels(t *testing.T) {
	require.False(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{}, 123, constant.ChannelTypeOpenAI, "gpt-5"),
		"switchable OpenAI channels must explicitly opt in to Responses",
	)
	require.True(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{ResponsesProtocol: true}, 123, constant.ChannelTypeOpenAI, "gpt-5"),
	)
}

func TestShouldChatCompletionsUseResponsesForChannelAcceptsLegacyProtocolSetting(t *testing.T) {
	for _, legacyValue := range []string{"responses", "response", "re"} {
		require.Truef(t, ShouldChatCompletionsUseResponsesForChannel(
			dto.ChannelSettings{UpstreamProtocol: legacyValue},
			123,
			constant.ChannelTypeOpenAI,
			"gpt-5",
		), "legacy protocol %q should select Responses", legacyValue)
		require.Truef(t, ResponsesProtocolRequiredForChannel(
			dto.ChannelSettings{UpstreamProtocol: legacyValue},
			constant.ChannelTypeOpenAI,
		), "legacy protocol %q should require Responses", legacyValue)
	}
}

func TestCodexChannelAlwaysUsesResponsesProtocol(t *testing.T) {
	require.True(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{}, 123, constant.ChannelTypeCodex, "gpt-5"),
	)
	require.True(t, ResponsesProtocolRequiredForChannel(
		dto.ChannelSettings{PassThroughBodyEnabled: true}, constant.ChannelTypeCodex),
	)
}

func TestShouldChatCompletionsUseResponsesPolicyStillSupportsLegacyMatching(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		ChannelIDs:    []int{123},
		ModelPatterns: []string{"^gpt-5"},
	}
	require.True(t, ShouldChatCompletionsUseResponsesPolicy(policy, 123, constant.ChannelTypeOpenAI, "gpt-5.4"))
	require.False(t, ShouldChatCompletionsUseResponsesPolicy(policy, 456, constant.ChannelTypeOpenAI, "gpt-5.4"))
	// The legacy global policy is no longer consulted for channel routing.
	require.False(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{}, 123, constant.ChannelTypeOpenAI, "gpt-5.4"),
	)
	require.False(t, ShouldChatCompletionsUseResponsesGlobal(123, constant.ChannelTypeOpenAI, "gpt-5.4"))
}

func TestAnthropicChannelNeverUsesResponsesProtocol(t *testing.T) {
	require.False(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{ResponsesProtocol: true}, 123, constant.ChannelTypeAnthropic, "claude"),
	)
	require.False(t, ResponsesProtocolRequiredForChannel(
		dto.ChannelSettings{ResponsesProtocol: true}, constant.ChannelTypeAnthropic),
	)
}

func TestResponsesProtocolRequiredForChannelUsesExplicitSwitchForSwitchableChannels(t *testing.T) {
	require.True(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{ResponsesProtocol: true}, constant.ChannelTypeOpenAI))
	require.False(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{}, constant.ChannelTypeOpenAI))
}
