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

// GetFallbackReasoningEffort resolves the effective reasoning effort override
// in priority order: model mapping target > channel fixed model level > fallback
// channel level. Each source overrides the client's own reasoning setting.
func (info *RelayInfo) GetFallbackReasoningEffort() string {
	if info == nil {
		return ""
	}
	if effort := NormalizeFallbackReasoningEffort(info.MappedReasoningEffort); effort != "" {
		return effort
	}
	if effort := NormalizeFallbackReasoningEffort(info.FixedModelReasoningEffort); effort != "" {
		return effort
	}
	return NormalizeFallbackReasoningEffort(info.FallbackReasoningEffort)
}

// SyncReasoningEffortFromOpenAIRequest keeps request metadata in sync with
// the effective Chat Completions/OpenRouter reasoning setting. Fallback and
// model-mapping overrides always take precedence over the client value.
func (info *RelayInfo) SyncReasoningEffortFromOpenAIRequest(request *dto.GeneralOpenAIRequest) {
	if info == nil {
		return
	}

	effort := ""
	if request != nil {
		effort = request.ReasoningEffort
		if effort == "" && len(request.Reasoning) > 0 {
			var reasoning struct {
				Effort string `json:"effort"`
			}
			if err := json.Unmarshal(request.Reasoning, &reasoning); err == nil {
				effort = reasoning.Effort
			}
		}
	}
	info.syncReasoningEffort(effort)
}

// SyncReasoningEffortFromResponsesRequest keeps request metadata in sync with
// the Responses API reasoning.effort field.
func (info *RelayInfo) SyncReasoningEffortFromResponsesRequest(request *dto.OpenAIResponsesRequest) {
	effort := ""
	if request != nil && request.Reasoning != nil {
		effort = request.Reasoning.Effort
	}
	if info != nil {
		info.syncReasoningEffort(effort)
	}
}

// SyncReasoningEffortFromClaudeRequest records Claude thinking requests even
// though the native Claude API represents the setting as a thinking object.
func (info *RelayInfo) SyncReasoningEffortFromClaudeRequest(request *dto.ClaudeRequest) {
	effort := ""
	if request != nil {
		effort = request.GetEfforts()
		if effort == "" && request.Thinking != nil {
			switch request.Thinking.Type {
			case "disabled":
				effort = "none"
			case "enabled", "adaptive":
				effort = "thinking"
			}
		}
	}
	if info != nil {
		info.syncReasoningEffort(effort)
	}
}

func (info *RelayInfo) syncReasoningEffort(effort string) {
	if info == nil {
		return
	}
	if override := info.GetFallbackReasoningEffort(); override != "" {
		if override == "none" {
			info.ReasoningEffort = ""
		} else {
			info.ReasoningEffort = override
		}
		return
	}
	effort = NormalizeFallbackReasoningEffort(effort)
	if effort == "none" {
		effort = ""
	}
	info.ReasoningEffort = effort
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
