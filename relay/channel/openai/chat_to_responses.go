package openai

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openrouter"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func chatChunkHasFinishReason(chunk *dto.ChatCompletionsStreamResponse) bool {
	if chunk == nil {
		return false
	}
	for _, choice := range chunk.Choices {
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			return true
		}
	}
	return false
}

// ChatCompletionsToResponsesHandler adapts a Chat Completions response from
// an upstream Chat channel to the Responses format expected by the caller.
func ChatCompletionsToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		if info.ChannelType == constant.ChannelTypeOpenRouter && info.ChannelOtherSettings.IsOpenRouterEnterprise() {
			var enterpriseResponse openrouter.OpenRouterEnterpriseResponse
			if enterpriseErr := common.Unmarshal(body, &enterpriseResponse); enterpriseErr == nil && enterpriseResponse.Success {
				body = enterpriseResponse.Data
				err = common.Unmarshal(body, &chatResp)
			}
		}
	}
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responseID := helper.GetResponseID(c)
	responsesResp, usage, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp, responseID)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if usage == nil || usage.TotalTokens == 0 {
		text := ""
		if len(chatResp.Choices) > 0 {
			text = chatResp.Choices[0].Message.StringContent()
		}
		usage = service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
		responsesResp.Usage = usage
	}
	if relaycommon.ShouldRetryZeroOutputUsage(info, usage) {
		return nil, relaycommon.NewZeroOutputRetryError(info, usage)
	}

	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	responseBody = relaycommon.OverrideResponseModel(responseBody, info)
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

// ChatCompletionsToResponsesStreamFromJSONHandler converts a non-streaming
// Chat response into a small, valid Responses SSE sequence. This covers
// gateways that ignore stream=true without exposing a JSON body to an SSE
// client.
func ChatCompletionsToResponsesStreamFromJSONHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responsesResp, usage, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp, helper.GetResponseID(c))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if usage == nil || usage.TotalTokens == 0 {
		text := ""
		if len(chatResp.Choices) > 0 {
			text = chatResp.Choices[0].Message.StringContent()
		}
		usage = service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
		responsesResp.Usage = usage
	}
	if relaycommon.ShouldRetryZeroOutputUsage(info, usage) {
		return nil, relaycommon.NewZeroOutputRetryError(info, usage)
	}

	helper.SetEventStreamHeaders(c)
	send := func(event dto.ResponsesStreamResponse) bool {
		data, marshalErr := common.Marshal(event)
		if marshalErr != nil {
			return false
		}
		data = []byte(relaycommon.OverrideStreamChunkModel(string(data), info))
		helper.ResponseChunkData(c, event, string(data))
		return true
	}

	inProgress := *responsesResp
	inProgress.Status = []byte(`"in_progress"`)
	inProgress.Output = []dto.ResponsesOutput{}
	inProgress.Usage = nil
	if !send(dto.ResponsesStreamResponse{Type: "response.created", Response: &inProgress}) {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to write Responses stream"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	for outputIndex, output := range responsesResp.Output {
		index := outputIndex
		if !send(dto.ResponsesStreamResponse{
			Type:        "response.output_item.added",
			OutputIndex: &index,
			Item:        &output,
		}) {
			return nil, types.NewOpenAIError(fmt.Errorf("failed to write Responses stream"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		contents := output.Content
		eventType := "response.output_text.delta"
		if output.Type == "reasoning" {
			contents = output.Summary
			eventType = "response.reasoning_summary_text.delta"
		}
		for contentIndex, content := range contents {
			if content.Text == "" {
				continue
			}
			contentIdx := contentIndex
			if !send(dto.ResponsesStreamResponse{
				Type:         eventType,
				Delta:        content.Text,
				OutputIndex:  &index,
				ContentIndex: &contentIdx,
				ItemID:       output.ID,
			}) {
				return nil, types.NewOpenAIError(fmt.Errorf("failed to write Responses stream"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
			}
		}
		if !send(dto.ResponsesStreamResponse{
			Type:        "response.output_item.done",
			OutputIndex: &index,
			Item:        &output,
		}) {
			return nil, types.NewOpenAIError(fmt.Errorf("failed to write Responses stream"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}
	if !send(dto.ResponsesStreamResponse{Type: "response.completed", Response: responsesResp}) {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to write Responses stream"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	return usage, nil
}

// ChatCompletionsToResponsesBufferedStreamHandler consumes an upstream Chat
// Completions SSE response and returns one Responses JSON object. A channel
// can ignore stream=false, so the downstream format must be selected from the
// client's request rather than from the upstream Content-Type.
func ChatCompletionsToResponsesBufferedStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	state := service.NewChatToResponsesStreamState(helper.GetResponseID(c), info.UpstreamModelName)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), helper.DefaultMaxScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if _, err := service.ChatCompletionsStreamChunkToResponsesEvents(&chunk, state); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if chatChunkHasFinishReason(&chunk) {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	usage := state.Usage
	if usage == nil {
		usage = &dto.Usage{}
	}
	if usage.CompletionTokens == 0 {
		text := state.UsageText()
		if strings.TrimSpace(text) != "" {
			estimated := service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
			usage.CompletionTokens = estimated.CompletionTokens
		}
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	state.Usage = usage
	if relaycommon.ShouldRetryZeroOutputUsage(info, usage) {
		return nil, relaycommon.NewZeroOutputRetryError(info, usage)
	}

	var responsesResp *dto.OpenAIResponsesResponse
	for _, event := range service.FinalizeChatCompletionsStreamToResponses(state) {
		if event.Payload.Response != nil {
			responsesResp = event.Payload.Response
		}
	}
	if responsesResp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream stream contained no completed response"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	responsesResp.Usage = usage
	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	responseBody = relaycommon.OverrideResponseModel(responseBody, info)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Del("Cache-Control")
	resp.Header.Del("Connection")
	resp.Header.Del("Transfer-Encoding")
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

// ChatCompletionsToResponsesStreamHandler adapts Chat Completions SSE events
// to Responses SSE events while preserving text, reasoning, tool calls, and
// usage information.
func ChatCompletionsToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	state := service.NewChatToResponsesStreamState(helper.GetResponseID(c), info.UpstreamModelName)
	var streamErr *types.NewAPIError
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		events, err := service.ChatCompletionsStreamChunkToResponsesEvents(&chunk, state)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		for _, event := range events {
			data, err := common.Marshal(event.Payload)
			if err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
				sr.Stop(streamErr)
				return
			}
			data = []byte(relaycommon.OverrideStreamChunkModel(string(data), info))
			helper.ResponseChunkData(c, event.Payload, string(data))
		}
		if chatChunkHasFinishReason(&chunk) {
			sr.Done()
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}
	usage := state.Usage
	if usage == nil {
		usage = &dto.Usage{}
	}
	if usage.CompletionTokens == 0 {
		text := state.UsageText()
		if strings.TrimSpace(text) != "" {
			estimated := service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
			usage.CompletionTokens = estimated.CompletionTokens
		}
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	state.Usage = usage

	if relaycommon.ShouldRetryZeroOutputUsageAfterStream(info, usage) {
		return nil, relaycommon.NewZeroOutputRetryError(info, usage)
	}
	for _, event := range service.FinalizeChatCompletionsStreamToResponses(state) {
		data, err := common.Marshal(event.Payload)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
		data = []byte(relaycommon.OverrideStreamChunkModel(string(data), info))
		helper.ResponseChunkData(c, event.Payload, string(data))
	}
	return usage, nil
}
