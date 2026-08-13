package service

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// Use high channel IDs to avoid collisions with other tests sharing the
// process-wide tracker map.
func TestConcurrencyTrackerTryAcquireRejectsWhenFull(t *testing.T) {
	channelId := 900001
	t.Cleanup(func() { RemoveConcurrencyTracker(channelId) })

	tracker := GetConcurrencyTracker(channelId, 2)
	require.True(t, tracker.TryAcquire())
	require.True(t, tracker.TryAcquire())
	require.False(t, tracker.TryAcquire())
	require.Equal(t, 2, tracker.GetCurrent())
	require.True(t, tracker.IsFull())

	tracker.Release()
	require.Equal(t, 1, tracker.GetCurrent())
	require.False(t, tracker.IsFull())
	require.True(t, tracker.TryAcquire())
	require.False(t, tracker.TryAcquire())
}

func TestConcurrencyTrackerUnlimitedWhenMaxZero(t *testing.T) {
	channelId := 900002
	t.Cleanup(func() { RemoveConcurrencyTracker(channelId) })

	tracker := GetConcurrencyTracker(channelId, 0)
	for i := 0; i < 100; i++ {
		require.True(t, tracker.TryAcquire())
	}
	require.False(t, tracker.IsFull())
	// Unlimited mode does not track counts
	require.Equal(t, 0, tracker.GetCurrent())
}

func TestConcurrencyTrackerConcurrentAcquire(t *testing.T) {
	channelId := 900003
	t.Cleanup(func() { RemoveConcurrencyTracker(channelId) })

	tracker := GetConcurrencyTracker(channelId, 5)
	var wg sync.WaitGroup
	var acquired int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tracker.TryAcquire() {
				atomic.AddInt32(&acquired, 1)
			}
		}()
	}
	wg.Wait()
	// Exactly 5 concurrent slots can be held
	require.Equal(t, int32(5), acquired)
	require.Equal(t, 5, tracker.GetCurrent())
	require.True(t, tracker.IsFull())

	for i := 0; i < 5; i++ {
		tracker.Release()
	}
	require.Equal(t, 0, tracker.GetCurrent())
	require.False(t, tracker.IsFull())
}

func TestConcurrencyTrackerUpdatesMaxConcurrency(t *testing.T) {
	channelId := 900004
	t.Cleanup(func() { RemoveConcurrencyTracker(channelId) })

	tracker := GetConcurrencyTracker(channelId, 1)
	require.True(t, tracker.TryAcquire())
	require.False(t, tracker.TryAcquire())

	// Limit raised in DB is picked up on the next lookup
	updated := GetConcurrencyTracker(channelId, 5)
	require.True(t, updated.TryAcquire())
	require.Equal(t, 2, updated.GetCurrent())
}

func TestGetCurrentConcurrencyForChannel(t *testing.T) {
	channelId := 900005
	t.Cleanup(func() { RemoveConcurrencyTracker(channelId) })

	require.Equal(t, 0, GetCurrentConcurrencyForChannel(channelId))
	tracker := GetConcurrencyTracker(channelId, 3)
	require.True(t, tracker.TryAcquire())
	require.Equal(t, 1, GetCurrentConcurrencyForChannel(channelId))

	RemoveConcurrencyTracker(channelId)
	require.Equal(t, 0, GetCurrentConcurrencyForChannel(channelId))
}

// TestConcurrencyFullRequestQueuesAndWakesOnRelease verifies the full queue
// path shared with RPM: a request rejected by a full concurrency limit joins
// the RPM queue, and releasing a slot wakes the queued request for a retry.
func TestConcurrencyFullRequestQueuesAndWakesOnRelease(t *testing.T) {
	channelId := 900006
	t.Cleanup(func() {
		RemoveConcurrencyTracker(channelId)
		for GetRpmQueue().GetQueueLength() > 0 {
			GetRpmQueue().Dequeue()
		}
	})

	tracker := GetConcurrencyTracker(channelId, 1)
	require.True(t, tracker.TryAcquire())
	// The second request cannot get a concurrency slot...
	require.False(t, tracker.TryAcquire())
	// ...so it joins the shared RPM queue, exactly like an RPM-full request.
	item := GetRpmQueue().Enqueue()
	require.Equal(t, 1, GetRpmQueue().GetQueueLength())

	// Freeing the concurrency slot wakes the queued request (the relay defer
	// calls NotifyRpmRelease for every released slot).
	tracker.Release()
	GetRpmQueue().NotifyRpmRelease()
	require.True(t, item.WaitWithTimeout())
	require.Equal(t, 0, GetRpmQueue().GetQueueLength())

	// The woken request can now acquire the freed slot.
	require.True(t, tracker.TryAcquire())
}
