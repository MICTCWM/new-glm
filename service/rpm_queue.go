package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// RpmQueueItem represents a request waiting in the RPM queue.
type RpmQueueItem struct {
	Group       string
	ModelName   string
	EnqueueTime time.Time
	NotifyCh    chan struct{} // closed when the request should be retried
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
func (q *RpmQueueManager) Enqueue() *RpmQueueItem {
	item := &RpmQueueItem{
		EnqueueTime: time.Now(),
		NotifyCh:    make(chan struct{}),
	}

	q.mu.Lock()
	q.queue = append(q.queue, item)
	q.mu.Unlock()

	queueLength.Add(1)
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
	queueLength.Add(-1)
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

// GetQueueLength exported for use by controller
func GetQueueLength() int {
	return GetRpmQueue().GetQueueLength()
}

// WaitWithTimeout waits for either the notify channel or a timeout.
// The timeout is configurable via common.RpmQueueTimeout (default 60s).
// Returns true if notified, false if timed out.
func (item *RpmQueueItem) WaitWithTimeout() bool {
	timeout := common.RpmQueueTimeout
	select {
	case <-item.NotifyCh:
		return true
	case <-time.After(timeout):
		// Remove from queue on timeout (best effort)
		GetRpmQueue().RemoveItem(item)
		return false
	}
}

// RemoveItem removes a specific item from the queue (used on timeout/cancel).
func (q *RpmQueueManager) RemoveItem(target *RpmQueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.queue {
		if item == target {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			queueLength.Add(-1)
			// Don't close the channel here - the goroutine might still be reading from it
			return
		}
	}
}
