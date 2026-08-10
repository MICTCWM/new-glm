package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// RpmTracker manages per-channel RPM (Requests Per Minute) tracking
// using a sliding window of request timestamps.
type RpmTracker struct {
	mu         sync.Mutex
	timestamps []time.Time // request timestamps within the current window
	windowSize time.Duration
	maxRPM     int
	currentRPM atomic.Int64 // for fast read without lock
}

// Global tracker map: channelId -> *RpmTracker
var (
	rpmTrackers  = make(map[int]*RpmTracker)
	rpmTrackerMu sync.RWMutex
)

const RpmWindowSize = 60 * time.Second

// GlobalRpmTracker tracks requests across the channels selected for overload
// protection. Unlike a channel tracker it does not have its own limit; the
// limit is read from common.OverloadProtectionRPM so changes in System
// Behavior take effect immediately.
type GlobalRpmTracker struct {
	mu         sync.Mutex
	timestamps []time.Time
	windowSize time.Duration
	currentRPM atomic.Int64
}

var globalRpmTracker = &GlobalRpmTracker{
	timestamps: make([]time.Time, 0, 128),
	windowSize: RpmWindowSize,
}

func GetGlobalRpmTracker() *GlobalRpmTracker {
	return globalRpmTracker
}

// effectiveLoad returns the number of in-flight selected-channel requests that
// count against the overload threshold. It is the sum of requests already
// admitted (recorded in t.timestamps) and requests currently waiting in the
// RPM queue. Without counting the queue, the global tracker would never see
// pressure that is absorbed by per-channel RPM limits: when a channel's own
// RPM cap is below OverloadProtectionRPM, every excess request is parked in the
// queue and the admitted count can never reach the threshold, so overload
// protection would silently never trigger.
func (t *GlobalRpmTracker) effectiveLoad() int {
	return len(t.timestamps) + GetRpmQueue().GetQueueLength()
}

// IsOverloaded reports whether the next selected-channel request would exceed
// the configured RPM threshold, counting both admitted and queued requests.
// A non-positive threshold disables the feature.
func (t *GlobalRpmTracker) IsOverloaded() bool {
	limit := common.OverloadProtectionRPM
	if limit <= 0 {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanupExpired()
	return t.effectiveLoad() >= limit
}

// TryAcquire records a request and reports whether it was above the
// configured threshold. The check and increment are atomic, so concurrent
// requests cannot all observe the same available slot. The threshold check
// accounts for queued requests so overload protection triggers as soon as the
// system is actually saturated (admitted + queued), not only when admitted
// requests alone cross the limit.
func (t *GlobalRpmTracker) TryAcquire() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanupExpired()
	overloaded := common.OverloadProtectionRPM > 0 && t.effectiveLoad() >= common.OverloadProtectionRPM
	t.timestamps = append(t.timestamps, time.Now())
	t.currentRPM.Store(int64(len(t.timestamps)))
	return overloaded
}

// TryIncrement records an admitted request in the selected-channel sliding window.
// Deprecated: use TryAcquire so the threshold check and increment are atomic.
func (t *GlobalRpmTracker) TryIncrement() {
	_ = t.TryAcquire()
}

// Decrement releases a request when its relay attempt completes. This keeps
// the global tracker consistent with the existing per-channel RPM tracker.
func (t *GlobalRpmTracker) Decrement() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanupExpired()
	if len(t.timestamps) > 0 {
		t.timestamps = t.timestamps[1:]
	}
	t.currentRPM.Store(int64(len(t.timestamps)))
}

func (t *GlobalRpmTracker) GetCurrentRPM() int {
	return int(t.currentRPM.Load())
}

func (t *GlobalRpmTracker) cleanupExpired() {
	cutoff := time.Now().Add(-t.windowSize)
	validIdx := 0
	for validIdx < len(t.timestamps) && t.timestamps[validIdx].Before(cutoff) {
		validIdx++
	}
	if validIdx > 0 {
		n := copy(t.timestamps, t.timestamps[validIdx:])
		t.timestamps = t.timestamps[:n]
	}
}

// ResetGlobalRpmTracker clears global state. It is primarily useful for
// tests and for process-level maintenance operations.
func ResetGlobalRpmTracker() {
	globalRpmTracker.mu.Lock()
	defer globalRpmTracker.mu.Unlock()
	globalRpmTracker.timestamps = globalRpmTracker.timestamps[:0]
	globalRpmTracker.currentRPM.Store(0)
}

// GetRpmTracker returns or creates the RpmTracker for a given channel.
func GetRpmTracker(channelId int, maxRPM int) *RpmTracker {
	rpmTrackerMu.RLock()
	tracker, exists := rpmTrackers[channelId]
	rpmTrackerMu.RUnlock()

	if exists {
		// Update maxRPM in case it changed in DB
		tracker.mu.Lock()
		tracker.maxRPM = maxRPM
		tracker.mu.Unlock()
		return tracker
	}

	rpmTrackerMu.Lock()
	defer rpmTrackerMu.Unlock()

	// Double-check after acquiring write lock
	if tracker, exists := rpmTrackers[channelId]; exists {
		tracker.mu.Lock()
		tracker.maxRPM = maxRPM
		tracker.mu.Unlock()
		return tracker
	}

	tracker = &RpmTracker{
		timestamps: make([]time.Time, 0, 128),
		windowSize: RpmWindowSize,
		maxRPM:     maxRPM,
	}
	tracker.currentRPM.Store(0)
	rpmTrackers[channelId] = tracker
	return tracker
}

// init registers the RPM check hook for the channel cache.
func init() {
	model.CheckChannelRpmFullFunc = func(channelId int) bool {
		// Get tracker without modifying its maxRPM
		rpmTrackerMu.RLock()
		tracker, exists := rpmTrackers[channelId]
		rpmTrackerMu.RUnlock()
		if !exists {
			return false
		}
		return tracker.IsFull()
	}
}

// RemoveRpmTracker removes the tracker for a channel (e.g., when channel is deleted).
func RemoveRpmTracker(channelId int) {
	rpmTrackerMu.Lock()
	defer rpmTrackerMu.Unlock()
	delete(rpmTrackers, channelId)
}

// cleanupExpired removes timestamps older than windowSize from the slice.
// Must be called while holding t.mu.
func (t *RpmTracker) cleanupExpired(now time.Time) {
	cutoff := now.Add(-t.windowSize)
	// Find the first timestamp that is still valid
	validIdx := 0
	for validIdx < len(t.timestamps) && t.timestamps[validIdx].Before(cutoff) {
		validIdx++
	}
	if validIdx > 0 {
		// Shift remaining elements (avoids re-allocation)
		n := copy(t.timestamps, t.timestamps[validIdx:])
		t.timestamps = t.timestamps[:n]
	}
}

// TryIncrement attempts to add a new request to the RPM counter.
// Returns true if the request was accepted (under maxRPM), false if at capacity.
// maxRPM=0 means no limit, always returns true.
func (t *RpmTracker) TryIncrement() bool {
	if t.maxRPM <= 0 {
		// For unlimited RPM, we don't track individual timestamps to avoid memory overhead
		// The currentRPM counter is not used in unlimited mode
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.cleanupExpired(now)

	currentCount := len(t.timestamps)
	if currentCount >= t.maxRPM {
		return false
	}

	t.timestamps = append(t.timestamps, now)
	t.currentRPM.Store(int64(len(t.timestamps)))
	return true
}

// Decrement releases one RPM slot. Called when a request finishes (success or final failure after retries).
// Note: This method removes the oldest timestamp from the window, as we cannot track which specific
// timestamp corresponds to which request. This is an approximation that works well for high-throughput scenarios.
func (t *RpmTracker) Decrement() {
	// Skip decrement if maxRPM is unlimited (no tracking)
	if t.maxRPM <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.cleanupExpired(now)

	// Remove the oldest timestamp (approximation)
	if len(t.timestamps) > 0 {
		t.timestamps = t.timestamps[1:]
	}
	t.currentRPM.Store(int64(len(t.timestamps)))
}

// GetCurrentRPM returns the current request count within the sliding window.
func (t *RpmTracker) GetCurrentRPM() int {
	// Fast path: use atomic counter (approximate)
	return int(t.currentRPM.Load())
}

// GetUsageRatio returns the current RPM usage ratio (0.0 to 1.0).
// Returns 0 if maxRPM is 0 (unlimited).
func (t *RpmTracker) GetUsageRatio() float64 {
	if t.maxRPM <= 0 {
		return 0
	}
	current := float64(t.GetCurrentRPM())
	if current <= 0 {
		return 0
	}
	ratio := current / float64(t.maxRPM)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return ratio
}

// IsFull returns true if the channel RPM has reached its maximum.
func (t *RpmTracker) IsFull() bool {
	if t.maxRPM <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanupExpired(time.Now())
	t.currentRPM.Store(int64(len(t.timestamps)))
	return len(t.timestamps) >= t.maxRPM
}

// GetCurrentRpmForChannel is a convenience function that returns the current RPM count
// for a given channel ID, used by the frontend API.
func GetCurrentRpmForChannel(channelId int) int {
	rpmTrackerMu.RLock()
	tracker, exists := rpmTrackers[channelId]
	rpmTrackerMu.RUnlock()
	if !exists {
		return 0
	}
	return tracker.GetCurrentRPM()
}

// GetRpmUsageRatioForChannel returns the usage ratio for a given channel ID.
func GetRpmUsageRatioForChannel(channelId int) float64 {
	rpmTrackerMu.RLock()
	tracker, exists := rpmTrackers[channelId]
	rpmTrackerMu.RUnlock()
	if !exists {
		return 0
	}
	return tracker.GetUsageRatio()
}
