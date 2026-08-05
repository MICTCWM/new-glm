package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestRequestOpenAI2ClaudeMessageSupportsXHighReasoning(t *testing.T) {
	request, err := RequestOpenAI2ClaudeMessage(nil, dto.GeneralOpenAIRequest{
		Model:           "claude-sonnet-4-5",
		ReasoningEffort: "xhigh",
	})
	if err != nil {
		t.Fatalf("RequestOpenAI2ClaudeMessage() error = %v", err)
	}
	if request.Thinking == nil || request.Thinking.GetBudgetTokens() != 8192 {
		t.Fatalf("thinking = %+v, want enabled with 8192 tokens", request.Thinking)
	}
}
