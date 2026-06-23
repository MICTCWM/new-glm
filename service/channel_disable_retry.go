package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
)

type channelDisableRetryState struct {
	mu        sync.Mutex
	channelID int
	inRetry   bool
}

var retryStates sync.Map

// ChannelDisableRetryTestFunc is injected by controller to test a channel before disabling it.
var ChannelDisableRetryTestFunc func(channelError types.ChannelError) *types.NewAPIError

// TryAcquireRetrySlot 尝试获取该 channel 的检测权
// 如果已有检测在进行（inRetry == true），返回 false
// 如果无检测在进行，设置 inRetry = true，返回 true
// 防累积核心：同一 channel 多个 429 只触发一次检测
func TryAcquireRetrySlot(channelID int) bool {
	state := &channelDisableRetryState{channelID: channelID}
	actual, loaded := retryStates.LoadOrStore(channelID, state)
	if loaded {
		actualState := actual.(*channelDisableRetryState)
		actualState.mu.Lock()
		defer actualState.mu.Unlock()
		if actualState.inRetry {
			return false
		}
		actualState.inRetry = true
		return true
	}
	state.mu.Lock()
	state.inRetry = true
	state.mu.Unlock()
	return true
}

// ReleaseRetrySlot 释放检测权（无论成功失败都要确保释放）
func ReleaseRetrySlot(channelID int) {
	value, ok := retryStates.Load(channelID)
	if !ok {
		return
	}
	state := value.(*channelDisableRetryState)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.inRetry = false
}

func IsChannelDisableRetryRunning(channelID int) bool {
	value, ok := retryStates.Load(channelID)
	if !ok {
		return false
	}
	state := value.(*channelDisableRetryState)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.inRetry
}

// StartRetryCheck starts the delayed 429 confirmation flow.
// The first 429 only schedules checks. Three delayed checks must all return 429
// before the channel is auto-disabled, and duplicate schedules are ignored.
func StartRetryCheck(channelError types.ChannelError, reason string, testFn func(channelError types.ChannelError) *types.NewAPIError) {
	if testFn == nil {
		testFn = ChannelDisableRetryTestFunc
	}
	if testFn == nil {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）429 检测函数未配置，跳过延迟禁用", channelError.ChannelName, channelError.ChannelId))
		return
	}
	if !TryAcquireRetrySlot(channelError.ChannelId) {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）429 检测已在进行中，跳过本次检测", channelError.ChannelName, channelError.ChannelId))
		return
	}

	gopool.Go(func() {
		defer ReleaseRetrySlot(channelError.ChannelId)

		delays := []time.Duration{5 * time.Second, 10 * time.Second, 10 * time.Second}

		for i, delay := range delays {
			time.Sleep(delay)

			ch, err := model.CacheGetChannel(channelError.ChannelId)
			if err != nil {
				common.SysLog(fmt.Sprintf("通道「%s」（#%d）第 %d 轮检测获取渠道信息失败：%v，中断检测", channelError.ChannelName, channelError.ChannelId, i+1, err))
				return
			}
			if ch.Status != common.ChannelStatusEnabled {
				common.SysLog(fmt.Sprintf("通道「%s」（#%d）第 %d 轮检测发现渠道已被禁用，中断检测", channelError.ChannelName, channelError.ChannelId, i+1))
				return
			}

			errResult := testFn(channelError)
			if errResult == nil {
				common.SysLog(fmt.Sprintf("通道「%s」（#%d）第 %d 轮检测恢复正常，无需禁用", channelError.ChannelName, channelError.ChannelId, i+1))
				return
			}
			if errResult.StatusCode != 429 {
				common.SysLog(fmt.Sprintf("通道「%s」（#%d）第 %d 轮检测返回非 429 错误（StatusCode=%d），中断检测", channelError.ChannelName, channelError.ChannelId, i+1, errResult.StatusCode))
				return
			}
			common.SysLog(fmt.Sprintf("通道「%s」（#%d）第 %d 轮检测仍返回 429，继续等待下一轮", channelError.ChannelName, channelError.ChannelId, i+1))
		}

		common.SysLog(fmt.Sprintf("通道「%s」（#%d）三轮检测全部返回 429，即将禁用", channelError.ChannelName, channelError.ChannelId))
		if reason == "" {
			reason = "上游429限流，三轮检测后仍未恢复"
		}
		DisableChannel(channelError, reason)
	})
}
