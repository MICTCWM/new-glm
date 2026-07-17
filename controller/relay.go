package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const HiddenUpstreamModelID = "gpt-5.4-mini"

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

// getRetryDelay 根据重试次数返回对应的延迟时间
// retryCount 从 1 开始，表示第几次重试
func getRetryDelay(retryCount int) time.Duration {
	if retryCount <= 0 || retryCount > len(common.RetryDelays) {
		return 0
	}
	return common.RetryDelays[retryCount-1]
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {
	// 请求级别总超时保护：超过阈值自动断开，避免请求被无限拉长
	// Realtime WebSocket 是长连接，不适用此超时
	if common.RequestMaxDuration > 0 && relayFormat != types.RelayFormatOpenAIRealtime {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(common.RequestMaxDuration)*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
	}

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
		relayInfo   *relaycommon.RelayInfo
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", newAPIError.Error()))
			// 使用用户友好的错误消息返回给客户端
			userFriendlyMsg := newAPIError.GetUserFriendlyMessage()
			// 如果错误信息中包含上游内部模型 ID，则替换为用户实际请求的原始模型名
			if relayInfo != nil && relayInfo.OriginModelName != "" {
				userFriendlyMsg = strings.ReplaceAll(userFriendlyMsg, HiddenUpstreamModelID, relayInfo.OriginModelName)
			}
			// 请求总超时触发时返回友好提示（GetUserFriendlyMessage 不特判此错误码，需在此覆盖）
			if errors.Is(newAPIError.Err, context.DeadlineExceeded) {
				userFriendlyMsg = "请求超时，请稍后重试"
			}
			newAPIError.SetMessage(common.MessageWithRequestId(userFriendlyMsg, requestId))

			// 如果已经开始流式输出（HTTP 响应头已发送），将错误信息发送到正文内容
			// 因为此时无法再通过 HTTP 状态码传递错误
			canStreamError := relayInfo != nil && relayInfo.IsStream && c.Writer.Written() && relayFormat != types.RelayFormatOpenAIRealtime
			if canStreamError {
				if !relay.SendErrorNotice(c, relayInfo, userFriendlyMsg) {
					logger.LogWarn(c, "failed to send error notice to stream, client may hang")
				}
				return
			}

			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err = relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	// 初始化 BillingSource：GPT 专有分组使用 GPT 钱包扣费
	// 此处提前设置，确保即使后续路径绕过 PreConsumeBilling（如免费模型），
	// PostConsumeQuota 也能正确识别资金来源扣减 GPT 额度
	if ratio_setting.ContainsGptGroupRatio(relayInfo.UsingGroup) {
		relayInfo.BillingSource = service.BillingSourceGptWallet
	}

	// 流式请求：提前建立 SSE 响应头与状态码，确保排队通知与重试 wait 帧能"立即"到达客户端，
	// 避免浏览器 EventSource 在看到 200 + text/event-stream 之前因 TCP 缓冲而感知"延后"。
	if relayInfo.IsStream && relayFormat != types.RelayFormatOpenAIRealtime {
		helper.SetEventStreamHeaders(c)
		_ = helper.FlushWriter(c)
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	// 视觉路由：检测到图片时自动路由到视觉模型描述图片
	if service.ShouldVisionRoute(relayFormat, relayInfo.OriginModelName, request) {
		// 发送思考提示（流式请求才发送）
		sendVisionRouteNotice(c, relayInfo)
		// 执行视觉路由（调用 Kimi + 替换图片）
		// 如果 Kimi 失败，降级为丢弃图片，仅保留文本
		routedRequest, routeErr := service.ProcessVisionRoute(c, relayInfo, request)
		if routeErr != nil {
			logger.LogWarn(c, "vision route failed, falling back to text-only: "+routeErr.Error())
			// 降级：丢弃图片，仅保留文本，按 glm-5.2 正常计费
			request = service.StripImagesFromRequest(relayFormat, request)
			relayInfo.Request = request
		} else {
			request = routedRequest
			relayInfo.Request = request
			relayInfo.VisionRouteTriggered = true
		}
	}

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// 视觉路由触发时，覆盖预扣费为固定金额
	if relayInfo.VisionRouteTriggered {
		feeQuota := service.CalcVisionRouteFeeQuota(priceData.GroupRatioInfo.GroupRatio)
		priceData.QuotaToPreConsume = feeQuota
		priceData.FreeModel = false
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.GetDisplayModelName()))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:            c,
		TokenGroup:     relayInfo.TokenGroup,
		ModelName:      relayInfo.OriginModelName,
		Retry:          common.GetPointer(0),
		UsedChannelIds: make([]int, 0), // 初始化已使用渠道ID列表
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	// 数据点1：捕获用户原始请求体（GetBodyStorage 内部有缓存，循环内重复调用安全）
	if bodyStorage, bodyErr := common.GetBodyStorage(c); bodyErr == nil {
		if rawBytes, e := bodyStorage.Bytes(); e == nil {
			relayInfo.UserRequestBody = rawBytes
		}
	}

	// 数据点4：包装 c.Writer 以捕获下游响应体（排除 Realtime WebSocket 场景，避免 Hijack 失效）
	if relayFormat != types.RelayFormatOpenAIRealtime {
		downstreamBuf := &bytes.Buffer{}
		origWriter := c.Writer
		c.Writer = &common.CapturingResponseWriter{ResponseWriter: origWriter, Buf: downstreamBuf}
		defer func() {
			relayInfo.DownstreamResponseRaw = downstreamBuf.Bytes()
			c.Writer = origWriter
		}()
	}

	var retryDelays []int
	var selectedChannel *model.Channel // track selected channel for RPM management
	acquiredTrackers := make([]*service.RpmTracker, 0)
	wasQueued := false // track whether request ever entered the RPM queue
	queueDeadline := time.Time{}
	queueNoticeSent := false
	runtimeRpmFull := false
	var firstSpecificError *types.NewAPIError // 第一条非网络层错误（更具体的上游错误），用于最终返回时替代 do_request_failed

	defer func() {
		// Release the RPM slot and wake one queued request when a request completes.
		for _, tracker := range acquiredTrackers {
			tracker.Decrement()
		}
		if selectedChannel != nil || len(acquiredTrackers) > 0 {
			service.GetRpmQueue().NotifyRpmRelease()
		}
	}()

	maxRetryEnabled := common.GetContextKeyBool(c, constant.ContextKeyTokenMaxRetryEnabled)
	maxRetryTimes := common.RetryTimes
	if maxRetryEnabled {
		maxRetryTimes = common.MaxRetryTimes
	}

	for ; retryParam.GetRetry() <= maxRetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			// Check if all channels are RPM-full -> enter queue
			if errors.Is(channelErr.Err, service.ErrAllChannelsRpmFull) || runtimeRpmFull {
				if waitForRpmQueue(c, relayInfo, &queueDeadline, &queueNoticeSent) {
					wasQueued = true
					runtimeRpmFull = false
					retryParam.UsedChannelIds = retryParam.UsedChannelIds[:0]
					retryParam.SetRetry(0)
					retryParam.ResetRetryNextTry()
					continue
				}
				wasQueued = true
				newAPIError = newRpmQueueTimeoutError()
				break
			}
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}
		// 应急预案渠道标记
		if channel != nil && channel.IsEmergencyPlanEnabled() {
			c.Set("emergency_used", true)
			c.Set("emergency_channel_id", channel.Id)
		}
		retryParam.InitialSelectionDone = true

		// GPT 专用渠道走原生模式：跳过重试和参数覆盖
		if channelSetting := channel.GetSetting(); channelSetting.GptModeRequired && relayInfo.UserGptMode {
			relayInfo.NativeMode = true
			maxRetryTimes = 0 // 确保不重试
		}

		// Try to increment RPM counter for the selected channel
		if channel.MaxRPM > 0 {
			tracker := service.GetRpmTracker(channel.Id, channel.MaxRPM)
			if !tracker.TryIncrement() {
				// 兜底渠道 RPM 满：清除 fallback_triggered 标志，避免兜底永久失效，
				// 将该渠道加入 UsedChannelIds 后 break，让循环外兜底检查选择其他兜底渠道。
				// 不进入 RPM 队列等待（兜底渠道不应阻塞正常请求的 RPM 队列）。
				if c.GetBool("fallback_triggered") {
					common.SysLog(fmt.Sprintf("兜底渠道 RPM 已满: fallback_channel_id=%d, 清除标志, 尝试其他兜底渠道", channel.Id))
					c.Set("fallback_triggered", false)
					c.Set("fallback_used", false)
					retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, channel.Id)
					runtimeRpmFull = false
					break
				}
				// RPM just filled up, skip this channel and retry another
				retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, channel.Id)
				runtimeRpmFull = true
				if retryParam.GetRetry() >= maxRetryTimes {
					if waitForRpmQueue(c, relayInfo, &queueDeadline, &queueNoticeSent) {
						wasQueued = true
						runtimeRpmFull = false
						retryParam.UsedChannelIds = retryParam.UsedChannelIds[:0]
						retryParam.SetRetry(0)
						retryParam.ResetRetryNextTry()
						continue
					}
					wasQueued = true
					newAPIError = newRpmQueueTimeoutError()
					break
				}
				continue
			}
			runtimeRpmFull = false
			selectedChannel = channel
			acquiredTrackers = append(acquiredTrackers, tracker)
		}

		if c.GetBool("fallback_triggered") {
			if channel.MaxRPM > 0 && len(acquiredTrackers) > 0 {
				tracker := acquiredTrackers[len(acquiredTrackers)-1]
				tracker.Decrement()
				acquiredTrackers = acquiredTrackers[:len(acquiredTrackers)-1]
				selectedChannel = nil
			}
			c.Set("fallback_triggered", false)
			c.Set("fallback_used", false)
			common.SysLog(fmt.Sprintf("主循环内选中兜底渠道，转交循环外兜底检查处理: fallback_channel_id=%d", channel.Id))
			break
		}
		addUsedChannel(c, channel)
		// 将当前渠道ID添加到已使用列表，以便下次重试时排除
		retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			if len(retryDelays) > 0 {
				c.Set("retry_delays", retryDelays)
			}
			return
		}

		// 客户端已断开连接：保留第一条真正的上游错误，不用断开导致的错误覆盖
		if c.Request != nil && c.Request.Context().Err() != nil && relayInfo.LastError != nil {
			common.SysLog("客户端已断开连接，保留之前的错误不覆盖")
			newAPIError = relayInfo.LastError
			break
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		newAPIError = service.NormalizeSensitiveWordsError(newAPIError)
		relayInfo.LastError = newAPIError
		// 记录第一条非网络层错误，用于最终返回时替代 do_request_failed
		if firstSpecificError == nil && newAPIError.GetErrorCode() != types.ErrorCodeDoRequestFailed {
			firstSpecificError = newAPIError
		}

		// 渠道已被上游实际调用但请求失败，扣减渠道配额（重试失败补偿，避免配额漏扣）
		// 使用 UpstreamRetryCount（内部重试次数）+ 本次失败 = 总调用次数
		callCount := 1
		if relayInfo.UpstreamRetryCount > 0 {
			callCount = relayInfo.UpstreamRetryCount + 1
		}
		model.UpdateChannelCallCount(channel.Id, callCount)

		c.Set("upstream_retry_count", relayInfo.UpstreamRetryCount)
		// 重试过程中不记录错误日志，避免兜底成功后仍残留错误日志影响错误率
		c.Set("is_retry_attempt", true)
		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError, relayInfo)

		// 重试判断（缓存结果避免重复调用，shouldRetry 是纯函数无副作用）
		shouldRetryResult := shouldRetry(c, newAPIError, maxRetryTimes-retryParam.GetRetry())
		common.SysLog(fmt.Sprintf("重试迭代失败: retry=%d, shouldRetry=%v, status_code=%d, error_code=%v, FailoverRetryTimes=%d, maxRetryTimes=%d, used_channel_ids=%v",
			retryParam.GetRetry(), shouldRetryResult, newAPIError.StatusCode, newAPIError.GetErrorCode(), common.FailoverRetryTimes, maxRetryTimes, retryParam.UsedChannelIds))

		// 兜底检查：shouldRetry=false 或 retry >= FailoverRetryTimes 时检查并触发兜底
		// 修复缺陷1：原代码 shouldRetry=false 时兜底检查受 retry>=FailoverRetryTimes 限制，
		//           但 shouldRetry=false 时往往 retry=0，条件不满足直接 break，导致 400 等错误无法触发兜底。
		// 修复缺陷2：原代码 shouldRetry=true 时整个兜底检查块被跳过，即使 retry>=FailoverRetryTimes 也无法触发兜底。
		// 修复方案：
		//   - shouldRetry=false：立即检查兜底（否则循环 break，兜底永远不触发）
		//   - shouldRetry=true：retry >= FailoverRetryTimes 时触发兜底（达到故障转移阈值）
		// 注意：必须排除已使用渠道，与 getChannel 内部的 availableFallback 判断口径保持一致（根因5），
		// 否则主循环认为有可用兜底渠道，但 getChannel 实际无法选出兜底渠道，导致兜底永远不触发。
		if !c.GetBool("fallback_triggered") {
			// 硬编码故障转移阈值=2，不受系统设置影响
			shouldCheckFallback := !shouldRetryResult || retryParam.GetRetry() >= 2
			if shouldCheckFallback && model.HasAvailableFallbackChannelsExcludingUsed(retryParam.UsedChannelIds) {
				common.SysLog(fmt.Sprintf("兜底检查触发: retry=%d, FailoverRetryTimes=%d, shouldRetry=%v, status_code=%d, error_code=%v, 有可用兜底渠道(排除已使用)",
					retryParam.GetRetry(), common.FailoverRetryTimes, shouldRetryResult, newAPIError.StatusCode, newAPIError.GetErrorCode()))
				c.Set("fallback_force_next", true)
				// 重置 retry 为 maxRetryTimes-1，使 IncreaseRetry 后 retry=maxRetryTimes，
				// 循环条件 retry<=maxRetryTimes 成立，继续执行 getChannel 触发兜底。
				// 避免直接 continue 导致 IncreaseRetry 后 retry=maxRetryTimes+1 退出循环、getChannel 永不被调用。
				retryParam.SetRetry(maxRetryTimes - 1)
				continue
			}
			if !shouldCheckFallback {
				common.SysLog(fmt.Sprintf("兜底未触发: retry(%d) < FailoverRetryTimes(%d) 且 shouldRetry=%v, status_code=%d, error_code=%v",
					retryParam.GetRetry(), common.FailoverRetryTimes, shouldRetryResult, newAPIError.StatusCode, newAPIError.GetErrorCode()))
			} else {
				common.SysLog(fmt.Sprintf("兜底未触发: 无可用兜底渠道(排除已用), retry=%d, FailoverRetryTimes=%d, shouldRetry=%v, used_channel_ids=%v",
					retryParam.GetRetry(), common.FailoverRetryTimes, shouldRetryResult, retryParam.UsedChannelIds))
			}
		} else {
			common.SysLog(fmt.Sprintf("兜底未触发: 已触发过兜底, retry=%d, shouldRetry=%v", retryParam.GetRetry(), shouldRetryResult))
		}

		if !shouldRetryResult {
			break
		}

		currentRetry := retryParam.GetRetry() + 1
		if maxRetryEnabled && currentRetry > common.RetryTimes {
			// 极限重试模式：第6次起每10秒一次，发送 "retry X/total" 提示
			if currentRetry <= maxRetryTimes {
				retryDelays = append(retryDelays, int(common.MaxRetryDelay.Seconds()))
				relay.WaitBeforeMaxRetry(c, relayInfo, currentRetry, maxRetryTimes)
			}
		} else {
			// 默认逻辑：前5次用现有延迟和提示
			if delay := getRetryDelay(currentRetry); delay > 0 {
				retryDelays = append(retryDelays, int(delay.Seconds()))
				relay.WaitBeforeRetry(c, relayInfo, delay, currentRetry, "Relay retry")
			}
		}
	}

	// 根因3：主循环退出后兜底检查
	// 处理 break 退出（如所有正常渠道用完、getChannel 返回错误、shouldRetry=false）导致 fallback_triggered=false 的情况。
	// 循环内已处理 shouldRetry=false 时立即检查兜底的路径，但 break 退出场景下兜底可能仍未触发
	// （如 getChannel 返回错误时 retry=0 < FailoverRetryTimes，循环内条件不满足但循环外应补救）。
	// 这里补一次兜底执行（去掉 retry>=FailoverRetryTimes 限制，因为 break 退出时 retry 可能很小）。
	// 使用 for 循环：依次尝试所有未使用的兜底渠道，每个渠道内部最多重试 fallbackMaxRetries 次。
	// 每次失败的兜底渠道都加入 UsedChannelIds，HasAvailableFallbackChannelsExcludingUsed 最终返回 false，不会死循环。
	for model.HasAvailableFallbackChannelsExcludingUsed(retryParam.UsedChannelIds) {
		common.SysLog(fmt.Sprintf("主循环退出后兜底检查: retry=%d, 剩余可用兜底渠道(排除已使用)", retryParam.GetRetry()))
		c.Set("fallback_force_next", true)
		fallbackChannel, fallbackErr := getChannel(c, relayInfo, retryParam)
		if fallbackErr != nil || fallbackChannel == nil {
			if fallbackErr != nil {
				// getChannel 内部 SetupContext 失败时已将失败渠道加入 UsedChannelIds 并清理 fallback_force_next，
				// 此处 continue 让 for 循环条件重新检查是否有其他可用兜底渠道。
				// 非 SetupContext 失败的错误（如无兜底渠道配置）会导致 HasAvailableFallbackChannelsExcludingUsed 返回 false，自然退出。
				common.SysLog(fmt.Sprintf("主循环退出后兜底 getChannel 失败, 尝试其他兜底渠道: %v", fallbackErr.Error()))
				continue
			}
			// fallbackErr == nil 但 fallbackChannel == nil：无可用兜底渠道，退出
			break
		}
		// RPM tracker 资源管理（与循环内逻辑保持一致，defer 会统一释放 acquiredTrackers）
		// 若兜底渠道 RPM 已满，清除 fallback_triggered 并将该渠道加入 UsedChannelIds，继续尝试其他兜底渠道
		rpmAcquired := true
		if fallbackChannel.MaxRPM > 0 {
			tracker := service.GetRpmTracker(fallbackChannel.Id, fallbackChannel.MaxRPM)
			if tracker.TryIncrement() {
				selectedChannel = fallbackChannel
				acquiredTrackers = append(acquiredTrackers, tracker)
			} else {
				rpmAcquired = false
				common.SysLog(fmt.Sprintf("主循环退出后兜底渠道 RPM 已满: fallback_channel_id=%d, 尝试其他兜底渠道", fallbackChannel.Id))
				// 清除 fallback_triggered，将该渠道加入 UsedChannelIds，continue 尝试其他兜底渠道
				c.Set("fallback_triggered", false)
				c.Set("fallback_used", false)
				retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, fallbackChannel.Id)
			}
		}
		if !rpmAcquired {
			continue
		}
		addUsedChannel(c, fallbackChannel)
		retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, fallbackChannel.Id)
		// 与循环内 313 行写法保持一致，确保上游请求的 is_retry 标记准确
		relayInfo.RetryIndex = retryParam.GetRetry()
		// 兜底渠道内部重试：每渠道最多 3 次请求（1次初始 + 2次重试）
		const fallbackMaxRetries = 2
		fallbackSuccess := false
		fallbackAttempt := 0
		for ; fallbackAttempt <= fallbackMaxRetries; fallbackAttempt++ {
			if fallbackAttempt > 0 {
				common.SysLog(fmt.Sprintf("兜底渠道内部重试: fallback_channel_id=%d, attempt=%d/%d", fallbackChannel.Id, fallbackAttempt, fallbackMaxRetries))
			}
			bodyStorage, bodyErr := common.GetBodyStorage(c)
			if bodyErr != nil {
				common.SysLog(fmt.Sprintf("主循环退出后兜底 bodyStorage 读取失败: fallback_channel_id=%d, err=%v", fallbackChannel.Id, bodyErr))
				break
			}
			c.Request.Body = io.NopCloser(bodyStorage)
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				newAPIError = relay.WssHelper(c, relayInfo)
			case types.RelayFormatClaude:
				newAPIError = relay.ClaudeHelper(c, relayInfo)
			case types.RelayFormatGemini:
				newAPIError = geminiRelayHandler(c, relayInfo)
			default:
				newAPIError = relayHandler(c, relayInfo)
			}
			if newAPIError == nil {
				fallbackSuccess = true
				break
			}
			// 客户端已断开连接：保留第一条真正的上游错误，不用断开导致的错误覆盖
			if c.Request != nil && c.Request.Context().Err() != nil && relayInfo.LastError != nil {
				common.SysLog("兜底阶段客户端已断开连接，保留之前的错误")
				newAPIError = relayInfo.LastError
				break
			}
			// 最后一次重试失败，不重试了
			if fallbackAttempt >= fallbackMaxRetries {
				break
			}
			// 判断是否值得重试（网络错误/5xx等重试才有意义）
			if !shouldRetry(c, newAPIError, 1) {
				common.SysLog(fmt.Sprintf("兜底渠道内部重试跳过: fallback_channel_id=%d, status_code=%d, error_code=%v", fallbackChannel.Id, newAPIError.StatusCode, newAPIError.GetErrorCode()))
				break
			}
		}
		if fallbackSuccess {
			relayInfo.LastError = nil
			c.Set("fallback_retry_count", fallbackAttempt)
			if len(retryDelays) > 0 {
				c.Set("retry_delays", retryDelays)
			}
			return
		}
		// 兜底 handler 执行失败，更新错误状态并走原有错误处理
		// 客户端已断开连接：保留第一条错误，不用断开错误覆盖，并退出兜底循环
		if c.Request != nil && c.Request.Context().Err() != nil && relayInfo.LastError != nil {
			common.SysLog("兜底失败阶段客户端已断开连接，保留之前的错误，退出兜底循环")
			newAPIError = relayInfo.LastError
			break
		}
		relayInfo.LastError = newAPIError
		// 记录第一条非网络层错误，用于最终返回时替代 do_request_failed
		if firstSpecificError == nil && newAPIError.GetErrorCode() != types.ErrorCodeDoRequestFailed {
			firstSpecificError = newAPIError
		}
		// 与循环内 411-420 行写法保持一致：扣减渠道配额、记录错误日志字段、走渠道错误处理流程
		// 渠道已被上游实际调用但请求失败，扣减渠道配额（重试失败补偿，避免配额漏扣）
		callCount := 1
		if relayInfo.UpstreamRetryCount > 0 {
			callCount = relayInfo.UpstreamRetryCount + 1
		}
		model.UpdateChannelCallCount(fallbackChannel.Id, callCount)
		c.Set("upstream_retry_count", relayInfo.UpstreamRetryCount)
		// 兜底失败也是重试过程的一部分，不记录错误日志
		c.Set("is_retry_attempt", true)
		processChannelError(c, *types.NewChannelError(fallbackChannel.Id, fallbackChannel.Type, fallbackChannel.Name, fallbackChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), fallbackChannel.GetAutoBan()), newAPIError, relayInfo)
		// 兜底渠道重试全部失败，清除 fallback_triggered，继续尝试其他兜底渠道
		common.SysLog(fmt.Sprintf("兜底渠道 %d 全部重试失败，尝试下一个兜底渠道", fallbackChannel.Id))
		c.Set("fallback_triggered", false)
		c.Set("fallback_used", false)
	}

	if len(retryDelays) > 0 {
		c.Set("retry_delays", retryDelays)
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
	if newAPIError != nil {
		// 保护：如果最终错误是网络层错误(do_request_failed)，但之前有更具体的上游错误，返回之前的错误
		if newAPIError.GetErrorCode() == types.ErrorCodeDoRequestFailed && firstSpecificError != nil {
			common.SysLog(fmt.Sprintf("最终错误为网络层错误，使用之前的上游错误: prev_error_code=%v", firstSpecificError.GetErrorCode()))
			newAPIError = firstSpecificError
		}
		// 最终失败：清除重试标志，记录一条最终错误日志（避免重试过程中的中间错误污染错误率）
		c.Set("is_retry_attempt", false)
		if selectedChannel != nil && constant.ErrorLogEnabled && types.IsRecordErrorLog(newAPIError) {
			processChannelError(c, *types.NewChannelError(selectedChannel.Id, selectedChannel.Type, selectedChannel.Name, selectedChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), selectedChannel.GetAutoBan()), newAPIError, relayInfo)
		}
		// Only return "hard inference failed" for requests that went through the queue
		// AND exhausted all retries. Normal (non-queued) failures preserve their original error.
		if wasQueued && newAPIError.GetErrorCode() != types.ErrorCodeRpmQueueTimeout {
			// Request was queued, dequeued, tried all retries but all failed
			// Return controlled message - NEVER expose raw upstream content
			newAPIError = types.NewErrorWithStatusCode(
				errors.New(common.UserMessageRpmFailed),
				types.ErrorCodeRpmHardInferFailed,
				http.StatusServiceUnavailable,
			)
			// Also log this error for visibility in usage records
			if constant.ErrorLogEnabled {
				logRpmFinalError(c, selectedChannel, common.UserMessageRpmFailed, relayInfo)
			}
		}
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

func sendRpmQueueThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info == nil || !info.IsStream {
		return false
	}
	if info.ChannelMeta == nil {
		info.InitChannelMeta(c)
	}
	if info.ChannelMeta == nil {
		logger.LogWarn(c, "failed to send rpm queue notice: channel meta is nil")
		return false
	}
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return sendClaudeRpmQueueThinkingNotice(c, info)
	case types.RelayFormatGemini:
		if info.RelayMode == relayconstant.RelayModeGemini {
			return sendGeminiRpmQueueThinkingNotice(c, info)
		}
	case types.RelayFormatOpenAI:
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			return sendResponsesRpmQueueThinkingNotice(c, info)
		}
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return false
	}
	return sendOpenAIChatRpmQueueThinkingNotice(c, info)
}

func flushRpmQueueThinkingNotice(c *gin.Context) bool {
	if err := helper.FlushWriter(c); err != nil {
		logger.LogWarn(c, "failed to flush rpm queue notice: "+err.Error())
		return false
	}
	return true
}

func sendOpenAIChatRpmQueueThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	notice := common.UserMessageRpmQueuedThinking + "\n"
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   info.GetDisplayModelName(),
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
				},
			},
		},
	}
	chunk.Choices[0].Delta.SetReasoningContent(notice)

	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		data, err := common.Marshal(chunk)
		if err != nil {
			logger.LogWarn(c, "failed to marshal rpm queue notice: "+err.Error())
			return false
		}
		if err := openai.HandleStreamFormat(c, info, string(data), info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent); err != nil {
			logger.LogWarn(c, "failed to send rpm queue notice: "+err.Error())
			return false
		}
		info.RpmQueueThinkingNoticeSent = true
		return flushRpmQueueThinkingNotice(c)
	case types.RelayFormatClaude, types.RelayFormatGemini:
		data, err := common.Marshal(chunk)
		if err != nil {
			logger.LogWarn(c, "failed to marshal rpm queue notice: "+err.Error())
			return false
		}
		if err := openai.HandleStreamFormat(c, info, string(data), false, false); err != nil {
			logger.LogWarn(c, "failed to send rpm queue notice: "+err.Error())
			return false
		}
		info.RpmQueueThinkingNoticeSent = true
		if info.RelayFormat == types.RelayFormatClaude {
			info.ClaudeRpmQueueThinkingOpen = true
		}
		return flushRpmQueueThinkingNotice(c)
	}
	return false
}

func sendClaudeRpmQueueThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	msg := &dto.ClaudeMediaMessage{
		Id:    helper.GetResponseID(c),
		Model: info.GetDisplayModelName(),
		Type:  "message",
		Role:  "assistant",
		Usage: &dto.ClaudeUsage{
			InputTokens:  info.GetEstimatePromptTokens(),
			OutputTokens: 0,
		},
	}
	msg.SetContent(make([]any, 0))
	if err := helper.ClaudeData(c, dto.ClaudeResponse{Type: "message_start", Message: msg}); err != nil {
		logger.LogWarn(c, "failed to send rpm queue claude message_start: "+err.Error())
		return false
	}
	idx := 0
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_start",
		Index: &idx,
		ContentBlock: &dto.ClaudeMediaMessage{
			Type:     "thinking",
			Thinking: common.GetPointer(""),
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send rpm queue claude thinking start: "+err.Error())
		return false
	}
	thinking := common.UserMessageRpmQueuedThinking + "\n"
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &idx,
		Delta: &dto.ClaudeMediaMessage{
			Type:     "thinking_delta",
			Thinking: &thinking,
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send rpm queue claude thinking delta: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	info.ClaudeRpmQueueThinkingOpen = true
	return true
}

// sendVisionRouteNotice 发送视觉路由思考提示（流式请求才发送）
// 复用 RPM 排队提示机制，但使用视觉路由专用文案
func sendVisionRouteNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info == nil || !info.IsStream {
		return false
	}
	if info.ChannelMeta == nil {
		info.InitChannelMeta(c)
	}
	if info.ChannelMeta == nil {
		logger.LogWarn(c, "failed to send vision route notice: channel meta is nil")
		return false
	}
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return sendClaudeVisionRouteNotice(c, info)
	case types.RelayFormatOpenAI:
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			return false
		}
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return false
	}
	return sendOpenAIChatVisionRouteNotice(c, info)
}

// sendOpenAIChatVisionRouteNotice 发送 OpenAI 格式的视觉路由思考提示
func sendOpenAIChatVisionRouteNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	notice := common.VisionRouteNotice + "\n"
	chunk := &dto.ChatCompletionsStreamResponse{
		Id:      helper.GetResponseID(c),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   info.GetDisplayModelName(),
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
				},
			},
		},
	}
	chunk.Choices[0].Delta.SetReasoningContent(notice)

	data, err := common.Marshal(chunk)
	if err != nil {
		logger.LogWarn(c, "failed to marshal vision route notice: "+err.Error())
		return false
	}
	if err := openai.HandleStreamFormat(c, info, string(data), info.ChannelSetting.ForceFormat, info.ChannelSetting.ThinkingToContent); err != nil {
		logger.LogWarn(c, "failed to send vision route notice: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	return flushRpmQueueThinkingNotice(c)
}

// sendClaudeVisionRouteNotice 发送 Claude 格式的视觉路由思考提示
func sendClaudeVisionRouteNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	msg := &dto.ClaudeMediaMessage{
		Id:    helper.GetResponseID(c),
		Model: info.GetDisplayModelName(),
		Type:  "message",
		Role:  "assistant",
		Usage: &dto.ClaudeUsage{
			InputTokens:  info.GetEstimatePromptTokens(),
			OutputTokens: 0,
		},
	}
	msg.SetContent(make([]any, 0))
	if err := helper.ClaudeData(c, dto.ClaudeResponse{Type: "message_start", Message: msg}); err != nil {
		logger.LogWarn(c, "failed to send vision route claude message_start: "+err.Error())
		return false
	}
	idx := 0
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_start",
		Index: &idx,
		ContentBlock: &dto.ClaudeMediaMessage{
			Type:     "thinking",
			Thinking: common.GetPointer(""),
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send vision route claude thinking start: "+err.Error())
		return false
	}
	thinking := common.VisionRouteNotice + "\n"
	if err := helper.ClaudeData(c, dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &idx,
		Delta: &dto.ClaudeMediaMessage{
			Type:     "thinking_delta",
			Thinking: &thinking,
		},
	}); err != nil {
		logger.LogWarn(c, "failed to send vision route claude thinking delta: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	info.ClaudeRpmQueueThinkingOpen = true
	return true
}

func sendGeminiRpmQueueThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	notice := common.UserMessageRpmQueuedThinking + "\n"
	resp := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Index:         0,
				SafetyRatings: []dto.GeminiChatSafetyRating{},
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{
							Text:    notice,
							Thought: true,
						},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount: info.GetEstimatePromptTokens(),
			TotalTokenCount:  info.GetEstimatePromptTokens(),
		},
	}
	data, err := common.Marshal(resp)
	if err != nil {
		logger.LogWarn(c, "failed to marshal rpm queue gemini notice: "+err.Error())
		return false
	}
	c.Render(-1, common.CustomEvent{Data: "data: " + string(data)})
	if err := helper.FlushWriter(c); err != nil {
		logger.LogWarn(c, "failed to send rpm queue gemini notice: "+err.Error())
		return false
	}
	info.RpmQueueThinkingNoticeSent = true
	return true
}

func sendResponsesRpmQueueThinkingNotice(c *gin.Context, info *relaycommon.RelayInfo) bool {
	itemID := fmt.Sprintf("rs_%s", helper.GetResponseID(c))
	events := []dto.ResponsesStreamResponse{
		{
			Type: "response.reasoning_summary_part.added",
			Item: &dto.ResponsesOutput{
				Type:   "reasoning",
				ID:     itemID,
				Status: "in_progress",
			},
			ItemID:       itemID,
			OutputIndex:  common.GetPointer(0),
			SummaryIndex: common.GetPointer(0),
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "summary_text",
				Text: "",
			},
		},
		{
			Type:         "response.reasoning_summary_text.delta",
			Delta:        common.UserMessageRpmQueuedThinking + "\n",
			ItemID:       itemID,
			OutputIndex:  common.GetPointer(0),
			SummaryIndex: common.GetPointer(0),
		},
	}
	for _, event := range events {
		data, err := common.Marshal(event)
		if err != nil {
			logger.LogWarn(c, "failed to marshal rpm queue responses notice: "+err.Error())
			return false
		}
		helper.ResponseChunkData(c, event, string(data))
	}
	info.RpmQueueThinkingNoticeSent = true
	return flushRpmQueueThinkingNotice(c)
}

func newRpmQueueTimeoutError() *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New(common.UserMessageRpmQueue),
		types.ErrorCodeRpmQueueTimeout,
		http.StatusTooManyRequests,
	)
}

func waitForRpmQueue(c *gin.Context, relayInfo *relaycommon.RelayInfo, queueDeadline *time.Time, queueNoticeSent *bool) bool {
	logger.LogInfo(c, "All channels RPM full, entering queue...")
	if queueDeadline.IsZero() {
		*queueDeadline = time.Now().Add(common.RpmQueueTimeout)
	}
	if !time.Now().Before(*queueDeadline) {
		return false
	}
	queueItem := service.GetRpmQueue().Enqueue(service.RpmQueueItemMeta{
		RequestID:    relayInfo.RequestId,
		Username:     c.GetString("username"),
		UserID:       relayInfo.UserId,
		Group:        relayInfo.TokenGroup,
		ModelName:    relayInfo.GetDisplayModelName(),
		PromptTokens: relayInfo.GetEstimatePromptTokens(),
	})
	if !*queueNoticeSent {
		*queueNoticeSent = sendRpmQueueThinkingNotice(c, relayInfo)
	}
	var done <-chan struct{}
	if c != nil && c.Request != nil {
		done = c.Request.Context().Done()
	}
	if waitRpmQueueTurn(c, queueItem, *queueDeadline, done) {
		logger.LogInfo(c, "Dequeued from RPM queue, retrying...")
		relay.SendRetryWaitNotice(c, relayInfo)
		return true
	}
	return false
}

func waitRpmQueueTurn(c *gin.Context, item *service.RpmQueueItem, deadline time.Time, done <-chan struct{}) bool {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		service.GetRpmQueue().RemoveItem(item)
		return false
	}

	timer := time.NewTimer(remaining)
	defer timer.Stop()

	// 排队期间每 10s 写一次 SSE 注释心跳，避免被中间网络设备断开。
	pingTicker := time.NewTicker(10 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-item.NotifyCh:
			return true
		case <-timer.C:
			return !service.GetRpmQueue().RemoveItem(item)
		case <-done:
			service.GetRpmQueue().RemoveItem(item)
			return false
		case <-pingTicker.C:
			if c != nil {
				_ = helper.PingData(c)
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channel *model.Channel) {
	if channel == nil {
		return
	}
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channel.Id))
	c.Set("use_channel", useChannel)

	useChannelName := c.GetStringSlice("use_channel_name")
	useChannelName = append(useChannelName, channel.Name)
	c.Set("use_channel_name", useChannelName)
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	// 兜底模型逻辑：最后一次重试或 shouldRetry 返回 false 但有兜底渠道时触发（仅触发一次）
	if !c.GetBool("fallback_triggered") && (c.GetBool("fallback_force_next") || (retryParam.GetRetry() > 0 && retryParam.GetRetry() >= 2)) {
		fallbackChannels := model.GetFallbackChannels()
		// 排除已使用的渠道
		availableFallback := make([]*model.Channel, 0, len(fallbackChannels))
		for _, ch := range fallbackChannels {
			used := false
			for _, usedId := range retryParam.UsedChannelIds {
				if ch.Id == usedId {
					used = true
					break
				}
			}
			if !used {
				availableFallback = append(availableFallback, ch)
			}
		}
		if len(availableFallback) > 0 {
			// 随机选择一个可用的兜底渠道（跨分组）
			fallbackChannel := availableFallback[rand.Intn(len(availableFallback))]
			common.SysLog(fmt.Sprintf("兜底被触发: fallback_channel_id=%d, retry=%d, 可用兜底渠道数=%d, used_channel_ids=%v", fallbackChannel.Id, retryParam.GetRetry(), len(availableFallback), retryParam.UsedChannelIds))

			// 先执行 SetupContext，成功后再设置 fallback_triggered 标志
			// （避免 SetupContext 失败时 fallback_triggered 已置位，导致兜底永久失效）
			newAPIError := middleware.SetupContextForSelectedChannel(c, fallbackChannel, info.OriginModelName)
			if newAPIError != nil {
				// SetupContext 失败（如所有 key 被禁用），不设置 fallback_triggered，允许后续尝试其他兜底渠道
				common.SysLog(fmt.Sprintf("兜底渠道 SetupContext 失败: fallback_channel_id=%d, err=%v", fallbackChannel.Id, newAPIError))
				// 将该渠道加入已使用列表，避免下次再选它
				retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, fallbackChannel.Id)
				// 清理 fallback_force_next，让外层决定是否继续
				c.Set("fallback_force_next", false)
				return nil, newAPIError
			}

			// SetupContext 成功，设置兜底标志
			c.Set("fallback_triggered", true)
			c.Set("fallback_used", true)
			c.Set("fallback_channel_id", fallbackChannel.Id)

			// 复写上游模型名为兜底模型，保持 OriginModelName 不变
			fallbackModel := fallbackChannel.GetSetting().FallbackModel
			if fallbackModel != "" {
				info.UpstreamModelName = fallbackModel
				info.IsModelMapped = true
				// 通过 context 传递兜底模型名，确保后续 InitChannelMeta 读取到正确的上游模型名
				// （InitChannelMeta 会用 context 中的 original_model 重置 UpstreamModelName）
				c.Set("original_model", fallbackModel)
				// 清除 model_mapping，防止 ModelMappedHelper 覆盖兜底模型名（兜底模型名优先级最高）
				c.Set("model_mapping", "")
				c.Set("fallback_model", fallbackModel)
				// 修复计费：用兜底模型名重新计算 PriceData 的各项倍率（ModelRatio/CompletionRatio 等），
				// 否则兜底模型会按原模型计费（PriceData 在主循环前已按 OriginModelName 计算）。
				// 注意：preConsumedQuota 不重算（预扣费已按原模型完成），最终结算会用新倍率。
				helper.RecalcPriceDataForFallbackModel(info, fallbackModel)
			}
			info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)
			return fallbackChannel, nil
		}
		// 区分两种情况：无兜底渠道配置 vs 兜底渠道已全部使用
		if len(fallbackChannels) == 0 {
			common.SysLog(fmt.Sprintf("兜底未触发: 无兜底渠道配置, retry=%d, used_channel_ids=%v", retryParam.GetRetry(), retryParam.UsedChannelIds))
		} else {
			// 兜底渠道存在但全部已被使用过，无法选出可用兜底渠道
			common.SysLog(fmt.Sprintf("兜底未触发: 兜底渠道已全部使用, retry=%d, fallback_channels=%d, used_channel_ids=%v", retryParam.GetRetry(), len(fallbackChannels), retryParam.UsedChannelIds))
		}
		// 清理 fallback_force_next 标志，避免下次调用 getChannel 重复进入兜底逻辑分支
		c.Set("fallback_force_next", false)
	}

	if info.ChannelMeta == nil {
		// RPM 队列挂起时优先处理（即使是首次 retry==0）
		if common.GetContextKeyBool(c, constant.ContextKeyRpmQueuePending) {
			channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
			if err != nil {
				if errors.Is(err, model.ErrAllChannelsRpmFull) {
					return nil, types.NewErrorWithStatusCode(
						err,
						types.ErrorCodeGetChannelFailed,
						http.StatusTooManyRequests,
					)
				}
				return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %w", selectGroup, info.GetDisplayModelName(), err), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
			}
			if channel == nil {
				return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.GetDisplayModelName()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
			}
			newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
			if newAPIError != nil {
				return nil, newAPIError
			}
			info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)
			return channel, nil
		}

		if retryParam.GetRetry() == 0 && !retryParam.InitialSelectionDone {
			// 首次调用：使用 distributor 预选的渠道
			if channel, ok := common.GetContextKeyType[*model.Channel](c, constant.ContextKeySelectedChannel); ok && channel != nil {
				return channel, nil
			}
			// fallback 从 context 字段构造 Channel（兼容旧路径）
			autoBan := c.GetBool("auto_ban")
			autoBanInt := 1
			if !autoBan {
				autoBanInt = 0
			}
			return &model.Channel{
				Id:      c.GetInt("channel_id"),
				Type:    c.GetInt("channel_type"),
				Name:    c.GetString("channel_name"),
				AutoBan: &autoBanInt,
			}, nil
		}

		// 重试时：强制选新渠道，从 usedChannelIds 排除已试渠道
		channel, _, err := service.CacheGetRandomSatisfiedChannel(retryParam)
		if err != nil {
			if errors.Is(err, model.ErrAllChannelsRpmFull) {
				return nil, types.NewErrorWithStatusCode(
					err,
					types.ErrorCodeGetChannelFailed,
					http.StatusTooManyRequests,
				)
			}
			if errors.Is(err, model.ErrChannelSpecialUserUnauthorized) {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("未有权限调用模型 %s", info.GetDisplayModelName()),
					types.ErrorCodeModelNotFound,
					http.StatusForbidden,
					types.ErrOptionWithSkipRetry(),
				)
			}
			return nil, types.NewError(fmt.Errorf("获取模型 %s 的可用渠道失败（retry）: %w", info.GetDisplayModelName(), err), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
		}

		// 所有渠道都已经尝试过了，就不要把 usedChannelIds 清空后回退到同一条链路。
		// 这里直接返回无可用渠道，让外层重试逻辑继续按更高层级的策略处理。
		if channel == nil {
			return nil, types.NewError(
				fmt.Errorf("分组下模型 %s 无可用渠道", info.GetDisplayModelName()),
				types.ErrorCodeGetChannelFailed,
				types.ErrOptionWithSkipRetry(),
			)
		}

		newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
		if newAPIError != nil {
			return nil, newAPIError
		}
		return channel, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		if errors.Is(err, model.ErrChannelSpecialUserUnauthorized) {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("未有权限调用模型 %s", info.GetDisplayModelName()),
				types.ErrorCodeModelNotFound,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
			)
		}
		if errors.Is(err, model.ErrAllChannelsRpmFull) {
			return nil, types.NewErrorWithStatusCode(
				err,
				types.ErrorCodeGetChannelFailed,
				http.StatusTooManyRequests,
			)
		}
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %w", selectGroup, info.GetDisplayModelName(), err), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	// 所有渠道都已经尝试过了，就不要把 usedChannelIds 清空后回退到同一条链路。
	// 这里直接返回无可用渠道，让外层重试逻辑继续按更高层级的策略处理。
	if channel == nil {
		return nil, types.NewError(
			fmt.Errorf("分组 %s 下模型 %s 无可用渠道", selectGroup, info.GetDisplayModelName()),
			types.ErrorCodeGetChannelFailed,
			types.ErrOptionWithSkipRetry(),
		)
	}

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	// 客户端已断开连接，无需重试
	if c.Request != nil && c.Request.Context().Err() != nil {
		common.SysLog(fmt.Sprintf("shouldRetry=false: request context cancelled, error_code=%v", openaiErr.GetErrorCode()))
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		common.SysLog(fmt.Sprintf("shouldRetry=false: channel affinity failure skip, error_code=%v", openaiErr.GetErrorCode()))
		return false
	}
	// 注意：retryTimes <= 0 检查必须放在 IsChannelError 之前，否则渠道错误时
	// shouldRetry 会一直返回 true，主循环兜底检查永远不执行。
	if retryTimes <= 0 {
		common.SysLog(fmt.Sprintf("shouldRetry=false: no remaining retry times, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return false
	}
	if types.IsChannelError(openaiErr) {
		common.SysLog(fmt.Sprintf("shouldRetry=true: channel error, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		common.SysLog(fmt.Sprintf("shouldRetry=false: skip retry error, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		common.SysLog(fmt.Sprintf("shouldRetry=false: specific channel id set, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		common.SysLog(fmt.Sprintf("shouldRetry=false: 2xx status code, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return false
	}
	if code < 100 || code > 599 {
		common.SysLog(fmt.Sprintf("shouldRetry=true: non-http status code, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		common.SysLog(fmt.Sprintf("shouldRetry=false: always skip retry code, status_code=%d, error_code=%v", openaiErr.StatusCode, openaiErr.GetErrorCode()))
		return false
	}
	result := operation_setting.ShouldRetryByStatusCode(code)
	common.SysLog(fmt.Sprintf("shouldRetry=%v: status code range check, status_code=%d, error_code=%v, retryTimes=%d", result, openaiErr.StatusCode, openaiErr.GetErrorCode(), retryTimes))
	return result
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, err.Error()))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously

	if channelError.AutoBan {
		// 401/429/503/invalid token 错误先经过 3 轮复测确认，全部失败才禁用
		if service.ShouldDelayDisableChannel(err) {
			service.StartRetryCheck(channelError, err.ErrorWithStatusCode(), nil)
		} else if service.ShouldDisableChannel(err) {
			gopool.Go(func() {
				service.DisableChannel(channelError, err.ErrorWithStatusCode())
			})
		}
	}

	// 重试过程中的错误不记录错误日志，避免兜底成功后仍残留错误日志影响错误率
	// 最终失败时会清除 is_retry_attempt 标志后记录一条错误日志
	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) && !c.GetBool("is_retry_attempt") {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := common.GetContextKeyString(c, constant.ContextKeyDisplayModel)
		if modelName == "" {
			modelName = relayInfo.GetDisplayModelName()
		}
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		service.AppendChannelRetryAdminInfo(c, adminInfo)
		adminInfo["detail_error"] = err.ErrorWithStatusCode()
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		// 应急预案渠道标记（管理员可见）
		if c.GetBool("emergency_used") {
			adminInfo["emergency_used"] = true
			adminInfo["emergency_channel_id"] = c.GetInt("emergency_channel_id")
		}
		// 兜底模型标记（管理员可见）
		if c.GetBool("fallback_used") {
			adminInfo["fallback_used"] = true
			adminInfo["fallback_channel_id"] = c.GetInt("fallback_channel_id")
			if fallbackModel := c.GetString("fallback_model"); fallbackModel != "" {
				adminInfo["fallback_model"] = fallbackModel
			}
		}
		other["admin_info"] = adminInfo
		if retryCount, exists := c.Get("upstream_retry_count"); exists {
			other["upstream_retry_count"] = retryCount
		}
		if retryDelays, exists := c.Get("retry_delays"); exists {
			if delays, ok := retryDelays.([]int); ok && len(delays) > 0 {
				other["retry_delays"] = delays
			}
		}
		if routedModelName := common.GetContextKeyString(c, constant.ContextKeyAutoRouteModel); routedModelName != "" && routedModelName != modelName {
			other["auto_routed"] = true
			other["routed_model_name"] = routedModelName
		}
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		retryCountInt := 0
		if rc, exists := c.Get("upstream_retry_count"); exists {
			if rcInt, ok := rc.(int); ok {
				retryCountInt = rcInt
			}
		}
		logId := model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.GetUserFriendlyMessage(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, retryCountInt, other)
		service.RecordLogDetail(c, relayInfo, logId)
	}
}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	// 初始化 BillingSource：GPT 专有分组使用 GPT 钱包扣费
	// mjproxy_handler 直接调用 PostConsumeQuota 绕过 BillingSession，
	// 此处提前设置，确保 PostConsumeQuota 正确识别资金来源扣减 GPT 额度
	if ratio_setting.ContainsGptGroupRatio(relayInfo.UsingGroup) {
		relayInfo.BillingSource = service.BillingSourceGptWallet
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	// 请求级别总超时保护：超过阈值自动断开，避免请求被无限拉长
	if common.RequestMaxDuration > 0 {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(common.RequestMaxDuration)*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
	}
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	// 初始化 BillingSource：GPT 专有分组使用 GPT 钱包扣费
	// 任务流程可能因免费模型跳过 PreConsumeBilling，提前设置确保
	// SettleBilling/PostConsumeQuota 及异步 taskAdjustFunding 正确扣 GPT 额度
	if ratio_setting.ContainsGptGroupRatio(relayInfo.UsingGroup) {
		relayInfo.BillingSource = service.BillingSourceGptWallet
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:            c,
		TokenGroup:     relayInfo.TokenGroup,
		ModelName:      relayInfo.OriginModelName,
		Retry:          common.GetPointer(0),
		UsedChannelIds: make([]int, 0), // 初始化已使用渠道ID列表
	}

	var retryDelays []int

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}
		// 应急预案渠道标记
		if channel != nil && channel.IsEmergencyPlanEnabled() {
			c.Set("emergency_used", true)
			c.Set("emergency_channel_id", channel.Id)
		}
		retryParam.InitialSelectionDone = true

		addUsedChannel(c, channel)
		// 将当前渠道ID添加到已使用列表，以便下次重试时排除
		retryParam.UsedChannelIds = append(retryParam.UsedChannelIds, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			if len(retryDelays) > 0 {
				c.Set("retry_delays", retryDelays)
			}
			break
		}

		if !taskErr.LocalError {
			c.Set("upstream_retry_count", relayInfo.UpstreamRetryCount)
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode), relayInfo)
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}

		if delay := getRetryDelay(retryParam.GetRetry() + 1); delay > 0 {
			retryDelays = append(retryDelays, int(delay.Seconds()))
			relay.WaitBeforeRetry(c, relayInfo, delay, retryParam.GetRetry()+1, "Task relay retry")
		}
	}

	if len(retryDelays) > 0 {
		c.Set("retry_delays", retryDelays)
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)

		task := model.InitTask(result.Platform, relayInfo)
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "上游已饱和，请稍后重试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	// 客户端已断开或请求总超时（RequestMaxDuration），不再重试
	if c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}

// logRpmFinalError records an error log for RPM queue final failures,
// ensuring the user-controlled message appears in usage records.
func logRpmFinalError(c *gin.Context, selectedChannel *model.Channel, content string, relayInfo *relaycommon.RelayInfo) {
	userId := c.GetInt("id")
	tokenName := c.GetString("token_name")
	modelName := common.GetContextKeyString(c, constant.ContextKeyDisplayModel)
	if modelName == "" {
		modelName = c.GetString("original_model")
	}
	tokenId := c.GetInt("token_id")
	userGroup := c.GetString("group")
	channelId := 0
	if selectedChannel != nil {
		channelId = selectedChannel.Id
	}
	retryCountInt := 0
	if rc, exists := c.Get("upstream_retry_count"); exists {
		if rcInt, ok := rc.(int); ok {
			retryCountInt = rcInt
		}
	}
	other := map[string]interface{}{
		"queued":         true,
		"queued_timeout": false,
	}
	if routedModelName := common.GetContextKeyString(c, constant.ContextKeyAutoRouteModel); routedModelName != "" && routedModelName != modelName {
		other["auto_routed"] = true
		other["routed_model_name"] = routedModelName
	}
	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())
	logId := model.RecordErrorLog(c, userId, channelId, modelName, tokenName, content, tokenId, useTimeSeconds,
		common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, retryCountInt, other)
	service.RecordLogDetail(c, relayInfo, logId)
}
