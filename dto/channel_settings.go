package dto

import "sort"

type SpecialBillingPrice struct {
	MaxInputTokens *int64  `json:"max_input_tokens,omitempty"`
	Price          float64 `json:"price"`
}

type ChannelSettings struct {
	// ResponsesProtocol explicitly selects the OpenAI Responses (RE) protocol
	// for this channel. When false, OpenAI-compatible channels use Chat
	// Completions by default. Protocol selection is never inferred from the
	// channel type.
	ResponsesProtocol bool `json:"responses_protocol,omitempty"`
	// UpstreamProtocol is the legacy spelling used by channel settings created
	// before ResponsesProtocol was introduced. Keep reading it so existing
	// fallback channels configured with "responses" do not silently switch to
	// Chat Completions after an upgrade.
	UpstreamProtocol             string                           `json:"upstream_protocol,omitempty"`
	ForceFormat                  bool                             `json:"force_format,omitempty"`
	ThinkingToContent            bool                             `json:"thinking_to_content,omitempty"`
	Proxy                        string                           `json:"proxy"`
	PassThroughBodyEnabled       bool                             `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt                 string                           `json:"system_prompt,omitempty"`
	SystemPromptOverride         bool                             `json:"system_prompt_override,omitempty"`
	SpecialUserEnabled           bool                             `json:"special_user_enabled,omitempty"`
	SpecialUserIds               []int                            `json:"special_user_ids,omitempty"`
	GptModeRequired              bool                             `json:"gpt_mode_required,omitempty"`
	DisableAutoRetry             bool                             `json:"disable_auto_retry,omitempty"`          // 是否关闭该渠道的自动重试
	AutoRetrySkipErrorCodes      []string                         `json:"auto_retry_skip_error_codes,omitempty"` // 命中这些 HTTP 状态码或内部错误码时不重试
	EmergencyPlanEnabled         bool                             `json:"emergency_plan_enabled,omitempty"`
	FallbackModelEnabled         bool                             `json:"fallback_model_enabled,omitempty"`          // 是否启用兜底模式
	FallbackModel                string                           `json:"fallback_model,omitempty"`                  // 兜底模型名（上游实际请求的模型名）
	FallbackModelReasoningEffort string                           `json:"fallback_model_reasoning_effort,omitempty"` // 兜底模型思考等级
	FallbackPriority             int                              `json:"fallback_priority,omitempty"`               // 兜底渠道优先级，数值越大越优先
	SupportFallback              bool                             `json:"support_fallback,omitempty"`                // 是否支持错误转移（该渠道失败时是否触发转移到兜底渠道）
	ProbeEnabled                 bool                             `json:"probe_enabled,omitempty"`                   // 是否启用渠道探针
	SpecialBilling               bool                             `json:"special_billing,omitempty"`
	SpecialBillingPrices         map[string][]SpecialBillingPrice `json:"special_billing_prices,omitempty"`
	FixedModelReasoningEnabled   bool                             `json:"fixed_model_reasoning_enabled,omitempty"` // 是否启用模型固定思考等级
	FixedModelReasoningEfforts   map[string]string                `json:"fixed_model_reasoning_efforts,omitempty"` // 模型→思考等级映射
}

func (s ChannelSettings) ResolveSpecialBillingPrice(model string, inputTokens int) (float64, bool) {
	if !s.SpecialBilling {
		return 0, false
	}
	prices, ok := s.SpecialBillingPrices[model]
	if !ok || len(prices) == 0 {
		return 0, false
	}
	ordered := append([]SpecialBillingPrice(nil), prices...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].MaxInputTokens == nil {
			return false
		}
		if ordered[j].MaxInputTokens == nil {
			return true
		}
		return *ordered[i].MaxInputTokens < *ordered[j].MaxInputTokens
	})
	for _, tier := range ordered {
		if tier.MaxInputTokens == nil || int64(inputTokens) <= *tier.MaxInputTokens {
			return tier.Price, tier.Price >= 0
		}
	}
	return 0, false
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string        `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool         `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool          `json:"claude_beta_query,omitempty"`         // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool          `json:"allow_service_tier,omitempty"`        // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool          `json:"allow_inference_geo,omitempty"`       // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool          `json:"allow_speed,omitempty"`               // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool          `json:"allow_safety_identifier,omitempty"`   // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool          `json:"disable_store,omitempty"`             // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool          `json:"allow_include_obfuscation,omitempty"` // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	AwsKeyType                            AwsKeyType    `json:"aws_key_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool          `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool          `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64         `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string      `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string      `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string      `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
