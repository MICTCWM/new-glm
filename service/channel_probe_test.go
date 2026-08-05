package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestIsChannelProbeErrorMatchesConfiguredMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *types.NewAPIError
		want bool
	}{
		{
			name: "exact message",
			err:  types.NewOpenAIError(errors.New(ChannelProbeErrorKeyword), types.ErrorCodeBadResponse, 503),
			want: true,
		},
		{
			name: "message embedded in response details",
			err:  types.NewOpenAIError(errors.New("upstream: Service temporarily unavailable while routing"), types.ErrorCodeBadResponse, 500),
			want: true,
		},
		{
			name: "different message",
			err:  types.NewOpenAIError(errors.New("invalid token"), types.ErrorCodeBadResponse, 401),
			want: false,
		},
		{
			name: "nil error",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsChannelProbeError(tt.err))
		})
	}
}
