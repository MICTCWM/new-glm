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

	var requestBody io.Reader = bytes.NewBuffer(jsonData)

	upstreamRetryTimes := common.UpstreamRetryTimes
	var httpResp *http.Response
	var lastApiErr *types.NewAPIError

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
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
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

		if info.IsStream {
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

	// 流式响应：在循环外处理，不重试（stream handler 内部已处理零输出检测）
	if info.IsStream {
		usage, napiErr := openaichannel.OaiResponsesToChatStreamHandler(c, info, httpResp)
		if napiErr != nil {
			return nil, napiErr
		}
		return usage, nil
	}

	return nil, lastApiErr
}