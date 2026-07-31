package common

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

// NormalizeFallbackReasoningEffort returns the supported persisted values.
// An empty value or "inherit" keeps the request's original reasoning setting.
func NormalizeFallbackReasoningEffort(effort string) string {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	switch normalized {
	case "none", "minimal", "low", "medium", "high", "xhigh", "max", "thinking":
		return normalized
	default:
		return ""
	}
}

func (info *RelayInfo) GetFallbackReasoningEffort() string {
	if info == nil {
		return ""
	}
	if effort := NormalizeFallbackReasoningEffort(info.MappedReasoningEffort); effort != "" {
		return effort
	}
	return NormalizeFallbackReasoningEffort(info.FallbackReasoningEffort)
}

// ApplyFallbackReasoningToOpenAIRequest applies the configured fallback level
// before protocol-specific adaptors convert the request.
func (info *RelayInfo) ApplyFallbackReasoningToOpenAIRequest(request *dto.GeneralOpenAIRequest) {
	effort := info.GetFallbackReasoningEffort()
	if request == nil || effort == "" {
		return
	}

	request.Model = stripFallbackReasoningSuffix(request.Model)
	request.ReasoningEffort = effort
	if effort == "none" {
		info.ReasoningEffort = ""
	} else {
		info.ReasoningEffort = effort
	}
	if effort == "none" {
		request.Reasoning = nil
	}

	// DeepSeek uses its own thinking object instead of reasoning_effort.
	if info.channelType() == constant.ChannelTypeDeepSeek {
		thinkingType := "enabled"
		if effort == "none" {
			thinkingType = "disabled"
		}
		request.THINKING, _ = json.Marshal(map[string]string{"type": thinkingType})
		request.ReasoningEffort = ""
	}
}

func (info *RelayInfo) ApplyFallbackReasoningToResponsesRequest(request *dto.OpenAIResponsesRequest) {
	effort := info.GetFallbackReasoningEffort()
	if request == nil || effort == "" {
		return
	}
	request.Model = stripFallbackReasoningSuffix(request.Model)
	request.Reasoning = &dto.Reasoning{
		Effort:  effort,
		Summary: "detailed",
	}
	if effort == "none" {
		info.ReasoningEffort = ""
	} else {
		info.ReasoningEffort = effort
	}
}

func (info *RelayInfo) ApplyFallbackReasoningToGeminiRequest(request *dto.GeminiChatRequest) {
	effort := info.GetFallbackReasoningEffort()
	if request == nil || effort == "" {
		return
	}

	if effort == "none" {
		request.GenerationConfig.ThinkingConfig = &dto.GeminiThinkingConfig{
			ThinkingBudget: commonIntPointer(0),
		}
		info.ReasoningEffort = ""
		return
	}
	request.GenerationConfig.ThinkingConfig = &dto.GeminiThinkingConfig{
		IncludeThoughts: true,
		ThinkingLevel:   effort,
	}
	info.ReasoningEffort = effort
}

func (info *RelayInfo) ApplyFallbackReasoningToClaudeRequest(request *dto.ClaudeRequest) {
	effort := info.GetFallbackReasoningEffort()
	if request == nil || effort == "" {
		return
	}

	baseModel := stripFallbackReasoningSuffix(request.Model)
	if info.channelType() == constant.ChannelTypeDeepSeek && (effort == "none" || effort == "max") {
		request.Model = baseModel + "-" + effort
		info.UpstreamModelName = request.Model
		return
	}

	if effort == "none" {
		request.Model = baseModel
		request.Thinking = nil
		info.ReasoningEffort = ""
		return
	}
	info.ReasoningEffort = effort

	if (strings.HasPrefix(baseModel, "claude-opus-4-6") || strings.HasPrefix(baseModel, "claude-opus-4-7")) &&
		effort != "thinking" {
		request.Model = baseModel + "-" + effort
		return
	}
	if effort == "thinking" {
		request.Model = baseModel + "-thinking"
		return
	}
	request.Model = baseModel

	budgetTokens := map[string]int{
		"minimal": 1280,
		"low":     1280,
		"medium":  2048,
		"high":    4096,
		"max":     8192,
		"xhigh":   8192,
	}[effort]
	if budgetTokens == 0 {
		return
	}
	if request.MaxTokens == nil || *request.MaxTokens < uint(budgetTokens) {
		maxTokens := uint(budgetTokens)
		request.MaxTokens = &maxTokens
	}
	request.Thinking = &dto.Thinking{
		Type:         "enabled",
		BudgetTokens: &budgetTokens,
	}
}

func stripFallbackReasoningSuffix(modelName string) string {
	for _, suffix := range []string{"-thinking", "-nothinking", "-minimal", "-low", "-medium", "-high", "-xhigh", "-max", "-none"} {
		if strings.HasSuffix(modelName, suffix) {
			return strings.TrimSuffix(modelName, suffix)
		}
	}
	return modelName
}

func (info *RelayInfo) channelType() int {
	if info == nil || info.ChannelMeta == nil {
		return 0
	}
	return info.ChannelMeta.ChannelType
}

func commonIntPointer(value int) *int {
	return &value
}
