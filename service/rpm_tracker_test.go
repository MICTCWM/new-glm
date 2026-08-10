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

func TestGlobalRpmTrackerCountsQueuedRequestsAsLoad(t *testing.T) {
	oldLimit := common.OverloadProtectionRPM
	common.OverloadProtectionRPM = 3
	ResetGlobalRpmTracker()
	for GetRpmQueue().GetQueueLength() > 0 {
		GetRpmQueue().Dequeue()
	}
	t.Cleanup(func() {
		common.OverloadProtectionRPM = oldLimit
		ResetGlobalRpmTracker()
		for GetRpmQueue().GetQueueLength() > 0 {
			GetRpmQueue().Dequeue()
		}
	})

	tracker := GetGlobalRpmTracker()
	// A selected channel has its own RPM cap of 2, so only 2 requests can be
	// admitted; the rest must wait in the queue. The admitted count therefore
	// stays at 2 and would never reach the overload threshold of 3 on its own.
	require.False(t, tracker.TryAcquire())
	require.False(t, tracker.TryAcquire())
	require.False(t, tracker.IsOverloaded())

	// Simulate 8 excess requests parked in the queue.
	for i := 0; i < 8; i++ {
		GetRpmQueue().Enqueue()
	}

	// Even though only 2 were admitted, the 8 queued requests mean the system
	// is actually saturated (2 admitted + 8 queued = 10 >= 3), so overload
	// protection must trigger and route to the fallback channel.
	require.True(t, tracker.IsOverloaded())
	require.True(t, tracker.TryAcquire())

	// Drain the queue so subsequent tests start clean.
	for GetRpmQueue().GetQueueLength() > 0 {
		GetRpmQueue().Dequeue()
	}
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
