package service

import (
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// VisionRouteCacheTTL controls how long a Kimi image description can be reused.
const VisionRouteCacheTTL = 30 * time.Minute

const (
	visionRouteCacheCleanupInterval = 5 * time.Minute
	visionRouteCacheMaxEntries      = 4096
)

type visionRouteCacheEntry struct {
	description string
	expiresAt   time.Time
	createdAt   time.Time
}

var visionRouteCache = struct {
	sync.Mutex
	entries map[string]visionRouteCacheEntry
}{
	entries: make(map[string]visionRouteCacheEntry),
}

var visionRouteCacheCleanupOnce sync.Once
var visionRouteDescriptionGroup singleflight.Group

func getVisionRouteCache(key string, now time.Time) (string, bool) {
	if key == "" {
		return "", false
	}

	visionRouteCache.Lock()
	defer visionRouteCache.Unlock()

	entry, ok := visionRouteCache.entries[key]
	if !ok {
		return "", false
	}
	if !now.Before(entry.expiresAt) {
		delete(visionRouteCache.entries, key)
		return "", false
	}
	return entry.description, true
}

func putVisionRouteCache(key, description string, now time.Time) {
	if key == "" || description == "" {
		return
	}

	visionRouteCache.Lock()
	defer visionRouteCache.Unlock()

	if _, exists := visionRouteCache.entries[key]; !exists && len(visionRouteCache.entries) >= visionRouteCacheMaxEntries {
		keys := make([]string, 0, len(visionRouteCache.entries))
		for cacheKey := range visionRouteCache.entries {
			keys = append(keys, cacheKey)
		}
		sort.Slice(keys, func(i, j int) bool {
			return visionRouteCache.entries[keys[i]].createdAt.Before(visionRouteCache.entries[keys[j]].createdAt)
		})
		delete(visionRouteCache.entries, keys[0])
	}

	visionRouteCache.entries[key] = visionRouteCacheEntry{
		description: description,
		expiresAt:   now.Add(VisionRouteCacheTTL),
		createdAt:   now,
	}
}

func cleanupVisionRouteCache(now time.Time) int {
	visionRouteCache.Lock()
	defer visionRouteCache.Unlock()

	removed := 0
	for key, entry := range visionRouteCache.entries {
		if !now.Before(entry.expiresAt) {
			delete(visionRouteCache.entries, key)
			removed++
		}
	}
	return removed
}

// ClearVisionRouteCache removes all cached descriptions. It is useful for
// shutdowns, tests, and operators that need to release the cache immediately.
func ClearVisionRouteCache() {
	visionRouteCache.Lock()
	visionRouteCache.entries = make(map[string]visionRouteCacheEntry)
	visionRouteCache.Unlock()
}

// StartVisionRouteCacheCleanupTask starts the bounded in-memory cache cleanup.
// The cache stores descriptions only, never the uploaded image bytes.
func StartVisionRouteCacheCleanupTask() {
	visionRouteCacheCleanupOnce.Do(func() {
		cleanupVisionRouteCache(time.Now())
		go func() {
			ticker := time.NewTicker(visionRouteCacheCleanupInterval)
			defer ticker.Stop()
			for now := range ticker.C {
				cleanupVisionRouteCache(now)
			}
		}()
	})
}
