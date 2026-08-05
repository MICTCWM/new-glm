package model

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChannelProbeEnabledSetting(t *testing.T) {
	channel := &Channel{}
	channel.SetSetting(dto.ChannelSettings{ProbeEnabled: true})
	require.True(t, channel.IsProbeEnabled())

	channel.SetSetting(dto.ChannelSettings{ProbeEnabled: false})
	require.False(t, channel.IsProbeEnabled())
}
