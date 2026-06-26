package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	channelQuotaResetTickInterval = 1 * time.Minute
	channelQuotaResetBatchSize    = 300
)

var (
	channelQuotaResetOnce    sync.Once
	channelQuotaResetRunning atomic.Bool
)

// StartChannelQuotaResetTask 启动渠道配额重置定时任务
// 多实例部署时只在主节点运行，每分钟检查一次到期的重置规则并分批处理。
func StartChannelQuotaResetTask() {
	channelQuotaResetOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("channel quota reset task started: tick=%s", channelQuotaResetTickInterval))
			ticker := time.NewTicker(channelQuotaResetTickInterval)
			defer ticker.Stop()

			runChannelQuotaResetOnce()
			for range ticker.C {
				runChannelQuotaResetOnce()
			}
		})
	})
}

// runChannelQuotaResetOnce 单次执行渠道配额重置
// 通过 CompareAndSwap 防止上一轮任务未完成时重入。
func runChannelQuotaResetOnce() {
	if !channelQuotaResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelQuotaResetRunning.Store(false)

	ctx := context.Background()
	now := common.GetTimestamp()
	totalReset := 0
	for {
		n, err := model.ResetDueChannelRules(now, channelQuotaResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("channel quota reset task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalReset += n
		if n < channelQuotaResetBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalReset > 0 {
		logger.LogDebug(ctx, "channel quota reset: reset_count=%d", totalReset)
	}
}
