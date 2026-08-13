package relay

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// shouldRetryUpstream reports whether the selected channel permits another
// internal upstream attempt for this error. An empty skip list means every
// error is retryable, preserving the historical default behavior.
func shouldRetryUpstream(info *relaycommon.RelayInfo, err *types.NewAPIError) bool {
	if info == nil {
		return false
	}
	var settings dto.ChannelSettings
	if info.ChannelMeta != nil {
		settings = info.ChannelMeta.ChannelSetting
	}
	if settings.DisableAutoRetry {
		return false
	}
	if err == nil {
		return true
	}

	errorCode := strings.ToLower(strings.TrimSpace(string(err.GetErrorCode())))
	statusCode := strconv.Itoa(err.StatusCode)
	for _, configured := range settings.AutoRetrySkipErrorCodes {
		configured = strings.ToLower(strings.TrimSpace(configured))
		if configured == "" {
			continue
		}
		if configured == errorCode || configured == statusCode {
			return false
		}
	}
	return true
}

func canRetryUpstream(info *relaycommon.RelayInfo, err *types.NewAPIError, attempt, maxAttempts int) bool {
	return attempt < maxAttempts && shouldRetryUpstream(info, err)
}
