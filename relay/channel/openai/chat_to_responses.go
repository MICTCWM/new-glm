package openai

import (
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
