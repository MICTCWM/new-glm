package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func AudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	audioReq, ok := info.Request.(*dto.AudioRequest)
	if !ok {
		return types.NewError(errors.New("invalid request type"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(audioReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to AudioRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
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

	ioReader, err := adaptor.ConvertAudioRequest(c, info, *request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	upstreamRetryTimes := common.UpstreamRetryTimes
	var httpResp *http.Response
	var lastApiErr *types.NewAPIError

	statusCodeMappingStr := c.GetString("status_code_mapping")

	for attempt := 0; attempt <= upstreamRetryTimes; attempt++ {
		var reqReader io.Reader
		if attempt == 0 {
			reqReader = ioReader
		} else {
			var cErr error
			reqReader, cErr = adaptor.ConvertAudioRequest(c, info, *request)
			if cErr != nil {
				lastApiErr = types.NewError(cErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
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

		resp, err := adaptor.DoRequest(c, info, reqReader)
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

		if usage.(*dto.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*dto.Usage).PromptTokensDetails.AudioTokens > 0 {
			service.PostAudioConsumeQuota(c, info, usage.(*dto.Usage), "")
		} else {
			service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
		}

		return nil
	}

	return lastApiErr
}
