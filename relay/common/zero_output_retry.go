package common

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

func ShouldRetryZeroOutputUsage(info *RelayInfo, usage *dto.Usage) bool {
	if info == nil || usage == nil || info.IsStream {
		return false
	}
	return hasZeroOutputUsage(info, usage)
}

func ShouldRetryZeroOutputUsageAfterStream(info *RelayInfo, usage *dto.Usage) bool {
	if info == nil || usage == nil || !info.IsStream {
		return false
	}
	// 客户端主动断开导致流提前结束，不应触发零输出重试
	if info.StreamStatus != nil && info.StreamStatus.EndReason == StreamEndReasonClientGone {
		return false
	}
	return hasZeroOutputUsage(info, usage)
}

func hasZeroOutputUsage(info *RelayInfo, usage *dto.Usage) bool {
	inputTokens := usage.PromptTokens
	if inputTokens == 0 {
		inputTokens = usage.InputTokens
	}
	if inputTokens == 0 && info.GetEstimatePromptTokens() > 0 {
		inputTokens = info.GetEstimatePromptTokens()
	}
	outputTokens := usage.CompletionTokens
	if outputTokens == 0 {
		outputTokens = usage.OutputTokens
	}
	return inputTokens > 0 && outputTokens == 0
}

func NewZeroOutputRetryError(info *RelayInfo, usage *dto.Usage) *types.NewAPIError {
	inputTokens := 0
	if usage != nil {
		inputTokens = usage.PromptTokens
		if inputTokens == 0 {
			inputTokens = usage.InputTokens
		}
	}
	if inputTokens == 0 && info != nil {
		inputTokens = info.GetEstimatePromptTokens()
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("upstream returned zero output tokens, input_tokens=%d", inputTokens),
		types.ErrorCodeChannelZeroOutputTokens,
		http.StatusBadGateway,
		types.ErrOptionWithNoRecordErrorLog(),
	)
}
