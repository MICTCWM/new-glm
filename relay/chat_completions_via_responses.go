package relay

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func applySystemPromptIfNeeded(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) {
	if info == nil || request == nil {
		return
	}

	// 渠道级 SystemPrompt 拼接
	if info.ChannelSetting.SystemPrompt != "" {
		systemRole := request.GetSystemRoleName()

		containSystemPrompt := false
		for _, message := range request.Messages {
			if message.Role == systemRole {
				containSystemPrompt = true
				break
			}
		}
		if !containSystemPrompt {
			systemMessage := dto.Message{
				Role:    systemRole,
				Content: info.ChannelSetting.SystemPrompt,
			}
			request.Messages = append([]dto.Message{systemMessage}, request.Messages...)
		} else if info.ChannelSetting.SystemPromptOverride {
			common.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
			for i, message := range request.Messages {
				if message.Role != systemRole {
					continue
				}
				if message.IsStringContent() {
					request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + message.StringContent())
				} else {
					contents := message.ParseContent()
					contents = append([]dto.MediaContent{
						{
							Type: dto.ContentTypeText,
							Text: info.ChannelSetting.SystemPrompt,
						},
					}, contents...)
					request.Messages[i].Content = contents
				}
				break
			}
		}
	}

	// 强制系统提示词拼接由 chatCompletionsViaResponses 在转换成 Responses 请求后统一处理
	// （ApplyForceSystemPromptToInstructions），避免 Messages 与 Instructions 重复拼接。
}

func chatCompletionsViaResponses(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.GeneralOpenAIRequest) (*dto.Usage, *types.NewAPIError) {
	info.ApplyFallbackReasoningToOpenAIRequest(request)
	chatJSON, err := common.Marshal(request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	chatJSON, err = relaycommon.RemoveDisabledFields(chatJSON, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		chatJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(chatJSON, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	var overriddenChatReq dto.GeneralOpenAIRequest
	if err := common.Unmarshal(chatJSON, &overriddenChatReq); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
	}

	responsesReq, err := service.ChatCompletionsRequestToResponsesRequest(&overriddenChatReq)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	// 强制系统提示词拼接：转换成 Responses 请求后统一处理 Instructions
	if err := ApplyForceSystemPromptToInstructions(responsesReq, info.OriginModelName); err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	info.AppendRequestConversion(types.RelayFormatOpenAIResponses)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *responsesReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	// Capture the final Responses payload, not the intermediate Chat payload.
	// This is especially important during fallback: it makes it possible to
	// verify that an Anthropic request was actually sent to the RE endpoint.
	info.UpstreamRequestBody = jsonData

	var requestBody io.Reader = bytes.NewBuffer(jsonData)

	upstreamRetryTimes := common.UpstreamRetryTimes
	if c.GetBool("fallback_triggered") {
		upstreamRetryTimes = 0
	}
	var httpResp *http.Response
	var lastApiErr *types.NewAPIError
	clientStream := info.IsStream
	upstreamStream := false
	savedIsStream := info.IsStream
	defer func() {
		info.IsStream = savedIsStream
	}()

	statusCodeMappingStr := c.GetString("status_code_mapping")

	for attempt := 0; attempt <= upstreamRetryTimes; attempt++ {
		var reqBody io.Reader
		if attempt == 0 {
			reqBody = requestBody
		} else {
			reqBody = bytes.NewBuffer(jsonData)
		}

		resp, err := adaptor.DoRequest(c, info, reqBody)
		if err != nil {
			lastApiErr = types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
			if attempt >= upstreamRetryTimes {
				return nil, lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			ApplyRetryDelay(c, info, attempt, "Upstream retry")
			continue
		}
		if resp == nil {
			lastApiErr = types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			if attempt >= upstreamRetryTimes {
				return nil, lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			ApplyRetryDelay(c, info, attempt, "Upstream retry")
			continue
		}

		// Drain and close previous response body to prevent resource leak during retries
		if httpResp != nil && httpResp.Body != nil {
			_, _ = io.Copy(io.Discard, httpResp.Body)
			httpResp.Body.Close()
		}
		httpResp = resp.(*http.Response)
		upstreamStream = isResponsesEventStreamContentType(httpResp.Header.Get("Content-Type"))
		info.IsStream = clientStream || upstreamStream
		if httpResp.StatusCode != http.StatusOK {
			napiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			lastApiErr = napiErr
			if attempt >= upstreamRetryTimes {
				return nil, lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			ApplyRetryDelay(c, info, attempt, "Upstream retry")
			continue
		}

		if upstreamStream {
			info.ActualApiCallCount = attempt + 1
			break
		}

		usage, napiErr := openaichannel.OaiResponsesToChatHandler(c, info, httpResp)
		if napiErr != nil {
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			if napiErr.GetErrorCode() == types.ErrorCodeChannelZeroOutputTokens {
				lastApiErr = napiErr
				if attempt >= upstreamRetryTimes {
					return nil, lastApiErr
				}
				info.UpstreamRetryCount = attempt + 1
				ApplyRetryDelay(c, info, attempt, "Zero output retry")
				continue
			}
			lastApiErr = napiErr
			if attempt >= upstreamRetryTimes {
				return nil, lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			ApplyRetryDelay(c, info, attempt, "Upstream retry")
			continue
		}

		info.UpstreamRetryCount = attempt
		info.ActualApiCallCount = attempt + 1
		return usage, nil
	}

	// Responses SSE must always be converted before it reaches a Chat client.
	// Keep the client and upstream stream flags separate: an upstream may stream
	// even when the client requested a buffered response, and vice versa.
	if upstreamStream {
		if !clientStream {
			return openaichannel.OaiResponsesToChatBufferedStreamHandler(c, info, httpResp)
		}
		usage, napiErr := openaichannel.OaiResponsesToChatStreamHandler(c, info, httpResp)
		if napiErr != nil {
			return nil, napiErr
		}
		return usage, nil
	}
	if clientStream && httpResp != nil {
		// Some gateways ignore stream=true and return one JSON Responses object.
		// Convert it to the requested Chat response instead of returning no body.
		return openaichannel.OaiResponsesToChatHandler(c, info, httpResp)
	}

	return nil, lastApiErr
}

func isResponsesEventStreamContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}
