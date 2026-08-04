package service

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func specialUsageUsageSource(inputTokens, outputTokens int, status string) string {
	if inputTokens > 0 || outputTokens > 0 {
		return "upstream"
	}
	if status == model.SpecialUsageStatusSuccess {
		return "fixed_price"
	}
	return "none"
}

// RecordSpecialUsageFromRelay writes the final upstream usage and user-side
// charge to the independent monitoring ledger. It intentionally does not use
// LogConsumeEnabled, so legacy log retention settings cannot disable cost
// accounting.
func RecordSpecialUsageFromRelay(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, inputTokens, outputTokens, userQuota int, status, errorMessage string) {
	if len(errorMessage) > 512 {
		errorMessage = errorMessage[:512]
	}
	if relayInfo == nil || relayInfo.ChannelId <= 0 {
		return
	}
	requestID := relayInfo.RequestId
	if requestID == "" && ctx != nil {
		requestID = ctx.GetString(common.RequestIdKey)
	}
	channelName := ""
	if ctx != nil {
		channelName = common.GetContextKeyString(ctx, constant.ContextKeyChannelName)
		if channelName == "" {
			channelName = ctx.GetString("channel_name")
		}
	}
	modelName := relayInfo.GetDisplayModelName()
	if modelName == "" {
		modelName = relayInfo.OriginModelName
	}
	groupName := strings.TrimSpace(relayInfo.UsingGroup)
	if groupName == "" || groupName == "auto" {
		groupName = strings.TrimSpace(relayInfo.TokenGroup)
	}
	if groupName == "" || groupName == "auto" {
		groupName = strings.TrimSpace(relayInfo.UserGroup)
	}
	channelSetting := dto.ChannelSettings{}
	if relayInfo.ChannelMeta != nil {
		channelSetting = relayInfo.ChannelMeta.ChannelSetting
	}
	monitorConfig := model.GetSpecialUsageConfig()
	selected := monitorConfig.Enabled && len(monitorConfig.GroupNames) > 0 && len(monitorConfig.ModelNames) > 0
	if selected {
		selected = model.SpecialUsageChannelMatches(monitorConfig, relayInfo.ChannelId, modelName, groupName)
	}
	model.RecordSpecialUsage(model.SpecialUsageCostInput{
		RequestID:                  requestID,
		UserID:                     relayInfo.UserId,
		ChannelID:                  relayInfo.ChannelId,
		ChannelName:                channelName,
		GroupName:                  groupName,
		ModelName:                  modelName,
		InputTokens:                inputTokens,
		OutputTokens:               outputTokens,
		UserChargeQuota:            userQuota,
		Status:                     status,
		ErrorMessage:               errorMessage,
		RequestTime:                time.Now().Unix(),
		ChannelSetting:             channelSetting,
		FrozenChannelSetting:       relayInfo.SpecialUsageChannelSetting,
		FrozenChannelSettingValid:  relayInfo.SpecialUsageChannelSettingValid,
		FrozenModelPrice:           relayInfo.PriceData.ModelPrice,
		FrozenModelRatio:           relayInfo.PriceData.ModelRatio,
		FrozenCompletionRatio:      relayInfo.PriceData.CompletionRatio,
		FrozenUsePrice:             relayInfo.PriceData.UsePrice,
		FrozenPriceValid:           true,
		FrozenSpecialBilling:       relayInfo.SpecialUsageConfigSpecialBilling,
		FrozenSpecialBillingValid:  relayInfo.SpecialUsageConfigBillingValid,
		SpecialUsageConfig:         monitorConfig,
		SpecialUsageConfigValid:    true,
		SpecialUsageSelected:       selected,
		SpecialUsageSelectionValid: true,
		FrozenMultiplier:           model.GetSpecialUsageMultiplier(monitorConfig, relayInfo.ChannelId, groupName),
		FrozenMultiplierValid:      true,
		FrozenPriceSource:          relayInfo.SpecialUsageBillingSource,
		Attempt:                    relayInfo.SpecialUsageAttempt,
		UsageSource:                specialUsageUsageSource(inputTokens, outputTokens, status),
	})
}

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other["request_path"] = path
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other["request_path"] = path
	}
}

func GenerateTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_ratio"] = modelRatio
	other["group_ratio"] = groupRatio
	other["completion_ratio"] = completionRatio
	other["cache_tokens"] = cacheTokens
	other["cache_ratio"] = cacheRatio
	other["model_price"] = modelPrice
	other["user_group_ratio"] = userGroupRatio
	other["frt"] = float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())
	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}

	isSystemPromptOverwritten := common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride)
	if isSystemPromptOverwritten {
		other["is_system_prompt_overwritten"] = true
	}

	adminInfo := make(map[string]interface{})
	AppendChannelRetryAdminInfo(ctx, adminInfo)
	isMultiKey := common.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex)
	}

	isLocalCountTokens := common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens)
	if isLocalCountTokens {
		adminInfo["local_count_tokens"] = isLocalCountTokens
	}

	AppendChannelAffinityAdminInfo(ctx, adminInfo)

	// 兜底模型标记
	if ctx.GetBool("fallback_used") {
		// 用户可见：仅告知被自动错误转移，不暴露渠道信息
		other["fallback_used"] = true
		// 管理员可见：包含渠道ID等详细信息
		adminInfo["fallback_used"] = true
		adminInfo["fallback_channel_id"] = ctx.GetInt("fallback_channel_id")
		if fallbackModel := ctx.GetString("fallback_model"); fallbackModel != "" {
			adminInfo["fallback_model"] = fallbackModel
		}
		fallbackRetryCount := ctx.GetInt("fallback_retry_count")
		if fallbackRetryCount > 0 {
			adminInfo["fallback_retry_count"] = fallbackRetryCount
		}
	}

	// 应急预案渠道标记（管理员可见）
	isEmergencyUsed := ctx.GetBool("emergency_used")
	if isEmergencyUsed {
		adminInfo["emergency_used"] = true
		adminInfo["emergency_channel_id"] = ctx.GetInt("emergency_channel_id")
	}

	// 模型映射信息：应急/兜底场景仅管理员可见，普通场景所有用户可见
	if relayInfo.IsModelMapped {
		if isEmergencyUsed || ctx.GetBool("fallback_used") {
			adminInfo["is_model_mapped"] = true
			adminInfo["upstream_model_name"] = relayInfo.UpstreamModelName
		} else {
			other["is_model_mapped"] = true
			other["upstream_model_name"] = relayInfo.UpstreamModelName
		}
	}

	other["admin_info"] = adminInfo
	appendRequestPath(ctx, relayInfo, other)
	appendRequestConversionChain(relayInfo, other)
	appendFinalRequestFormat(relayInfo, other)
	appendBillingInfo(relayInfo, other)
	appendAutoRouteInfo(relayInfo, other)
	appendParamOverrideInfo(relayInfo, other)
	appendStreamStatus(relayInfo, other)
	return other
}

func appendParamOverrideInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || len(relayInfo.ParamOverrideAudit) == 0 {
		return
	}
	other["po"] = relayInfo.ParamOverrideAudit
}

func appendStreamStatus(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || !relayInfo.IsStream || relayInfo.StreamStatus == nil {
		return
	}
	ss := relayInfo.StreamStatus
	status := "ok"
	if !ss.IsNormalEnd() || ss.HasErrors() {
		status = "error"
	}
	streamInfo := map[string]interface{}{
		"status":     status,
		"end_reason": string(ss.EndReason),
	}
	if ss.EndError != nil {
		streamInfo["end_error"] = ss.EndError.Error()
	}
	if ss.ErrorCount > 0 {
		streamInfo["error_count"] = ss.ErrorCount
		messages := make([]string, 0, len(ss.Errors))
		for _, e := range ss.Errors {
			messages = append(messages, e.Message)
		}
		streamInfo["errors"] = messages
	}
	other["stream_status"] = streamInfo
}

func appendBillingInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	// billing_source: "wallet", "subscription" or "gpt_wallet"
	if relayInfo.BillingSource != "" {
		other["billing_source"] = relayInfo.BillingSource
	}
	if relayInfo.UserSetting.BillingPreference != "" {
		other["billing_preference"] = relayInfo.UserSetting.BillingPreference
	}
	if relayInfo.BillingSource == "subscription" {
		if relayInfo.SubscriptionId != 0 {
			other["subscription_id"] = relayInfo.SubscriptionId
		}
		if relayInfo.SubscriptionPreConsumed > 0 {
			other["subscription_pre_consumed"] = relayInfo.SubscriptionPreConsumed
		}
		// post_delta: settlement delta applied after actual usage is known (can be negative for refund)
		if relayInfo.SubscriptionPostDelta != 0 {
			other["subscription_post_delta"] = relayInfo.SubscriptionPostDelta
		}
		if relayInfo.SubscriptionPlanId != 0 {
			other["subscription_plan_id"] = relayInfo.SubscriptionPlanId
		}
		if relayInfo.SubscriptionPlanTitle != "" {
			other["subscription_plan_title"] = relayInfo.SubscriptionPlanTitle
		}
		// Compute "this request" subscription consumed + remaining
		consumed := relayInfo.SubscriptionPreConsumed + relayInfo.SubscriptionPostDelta
		usedFinal := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		if consumed < 0 {
			consumed = 0
		}
		if usedFinal < 0 {
			usedFinal = 0
		}
		if relayInfo.SubscriptionAmountTotal > 0 {
			remain := relayInfo.SubscriptionAmountTotal - usedFinal
			if remain < 0 {
				remain = 0
			}
			other["subscription_total"] = relayInfo.SubscriptionAmountTotal
			other["subscription_used"] = usedFinal
			other["subscription_remain"] = remain
		}
		if consumed > 0 {
			other["subscription_consumed"] = consumed
		}
		// Wallet quota is not deducted when billed from subscription.
		other["wallet_quota_deducted"] = 0
	} else if relayInfo.BillingSource == BillingSourceGptWallet {
		if relayInfo.InitialPreConsumedQuota > 0 {
			other["gpt_pre_consumed"] = relayInfo.InitialPreConsumedQuota
		}
		if relayInfo.BillingPostDeltaQuota != 0 {
			other["gpt_post_delta"] = relayInfo.BillingPostDeltaQuota
		}
	}
}

func appendAutoRouteInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	displayModelName := relayInfo.GetDisplayModelName()
	routedModelName := relayInfo.GetAutoRouteModelName()
	if displayModelName == "" || routedModelName == "" || displayModelName == routedModelName {
		return
	}
	other["auto_routed"] = true
	other["routed_model_name"] = routedModelName
}

func appendRequestConversionChain(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case types.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case types.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case types.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case types.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) == 0 {
		return
	}
	other["request_conversion"] = chain
}

func appendFinalRequestFormat(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		// claude indicates the final upstream request format is Claude Messages.
		// Frontend log rendering uses this to keep the original Claude input display.
		other["claude"] = true
	}
}

func GenerateWssOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["ws"] = true
	info["audio_input"] = usage.InputTokenDetails.AudioTokens
	info["audio_output"] = usage.OutputTokenDetails.AudioTokens
	info["text_input"] = usage.InputTokenDetails.TextTokens
	info["text_output"] = usage.OutputTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateAudioOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["audio"] = true
	info["audio_input"] = usage.PromptTokensDetails.AudioTokens
	info["audio_output"] = usage.CompletionTokenDetails.AudioTokens
	info["text_input"] = usage.PromptTokensDetails.TextTokens
	info["text_output"] = usage.CompletionTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateClaudeOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64,
	cacheCreationTokens int, cacheCreationRatio float64,
	cacheCreationTokens5m int, cacheCreationRatio5m float64,
	cacheCreationTokens1h int, cacheCreationRatio1h float64,
	modelPrice float64, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice, userGroupRatio)
	info["claude"] = true
	info["cache_creation_tokens"] = cacheCreationTokens
	info["cache_creation_ratio"] = cacheCreationRatio
	if cacheCreationTokens5m != 0 {
		info["cache_creation_tokens_5m"] = cacheCreationTokens5m
		info["cache_creation_ratio_5m"] = cacheCreationRatio5m
	}
	if cacheCreationTokens1h != 0 {
		info["cache_creation_tokens_1h"] = cacheCreationTokens1h
		info["cache_creation_ratio_1h"] = cacheCreationRatio1h
	}
	return info
}

func GenerateMjOtherInfo(relayInfo *relaycommon.RelayInfo, priceData types.PriceData) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_price"] = priceData.ModelPrice
	other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio
	if priceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = priceData.GroupRatioInfo.GroupSpecialRatio
	}
	appendAutoRouteInfo(relayInfo, other)
	appendRequestPath(nil, relayInfo, other)
	appendBillingInfo(relayInfo, other)
	return other
}

// InjectTieredBillingInfo overlays tiered billing fields onto an existing
// module-specific other map. Call this after GenerateTextOtherInfo /
// GenerateClaudeOtherInfo / etc. when the request used tiered_expr billing.
func InjectTieredBillingInfo(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, result *billingexpr.TieredResult) {
	if relayInfo == nil || other == nil {
		return
	}
	snap := relayInfo.TieredBillingSnapshot
	if snap == nil {
		return
	}
	other["billing_mode"] = "tiered_expr"
	other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
	if result != nil {
		other["matched_tier"] = result.MatchedTier
	}
}
