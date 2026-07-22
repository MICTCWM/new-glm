package model

import (
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

// VisionRouteImageCache 持久化 Kimi 生成的图片描述，进程重启后缓存不丢失。
// 缓存键为 sha256(mimeType + "\x00" + url)，与 service 层 visionRouteImageCacheKey 保持一致。
type VisionRouteImageCache struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	CacheKey    string `json:"cache_key" gorm:"type:varchar(64);uniqueIndex;not null"`
	Description string `json:"description" gorm:"type:mediumtext;not null"`
	ImageUrl    string `json:"image_url" gorm:"type:text"`
	MimeType    string `json:"mime_type" gorm:"type:varchar(128);default:''"`
	ChannelId   int    `json:"channel_id" gorm:"index;default:0"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;index;not null"`
	ExpiresAt   int64  `json:"expires_at" gorm:"bigint;index;not null"`
	LastHitAt   int64  `json:"last_hit_at" gorm:"bigint;default:0"`
	HitCount    int    `json:"hit_count" gorm:"default:0"`
}

func (VisionRouteImageCache) TableName() string {
	return "vision_route_image_caches"
}

// GetVisionRouteCacheByKey retrieves a non-expired cache entry by key.
// Returns nil if not found or expired.
func GetVisionRouteCacheByKey(cacheKey string) (*VisionRouteImageCache, error) {
	var cache VisionRouteImageCache
	now := time.Now().Unix()
	err := DB.Where("cache_key = ? AND expires_at > ?", cacheKey, now).First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// CreateVisionRouteCache creates or updates a cache entry (upsert by cache_key).
func CreateVisionRouteCache(cache *VisionRouteImageCache) error {
	// Use upsert to handle the case where an expired-but-not-yet-cleaned entry exists
	return DB.Where("cache_key = ?", cache.CacheKey).
		Assign(cache).
		FirstOrCreate(cache).Error
}

// UpdateVisionRouteCacheHit updates hit count and last hit time.
func UpdateVisionRouteCacheHit(cacheKey string) error {
	now := time.Now().Unix()
	return DB.Model(&VisionRouteImageCache{}).
		Where("cache_key = ?", cacheKey).
		Updates(map[string]interface{}{
			"hit_count":   gorm.Expr("hit_count + 1"),
			"last_hit_at": now,
		}).Error
}

// DeleteExpiredVisionRouteCaches deletes all expired cache entries.
// Returns the number of deleted rows.
func DeleteExpiredVisionRouteCaches() (int64, error) {
	now := time.Now().Unix()
	result := DB.Where("expires_at <= ?", now).Delete(&VisionRouteImageCache{})
	return result.RowsAffected, result.Error
}

// visionRouteCacheCleanupInterval 定时清理间隔：每5分钟执行一次
const visionRouteCacheCleanupInterval = 5 * time.Minute

var visionRouteCacheCleanupOnce sync.Once

// StartVisionRouteCacheCleanupTask 启动定时清理过期视觉路由缓存的后台任务（每5分钟一次）。
// 仅在主节点运行，避免多节点重复清理。
func StartVisionRouteCacheCleanupTask() {
	visionRouteCacheCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog("vision route cache cleanup task started: tick=" + visionRouteCacheCleanupInterval.String())
			ticker := time.NewTicker(visionRouteCacheCleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				deleted, err := DeleteExpiredVisionRouteCaches()
				if err != nil {
					common.SysError("failed to clean up expired vision route caches: " + err.Error())
					continue
				}
				if deleted > 0 {
					common.SysLog("cleaned up expired vision route caches: " + strconv.FormatInt(deleted, 10))
				}
			}
		})
	})
}
