package service

import (
	"sync"
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

func TestGlobalRpmTrackerIgnoresQueueItemsOutsideOverloadProtection(t *testing.T) {
	oldLimit := common.OverloadProtectionRPM
	common.OverloadProtectionRPM = 1
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
	GetRpmQueue().Enqueue(RpmQueueItemMeta{ChannelID: 10, CountsForOverload: false})
	require.False(t, tracker.IsOverloaded())

	GetRpmQueue().Enqueue(RpmQueueItemMeta{ChannelID: 20, CountsForOverload: true})
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

// TestGlobalRpmTrackerConcurrentAcquireAndQueue documents the counting
// contract: an admitted request is counted exactly once (in timestamps), and
// a queued request is counted exactly once (in the queue length). The two must
// never be combined for the same request, otherwise effectiveLoad would
// double-count and overload protection would trigger prematurely.
func TestGlobalRpmTrackerConcurrentAcquireAndQueue(t *testing.T) {
	oldLimit := common.OverloadProtectionRPM
	common.OverloadProtectionRPM = 1000
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

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			// Half simulate admitted requests, half simulate queued requests.
			if i%2 == 0 {
				tracker.TryAcquire()
			} else {
				GetRpmQueue().Enqueue()
			}
		}()
	}
	wg.Wait()

	admitted := int(tracker.currentRPM.Load())
	queued := GetRpmQueue().GetQueueLength()
	// No request is counted in both places; the sum must equal the goroutine
	// count and effectiveLoad must reflect exactly that sum.
	require.Equal(t, goroutines, admitted+queued)
	require.False(t, tracker.IsOverloaded())
}
