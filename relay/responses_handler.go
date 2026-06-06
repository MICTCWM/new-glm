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
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

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

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
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
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
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

		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
		requestBody = bytes.NewBuffer(jsonData)
	}

	upstreamRetryTimes := common.UpstreamRetryTimes
	var httpResp *http.Response
	var lastApiErr *types.NewAPIError

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
			if attempt >= upstreamRetryTimes {
				return lastApiErr
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

		if resp != nil {
			httpResp = resp.(*http.Response)

			if httpResp.StatusCode != http.StatusOK {
				napiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
				service.ResetStatusCode(napiErr, statusCodeMappingStr)
				lastApiErr = napiErr
				if attempt >= upstreamRetryTimes {
					return lastApiErr
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
		}

		usage, napiErr := adaptor.DoResponse(c, httpResp, info)
		if napiErr != nil {
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			lastApiErr = napiErr
			if attempt >= upstreamRetryTimes {
				return lastApiErr
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
