package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestConvertClaudeRequestPreservesMappedReasoningEffort(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName:       "client-model",
		MappedReasoningEffort: "xhigh",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-5",
		},
	}
	request := &dto.ClaudeRequest{Model: "gpt-5"}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertClaudeRequest() error = %v", err)
	}
	openAIRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if openAIRequest.ReasoningEffort != "xhigh" {
		t.Fatalf("ReasoningEffort = %q, want xhigh", openAIRequest.ReasoningEffort)
	}
}

func TestConvertOpenAIRequestRecordsReasoningEffortForRegularModel(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-4.1",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           "gpt-4.1",
		ReasoningEffort: "high",
	}

	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest() error = %v", err)
	}
	if info.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", info.ReasoningEffort)
	}
}

func TestConvertOpenAIRequestPreservesReasoningBeforeOpenRouterCleanup(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "openrouter-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: "openrouter-model",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           "openrouter-model",
		ReasoningEffort: "high",
	}

	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest() error = %v", err)
	}
	if request.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort = %q, want cleared for OpenRouter", request.ReasoningEffort)
	}
	var reasoning map[string]any
	if err := json.Unmarshal(request.Reasoning, &reasoning); err != nil {
		t.Fatalf("request.Reasoning is invalid JSON: %v", err)
	}
	if reasoning["effort"] != "high" {
		t.Fatalf("reasoning effort = %v, want high", reasoning["effort"])
	}
	if info.ReasoningEffort != "high" {
		t.Fatalf("RelayInfo.ReasoningEffort = %q, want high", info.ReasoningEffort)
	}
}

func TestConvertOpenAIRequestDisablesOpenRouterReasoningForNone(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "openrouter-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: "openrouter-model",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           "openrouter-model",
		ReasoningEffort: "none",
	}

	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest() error = %v", err)
	}
	var reasoning map[string]any
	if err := json.Unmarshal(request.Reasoning, &reasoning); err != nil {
		t.Fatalf("request.Reasoning is invalid JSON: %v", err)
	}
	if reasoning["enabled"] != false {
		t.Fatalf("reasoning enabled = %v, want false", reasoning["enabled"])
	}
	if info.ReasoningEffort != "" {
		t.Fatalf("RelayInfo.ReasoningEffort = %q, want empty", info.ReasoningEffort)
	}
}

func TestConvertOpenAIResponsesRequestRecordsReasoningEffort(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	request := dto.OpenAIResponsesRequest{
		Model:     "gpt-5",
		Reasoning: &dto.Reasoning{Effort: "medium"},
	}

	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest() error = %v", err)
	}
	if info.ReasoningEffort != "medium" {
		t.Fatalf("ReasoningEffort = %q, want medium", info.ReasoningEffort)
	}
}
