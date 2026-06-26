package stream_notice

import (
	"encoding/json"
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
	return sendThinkingNotice(c, info, RandomRetryMessage(), "retry wait")
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
		Model:   info.GetDisplayModelName(),
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
			Model: info.GetDisplayModelName(),
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

// SendErrorNotice 将错误信息作为正文内容（content）流式输出给用户。
// 用于在已经开始流式输出（HTTP 响应头已发送 200）后，所有重试都失败的场景，
// 因为此时无法再通过 HTTP 状态码传递错误信息。
func SendErrorNotice(c *gin.Context, info *relaycommon.RelayInfo, errorMsg string) bool {
	return sendContentNotice(c, info, errorMsg, "error")
}

// sendContentNotice 是 sendThinkingNotice 的 content 版本，
// 路由分发逻辑与 sendThinkingNotice 保持一致，只是发送到 content 而非 thinking。
func sendContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
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
		return sendClaudeContentNotice(c, info, notice, logLabel)
	case types.RelayFormatGemini:
		if info.RelayMode == relayconstant.RelayModeGemini {
			return sendGeminiContentNotice(c, info, notice, logLabel)
		}
	case types.RelayFormatOpenAI:
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			return sendResponsesContentNotice(c, info, notice, logLabel)
		}
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return false
	}
	return sendOpenAIChatContentNotice(c, info, notice, logLabel)
}

// sendOpenAIChatContentNotice 将错误正文发送到 OpenAI Chat 协议的 content 字段，
// 并正确结束流式输出（发送 stop chunk 和 [DONE]）。
func sendOpenAIChatContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	// 1. 发送错误正文到 content 字段
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   info.GetDisplayModelName(),
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(notice),
				},
			},
		},
	}
	data, err := common.Marshal(chunk)
	if err != nil {
		logger.LogWarn(c, "failed to marshal "+logLabel+" notice: "+err.Error())
		return false
	}
	if err := openai.HandleStreamFormat(c, info, string(data), info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" notice: "+err.Error())
		return false
	}

	// 2. 发送带 finish_reason: stop 的结束 chunk
	stopReason := "stop"
	stopChunk := helper.GenerateStopResponse(helper.GetResponseID(c), time.Now().Unix(), info.GetDisplayModelName(), stopReason)
	stopData, err := common.Marshal(stopChunk)
	if err != nil {
		logger.LogWarn(c, "failed to marshal "+logLabel+" stop notice: "+err.Error())
		return false
	}
	if err := openai.HandleStreamFormat(c, info, string(stopData), info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" stop notice: "+err.Error())
		return false
	}

	// 3. 发送 [DONE] 结束流式输出
	helper.Done(c)
	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}

// sendClaudeContentNotice 将错误正文发送到 Claude 协议的 text content 字段，
// 需要先关闭可能开着的 thinking block，然后开启 text block，最后正确结束流。
func sendClaudeContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	// 0. 如果尚未发送过 message_start（即没发过 thinking notice），需要先补发
	// 触发场景：流式 Claude 请求，SSE header 已发送，但重试首次就失败（如计费失败）
	if !info.RpmQueueThinkingNoticeSent {
		msg := &dto.ClaudeMediaMessage{
			Id:    helper.GetResponseID(c),
			Model: info.GetDisplayModelName(),
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
		info.RpmQueueThinkingNoticeSent = true
	}

	// 1. 如果 thinking block 还开着，先关闭它
	if info.ClaudeRpmQueueThinkingOpen {
		thinkIdx := 0
		if err := helper.ClaudeData(c, dto.ClaudeResponse{
			Type:  "content_block_stop",
			Index: &thinkIdx,
		}); err != nil {
			logger.LogWarn(c, "failed to send "+logLabel+" claude thinking stop: "+err.Error())
			return false
		}
		info.ClaudeRpmQueueThinkingOpen = false
		info.ClaudeRpmQueueMergedThinking = false
		info.ClaudeRpmQueueIndexOffset = 1
	}

	// 2. 开启新的 text block
	textIdx := info.ClaudeRpmQueueIndexOffset
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_start",
		Index: &textIdx,
		ContentBlock: &dto.ClaudeMediaMessage{
			Type: "text",
			Text: common.GetPointer(""),
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude text start: "+err.Error())
		return false
	}

	// 3. 发送错误正文到 text_delta
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &textIdx,
		Delta: &dto.ClaudeMediaMessage{
			Type: "text_delta",
			Text: &notice,
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude text delta: "+err.Error())
		return false
	}

	// 4. 关闭 text block
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_stop",
		Index: &textIdx,
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude text stop: "+err.Error())
		return false
	}

	// 5. 发送 message_delta 结束消息（stop_reason=end_turn）
	stopReason := "end_turn"
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type: "message_delta",
		Delta: &dto.ClaudeMediaMessage{
			StopReason: &stopReason,
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude message_delta: "+err.Error())
		return false
	}

	// 6. 发送 message_stop 结束流
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type: "message_stop",
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude message_stop: "+err.Error())
		return false
	}

	return flushNotice(c, logLabel)
}

// sendGeminiContentNotice 将错误正文发送到 Gemini 协议的 text content 字段（不设置 Thought），
// 并设置 FinishReason=STOP 结束流。
func sendGeminiContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	finishReason := "STOP"
	resp := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Index:         0,
				SafetyRatings: []dto.GeminiChatSafetyRating{},
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{
							Text: notice,
						},
					},
				},
				FinishReason: &finishReason,
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
	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}

// sendResponsesContentNotice 将错误正文发送到 Responses 协议的 output_text 事件，
// 并发送 response.completed 事件结束响应。
func sendResponsesContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	itemID := fmt.Sprintf("rs_%s", helper.GetResponseID(c))
	responseID := helper.GetResponseID(c)
	outputIndex := 0
	contentIndex := 0
	events := []dto.ResponsesStreamResponse{
		{
			Type: "response.created",
			Response: &dto.OpenAIResponsesResponse{
				ID:     responseID,
				Object: "response",
				Status: json.RawMessage(`"in_progress"`),
				Model:  info.GetDisplayModelName(),
			},
		},
		{
			Type: dto.ResponsesOutputTypeItemAdded,
			Item: &dto.ResponsesOutput{
				Type:   "message",
				ID:     itemID,
				Status: "in_progress",
				Role:   "assistant",
			},
			OutputIndex: &outputIndex,
		},
		{
			Type:         "response.output_text.delta",
			Delta:        notice,
			ItemID:       itemID,
			OutputIndex:  &outputIndex,
			ContentIndex: &contentIndex,
		},
		{
			Type:         "response.output_text.done",
			ItemID:       itemID,
			OutputIndex:  &outputIndex,
			ContentIndex: &contentIndex,
		},
		{
			Type: dto.ResponsesOutputTypeItemDone,
			Item: &dto.ResponsesOutput{
				Type:   "message",
				ID:     itemID,
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type:        "output_text",
						Text:        notice,
						Annotations: []interface{}{},
					},
				},
			},
			OutputIndex: &outputIndex,
		},
		{
			Type: "response.completed",
			Response: &dto.OpenAIResponsesResponse{
				ID:     responseID,
				Object: "response",
				Status: json.RawMessage(`"completed"`),
				Model:  info.GetDisplayModelName(),
			},
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
