package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {

	info.InitChannelMeta(c)

	claudeReq, ok := info.Request.(*dto.ClaudeRequest)

	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.ClaudeRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(claudeReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ClaudeRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	info.ApplyFallbackReasoningToClaudeRequest(request)

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	if request.MaxTokens == nil || *request.MaxTokens == 0 {
		defaultMaxTokens := uint(model_setting.GetClaudeSettings().GetDefaultMaxTokens(request.Model))
		request.MaxTokens = &defaultMaxTokens
	}

	if baseModel, effortLevel, ok := reasoning.TrimEffortSuffix(request.Model); ok && effortLevel != "" &&
		(strings.HasPrefix(request.Model, "claude-opus-4-6") || strings.HasPrefix(request.Model, "claude-opus-4-7")) {
		request.Model = baseModel
		request.Thinking = &dto.Thinking{
			Type: "adaptive",
		}
		request.OutputConfig = json.RawMessage(fmt.Sprintf(`{"effort":"%s"}`, effortLevel))
		if strings.HasPrefix(request.Model, "claude-opus-4-7") {
			// Opus 4.7 rejects non-default temperature/top_p/top_k with 400
			// and defaults display to "omitted"; restore the 4.6 visible summary.
			request.Thinking.Display = "summarized"
			request.Temperature = nil
			request.TopP = nil
			request.TopK = nil
		} else {
			request.Temperature = common.GetPointer[float64](1.0)
		}
		info.UpstreamModelName = request.Model
	} else if model_setting.GetClaudeSettings().ThinkingAdapterEnabled &&
		strings.HasSuffix(request.Model, "-thinking") {
		if request.Thinking == nil {
			baseModel := strings.TrimSuffix(request.Model, "-thinking")
			if strings.HasPrefix(baseModel, "claude-opus-4-7") {
				// Opus 4.7 rejects thinking.type="enabled"; use adaptive at high effort.
				request.Thinking = &dto.Thinking{Type: "adaptive", Display: "summarized"}
				request.OutputConfig = json.RawMessage(`{"effort":"high"}`)
				request.Temperature = nil
				request.TopP = nil
				request.TopK = nil
			} else {
				// 因为BudgetTokens 必须大于1024
				if request.MaxTokens == nil || *request.MaxTokens < 1280 {
					request.MaxTokens = common.GetPointer[uint](1280)
				}

				// BudgetTokens 为 max_tokens 的 80%
				request.Thinking = &dto.Thinking{
					Type:         "enabled",
					BudgetTokens: common.GetPointer[int](int(float64(*request.MaxTokens) * model_setting.GetClaudeSettings().ThinkingAdapterBudgetTokensPercentage)),
				}
				// TODO: 临时处理
				// https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#important-considerations-when-using-extended-thinking
				request.Temperature = common.GetPointer[float64](1.0)
			}
		}
		if !model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
			request.Model = strings.TrimSuffix(request.Model, "-thinking")
		}
		info.UpstreamModelName = request.Model
	}

	// countCacheControls 统计请求中已有的 cache_control 断点数量（system + messages + tools 合计），
	// Anthropic 限制最多 4 个，超出会直接 400。
	countCacheControls := func() int {
		n := 0
		if request.System != nil {
			if !request.IsStringSystem() {
				for _, m := range request.ParseSystem() {
					if len(m.CacheControl) > 0 {
						n++
					}
				}
			}
		}
		for _, msg := range request.Messages {
			if msg.Content == nil {
				continue
			}
			if msg.IsStringContent() {
				continue
			}
			parts, _ := msg.ParseContent()
			for _, p := range parts {
				if len(p.CacheControl) > 0 {
					n++
				}
			}
		}
		return n
	}

	// appendSystemSuffix 将注入的系统提示词追加到 system 末尾，而不是前置，
	// 以最大程度保留客户端已设置的缓存前缀（cache_control 断点）。
	// 在末尾注入块上始终打上 cache_control 断点（受 4 断点上限约束），使这段稳定的注入前缀也能被上游缓存。
	// 当已用满 4 个断点或 ParseSystem 失败（无法保留原结构）时，仅追加纯文本而不打断点，保证请求合法且不覆盖客户端内容。
	appendSystemSuffix := func(prefix string) {
		if prefix == "" {
			return
		}
		makeBlock := func(text string, withCache bool) dto.ClaudeMediaMessage {
			block := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
			block.SetText(text)
			if withCache && countCacheControls() < 4 {
				block.CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
			}
			return block
		}
		if request.System == nil {
			// 无客户端 system：直接构造带断点的结构化数组
			request.System = []dto.ClaudeMediaMessage{makeBlock(prefix, true)}
			return
		}
		if request.IsStringSystem() {
			existing := strings.TrimSpace(request.GetStringSystem())
			combined := prefix
			if existing != "" {
				combined = existing + "\n" + prefix
			}
			// 字符串 system 无法携带 cache_control，转为结构化数组以启用缓存
			request.System = []dto.ClaudeMediaMessage{makeBlock(combined, true)}
			return
		}
		systemContents := request.ParseSystem()
		if len(systemContents) == 0 {
			// ParseSystem 解析失败（非法 system 内容）：保留原字节（转为字符串拼接），不覆盖客户端 system
			existing := ""
			if s, ok := request.System.(string); ok {
				existing = s
			}
			combined := prefix
			if strings.TrimSpace(existing) != "" {
				combined = strings.TrimSpace(existing) + "\n" + prefix
			}
			request.SetStringSystem(combined)
			return
		}
		request.System = append(systemContents, makeBlock(prefix, true))
	}

	if info.ChannelSetting.SystemPrompt != "" {
		if request.System == nil {
			// nil 时构造带断点的结构化数组，使渠道提示词也能被上游缓存
			appendSystemSuffix(info.ChannelSetting.SystemPrompt)
		} else if info.ChannelSetting.SystemPromptOverride {
			common.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
			// 覆盖模式：用渠道提示词替换客户端前缀（语义为覆盖，缓存由新前缀决定）
			if request.IsStringSystem() {
				existing := strings.TrimSpace(request.GetStringSystem())
				combined := info.ChannelSetting.SystemPrompt
				if existing != "" {
					combined = info.ChannelSetting.SystemPrompt + "\n" + existing
				}
			// 覆盖分支同样转为带断点的结构化数组，使 system 段可被缓存（避免纯字符串永不缓存）
			newOverrideSys := dto.ClaudeMediaMessage{
				Type:         dto.ContentTypeText,
				CacheControl: json.RawMessage(`{"type":"ephemeral"}`),
			}
			newOverrideSys.SetText(combined)
			request.System = []dto.ClaudeMediaMessage{newOverrideSys}
			} else {
				systemContents := request.ParseSystem()
				newSystem := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
				newSystem.SetText(info.ChannelSetting.SystemPrompt)
				// 覆盖模式下该块是最稳定前缀，打上断点以便缓存（受 4 断点上限约束）
				if countCacheControls() < 4 {
					newSystem.CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
				}
				if len(systemContents) == 0 {
					request.System = []dto.ClaudeMediaMessage{newSystem}
				} else {
					request.System = append([]dto.ClaudeMediaMessage{newSystem}, systemContents...)
				}
			}
		} else {
			// 非覆盖模式：追加到末尾，保留客户端缓存前缀
			appendSystemSuffix(info.ChannelSetting.SystemPrompt)
		}
	}

	// 强制系统提示词拼接：追加到末尾而非最前面，避免破坏客户端已设置的缓存前缀。
	// 强制提示词内容稳定（按模型名固定），因此自身也能被上游缓存。
	// 注意：走 Responses 路径时，强制提示词的注入统一由 chatCompletionsViaResponses 内的
	// ApplyForceSystemPromptToInstructions 处理（见 chat_completions_via_responses.go 注释），
	// 此处若也注入会导致强制提示词在 input item 与 instructions 中重复出现，故跳过。
	shouldUseResponses := service.ShouldChatCompletionsUseResponsesForChannel(info.ChannelSetting, info.ChannelId, info.ChannelType, info.OriginModelName)
	responsesRequired := service.ResponsesProtocolRequiredForChannel(info.ChannelSetting, info.ChannelType)
	// willUseResponsesPath 与下方进入 Responses 分支的条件严格一致，确保
	// 「跳过 forcePrompt 注入」与「走 Responses 路径」互为充要，避免漏注入。
	willUseResponsesPath := shouldUseResponses &&
		(!model_setting.GetGlobalSettings().PassThroughRequestEnabled && !info.ChannelSetting.PassThroughBodyEnabled || responsesRequired)
	forcePrompt := constant.GetForceSystemPrompt(info.OriginModelName)
	if !willUseResponsesPath && forcePrompt != "" {
		appendSystemSuffix(forcePrompt)
	}

	if willUseResponsesPath {
		openAIRequest, convErr := service.ClaudeToOpenAIRequest(*request, info)
		if convErr != nil {
			return types.NewError(convErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		// Keep the intermediate Chat conversion visible in the conversion
		// chain. The actual upstream hop is performed by the shared
		// Chat->Responses helper below; this is intentionally not a direct
		// Claude->Responses conversion.
		relaycommon.AppendRequestConversionFromRequest(info, openAIRequest)

		usage, newApiErr := chatCompletionsViaResponses(c, info, adaptor, openAIRequest)
		if newApiErr != nil {
			return newApiErr
		}

		service.PostTextConsumeQuota(c, info, usage, nil)
		return nil
	}

	var requestBody io.Reader
	var jsonData []byte
	var passThroughStorage io.ReadSeeker
	// OpenAI-compatible channels must normalize Anthropic input to Chat
	// Completions before sending it upstream. Otherwise pass-through would
	// send an Anthropic Messages body to a Chat endpoint. Native Anthropic
	// channels keep their existing pass-through behavior.
	passThroughClaudeRequest := model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled
	if isOpenAICompatibleAPIType(info.ApiType) {
		passThroughClaudeRequest = false
	}
	if passThroughClaudeRequest {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		passThroughStorage = storage
		requestBody = common.ReaderOnly(storage)
		// 捕获转换后请求体（数据点2，透传模式下等于用户原始请求）
		if b, e := storage.Bytes(); e == nil {
			info.UpstreamRequestBody = b
		}
	} else {
		convertedRequest, err := adaptor.ConvertClaudeRequest(c, info, request)
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
		// 捕获转换后请求体（数据点2）
		info.UpstreamRequestBody = jsonData
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
			info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
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

		if info.IsStream {
			info.ActualApiCallCount = attempt + 1
			break
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
		info.ActualApiCallCount = attempt + 1
		// 捕获上游返回的原始响应体（数据点3）
		if upstreamBuf != nil {
			info.UpstreamResponseRaw = upstreamBuf.Bytes()
		}

		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
		return nil
	}

	// 流式响应：在循环外处理，不重试（stream handler 内部已处理零输出检测）
	if info.IsStream {
		usage, napiErr := adaptor.DoResponse(c, httpResp, info)
		if napiErr != nil {
			return napiErr
		}
		// 捕获上游返回的原始响应体（数据点3）
		if upstreamBuf != nil {
			info.UpstreamResponseRaw = upstreamBuf.Bytes()
		}
		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
		return nil
	}

	return lastApiErr
}
