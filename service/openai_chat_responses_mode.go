package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	return openaicompat.ShouldChatCompletionsUseResponsesPolicy(policy, channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return openaicompat.ShouldChatCompletionsUseResponsesGlobal(channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesForChannel(settings dto.ChannelSettings, channelID int, channelType int, model string) bool {
	return openaicompat.ShouldChatCompletionsUseResponsesForChannel(settings, channelID, channelType, model)
}

func ResponsesProtocolRequiredForChannel(settings dto.ChannelSettings, channelType int) bool {
	return openaicompat.ResponsesProtocolRequiredForChannel(settings, channelType)
}
