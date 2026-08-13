package relay

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestShouldRetryUpstreamDefaultsToRetryAllErrors(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	err := types.NewErrorWithStatusCode(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)

	if !shouldRetryUpstream(info, err) {
		t.Fatal("expected empty skip list to retry")
	}
}

func TestShouldRetryUpstreamHandlesMissingChannelMeta(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	err := types.NewError(errors.New("upstream"), types.ErrorCodeDoRequestFailed)

	if !shouldRetryUpstream(info, err) {
		t.Fatal("expected missing channel metadata to use default retry settings")
	}
}

func TestShouldRetryUpstreamCanDisableAllRetries(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelSetting: dto.ChannelSettings{DisableAutoRetry: true},
	}}
	err := types.NewError(errors.New("upstream"), types.ErrorCodeDoRequestFailed)

	if shouldRetryUpstream(info, err) {
		t.Fatal("expected disabled auto retry to stop retries")
	}
}

func TestShouldRetryUpstreamSkipsConfiguredStatusOrErrorCode(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelSetting: dto.ChannelSettings{
			AutoRetrySkipErrorCodes: []string{"429", "channel:zero_output_tokens"},
		},
	}}

	statusErr := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	if shouldRetryUpstream(info, statusErr) {
		t.Fatal("expected configured HTTP status to stop retries")
	}

	codeErr := types.NewError(errors.New("zero output"), types.ErrorCodeChannelZeroOutputTokens)
	if shouldRetryUpstream(info, codeErr) {
		t.Fatal("expected configured internal error code to stop retries")
	}
}
