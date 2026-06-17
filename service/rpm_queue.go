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

// rpmQueueWakeInterval 是后台唤醒循环的轮询间隔。
// RPM 名额是通过时间戳滑出 60s 滑动窗口而释放的——这是一个纯时间事件，
// 没有任何"请求完成"回调与之对应。因此当所有请求都堵在队列里、没有正在执行的
// 请求可以触发 NotifyRpmRelease 时，必须靠这个循环主动唤醒排队请求去复查名额。
const rpmQueueWakeInterval = 1 * time.Second

// GetRpmQueue returns the singleton RpmQueueManager.
func GetRpmQueue() *RpmQueueManager {
	globalRpmQueueOnce.Do(func() {
		globalRpmQueue = &RpmQueueManager{
			queue: make([]*RpmQueueItem, 0),
		}
		// 启动后台唤醒循环，让排队请求在窗口名额过期后能够被唤醒复查。
		go globalRpmQueue.wakeLoop()
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

// wakeLoop periodically wakes all queued requests so they can re-check RPM
// capacity. This is the primary release mechanism for the sliding-window model:
// slots free up when timestamps age out of the window, a time-based event with
// no completion callback, so we poll and let woken requests re-attempt selection.
func (q *RpmQueueManager) wakeLoop() {
	ticker := time.NewTicker(rpmQueueWakeInterval)
	defer ticker.Stop()
	for range ticker.C {
		if queueLength.Load() == 0 {
			continue
		}
		q.wakeAll()
	}
}

// wakeAll dequeues and notifies every currently queued item, letting each one
// re-attempt channel selection. Items that still find no capacity will re-enqueue.
func (q *RpmQueueManager) wakeAll() {
	q.mu.Lock()
	items := q.queue
	q.queue = make([]*RpmQueueItem, 0)
	queueLength.Store(0)
	q.mu.Unlock()

	for _, item := range items {
		close(item.NotifyCh)
	}
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
