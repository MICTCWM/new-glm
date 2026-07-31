package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

func TestApplyFallbackReasoningToOpenAIRequest(t *testing.T) {
	info := &RelayInfo{
		FallbackReasoningEffort: " HIGH ",
		ChannelMeta:             &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	}
	request := &dto.GeneralOpenAIRequest{}

	info.ApplyFallbackReasoningToOpenAIRequest(request)

	if request.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", request.ReasoningEffort)
	}
}

func TestApplyFallbackReasoningPrefersModelMappingEffort(t *testing.T) {
	info := &RelayInfo{
		FallbackReasoningEffort: "low",
		MappedReasoningEffort:   "xhigh",
		ChannelMeta:             &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	}
	request := &dto.GeneralOpenAIRequest{}

	info.ApplyFallbackReasoningToOpenAIRequest(request)

	if request.ReasoningEffort != "xhigh" {
		t.Fatalf("ReasoningEffort = %q, want xhigh", request.ReasoningEffort)
	}
}

func TestApplyFallbackReasoningToDeepSeekRequestUsesThinkingObject(t *testing.T) {
	info := &RelayInfo{
		FallbackReasoningEffort: "max",
		ChannelMeta:             &ChannelMeta{ChannelType: constant.ChannelTypeDeepSeek},
	}
	request := &dto.GeneralOpenAIRequest{}

	info.ApplyFallbackReasoningToOpenAIRequest(request)

	if string(request.THINKING) != `{"type":"enabled"}` {
		t.Fatalf("THINKING = %s, want enabled thinking object", request.THINKING)
	}
	if request.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort = %q, want empty for DeepSeek", request.ReasoningEffort)
	}
}

func TestApplyFallbackReasoningToResponsesRequest(t *testing.T) {
	info := &RelayInfo{FallbackReasoningEffort: "low"}
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5"}

	info.ApplyFallbackReasoningToResponsesRequest(request)

	if request.Model != "gpt-5" || request.Reasoning == nil || request.Reasoning.Effort != "low" {
		t.Fatalf("unexpected Responses reasoning config: model=%q reasoning=%+v", request.Model, request.Reasoning)
	}
}

func TestApplyFallbackReasoningToGeminiRequest(t *testing.T) {
	info := &RelayInfo{FallbackReasoningEffort: "medium"}
	request := &dto.GeminiChatRequest{}

	info.ApplyFallbackReasoningToGeminiRequest(request)

	if request.GenerationConfig.ThinkingConfig == nil || request.GenerationConfig.ThinkingConfig.ThinkingLevel != "medium" {
		t.Fatalf("unexpected Gemini thinking config: %+v", request.GenerationConfig.ThinkingConfig)
	}
}

func TestApplyFallbackReasoningToClaudeRequest(t *testing.T) {
	info := &RelayInfo{FallbackReasoningEffort: "high"}
	request := &dto.ClaudeRequest{Model: "claude-sonnet-4-5"}

	info.ApplyFallbackReasoningToClaudeRequest(request)

	if request.Thinking == nil || request.Thinking.GetBudgetTokens() != 4096 {
		t.Fatalf("unexpected Claude thinking config: %+v", request.Thinking)
	}
}

func TestNormalizeFallbackReasoningEffortRejectsUnknownValue(t *testing.T) {
	if got := NormalizeFallbackReasoningEffort("turbo"); got != "" {
		t.Fatalf("NormalizeFallbackReasoningEffort() = %q, want empty", got)
	}
}
