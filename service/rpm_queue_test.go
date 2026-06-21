package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func newTestRpmQueue() *RpmQueueManager {
	queueLength.Store(0)
	return &RpmQueueManager{
		queue: make([]*RpmQueueItem, 0),
	}
}

func TestRpmQueueTryRemoveFrontOnlyRemovesHead(t *testing.T) {
	q := newTestRpmQueue()

	first := q.Enqueue()
	second := q.Enqueue()

	require.False(t, q.TryRemoveFront(second))
	require.Equal(t, 2, q.GetQueueLength())

	require.True(t, q.TryRemoveFront(first))
	require.Equal(t, 1, q.GetQueueLength())

	require.True(t, q.TryRemoveFront(second))
	require.Equal(t, 0, q.GetQueueLength())
}

func TestRpmQueueWaitWithTimeoutRemovesTimedOutItem(t *testing.T) {
	oldQueue := globalRpmQueue
	oldOnce := globalRpmQueueOnce
	oldTimeout := common.RpmQueueTimeout
	globalRpmQueue = nil
	globalRpmQueueOnce = sync.Once{}
	q := GetRpmQueue()
	queueLength.Store(0)
	common.RpmQueueTimeout = 20 * time.Millisecond
	defer func() {
		globalRpmQueue = oldQueue
		globalRpmQueueOnce = oldOnce
		common.RpmQueueTimeout = oldTimeout
		queueLength.Store(0)
	}()

	item := q.Enqueue()

	require.False(t, item.WaitWithTimeout())
	require.Equal(t, 0, q.GetQueueLength())
}

func TestRpmQueueDoesNotAutoDequeueWithoutRelease(t *testing.T) {
	q := newTestRpmQueue()
	item := q.Enqueue()

	time.Sleep(1100 * time.Millisecond)

	require.Equal(t, 1, q.GetQueueLength())

	q.NotifyRpmRelease()
	select {
	case <-item.NotifyCh:
	case <-time.After(time.Second):
		t.Fatal("expected rpm release to notify queued item")
	}
	require.Equal(t, 0, q.GetQueueLength())
}
