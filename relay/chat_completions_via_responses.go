package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
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
	if info.ChannelSetting.SystemPrompt == "" {
		return
	}

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
		return
	}

	if !info.ChannelSetting.SystemPromptOverride {
		return
	}

	common.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
	for i, message := range request.Messages {
		if message.Role != systemRole {
			continue
		}
		if message.IsStringContent() {
			request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + message.StringContent())
			return
		}
		contents := message.ParseContent()
		contents = append([]dto.MediaContent{
			{
				Type: dto.ContentTypeText,
				Text: info.ChannelSetting.SystemPrompt,
			},
		}, contents...)
		request.Messages[i].Content = contents
		return
	}
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
			// Add retry delay before next attempt
			var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}
			if delay > 0 {
				logger.LogInfo(c, fmt.Sprintf("Upstream retry #%d: waiting %v before next attempt", attempt+1, delay))
				time.Sleep(delay)
			}
			continue
		}
		if resp == nil {
			lastApiErr = types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			if attempt >= upstreamRetryTimes {
				return nil, lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			// Add retry delay before next attempt
			var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}
			if delay > 0 {
				logger.LogInfo(c, fmt.Sprintf("Upstream retry #%d: waiting %v before next attempt", attempt+1, delay))
				time.Sleep(delay)
			}
			continue
		}

		// Drain and close previous response body to prevent resource leak during retries
		if httpResp != nil && httpResp.Body != nil {
			io.Copy(io.Discard, httpResp.Body)
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
			// Add retry delay before next attempt
			var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}
			if delay > 0 {
				logger.LogInfo(c, fmt.Sprintf("Upstream retry #%d: waiting %v before next attempt", attempt+1, delay))
				time.Sleep(delay)
			}
			continue
		}

		if info.IsStream {
			break
		}

		usage, napiErr := openaichannel.OaiResponsesToChatHandler(c, info, httpResp)
		if napiErr != nil {
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			if napiErr.GetErrorCode() == types.ErrorCodeChannelZeroOutputTokens {
				return nil, napiErr
			}
			lastApiErr = napiErr
			if attempt >= upstreamRetryTimes {
				return nil, lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			// Add retry delay before next attempt
			var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}
			if delay > 0 {
				logger.LogInfo(c, fmt.Sprintf("Upstream retry #%d: waiting %v before next attempt", attempt+1, delay))
				time.Sleep(delay)
			}
			continue
		}

		info.UpstreamRetryCount = attempt
		return usage, nil
	}

	if info.IsStream {
		usage, napiErr := openaichannel.OaiResponsesToChatStreamHandler(c, info, httpResp)
		if napiErr != nil {
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			return nil, napiErr
		}
		return usage, nil
	}

	return nil, lastApiErr
}
