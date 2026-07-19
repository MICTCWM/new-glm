package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func TestShouldChatCompletionsUseResponsesForChannelRequiresExplicitOptIn(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeCodex} {
		require.False(t, ShouldChatCompletionsUseResponsesForChannel(
			dto.ChannelSettings{}, 123, channelType, "gpt-5"),
			"channel type must not implicitly select Responses",
		)
		require.True(t, ShouldChatCompletionsUseResponsesForChannel(
			dto.ChannelSettings{ResponsesProtocol: true}, 123, channelType, "gpt-5"),
		)
	}
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

func TestResponsesProtocolRequiredForChannelOnlyUsesExplicitSwitch(t *testing.T) {
	require.False(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{PassThroughBodyEnabled: true}, constant.ChannelTypeCodex))
	require.True(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{ResponsesProtocol: true}, constant.ChannelTypeOpenAI))
	require.False(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{}, constant.ChannelTypeOpenAI))
}
