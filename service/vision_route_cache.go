package service

import (
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
	"golang.org/x/sync/singleflight"
)

// VisionRouteCacheTTL controls how long a Kimi image description can be reused.
const VisionRouteCacheTTL = 30 * time.Minute

// visionRouteCacheL1TTL 是从数据库回填到内存后的较短 TTL，避免内存缓存与数据库长期不一致。
const visionRouteCacheL1TTL = 5 * time.Minute

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

	// L1: 内存缓存
	visionRouteCache.Lock()
	entry, ok := visionRouteCache.entries[key]
	if ok {
		if now.Before(entry.expiresAt) {
			visionRouteCache.Unlock()
			return entry.description, true
		}
		delete(visionRouteCache.entries, key)
	}
	visionRouteCache.Unlock()

	// L2: 数据库缓存
	dbCache, err := model.GetVisionRouteCacheByKey(key)
	if err != nil || dbCache == nil {
		return "", false
	}

	// 回填到 L1 内存缓存（使用较短的 TTL）
	visionRouteCache.Lock()
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
		description: dbCache.Description,
		expiresAt:   now.Add(visionRouteCacheL1TTL),
		createdAt:   now,
	}
	visionRouteCache.Unlock()

	// 异步更新命中信息（不阻塞主流程）
	gopool.Go(func() {
		if err := model.UpdateVisionRouteCacheHit(key); err != nil {
			common.SysError("failed to update vision route cache hit: " + err.Error())
		}
	})

	return dbCache.Description, true
}

func putVisionRouteCache(key, description, imageUrl, mimeType string, channelId int, now time.Time) {
	if key == "" || description == "" {
		return
	}

	// L1: 写入内存缓存
	visionRouteCache.Lock()
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
	visionRouteCache.Unlock()

	// L2: 写入数据库缓存
	dbCache := &model.VisionRouteImageCache{
		CacheKey:    key,
		Description: description,
		ImageUrl:    imageUrl,
		MimeType:    mimeType,
		ChannelId:   channelId,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(VisionRouteCacheTTL).Unix(),
	}
	if err := model.CreateVisionRouteCache(dbCache); err != nil {
		// 数据库写入失败不影响主流程，内存缓存已写入
		common.SysError("failed to persist vision route cache to db: " + err.Error())
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
