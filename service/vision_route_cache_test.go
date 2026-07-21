package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestVisionRouteCacheExpires(t *testing.T) {
	ClearVisionRouteCache()
	createdAt := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	putVisionRouteCache("image-key", "description", createdAt)

	if got, ok := getVisionRouteCache("image-key", createdAt.Add(VisionRouteCacheTTL-time.Second)); !ok || got != "description" {
		t.Fatalf("expected cache hit before TTL, got %q, %v", got, ok)
	}
	if _, ok := getVisionRouteCache("image-key", createdAt.Add(VisionRouteCacheTTL)); ok {
		t.Fatal("expected cache miss at TTL")
	}
}

func TestVisionRouteImageCacheKeyDoesNotRetainImage(t *testing.T) {
	first := dto.MediaContent{
		Type: dto.ContentTypeImageURL,
		ImageUrl: &dto.MessageImageUrl{
			Url: "data:image/png;base64,abc",
		},
	}
	second := first
	second.ImageUrl = &dto.MessageImageUrl{Url: "data:image/png;base64,abc"}

	if firstKey, secondKey := visionRouteImageCacheKey(first), visionRouteImageCacheKey(second); firstKey == "" || firstKey != secondKey {
		t.Fatalf("expected stable image cache key, got %q and %q", firstKey, secondKey)
	}
}

func TestCalcVisionRouteFeeQuotaByCache(t *testing.T) {
	groupRatio := 1.0
	createdFee := CalcVisionRouteFeeQuotaByCache(groupRatio, 1, 0)
	hitFee := CalcVisionRouteFeeQuotaByCache(groupRatio, 0, 1)
	mixedFee := CalcVisionRouteFeeQuotaByCache(groupRatio, 1, 1)

	if createdFee != int(common.VisionRouteFixedFee*common.QuotaPerUnit) {
		t.Fatalf("unexpected creation fee: %d", createdFee)
	}
	if hitFee != int(common.VisionRouteCacheHitFee*common.QuotaPerUnit) {
		t.Fatalf("unexpected cache hit fee: %d", hitFee)
	}
	if mixedFee != createdFee+hitFee {
		t.Fatalf("unexpected mixed fee: %d", mixedFee)
	}
}
