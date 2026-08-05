package openai

import (
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
