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

// responsesViaChatCompletions sends a Responses request to a channel whose
// configured upstream protocol is Chat Completions, then converts the Chat
// response back to Responses for the caller.
func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatRequest, err := service.ResponsesRequestToChatCompletionsRequest(request)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	info.ApplyFallbackReasoningToOpenAIRequest(chatRequest)
	applySystemPromptIfNeeded(c, info, chatRequest)
	info.AppendRequestConversion(types.RelayFormatOpenAI)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, chatRequest)
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
	if !info.NativeMode && len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}
	info.UpstreamRequestBody = jsonData

	requestBody := bytes.NewBuffer(jsonData)
	upstreamRetryTimes := common.UpstreamRetryTimes
	if c.GetBool("fallback_triggered") {
		upstreamRetryTimes = 0
	}
	if info.NativeMode {
		upstreamRetryTimes = 0
	}

	var lastApiErr *types.NewAPIError
	var httpResp *http.Response
	statusCodeMappingStr := c.GetString("status_code_mapping")
	for attempt := 0; attempt <= upstreamRetryTimes; attempt++ {
		var reqBody io.Reader = bytes.NewBuffer(jsonData)
		if attempt == 0 {
			reqBody = requestBody
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

		if httpResp != nil && httpResp.Body != nil {
			_, _ = io.Copy(io.Discard, httpResp.Body)
			httpResp.Body.Close()
		}
		httpResp = resp.(*http.Response)
		upstreamBuf := &bytes.Buffer{}
		httpResp.Body = &common.CapturingReadCloser{
			Reader: httpResp.Body,
			Closer: httpResp.Body,
			Buf:    upstreamBuf,
		}
		info.IsStream = info.IsStream || isEventStream(httpResp)
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

		var usage *dto.Usage
		var napiErr *types.NewAPIError
		if isOpenAICompatibleAPIType(info.ApiType) {
			if info.IsStream {
				usage, napiErr = openaichannel.ChatCompletionsToResponsesStreamHandler(c, info, httpResp)
			} else {
				usage, napiErr = openaichannel.ChatCompletionsToResponsesHandler(c, info, httpResp)
			}
		} else {
			// Native adaptors may return a provider-specific response even though
			// their request protocol is Chat. Normalize that response through the
			// adaptor first, then reuse the standard Chat -> Responses converter.
			usage, napiErr = normalizeAdaptorChatResponseToResponses(c, info, adaptor, httpResp)
		}
		if napiErr != nil {
			service.ResetStatusCode(napiErr, statusCodeMappingStr)
			if info.IsStream || napiErr.GetErrorCode() == types.ErrorCodeChannelZeroOutputTokens && attempt >= upstreamRetryTimes {
				return nil, napiErr
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
		if upstreamBuf != nil {
			info.UpstreamResponseRaw = upstreamBuf.Bytes()
		}
		return usage, nil
	}

	return nil, lastApiErr
}

// normalizeAdaptorChatResponseToResponses lets a native channel adaptor
// translate its provider response into the normal Chat Completions response
// before the shared Chat -> Responses conversion runs. This keeps protocol
// selection independent from the provider-specific API type.
func normalizeAdaptorChatResponseToResponses(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, upstreamResp *http.Response) (*dto.Usage, *types.NewAPIError) {
	originalWriter := c.Writer
	bufferWriter := &chatResponseBufferWriter{ResponseWriter: originalWriter}
	c.Writer = bufferWriter

	originalRelayFormat := info.RelayFormat
	info.RelayFormat = types.RelayFormatOpenAI
	_, adaptorErr := adaptor.DoResponse(c, upstreamResp, info)
	info.RelayFormat = originalRelayFormat
	c.Writer = originalWriter
	if adaptorErr != nil {
		return nil, adaptorErr
	}

	statusCode := bufferWriter.status
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	normalizedResp := &http.Response{
		StatusCode: statusCode,
		Header:     bufferWriter.Header().Clone(),
		Body:       io.NopCloser(bytes.NewReader(bufferWriter.buf.Bytes())),
	}
	if info.IsStream {
		return openaichannel.ChatCompletionsToResponsesStreamHandler(c, info, normalizedResp)
	}
	return openaichannel.ChatCompletionsToResponsesHandler(c, info, normalizedResp)
}

// chatResponseBufferWriter captures the Chat response emitted by a native
// adaptor without sending it to the user before it can be converted to RE.
type chatResponseBufferWriter struct {
	gin.ResponseWriter
	buf     bytes.Buffer
	status  int
	written bool
}

func (w *chatResponseBufferWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
	w.written = true
}

func (w *chatResponseBufferWriter) WriteHeaderNow() {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
}

func (w *chatResponseBufferWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	return w.buf.Write(data)
}

func (w *chatResponseBufferWriter) WriteString(data string) (int, error) {
	w.WriteHeaderNow()
	return w.buf.WriteString(data)
}

func (w *chatResponseBufferWriter) Status() int {
	return w.status
}

func (w *chatResponseBufferWriter) Written() bool {
	return w.written
}

func (w *chatResponseBufferWriter) Size() int {
	return w.buf.Len()
}

func (w *chatResponseBufferWriter) Flush() {
	w.WriteHeaderNow()
}

func isEventStream(resp *http.Response) bool {
	return resp != nil && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
}

func isOpenAICompatibleAPIType(apiType int) bool {
	switch apiType {
	case constant.APITypeOpenAI, constant.APITypeOpenRouter, constant.APITypeXinference, constant.APITypeXai:
		return true
	default:
		return false
	}
}
