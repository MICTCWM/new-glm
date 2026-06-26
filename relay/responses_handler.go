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

	// 强制系统提示词拼接：在 adaptor 调用前追加，确保强制提示词始终在最前面
	// 统一由 handler 层处理，adaptor 不再重复处理
	if err := ApplyForceSystemPromptToInstructions(request); err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
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
		// 捕获转换后请求体（数据点2，透传模式下等于用户原始请求）
		if b, e := storage.Bytes(); e == nil {
			info.UpstreamRequestBody = b
		}
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
		// 捕获转换后请求体（数据点2）
		info.UpstreamRequestBody = jsonData
		requestBody = bytes.NewBuffer(jsonData)
	}

	upstreamRetryTimes := common.UpstreamRetryTimes
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
				if attempt >= upstreamRetryTimes {
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
				WaitBeforeRetry(c, info, delay, attempt+1, "Upstream retry")
			}
			continue
		}

		info.UpstreamRetryCount = attempt
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
