package common

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestShouldRetryZeroOutputUsageOnlyForNonStream(t *testing.T) {
	info := &RelayInfo{}
	info.SetEstimatePromptTokens(10)
	usage := &dto.Usage{
		PromptTokens:     10,
		CompletionTokens: 0,
	}

	if !ShouldRetryZeroOutputUsage(info, usage) {
		t.Fatal("expected non-stream zero output usage to retry")
	}

	info.IsStream = true
	if ShouldRetryZeroOutputUsage(info, usage) {
		t.Fatal("expected normal zero output check to skip stream requests")
	}
}

func TestShouldRetryZeroOutputUsageAfterStream(t *testing.T) {
	info := &RelayInfo{IsStream: true}
	info.SetEstimatePromptTokens(10)
	usage := &dto.Usage{
		PromptTokens:     10,
		CompletionTokens: 0,
	}

	if !ShouldRetryZeroOutputUsageAfterStream(info, usage) {
		t.Fatal("expected stream final zero output usage to retry")
	}

	usage.CompletionTokens = 1
	if ShouldRetryZeroOutputUsageAfterStream(info, usage) {
		t.Fatal("expected stream final usage with output tokens not to retry")
	}
}

func TestShouldRetryZeroOutputUsageAfterStreamUsesEstimatedPromptTokens(t *testing.T) {
	info := &RelayInfo{IsStream: true}
	info.SetEstimatePromptTokens(10)

	if !ShouldRetryZeroOutputUsageAfterStream(info, &dto.Usage{}) {
		t.Fatal("expected estimated prompt tokens to count as input tokens")
	}
}
