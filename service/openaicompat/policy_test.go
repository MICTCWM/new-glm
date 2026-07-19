package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func TestShouldChatCompletionsUseResponsesForChannelAutoDetectsCodex(t *testing.T) {
	got := ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{UpstreamProtocol: "chat"},
		123,
		constant.ChannelTypeCodex,
		"gpt-5",
	)
	require.True(t, got, "Responses-only fallback channels must not receive Chat payloads")
}

func TestShouldChatCompletionsUseResponsesForChannelHonorsExplicitProtocol(t *testing.T) {
	require.True(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{UpstreamProtocol: "responses"},
		123,
		constant.ChannelTypeOpenAI,
		"gpt-5",
	))
	require.False(t, ShouldChatCompletionsUseResponsesForChannel(
		dto.ChannelSettings{UpstreamProtocol: "chat"},
		123,
		constant.ChannelTypeOpenAI,
		"gpt-5",
	))
}

func TestShouldChatCompletionsUseResponsesPolicyStillSupportsLegacyMatching(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		ChannelIDs:    []int{123},
		ModelPatterns: []string{"^gpt-5"},
	}
	require.True(t, ShouldChatCompletionsUseResponsesPolicy(policy, 123, constant.ChannelTypeOpenAI, "gpt-5.4"))
	require.False(t, ShouldChatCompletionsUseResponsesPolicy(policy, 456, constant.ChannelTypeOpenAI, "gpt-5.4"))
}

func TestResponsesProtocolRequiredForChannelOverridesPassThroughForCodex(t *testing.T) {
	require.True(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{PassThroughBodyEnabled: true}, constant.ChannelTypeCodex))
	require.True(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{UpstreamProtocol: "responses"}, constant.ChannelTypeOpenAI))
	require.False(t, ResponsesProtocolRequiredForChannel(dto.ChannelSettings{UpstreamProtocol: "chat"}, constant.ChannelTypeOpenAI))
}
