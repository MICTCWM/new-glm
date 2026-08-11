package openai

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func isResponsesTerminalEvent(eventType string) bool {
	switch eventType {
	case "response.completed", "response.done", "response.incomplete":
		return true
	default:
		return false
	}
}

func responsesStreamIndexKey(itemID string, idx *int) string {
	if itemID == "" {
		return ""
	}
	if idx == nil {
		return itemID
	}
	return fmt.Sprintf("%s:%d", itemID, *idx)
}

func stringDeltaFromPrefix(prev string, next string) string {
	if next == "" {
		return ""
	}
	if prev != "" && strings.HasPrefix(next, prev) {
		return next[len(prev):]
	}
	return next
}

func OaiResponsesToChatHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var responsesResp dto.OpenAIResponsesResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	if err := common.Unmarshal(body, &responsesResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if oaiError := responsesResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	chatId := helper.GetResponseID(c)
	chatResp, usage, err := service.ResponsesResponseToChatCompletionsResponse(&responsesResp, chatId)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if usage == nil || usage.TotalTokens == 0 {
		text := service.ExtractOutputTextFromResponses(&responsesResp)
		usage = service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
		chatResp.Usage = *usage
	}
	if shouldRetryZeroOutputUsage(info, usage) {
		return nil, zeroOutputRetryError(info, usage)
	}

	var responseBody []byte
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		claudeResp := service.ResponseOpenAI2Claude(chatResp, info)
		responseBody, err = common.Marshal(claudeResp)
	case types.RelayFormatGemini:
		geminiResp := service.ResponseOpenAI2Gemini(chatResp, info)
		responseBody, err = common.Marshal(geminiResp)
	default:
		if !info.ChannelSetting.ThinkingToContent {
			normalizeTextResponseThinkTags(chatResp)
		}
		responseBody, err = common.Marshal(chatResp)
	}
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

// OaiResponsesToChatBufferedStreamHandler consumes a Responses SSE stream and
// emits one regular Chat Completions response. This is needed when an upstream
// streams regardless of the client's stream=false request; forwarding the SSE
// events would make a Chat SDK attempt to validate response.created as a Chat
// response and fail with "expected choices".
func OaiResponsesToChatBufferedStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	aggregate := &dto.OpenAIResponsesResponse{
		ID:        helper.GetResponseID(c),
		Object:    "response",
		CreatedAt: int(time.Now().Unix()),
		Model:     info.UpstreamModelName,
		Status:    []byte(`"completed"`),
	}
	var outputText strings.Builder
	var reasoningSummary strings.Builder
	var outputItems []dto.ResponsesOutput
	terminal := false

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), helper.DefaultMaxScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if event.Type == "response.error" || event.Type == "response.failed" {
			if event.Response != nil {
				if oaiErr := event.Response.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
					return nil, types.WithOpenAIError(*oaiErr, http.StatusInternalServerError)
				}
			}
			return nil, types.NewOpenAIError(fmt.Errorf("responses stream error: %s", event.Type), types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}

		switch event.Type {
		case "response.created":
			if event.Response != nil {
				*aggregate = *event.Response
			}
		case "response.output_text.delta":
			outputText.WriteString(event.Delta)
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			reasoningSummary.WriteString(event.Delta)
		case "response.reasoning_summary_text.done":
			if event.Text != "" && reasoningSummary.Len() == 0 {
				reasoningSummary.WriteString(event.Text)
			}
		case "response.reasoning_summary_part.done":
			if event.Part != nil && reasoningSummary.Len() == 0 {
				reasoningSummary.WriteString(event.Part.Text)
			}
		case "response.output_item.done":
			if event.Item != nil {
				outputItems = append(outputItems, *event.Item)
			}
		case "response.completed", "response.done", "response.incomplete":
			terminal = isResponsesTerminalEvent(event.Type)
			if event.Response != nil {
				previousOutput := aggregate.Output
				*aggregate = *event.Response
				if len(aggregate.Output) == 0 {
					aggregate.Output = previousOutput
				}
			}
			if event.Type == "response.incomplete" && len(aggregate.Status) == 0 {
				aggregate.Status = []byte(`"incomplete"`)
			}
		}
		if terminal {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	if aggregate.ID == "" {
		aggregate.ID = helper.GetResponseID(c)
	}
	if aggregate.Model == "" {
		aggregate.Model = info.UpstreamModelName
	}
	for _, item := range outputItems {
		if !responsesOutputExists(aggregate.Output, item) {
			aggregate.Output = append(aggregate.Output, item)
		}
	}

	hasMessage := false
	hasReasoning := false
	for i := range aggregate.Output {
		switch aggregate.Output[i].Type {
		case "message":
			hasMessage = true
		case "reasoning":
			hasReasoning = true
			if reasoningSummary.Len() > 0 && len(aggregate.Output[i].Summary) == 0 && len(aggregate.Output[i].Content) == 0 {
				aggregate.Output[i].Summary = []dto.ResponsesOutputContent{{Type: "summary_text", Text: reasoningSummary.String()}}
			}
		}
	}
	if outputText.Len() > 0 && !hasMessage {
		aggregate.Output = append(aggregate.Output, dto.ResponsesOutput{
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []dto.ResponsesOutputContent{{
				Type: "output_text",
				Text: outputText.String(),
			}},
		})
	}
	if reasoningSummary.Len() > 0 && !hasReasoning {
		reasoning := dto.ResponsesOutput{
			Type:   "reasoning",
			Status: "completed",
			Summary: []dto.ResponsesOutputContent{{
				Type: "summary_text",
				Text: reasoningSummary.String(),
			}},
		}
		// Responses conventionally places reasoning before the assistant message.
		aggregate.Output = append([]dto.ResponsesOutput{reasoning}, aggregate.Output...)
	}

	body, err := common.Marshal(aggregate)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	chatResp := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
	chatResp.Header.Set("Content-Type", "application/json")
	return OaiResponsesToChatHandler(c, info, chatResp)
}

func responsesOutputExists(outputs []dto.ResponsesOutput, candidate dto.ResponsesOutput) bool {
	for _, output := range outputs {
		if candidate.ID != "" && output.ID == candidate.ID {
			return true
		}
		if candidate.CallId != "" && output.CallId == candidate.CallId {
			return true
		}
		if candidate.ID == "" && candidate.CallId == "" && output.Type == candidate.Type &&
			output.Role == candidate.Role && output.Name == candidate.Name &&
			output.ArgumentsString() == candidate.ArgumentsString() &&
			service.ExtractOutputTextFromResponses(&dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{output}}) ==
				service.ExtractOutputTextFromResponses(&dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{candidate}}) {
			return true
		}
	}
	return false
}

func OaiResponsesToChatStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	responseId := helper.GetResponseID(c)
	createAt := time.Now().Unix()
	model := info.UpstreamModelName

	var (
		usage                = &dto.Usage{}
		outputText           strings.Builder
		usageText            strings.Builder
		sentStart            bool
		sentStop             bool
		sawToolCall          bool
		responseFinishReason = "stop"
		streamErr            *types.NewAPIError
	)

	toolCallIndexByID := make(map[string]int)
	toolCallNameByID := make(map[string]string)
	toolCallArgsByID := make(map[string]string)
	toolCallNameSent := make(map[string]bool)
	toolCallCanonicalIDByItemID := make(map[string]string)
	hasSentReasoningSummary := false
	needsReasoningSummarySeparator := false
	reasoningSummaryTextByKey := make(map[string]string)

	if info.RelayFormat == types.RelayFormatClaude && info.ClaudeConvertInfo == nil {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}

	sendChatChunk := func(chunk *dto.ChatCompletionsStreamResponse) bool {
		if chunk == nil {
			return true
		}
		if publicModelName := info.GetDisplayModelName(); publicModelName != "" {
			chunk.Model = publicModelName
		}
		if info.RelayFormat == types.RelayFormatOpenAI {
			if !info.ChannelSetting.ThinkingToContent {
				normalizeStreamThinkTags(info, chunk)
			}
			if err := helper.ObjectData(c, chunk); err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				return false
			}
			return true
		}

		chunkData, err := common.Marshal(chunk)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			return false
		}
		if err := HandleStreamFormat(c, info, string(chunkData), false, false); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	sendStartIfNeeded := func() bool {
		if sentStart {
			return true
		}
		if !sendChatChunk(helper.GenerateStartEmptyResponse(responseId, createAt, model, nil)) {
			return false
		}
		sentStart = true
		return true
	}

	//sendReasoningDelta := func(delta string) bool {
	//	if delta == "" {
	//		return true
	//	}
	//	if !sendStartIfNeeded() {
	//		return false
	//	}
	//
	//	usageText.WriteString(delta)
	//	chunk := &dto.ChatCompletionsStreamResponse{
	//		Id:      responseId,
	//		Object:  "chat.completion.chunk",
	//		Created: createAt,
	//		Model:   model,
	//		Choices: []dto.ChatCompletionsStreamResponseChoice{
	//			{
	//				Index: 0,
	//				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
	//					ReasoningContent: &delta,
	//				},
	//			},
	//		},
	//	}
	//	if err := helper.ObjectData(c, chunk); err != nil {
	//		streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	//		return false
	//	}
	//	return true
	//}

	sendReasoningSummaryDelta := func(delta string) bool {
		if delta == "" {
			return true
		}
		if needsReasoningSummarySeparator {
			if strings.HasPrefix(delta, "\n\n") {
				needsReasoningSummarySeparator = false
			} else if strings.HasPrefix(delta, "\n") {
				delta = "\n" + delta
				needsReasoningSummarySeparator = false
			} else {
				delta = "\n\n" + delta
				needsReasoningSummarySeparator = false
			}
		}
		if !sendStartIfNeeded() {
			return false
		}

		usageText.WriteString(delta)
		chunk := &dto.ChatCompletionsStreamResponse{
			Id:      responseId,
			Object:  "chat.completion.chunk",
			Created: createAt,
			Model:   model,
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ReasoningContent: &delta,
					},
				},
			},
		}
		if !sendChatChunk(chunk) {
			return false
		}
		hasSentReasoningSummary = true
		return true
	}

	sendReasoningSummarySnapshot := func(itemID string, summaryIndex *int, text string) bool {
		if text == "" {
			return true
		}
		key := responsesStreamIndexKey(strings.TrimSpace(itemID), summaryIndex)
		previous := reasoningSummaryTextByKey[key]
		delta := stringDeltaFromPrefix(previous, text)
		reasoningSummaryTextByKey[key] = text
		return sendReasoningSummaryDelta(delta)
	}

	sendToolCallDelta := func(callID string, name string, argsDelta string) bool {
		if callID == "" {
			return true
		}
		if !sendStartIfNeeded() {
			return false
		}

		idx, ok := toolCallIndexByID[callID]
		if !ok {
			idx = len(toolCallIndexByID)
			toolCallIndexByID[callID] = idx
		}
		if name != "" {
			toolCallNameByID[callID] = name
		}
		if toolCallNameByID[callID] != "" {
			name = toolCallNameByID[callID]
		}

		tool := dto.ToolCallResponse{
			ID:   callID,
			Type: "function",
			Function: dto.FunctionResponse{
				Arguments: argsDelta,
			},
		}
		tool.SetIndex(idx)
		if name != "" && !toolCallNameSent[callID] {
			tool.Function.Name = name
			toolCallNameSent[callID] = true
		}

		chunk := &dto.ChatCompletionsStreamResponse{
			Id:      responseId,
			Object:  "chat.completion.chunk",
			Created: createAt,
			Model:   model,
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{tool},
					},
				},
			},
		}
		if !sendChatChunk(chunk) {
			return false
		}
		sawToolCall = true

		// Include tool call data in the local builder for fallback token estimation.
		if tool.Function.Name != "" {
			usageText.WriteString(tool.Function.Name)
		}
		if argsDelta != "" {
			usageText.WriteString(argsDelta)
		}
		return true
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}

		var streamResp dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResp); err != nil {
			logger.LogError(c, "failed to unmarshal responses stream event: "+err.Error())
			sr.Error(err)
			return
		}

		switch streamResp.Type {
		case "response.created":
			if streamResp.Response != nil {
				if streamResp.Response.Model != "" {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
			}

		//case "response.reasoning_text.delta":
		//if !sendReasoningDelta(streamResp.Delta) {
		//	sr.Stop(streamErr)
		//	return
		//}

		//case "response.reasoning_text.done":

		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			key := responsesStreamIndexKey(strings.TrimSpace(streamResp.ItemID), streamResp.SummaryIndex)
			reasoningSummaryTextByKey[key] += streamResp.Delta
			if !sendReasoningSummaryDelta(streamResp.Delta) {
				sr.Stop(streamErr)
				return
			}

		case "response.reasoning_summary_text.done":
			if !sendReasoningSummarySnapshot(streamResp.ItemID, streamResp.SummaryIndex, streamResp.Text) {
				sr.Stop(streamErr)
				return
			}
			if hasSentReasoningSummary {
				needsReasoningSummarySeparator = true
			}

		case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
			if streamResp.Part != nil && (streamResp.Part.Type == "" || streamResp.Part.Type == "summary_text") {
				if !sendReasoningSummarySnapshot(streamResp.ItemID, streamResp.SummaryIndex, streamResp.Part.Text) {
					sr.Stop(streamErr)
					return
				}
			}

		case "response.output_text.delta":
			if !sendStartIfNeeded() {
				sr.Stop(streamErr)
				return
			}

			if streamResp.Delta != "" {
				outputText.WriteString(streamResp.Delta)
				usageText.WriteString(streamResp.Delta)
				delta := streamResp.Delta
				chunk := &dto.ChatCompletionsStreamResponse{
					Id:      responseId,
					Object:  "chat.completion.chunk",
					Created: createAt,
					Model:   model,
					Choices: []dto.ChatCompletionsStreamResponseChoice{
						{
							Index: 0,
							Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
								Content: &delta,
							},
						},
					},
				}
				if !sendChatChunk(chunk) {
					sr.Stop(streamErr)
					return
				}
			}

		case "response.output_item.added", "response.output_item.done":
			if streamResp.Item == nil {
				break
			}
			if streamResp.Item.Type == "reasoning" {
				summaries := streamResp.Item.Summary
				if len(summaries) == 0 {
					summaries = streamResp.Item.Content
				}
				for index, summary := range summaries {
					if summary.Type != "" && summary.Type != "summary_text" {
						continue
					}
					if !sendReasoningSummarySnapshot(streamResp.Item.ID, &index, summary.Text) {
						sr.Stop(streamErr)
						return
					}
				}
				break
			}
			if streamResp.Item.Type != "function_call" {
				break
			}

			itemID := strings.TrimSpace(streamResp.Item.ID)
			callID := strings.TrimSpace(streamResp.Item.CallId)
			if callID == "" {
				callID = itemID
			}
			if itemID != "" && callID != "" {
				toolCallCanonicalIDByItemID[itemID] = callID
			}
			name := strings.TrimSpace(streamResp.Item.Name)
			if name != "" {
				toolCallNameByID[callID] = name
			}

			newArgs := streamResp.Item.ArgumentsString()
			prevArgs := toolCallArgsByID[callID]
			argsDelta := ""
			if newArgs != "" {
				if strings.HasPrefix(newArgs, prevArgs) {
					argsDelta = newArgs[len(prevArgs):]
				} else {
					argsDelta = newArgs
				}
				toolCallArgsByID[callID] = newArgs
			}

			if !sendToolCallDelta(callID, name, argsDelta) {
				sr.Stop(streamErr)
				return
			}

		case "response.function_call_arguments.delta":
			itemID := strings.TrimSpace(streamResp.ItemID)
			callID := strings.TrimSpace(streamResp.CallID)
			if callID == "" {
				callID = toolCallCanonicalIDByItemID[itemID]
			}
			if callID == "" {
				callID = itemID
			}
			if callID == "" {
				break
			}
			toolCallArgsByID[callID] += streamResp.Delta
			if !sendToolCallDelta(callID, "", streamResp.Delta) {
				sr.Stop(streamErr)
				return
			}

		case "response.function_call_arguments.done":
			// Some providers emit the complete argument string only in the done event.
			itemID := strings.TrimSpace(streamResp.ItemID)
			callID := strings.TrimSpace(streamResp.CallID)
			if callID == "" {
				callID = toolCallCanonicalIDByItemID[itemID]
			}
			if callID == "" {
				callID = itemID
			}
			if callID != "" && len(streamResp.Arguments) > 0 {
				fullArgs := common.JsonRawMessageToString(streamResp.Arguments)
				previous := toolCallArgsByID[callID]
				if strings.HasPrefix(fullArgs, previous) {
					if !sendToolCallDelta(callID, "", fullArgs[len(previous):]) {
						sr.Stop(streamErr)
						return
					}
				} else if fullArgs != previous {
					if !sendToolCallDelta(callID, "", fullArgs) {
						sr.Stop(streamErr)
						return
					}
				}
				toolCallArgsByID[callID] = fullArgs
			}

		case "response.completed", "response.done", "response.incomplete":
			if streamResp.Response != nil {
				if streamResp.Response.Model != "" {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
				if mapped, ok := service.ResponsesFinishReasonFromStatus(streamResp.Response); ok {
					responseFinishReason = mapped
				} else if streamResp.Type == "response.incomplete" {
					responseFinishReason = "length"
				}
				if streamResp.Response.Usage != nil {
					*usage = *service.UsageFromResponsesUsage(streamResp.Response.Usage)
				}
			} else if streamResp.Type == "response.incomplete" {
				responseFinishReason = "length"
			}

			if !sendStartIfNeeded() {
				sr.Stop(streamErr)
				return
			}
			if !sentStop {
				if info.RelayFormat == types.RelayFormatClaude && info.ClaudeConvertInfo != nil {
					info.ClaudeConvertInfo.Usage = usage
				}
				finishReason := responseFinishReason
				if finishReason == "stop" && sawToolCall {
					finishReason = "tool_calls"
				}
				stop := helper.GenerateStopResponse(responseId, createAt, model, finishReason)
				if !sendChatChunk(stop) {
					sr.Stop(streamErr)
					return
				}
				sentStop = true
			}
			sr.Done()

		case "response.error", "response.failed":
			if streamResp.Response != nil {
				if oaiErr := streamResp.Response.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
					streamErr = types.WithOpenAIError(*oaiErr, http.StatusInternalServerError)
					sr.Stop(streamErr)
					return
				}
			}
			streamErr = types.NewOpenAIError(fmt.Errorf("responses stream error: %s", streamResp.Type), types.ErrorCodeBadResponse, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return

		default:
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}

	if usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, usageText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}

	if relaycommon.ShouldRetryZeroOutputUsageAfterStream(info, usage) {
		return nil, relaycommon.NewZeroOutputRetryError(info, usage)
	}

	if !sentStart {
		if !sendChatChunk(helper.GenerateStartEmptyResponse(responseId, createAt, model, nil)) {
			return nil, streamErr
		}
	}
	if !sentStop {
		if info.RelayFormat == types.RelayFormatClaude && info.ClaudeConvertInfo != nil {
			info.ClaudeConvertInfo.Usage = usage
		}
		finishReason := responseFinishReason
		if finishReason == "stop" && sawToolCall {
			finishReason = "tool_calls"
		}
		stop := helper.GenerateStopResponse(responseId, createAt, model, finishReason)
		if !sendChatChunk(stop) {
			return nil, streamErr
		}
	}
	if info.RelayFormat == types.RelayFormatOpenAI && info.ShouldIncludeUsage && usage != nil {
		finalUsageResponse := helper.GenerateFinalUsageResponse(responseId, createAt, model, *usage)
		if publicModelName := info.GetDisplayModelName(); publicModelName != "" {
			finalUsageResponse.Model = publicModelName
		}
		if err := helper.ObjectData(c, finalUsageResponse); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}

	if info.RelayFormat == types.RelayFormatOpenAI {
		helper.Done(c)
	}
	return usage, nil
}
