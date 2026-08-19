package stream_notice

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
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

// SendRetryMessage 发送固定格式的重试提示消息（如 "retry 6/50"）
func SendRetryMessage(c *gin.Context, info *relaycommon.RelayInfo, message string) bool {
	return sendThinkingNotice(c, info, message, "retry message")
}

func sendThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	if info == nil || !info.IsStream {
		return false
	}
	if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	channelID := common.GetContextKeyInt(c, appconstant.ContextKeyChannelId)
	if channelID == 0 && info.ChannelMeta != nil {
		channelID = info.ChannelMeta.ChannelId
	}
	if !common.IsReassuranceChannel(channelID) {
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
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
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
	// 标记为安抚性思考 chunk，跳过 sendStreamData 的自动兜底转换和 think 标签包装，
	// 避免安抚内容被转成 content 污染正文
	info.IsNoticeChunk = true
	err = openai.HandleStreamFormat(c, info, string(data), info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent)
	info.IsNoticeChunk = false
	if err != nil {
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
	responseID := helper.GetResponseID(c)
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
			Type: "response.in_progress",
			Response: &dto.OpenAIResponsesResponse{
				ID:     responseID,
				Object: "response",
				Status: json.RawMessage(`"in_progress"`),
				Model:  info.GetDisplayModelName(),
			},
		},
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

// SendErrorNotice 将错误信息以标准错误 chunk 输出给用户，而非正文 content：
// 即在各协议（OpenAI/Claude/Gemini/Responses）对应的 error 事件中携带错误信息。
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
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			return sendResponsesContentNotice(c, info, notice, logLabel)
		}
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return false
	}
	return sendOpenAIChatContentNotice(c, info, notice, logLabel)
}

// sendOpenAIChatContentNotice 发送 OpenAI Chat 协议的标准 SSE 错误 chunk
// （data: {"error":{...}}），随后发送 [DONE] 结束流式输出。
func sendOpenAIChatContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	data, err := common.Marshal(struct {
		Error types.OpenAIError `json:"error"`
	}{
		Error: types.OpenAIError{
			Message: notice,
			Type:    string(types.ErrorTypeNewAPIError),
		},
	})
	if err != nil {
		logger.LogWarn(c, "failed to marshal "+logLabel+" notice: "+err.Error())
		return false
	}
	if err := helper.StringData(c, string(data)); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" notice: "+err.Error())
		return false
	}
	helper.Done(c)
	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}

// sendClaudeContentNotice 发送 Claude 协议的标准流式错误
// （event: error + data: {"type":"error","error":{"type":"api_error","message":"..."}}）。
// 依据 Anthropic 流式规范，error 事件本身即流终止信号，发出后无需再补发
// message_delta/message_stop；error 之后直接 flushNotice 结束流。
// 说明：若此前已开启 thinking block 且尚未发出对应的 content_block_stop，无需在
// error 前强制关闭——error 事件即终止整个流的解析，客户端收到后停止交付后续事件
// （与官方行为一致）。原 content 版先发 content_block_stop 仅是为了正文承接，改为
// error 后此结构不再需要。
func sendClaudeContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type: "error",
		Error: types.ClaudeError{
			Type:    "api_error",
			Message: notice,
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" claude error: "+err.Error())
		return false
	}

	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}

// sendGeminiContentNotice 发送 Gemini 协议的标准 SSE 错误
// （data: {"error":{...}}）。
func sendGeminiContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	data, err := common.Marshal(struct {
		Error types.OpenAIError `json:"error"`
	}{
		Error: types.OpenAIError{
			Message: notice,
			Type:    string(types.ErrorTypeNewAPIError),
		},
	})
	if err != nil {
		logger.LogWarn(c, "failed to marshal "+logLabel+" gemini notice: "+err.Error())
		return false
	}
	if err := helper.StringData(c, string(data)); err != nil {
		logger.LogWarn(c, "failed to send "+logLabel+" gemini notice: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	return flushNotice(c, logLabel)
}

// sendResponsesContentNotice 发送 Responses 协议的标准流式错误：
// 只发送 response.failed 事件（携带 error），随后即结束响应流。
// 说明：一个 response 只能处于一个终态，failed 之后不应再发送 response.completed
// （协议状态机中两者互斥，不能既 failed 又 completed）。客户端通过 response.failed
// 或连接关闭即可感知流结束，这与 OpenAI Responses API 遇到错误时的非正常终止行为
// 保持一致，因此此处选择去掉尾部的 response.completed 而非补发一个 Status 为
// "failed" 的 completed 事件（补发 completed 虽保持状态机一致，但多出冗余事件增加
// 不确定性），故采用"failed 后直接 flushNotice 结束事件循环"这一更稳妥的方式。
func sendResponsesContentNotice(c *gin.Context, info *relaycommon.RelayInfo, notice string, logLabel string) bool {
	responseID := helper.GetResponseID(c)
	errorObj := types.OpenAIError{
		Message: notice,
		Type:    string(types.ErrorTypeNewAPIError),
	}
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
			Type: "response.failed",
			Response: &dto.OpenAIResponsesResponse{
				ID:     responseID,
				Object: "response",
				Status: json.RawMessage(`"failed"`),
				Model:  info.GetDisplayModelName(),
				Error:  errorObj,
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
