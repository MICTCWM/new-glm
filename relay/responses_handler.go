package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// shouldRouteResponsesThroughChat selects the bridge from the request's
// incoming Responses protocol to the selected channel's upstream protocol.
// The channel setting is authoritative; API type is only an implementation
// detail and must not force a Responses request onto a Chat channel.
func shouldRouteResponsesThroughChat(info *relaycommon.RelayInfo) bool {
	if info == nil || info.RelayMode != relayconstant.RelayModeResponses {
		return false
	}
	return !service.ResponsesProtocolRequiredForChannel(info.ChannelSetting, info.ChannelType)
}

func isUnimplementedAdaptorError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not implemented") || strings.Contains(message, "is not implemented")
}

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	info.ApplyFallbackReasoningToResponsesRequest(request)
	info.SyncReasoningEffortFromResponsesRequest(request)

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	if !info.IsStream && info.BillingSource == service.BillingSourceGptWallet {
		service.EnableDeferredResponse(c)
	}
	adaptor.Init(info)

	// 强制系统提示词拼接：在 adaptor 调用前追加，确保强制提示词始终在最前面
	// 统一由 handler 层处理，adaptor 不再重复处理
	if err := ApplyForceSystemPromptToInstructions(request, info.OriginModelName); err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	// A Responses request must follow the selected channel's configured
	// protocol. Chat channels receive a Chat request and their normalized Chat
	// response is converted back to Responses for the caller. Do not infer the
	// upstream protocol from the adaptor/API type: the same model can be served
	// by different channel adaptors.
	if shouldRouteResponsesThroughChat(info) {
		usage, newApiErr := responsesViaChatCompletions(c, info, adaptor, request)
		if newApiErr != nil {
			return newApiErr
		}
		service.PostTextConsumeQuota(c, info, usage, nil)
		return nil
	}

	var requestBody io.Reader
	var jsonData []byte
	var passThroughStorage io.ReadSeeker
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		passThroughStorage = storage
		requestBody = common.ReaderOnly(storage)
		if body, bodyErr := storage.Bytes(); bodyErr == nil {
			mappedBody, mappingErr := helper.ApplyModelMappingToRawJSON(body, info, true)
			if mappingErr != nil {
				return types.NewError(mappingErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			patched, patchErr := ensureOpenAIPromptCacheKey(c, info, mappedBody, responsesPromptCacheKeyString(request.PromptCacheKey))
			if patchErr != nil {
				return types.NewError(patchErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			if !bytes.Equal(patched, body) {
				jsonData = patched
				requestBody = bytes.NewBuffer(jsonData)
				passThroughStorage = nil
				info.UpstreamRequestBody = jsonData
			}
		}
		// 捕获转换后请求体（数据点2，透传模式下等于用户原始请求）
		if len(info.UpstreamRequestBody) == 0 {
			if b, e := storage.Bytes(); e == nil {
				info.UpstreamRequestBody = b
			}
		}
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			// A channel that is configured as Chat must already be handled by the
			// bridge above. For legacy channel records that still say Responses,
			// only fall back when the adaptor explicitly reports that its Responses
			// converter is absent. Other conversion errors remain hard errors so a
			// malformed request is never silently changed into another protocol.
			if isUnimplementedAdaptorError(err) {
				usage, bridgeErr := responsesViaChatCompletions(c, info, adaptor, request)
				if bridgeErr != nil {
					return bridgeErr
				}
				service.PostTextConsumeQuota(c, info, usage, nil)
				return nil
			}
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err = common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}
		switch convertedRequest.(type) {
		case *dto.OpenAIResponsesRequest, dto.OpenAIResponsesRequest:
			jsonData, err = helper.ApplyModelMappingToRawJSON(jsonData, info, true)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
		}
		jsonData, err = ensureOpenAIPromptCacheKey(c, info, jsonData, responsesPromptCacheKeyString(request.PromptCacheKey))
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
		// 捕获转换后请求体（数据点2）
		info.UpstreamRequestBody = jsonData
		requestBody = bytes.NewBuffer(jsonData)
	}

	upstreamRetryTimes := common.UpstreamRetryTimes
	if c.GetBool("fallback_triggered") {
		upstreamRetryTimes = 0
	}
	var httpResp *http.Response
	var lastApiErr *types.NewAPIError
	var upstreamBuf *bytes.Buffer

	statusCodeMappingStr := c.GetString("status_code_mapping")

	for attempt := 0; attempt <= upstreamRetryTimes; attempt++ {
		var reqBody io.Reader
		if attempt == 0 {
			reqBody = requestBody
		} else {
			if passThroughStorage != nil {
				passThroughStorage.Seek(0, io.SeekStart)
				reqBody = common.ReaderOnly(passThroughStorage)
			} else {
				reqBody = bytes.NewBuffer(jsonData)
			}
		}

		resp, err := adaptor.DoRequest(c, info, reqBody)
		if err != nil {
			lastApiErr = types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
			if !canRetryUpstream(info, lastApiErr, attempt, upstreamRetryTimes) {
				return lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			// Add retry delay before next attempt
			var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}
			if delay > 0 {
				WaitBeforeRetry(c, info, delay, attempt+1, "Upstream retry")
			}
			continue
		}

		if resp != nil {
			// Drain and close previous response body to prevent resource leak during retries
			if httpResp != nil && httpResp.Body != nil {
				io.Copy(io.Discard, httpResp.Body)
				httpResp.Body.Close()
			}
			httpResp = resp.(*http.Response)
			// 包装 Body 以捕获上游返回的原始响应体（数据点3）
			upstreamBuf = &bytes.Buffer{}
			httpResp.Body = &common.CapturingReadCloser{
				Reader: httpResp.Body,
				Closer: httpResp.Body,
				Buf:    upstreamBuf,
			}

			if httpResp.StatusCode != http.StatusOK {
				napiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
				service.ResetStatusCode(napiErr, statusCodeMappingStr)
				lastApiErr = napiErr
				if !canRetryUpstream(info, lastApiErr, attempt, upstreamRetryTimes) {
					return lastApiErr
				}
				info.UpstreamRetryCount = attempt + 1
				// Add retry delay before next attempt
				var delay time.Duration
				if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
					delay = common.RetryDelays[attempt]
				}
				if delay > 0 {
					WaitBeforeRetry(c, info, delay, attempt+1, "Upstream retry")
				}
				continue
			}
		}

		usage, napiErr := adaptor.DoResponse(c, httpResp, info)
		if napiErr != nil {
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			if napiErr.GetErrorCode() == types.ErrorCodeChannelZeroOutputTokens {
				lastApiErr = napiErr
				if !canRetryUpstream(info, lastApiErr, attempt, upstreamRetryTimes) {
					return lastApiErr
				}
				info.UpstreamRetryCount = attempt + 1
				var delay time.Duration
				if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
					delay = common.RetryDelays[attempt]
				}
				if delay > 0 {
					WaitBeforeRetry(c, info, delay, attempt+1, "Zero output retry")
				}
				continue
			}
			lastApiErr = napiErr
			if !canRetryUpstream(info, lastApiErr, attempt, upstreamRetryTimes) {
				return lastApiErr
			}
			info.UpstreamRetryCount = attempt + 1
			// Add retry delay before next attempt
			var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}
			if delay > 0 {
				WaitBeforeRetry(c, info, delay, attempt+1, "Upstream retry")
			}
			continue
		}

		info.UpstreamRetryCount = attempt
		info.ActualApiCallCount = attempt + 1
		// 捕获上游返回的原始响应体（数据点3）
		if upstreamBuf != nil {
			info.UpstreamResponseRaw = upstreamBuf.Bytes()
		}

		usageDto := usage.(*dto.Usage)
		if info.RelayMode == relayconstant.RelayModeResponsesCompact {
			originModelName := info.OriginModelName
			originPriceData := info.PriceData

			_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
			if err != nil {
				info.OriginModelName = originModelName
				info.PriceData = originPriceData
				return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
			}
			service.PostTextConsumeQuota(c, info, usageDto, nil)

			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return nil
		}

		if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
			service.PostAudioConsumeQuota(c, info, usageDto, "")
		} else {
			service.PostTextConsumeQuota(c, info, usageDto, nil)
		}
		return nil
	}

	return lastApiErr
}
