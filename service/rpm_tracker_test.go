package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGlobalRpmTrackerTryAcquireRoutesRequestsAboveThreshold(t *testing.T) {
	oldLimit := common.OverloadProtectionRPM
	common.OverloadProtectionRPM = 2
	ResetGlobalRpmTracker()
	t.Cleanup(func() {
		common.OverloadProtectionRPM = oldLimit
		ResetGlobalRpmTracker()
	})

	tracker := GetGlobalRpmTracker()
	require.False(t, tracker.TryAcquire())
	require.False(t, tracker.TryAcquire())
	require.True(t, tracker.TryAcquire())
	require.Equal(t, 3, tracker.GetCurrentRPM())

	tracker.Decrement()
	require.Equal(t, 2, tracker.GetCurrentRPM())
	require.True(t, tracker.IsOverloaded())
}

func TestGlobalRpmTrackerDisabledAtZero(t *testing.T) {
	oldLimit := common.OverloadProtectionRPM
	common.OverloadProtectionRPM = 0
	ResetGlobalRpmTracker()
	t.Cleanup(func() {
		common.OverloadProtectionRPM = oldLimit
		ResetGlobalRpmTracker()
	})

	tracker := GetGlobalRpmTracker()
	require.False(t, tracker.TryAcquire())
	require.False(t, tracker.IsOverloaded())
}
