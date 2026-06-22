package stream_notice

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func SendRetryWaitNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	return sendThinkingNotice(c, info, common.UserMessageRetryWaitThinking+"\n", "retry wait")
}

func sendThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	if info == nil || !info.IsStream {
		return false
	}
	if info.ChannelMeta == nil {
		info.InitChannelMeta(c)
	}
	if info.ChannelMeta == nil {
		logger.LogWarn(c, "failed to send "+logLabel+" notice: channel meta is nil")
		return false
	}
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return sendClaudeThinkingNotice(c, info, notice, logLabel)
	case types.RelayFormatGemini:
		if info.RelayMode == relayconstant.RelayModeGemini {
			return sendGeminiThinkingNotice(c, info, notice, logLabel)
		}
	case types.RelayFormatOpenAI:
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			return sendResponsesThinkingNotice(c, info, notice, logLabel)
		}
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return false
	}
	return sendOpenAIChatThinkingNotice(c, info, notice, logLabel)
}

func flushNotice(c *gin.Context, logLabel string) bool {
	if err := helper.FlushWriter(c); err != nil {
		logger.LogWarn(c, "failed to flush "+logLabel+" notice: "+err.Error())
		return false
	}
	return true
}

func sendOpenAIChatThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   info.OriginModelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
				},
			},
		},
	}
	chunk.Choices[0].Delta.SetReasoningContent(notice)

	data, err := common.Marshal(chunk)
	if err != nil {
		logger.LogWarn(c, "failed to marshal "+logLabel+" notice: "+err.Error())
		return false
	}
	if err := openai.HandleStreamFormat(c, info, string(data), info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" notice: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}

func sendClaudeThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	idx := 0
	if !info.ClaudeRpmQueueThinkingOpen {
		msg := &dto.ClaudeMediaMessage{
			Id:    helper.GetResponseID(c),
			Model: info.OriginModelName,
			Type:  "message",
			Role:  "assistant",
			Usage: &dto.ClaudeUsage{
				InputTokens:  info.GetEstimatePromptTokens(),
				OutputTokens: 0,
			},
		}
		msg.SetContent(make([]any, 0))
		if err := helper.ClaudeData(c, dto.ClaudeResponse{Type: "message_start", Message: msg}); err != nil {
			logger.LogWarn(c, "failed to send "+logLabel+" claude message_start: "+err.Error())
			return false
		}
		if err := helper.ClaudeData(c, dto.ClaudeResponse{
			Type:  "content_block_start",
			Index: &idx,
			ContentBlock: &dto.ClaudeMediaMessage{
				Type:     "thinking",
				Thinking: common.GetPointer(""),
			},
		}); err != nil {
			logger.LogWarn(c, "failed to send "+logLabel+" claude thinking start: "+err.Error())
			return false
		}
	}
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &idx,
		Delta: &dto.ClaudeMediaMessage{
			Type:     "thinking_delta",
			Thinking: &notice,
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude thinking delta: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	info.ClaudeRpmQueueThinkingOpen = true
	return true
}

func sendGeminiThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	resp := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Index:         0,
				SafetyRatings: []dto.GeminiChatSafetyRating{},
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{
							Text:    notice,
							Thought: true,
						},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount: info.GetEstimatePromptTokens(),
			TotalTokenCount:  info.GetEstimatePromptTokens(),
		},
	}
	data, err := common.Marshal(resp)
	if err != nil {
		logger.LogWarn(c, "failed to marshal "+logLabel+" gemini notice: "+err.Error())
		return false
	}
	c.Render(-1, common.CustomEvent{Data: "data: " + string(data)})
	if !flushNotice(c, logLabel) {
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	return true
}

func sendResponsesThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	itemID := fmt.Sprintf("rs_%s", helper.GetResponseID(c))
	events := []dto.ResponsesStreamResponse{
		{
			Type: "response.reasoning_summary_part.added",
			Item: &dto.ResponsesOutput{
				Type:   "reasoning",
				ID:     itemID,
				Status: "in_progress",
			},
			ItemID:       itemID,
			OutputIndex:  common.GetPointer(0),
			SummaryIndex: common.GetPointer(0),
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "summary_text",
				Text: "",
			},
		},
		{
			Type:         "response.reasoning_summary_text.delta",
			Delta:        notice,
			ItemID:       itemID,
			OutputIndex:  common.GetPointer(0),
			SummaryIndex: common.GetPointer(0),
		},
	}
	for _, event := range events {
		data, err := common.Marshal(event)
		if err != nil {
			logger.LogWarn(c, "failed to marshal "+logLabel+" responses notice: "+err.Error())
			return false
		}
		helper.ResponseChunkData(c, event, string(data))
	}
	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}
