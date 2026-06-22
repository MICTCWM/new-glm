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
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	imageReq, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ImageRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(imageReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ImageRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
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
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		passThroughStorage = storage
		requestBody = common.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertImageRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed)
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		switch convertedRequest.(type) {
		case *bytes.Buffer:
			jsonData = convertedRequest.(*bytes.Buffer).Bytes()
			requestBody = convertedRequest.(io.Reader)
		default:
			var marshalErr error
			jsonData, marshalErr = common.Marshal(convertedRequest)
			if marshalErr != nil {
				return types.NewError(marshalErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}

			if len(info.ParamOverride) > 0 {
				jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
				if err != nil {
					return newAPIErrorFromParamOverride(err)
				}
			}

			if common.DebugEnabled {
				logger.LogDebug(c, fmt.Sprintf("image request body: %s", string(jsonData)))
			}
			requestBody = bytes.NewBuffer(jsonData)
		}
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
			info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
			if httpResp.StatusCode != http.StatusOK {
				if httpResp.StatusCode == http.StatusCreated && info.ApiType == constant.APITypeReplicate {
					httpResp.StatusCode = http.StatusOK
				} else {
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
				WaitBeforeRetry(c, info, delay, attempt+1, "Upstream retry")
			}
			continue
		}

		info.UpstreamRetryCount = attempt

		imageN := uint(1)
		if request.N != nil {
			imageN = *request.N
		}

		if info.PriceData.UsePrice {
			if _, hasN := info.PriceData.OtherRatios["n"]; !hasN {
				info.PriceData.AddOtherRatio("n", float64(imageN))
			}
		}

		if usage.(*dto.Usage).TotalTokens == 0 {
			usage.(*dto.Usage).TotalTokens = 1
		}
		if usage.(*dto.Usage).PromptTokens == 0 {
			usage.(*dto.Usage).PromptTokens = 1
		}

		quality := "standard"
		if request.Quality == "hd" {
			quality = "hd"
		}

		var logContent []string

		if len(request.Size) > 0 {
			logContent = append(logContent, fmt.Sprintf("大小 %s", request.Size))
		}
		if len(quality) > 0 {
			logContent = append(logContent, fmt.Sprintf("品质 %s", quality))
		}
		if imageN > 0 {
			logContent = append(logContent, fmt.Sprintf("生成数量 %d", imageN))
		}

		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), logContent)
		return nil
	}

	return lastApiErr
}
