package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

// VendorMonitorSample 供应商监控采样数据
// 用于在供应商卡片下方绘制柱形图：柱子高度=响应时间，颜色=成功/失败/无数据
type VendorMonitorSample struct {
	VendorID   int    `json:"vendor_id"`
	VendorName string `json:"vendor_name"`
	UseTimeMs  int    `json:"use_time_ms"` // 响应时间（毫秒）。logs 表 use_time 字段单位为秒，这里已转换为毫秒
	Success    bool   `json:"success"`     // true=成功(type=2)，false=失败(type=5)
	CreatedAt  int64  `json:"created_at"`  // Unix 时间戳（秒）
	HasData    bool   `json:"has_data"`    // false 表示该 60 秒时段内无请求
}

const (
	vendorMonitorSampleInterval  = 60 * time.Second // 采样间隔
	vendorMonitorSampleMaxKeep   = 30               // 每个供应商保留的样本数量
	vendorMonitorSamplePickSize  = 50               // 单次采样候选记录上限

	vendorMonitorSampleKeyPrefix = "vendor_monitor:sample:" // 单供应商样本 List 的 key 前缀
	vendorMonitorVendorsKey      = "vendor_monitor:vendors" // 所有有采样供应商的 Set key
)

var (
	vendorMonitorOnce sync.Once
)

// StartVendorMonitorSampleTask 启动供应商监控采样任务
// 多实例部署时只在主节点运行，每 60 秒被动采集一次每个绑定了渠道的供应商的最新用户请求样本。
// 不主动调用渠道 API，仅从 logs 表抽取已有请求记录。
func StartVendorMonitorSampleTask() {
	vendorMonitorOnce.Do(func() {
		// 多节点部署守卫：仅主节点执行
		if !common.IsMasterNode {
			return
		}
		// 采样数据依赖 Redis 存储
		if !common.RedisEnabled || common.RDB == nil {
			common.SysError("vendor monitor sample task requires redis, but redis is not enabled")
			return
		}
		gopool.Go(func() {
			common.SysLog("vendor monitor sample task started: tick=60s")
			ticker := time.NewTicker(vendorMonitorSampleInterval)
			defer ticker.Stop()

			// 启动时立即执行一次，之后每 60 秒执行一次
			runVendorMonitorSampleOnce()
			for range ticker.C {
				func() {
					defer func() {
						if r := recover(); r != nil {
							common.SysError(fmt.Sprintf("[VendorMonitor] sample task panic recovered: %v", r))
						}
					}()
					runVendorMonitorSampleOnce()
				}()
			}
		})
	})
}

// runVendorMonitorSampleOnce 执行一次供应商监控采样
// 遍历所有绑定了渠道的供应商，从 logs 表抽取最近 60 秒内的请求样本并写入 Redis。
// 单个供应商采样失败不会中断整个任务。
func runVendorMonitorSampleOnce() {
	ctx := context.Background()
	now := time.Now()
	windowStart := now.Add(-vendorMonitorSampleInterval).Unix()
	windowEnd := now.Unix()

	// 从 DB 获取所有绑定了渠道的供应商（vendor 表位于主库 DB）
	vendors, err := model.GetAllVendorsWithChannels()
	if err != nil {
		common.SysError(fmt.Sprintf("vendor monitor get vendors failed: %v", err))
		return
	}
	if len(vendors) == 0 {
		return
	}

	// 记录当前存在绑定的供应商 ID 集合，用于后续清理 Set 中已不存在的 vendor
	activeVendorIDs := make(map[int]bool, len(vendors))
	for i := range vendors {
		vendor := &vendors[i]
		activeVendorIDs[vendor.Id] = true

		channelIDs := vendor.GetChannelIDList()
		// 没有绑定渠道的供应商跳过采样
		if len(channelIDs) == 0 {
			continue
		}

		sample, err := pickVendorMonitorSample(vendor.Id, vendor.Name, channelIDs, windowStart, windowEnd)
		if err != nil {
			common.SysError(fmt.Sprintf("vendor monitor sample pick failed for %d: %v", vendor.Id, err))
			continue
		}
		if err := saveVendorMonitorSample(ctx, vendor.Id, sample); err != nil {
			common.SysError(fmt.Sprintf("vendor monitor sample save failed for %d: %v", vendor.Id, err))
			continue
		}
	}

	// 清理 Set 中已不存在于当前供应商列表的 vendor，避免残留数据
	existingMembers, err := common.RDB.SMembers(ctx, vendorMonitorVendorsKey).Result()
	if err != nil {
		common.SysError(fmt.Sprintf("vendor monitor smembers failed when cleanup: %v", err))
	} else {
		for _, member := range existingMembers {
			id, parseErr := strconv.Atoi(member)
			if parseErr != nil {
				continue
			}
			if !activeVendorIDs[id] {
				common.RDB.SRem(ctx, vendorMonitorVendorsKey, member)
				common.RDB.Del(ctx, vendorMonitorSampleKeyPrefix+member)
			}
		}
	}
}

// pickVendorMonitorSample 从 logs 表抽取指定供应商在时间窗口内的一个随机样本
// 查询最近 60 秒内绑定渠道的成功(type=2)或失败(type=5)请求记录，最多取 vendorMonitorSamplePickSize 条，
// 然后在应用层随机抽取一条作为样本。若窗口内无请求，返回 has_data=false 的样本。
// 注意：logs 表位于 LOG_DB（独立数据库），通过 channel_id 字段关联到渠道
func pickVendorMonitorSample(vendorID int, vendorName string, channelIDs []int, windowStart, windowEnd int64) (*VendorMonitorSample, error) {
	if len(channelIDs) == 0 {
		return &VendorMonitorSample{
			VendorID:   vendorID,
			VendorName: vendorName,
			UseTimeMs:  0,
			Success:    false,
			CreatedAt:  windowEnd,
			HasData:    false,
		}, nil
	}

	var logs []model.Log
	err := model.LOG_DB.
		Select("use_time", "type", "created_at").
		Where("channel_id IN ? AND created_at >= ? AND created_at < ? AND (type = ? OR type = ?)",
			channelIDs, windowStart, windowEnd, model.LogTypeConsume, model.LogTypeError).
		Limit(vendorMonitorSamplePickSize).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	// 该供应商在 60 秒内无请求，记录 has_data=false 的样本
	if len(logs) == 0 {
		return &VendorMonitorSample{
			VendorID:   vendorID,
			VendorName: vendorName,
			UseTimeMs:  0,
			Success:    false,
			CreatedAt:  windowEnd,
			HasData:    false,
		}, nil
	}

	// 应用层随机抽取一条
	picked := logs[rand.Intn(len(logs))]
	// logs.UseTime 字段单位为秒（参见 controller/relay.go 中 int(time.Since(startTime).Seconds())），转换为毫秒
	useTimeMs := picked.UseTime * 1000
	return &VendorMonitorSample{
		VendorID:   vendorID,
		VendorName: vendorName,
		UseTimeMs:  useTimeMs,
		Success:    picked.Type == model.LogTypeConsume,
		CreatedAt:  picked.CreatedAt,
		HasData:    true,
	}, nil
}

// saveVendorMonitorSample 将样本写入 Redis，并维护供应商集合
// Key 格式：vendor_monitor:sample:{vendor_id}（List 类型）
// 每次 LPUSH 一个新样本到 List 头部，LTRIM 保留最新 vendorMonitorSampleMaxKeep 个元素
// 同时维护 vendor_monitor:vendors Set，记录所有有采样的供应商 ID
func saveVendorMonitorSample(ctx context.Context, vendorID int, sample *VendorMonitorSample) error {
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("marshal sample: %w", err)
	}

	key := vendorMonitorSampleKeyPrefix + strconv.Itoa(vendorID)
	// LPUSH 新样本到头部
	if err := common.RDB.LPush(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("lpush sample: %w", err)
	}
	// LTRIM 保留最新 N 个
	if err := common.RDB.LTrim(ctx, key, 0, int64(vendorMonitorSampleMaxKeep-1)).Err(); err != nil {
		return fmt.Errorf("ltrim sample: %w", err)
	}
	// 维护供应商集合，便于批量查询
	if err := common.RDB.SAdd(ctx, vendorMonitorVendorsKey, strconv.Itoa(vendorID)).Err(); err != nil {
		return fmt.Errorf("sadd vendor set: %w", err)
	}
	return nil
}

// GetVendorMonitorSamplesFromRedis 从 Redis 读取指定供应商的样本列表
// 按 created_at 升序排列（旧到新），便于前端按时间从左到右绘制柱形图。
func GetVendorMonitorSamplesFromRedis(ctx context.Context, vendorID int) ([]VendorMonitorSample, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, fmt.Errorf("redis is not enabled")
	}
	key := vendorMonitorSampleKeyPrefix + strconv.Itoa(vendorID)
	values, err := common.RDB.LRange(ctx, key, 0, int64(vendorMonitorSampleMaxKeep-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange sample: %w", err)
	}
	samples := make([]VendorMonitorSample, 0, len(values))
	for _, v := range values {
		var s VendorMonitorSample
		if err := json.Unmarshal([]byte(v), &s); err != nil {
			// 单条反序列化失败跳过，不影响其他样本
			continue
		}
		samples = append(samples, s)
	}
	// 按 created_at 升序排列（旧到新）
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].CreatedAt < samples[j].CreatedAt
	})
	return samples, nil
}

// GetAllVendorMonitorSamplesFromRedis 从 Redis 读取所有供应商的样本列表
// 返回 map[int][]VendorMonitorSample，key 为供应商 ID。
// 用于前端在供应商卡片网格中一次性加载所有供应商的监控数据。
func GetAllVendorMonitorSamplesFromRedis(ctx context.Context) (map[int][]VendorMonitorSample, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, fmt.Errorf("redis is not enabled")
	}
	vendorIDStrs, err := common.RDB.SMembers(ctx, vendorMonitorVendorsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers vendor set: %w", err)
	}

	result := make(map[int][]VendorMonitorSample, len(vendorIDStrs))
	for _, idStr := range vendorIDStrs {
		id, parseErr := strconv.Atoi(idStr)
		if parseErr != nil {
			continue
		}
		samples, err := GetVendorMonitorSamplesFromRedis(ctx, id)
		if err != nil {
			common.SysError(fmt.Sprintf("vendor monitor get samples failed for %d: %v", id, err))
			continue
		}
		result[id] = samples
	}
	return result, nil
}
