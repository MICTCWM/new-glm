package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]rateLimitEntry
	mutex              sync.Mutex
	expirationDuration time.Duration
	sequence           uint64
}

type rateLimitEntry struct {
	timestamp int64
	token     uint64
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	if l.store == nil {
		l.store = make(map[string]*[]rateLimitEntry)
		l.expirationDuration = expirationDuration
		if expirationDuration > 0 {
			go l.clearExpiredItems()
		}
	}
	l.mutex.Unlock()
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key := range l.store {
			queue := l.store[key]
			size := len(*queue)
			if size == 0 || now-(*queue)[size-1].timestamp > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	_, allowed := l.RequestToken(key, maxRequestNum, duration)
	return allowed
}

// RequestToken reserves a slot and returns a token that can be rolled back if
// the request later fails. This is required for success-only limits: a failed
// request must not consume a successful-request slot.
func (l *InMemoryRateLimiter) RequestToken(key string, maxRequestNum int, duration int64) (uint64, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if maxRequestNum <= 0 {
		return 0, true
	}
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		firstLive := 0
		for firstLive < len(*queue) && now-(*queue)[firstLive].timestamp >= duration {
			firstLive++
		}
		if firstLive > 0 {
			*queue = (*queue)[firstLive:]
		}
		if len(*queue) >= maxRequestNum {
			return 0, false
		}
	} else {
		s := make([]rateLimitEntry, 0, maxRequestNum)
		l.store[key] = &s
	}
	l.sequence++
	entry := rateLimitEntry{timestamp: now, token: l.sequence}
	*l.store[key] = append(*l.store[key], entry)
	return entry.token, true
}

// Rollback removes a previously reserved slot. Unknown tokens are ignored so
// cleanup remains safe when a request was already expired.
func (l *InMemoryRateLimiter) Rollback(key string, token uint64) {
	if token == 0 {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	queue := l.store[key]
	if queue == nil {
		return
	}
	for i, entry := range *queue {
		if entry.token == token {
			*queue = append((*queue)[:i], (*queue)[i+1:]...)
			return
		}
	}
}
