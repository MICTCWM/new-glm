package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// RpmQueueItem represents a request waiting in the RPM queue.
type RpmQueueItem struct {
	RequestID    string
	Username     string
	UserID       int
	Group        string
	ModelName    string
	PromptTokens int
	EnqueueTime  time.Time
	NotifyCh     chan struct{} // closed when the request should be retried
}

type RpmQueueItemMeta struct {
	RequestID    string
	Username     string
	UserID       int
	Group        string
	ModelName    string
	PromptTokens int
}

type RpmQueueSnapshotItem struct {
	RequestID    string `json:"request_id"`
	Username     string `json:"username"`
	UserID       int    `json:"user_id"`
	Group        string `json:"group"`
	ModelName    string `json:"model_name"`
	PromptTokens int    `json:"prompt_tokens"`
	EnqueueTime  int64  `json:"enqueue_time"`
	WaitSeconds  int64  `json:"wait_seconds"`
}

// RpmQueueManager manages a FIFO queue of requests waiting for RPM capacity.
type RpmQueueManager struct {
	mu    sync.Mutex
	queue []*RpmQueueItem
}

var (
	globalRpmQueue     *RpmQueueManager
	globalRpmQueueOnce sync.Once
	queueLength        atomic.Int64
)

// GetRpmQueue returns the singleton RpmQueueManager.
func GetRpmQueue() *RpmQueueManager {
	globalRpmQueueOnce.Do(func() {
		globalRpmQueue = &RpmQueueManager{
			queue: make([]*RpmQueueItem, 0),
		}
	})
	return globalRpmQueue
}

// Enqueue adds a request to the queue and returns a channel that will be
// closed when the request should be retried (i.e., RPM capacity freed up).
// Returns also a timeout channel for the caller to handle 60s timeout.
func (q *RpmQueueManager) Enqueue(meta ...RpmQueueItemMeta) *RpmQueueItem {
	item := &RpmQueueItem{
		EnqueueTime: time.Now(),
		NotifyCh:    make(chan struct{}),
	}
	if len(meta) > 0 {
		item.RequestID = meta[0].RequestID
		item.Username = meta[0].Username
		item.UserID = meta[0].UserID
		item.Group = meta[0].Group
		item.ModelName = meta[0].ModelName
		item.PromptTokens = meta[0].PromptTokens
	}

	q.mu.Lock()
	q.queue = append(q.queue, item)
	queueLength.Store(int64(len(q.queue)))
	q.mu.Unlock()

	return item
}

// Dequeue removes and returns the oldest item from the queue.
// Returns nil if the queue is empty.
func (q *RpmQueueManager) Dequeue() *RpmQueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	item := q.queue[0]
	q.queue = q.queue[1:]
	queueLength.Store(int64(len(q.queue)))
	return item
}

// NotifyRpmRelease should be called whenever an RPM slot is freed.
// It dequeues one waiting request and notifies it via its channel.
func (q *RpmQueueManager) NotifyRpmRelease() {
	item := q.Dequeue()
	if item == nil {
		return
	}
	close(item.NotifyCh)
}

// GetQueueLength returns the current number of queued requests.
func (q *RpmQueueManager) GetQueueLength() int {
	return int(queueLength.Load())
}

func (q *RpmQueueManager) Snapshot() []RpmQueueSnapshotItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	items := make([]RpmQueueSnapshotItem, 0, len(q.queue))
	for _, item := range q.queue {
		items = append(items, RpmQueueSnapshotItem{
			RequestID:    item.RequestID,
			Username:     item.Username,
			UserID:       item.UserID,
			Group:        item.Group,
			ModelName:    item.ModelName,
			PromptTokens: item.PromptTokens,
			EnqueueTime:  item.EnqueueTime.Unix(),
			WaitSeconds:  int64(now.Sub(item.EnqueueTime).Seconds()),
		})
	}
	return items
}

// GetQueueLength exported for use by controller
func GetQueueLength() int {
	return GetRpmQueue().GetQueueLength()
}

func GetQueueSnapshot() []RpmQueueSnapshotItem {
	return GetRpmQueue().Snapshot()
}

// WaitWithTimeout waits for either the notify channel or a timeout.
// The timeout is configurable via common.RpmQueueTimeout (default 60s).
// Returns true if notified (should retry), false if timed out.
func (item *RpmQueueItem) WaitWithTimeout() bool {
	timer := time.NewTimer(common.RpmQueueTimeout)
	defer timer.Stop()

	select {
	case <-item.NotifyCh:
		return true
	case <-timer.C:
		// On timeout, remove ourselves from the queue. If RemoveItem reports the
		// item was no longer in the queue, a notifier already dequeued it and is
		// about to (or did) close NotifyCh — honor that wake instead of losing it.
		if !GetRpmQueue().RemoveItem(item) {
			return true
		}
		return false
	}
}

// RemoveItem removes a specific item from the queue (used on timeout/cancel).
// Returns true if the item was found and removed, false if it was no longer
// queued (e.g. already dequeued by a notifier).
func (q *RpmQueueManager) RemoveItem(target *RpmQueueItem) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.queue {
		if item == target {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			queueLength.Store(int64(len(q.queue)))
			// Don't close the channel here - the goroutine might still be reading from it
			return true
		}
	}
	return false
}

func (q *RpmQueueManager) TryRemoveFront(target *RpmQueueItem) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 || q.queue[0] != target {
		return false
	}
	q.queue = q.queue[1:]
	queueLength.Add(-1)
	return true
}
