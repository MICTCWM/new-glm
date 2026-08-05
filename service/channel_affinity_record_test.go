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
