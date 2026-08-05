package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestConvertOpenAIRequestMapsReasoningEffortToThinking(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeDeepSeek,
			UpstreamModelName: "deepseek-chat",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           "deepseek-chat",
		ReasoningEffort: "high",
	}

	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest() error = %v", err)
	}
	if string(request.THINKING) != `{"type":"enabled"}` {
		t.Fatalf("THINKING = %s, want enabled thinking object", request.THINKING)
	}
	if request.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort = %q, want empty after conversion", request.ReasoningEffort)
	}
}
