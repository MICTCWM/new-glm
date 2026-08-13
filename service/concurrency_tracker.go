package service

import (
	"sync"
)

// ConcurrencyTracker manages per-channel concurrency (in-flight request)
// limiting using a simple counting semaphore. maxConcurrency <= 0 means
// unlimited and TryAcquire always succeeds.
type ConcurrencyTracker struct {
	mu             sync.Mutex
	maxConcurrency int
	currentConcurr int
}

// Global tracker map: channelId -> *ConcurrencyTracker
var (
	concurrencyTrackers  = make(map[int]*ConcurrencyTracker)
	concurrencyTrackerMu sync.RWMutex
)

// GetConcurrencyTracker returns or creates the ConcurrencyTracker for a given channel.
func GetConcurrencyTracker(channelId int, maxConcurrency int) *ConcurrencyTracker {
	concurrencyTrackerMu.RLock()
	tracker, exists := concurrencyTrackers[channelId]
	concurrencyTrackerMu.RUnlock()

	if exists {
		// Update maxConcurrency in case it changed in DB
		tracker.mu.Lock()
		tracker.maxConcurrency = maxConcurrency
		tracker.mu.Unlock()
		return tracker
	}

	concurrencyTrackerMu.Lock()
	defer concurrencyTrackerMu.Unlock()

	// Double-check after acquiring write lock
	if tracker, exists := concurrencyTrackers[channelId]; exists {
		tracker.mu.Lock()
		tracker.maxConcurrency = maxConcurrency
		tracker.mu.Unlock()
		return tracker
	}

	tracker = &ConcurrencyTracker{
		maxConcurrency: maxConcurrency,
	}
	concurrencyTrackers[channelId] = tracker
	return tracker
}

// TryAcquire increments the in-flight counter if capacity is available.
// Returns true if the slot was acquired, false if at capacity.
// maxConcurrency=0 means no limit, always returns true.
func (t *ConcurrencyTracker) TryAcquire() bool {
	if t.maxConcurrency <= 0 {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.currentConcurr >= t.maxConcurrency {
		return false
	}
	t.currentConcurr++
	return true
}

// Release decrements the in-flight counter. Called when a request finishes.
func (t *ConcurrencyTracker) Release() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.currentConcurr > 0 {
		t.currentConcurr--
	}
}

// IsFull returns true if the channel concurrency has reached its maximum.
func (t *ConcurrencyTracker) IsFull() bool {
	if t.maxConcurrency <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentConcurr >= t.maxConcurrency
}

// GetCurrent returns the current in-flight request count.
func (t *ConcurrencyTracker) GetCurrent() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentConcurr
}

// GetCurrentConcurrencyForChannel is a convenience function that returns the
// current in-flight request count for a given channel ID, used by the frontend API.
func GetCurrentConcurrencyForChannel(channelId int) int {
	concurrencyTrackerMu.RLock()
	tracker, exists := concurrencyTrackers[channelId]
	concurrencyTrackerMu.RUnlock()
	if !exists {
		return 0
	}
	return tracker.GetCurrent()
}

// RemoveConcurrencyTracker removes the tracker for a channel (e.g., when channel is deleted).
func RemoveConcurrencyTracker(channelId int) {
	concurrencyTrackerMu.Lock()
	defer concurrencyTrackerMu.Unlock()
	delete(concurrencyTrackers, channelId)
}
