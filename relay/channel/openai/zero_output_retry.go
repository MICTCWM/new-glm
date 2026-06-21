package openai

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func shouldRetryZeroOutputUsage(info *relaycommon.RelayInfo, usage *dto.Usage) bool {
	return relaycommon.ShouldRetryZeroOutputUsage(info, usage)
}

func zeroOutputRetryError(info *relaycommon.RelayInfo, usage *dto.Usage) *types.NewAPIError {
	return relaycommon.NewZeroOutputRetryError(info, usage)
}
