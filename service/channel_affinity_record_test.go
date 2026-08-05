package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordChannelAffinityDoesNotPersistFallbackChannel(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, setting)
	require.True(t, setting.Enabled)
	originalAllowed := append([]int(nil), setting.AllowedChannelIDs...)
	setting.AllowedChannelIDs = []int{1201}
	t.Cleanup(func() { setting.AllowedChannelIDs = originalAllowed })
	cacheKeySuffix := fmt.Sprintf("record-fallback-%d", time.Now().UnixNano())
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 1201, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKey:   channelAffinityCacheNamespace + ":" + cacheKeySuffix,
		TTLSeconds: 600,
	})
	ctx.Set("channel_id", 1202)
	ctx.Set("fallback_used", true)

	RecordChannelAffinity(ctx, 1201)

	channelID, found, err := cache.Get(cacheKeySuffix)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 1201, channelID)
}

func TestRecordChannelAffinityIgnoresChannelOutsideAllowList(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, setting)
	originalAllowed := append([]int(nil), setting.AllowedChannelIDs...)
	setting.AllowedChannelIDs = []int{1202}
	t.Cleanup(func() { setting.AllowedChannelIDs = originalAllowed })

	cacheKeySuffix := fmt.Sprintf("record-not-allowed-%d", time.Now().UnixNano())
	cache := getChannelAffinityCache()
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKey:   channelAffinityCacheNamespace + ":" + cacheKeySuffix,
		TTLSeconds: 600,
	})

	RecordChannelAffinity(ctx, 1201)

	_, found, err := cache.Get(cacheKeySuffix)
	require.NoError(t, err)
	require.False(t, found)
}

func TestChannelAffinityAllowListControlsRouteKey(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, setting)
	originalAllowed := append([]int(nil), setting.AllowedChannelIDs...)
	t.Cleanup(func() { setting.AllowedChannelIDs = originalAllowed })

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Session_id", "allow-list-test")

	setting.AllowedChannelIDs = nil
	_, ok := GetChannelAffinityRouteKey(ctx)
	require.False(t, ok)

	setting.AllowedChannelIDs = []int{1202}
	_, ok = GetChannelAffinityRouteKey(ctx)
	require.True(t, ok)
}
