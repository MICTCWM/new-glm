package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

// ModelMonitorSample 模型监控采样数据
// 用于在模型卡片下方绘制柱形图：柱子高度=响应时间，颜色=成功/失败/无数据
type ModelMonitorSample struct {
	ModelName string `json:"model_name"`
	UseTimeMs int    `json:"use_time_ms"` // 响应时间（毫秒）。logs 表 use_time 字段单位为秒，这里已转换为毫秒
	Success   bool   `json:"success"`     // true=成功(type=2)，false=失败(type=5)
	CreatedAt int64  `json:"created_at"`  // Unix 时间戳（秒）
	HasData   bool   `json:"has_data"`    // false 表示该 60 秒时段内无请求
}

const (
	modelMonitorSampleInterval = 60 * time.Second // 采样间隔
	modelMonitorSampleMaxKeep  = 30               // 每个模型保留的样本数量
	modelMonitorSamplePickSize = 50               // 单次采样候选记录上限

	modelMonitorSampleKeyPrefix = "model_monitor:sample:" // 单模型样本 List 的 key 前缀
	modelMonitorModelsKey       = "model_monitor:models"  // 所有有采样模型的 Set key
)

var (
	modelMonitorOnce sync.Once
)

// StartModelMonitorSampleTask 启动模型监控采样任务
// 多实例部署时只在主节点运行，每 60 秒被动采集一次每个启用模型的最新用户请求样本。
// 不主动调用模型 API，仅从 logs 表抽取已有请求记录。
func StartModelMonitorSampleTask() {
	modelMonitorOnce.Do(func() {
		// 多节点部署守卫：仅主节点执行
		if !common.IsMasterNode {
			return
		}
		// 采样数据依赖 Redis 存储
		if !common.RedisEnabled || common.RDB == nil {
			common.SysError("model monitor sample task requires redis, but redis is not enabled")
			return
		}
		gopool.Go(func() {
			common.SysLog("model monitor sample task started: tick=60s")
			ticker := time.NewTicker(modelMonitorSampleInterval)
			defer ticker.Stop()

			// 启动时立即执行一次，之后每 60 秒执行一次
			runModelMonitorSampleOnce()
			for range ticker.C {
				func() {
					defer func() {
						if r := recover(); r != nil {
							common.SysError(fmt.Sprintf("[ModelMonitor] sample task panic recovered: %v", r))
						}
					}()
					runModelMonitorSampleOnce()
				}()
			}
		})
	})
}

// runModelMonitorSampleOnce 执行一次模型监控采样
// 遍历所有启用模型，从 logs 表抽取最近 60 秒内的请求样本并写入 Redis。
// 单个模型采样失败不会中断整个任务。
func runModelMonitorSampleOnce() {
	ctx := context.Background()
	now := time.Now()
	windowStart := now.Add(-modelMonitorSampleInterval).Unix()
	windowEnd := now.Unix()

	// 从 pricing 缓存获取所有启用模型
	pricing := model.GetPricing()
	if len(pricing) == 0 {
		return
	}

	for _, p := range pricing {
		modelName := p.ModelName
		if modelName == "" {
			continue
		}
		sample, err := pickModelMonitorSample(modelName, windowStart, windowEnd)
		if err != nil {
			common.SysError(fmt.Sprintf("model monitor sample pick failed for %s: %v", modelName, err))
			continue
		}
		if err := saveModelMonitorSample(ctx, modelName, sample); err != nil {
			common.SysError(fmt.Sprintf("model monitor sample save failed for %s: %v", modelName, err))
			continue
		}
	}

	// 清理 Set 中已不存在于当前 pricing 列表的模型，避免残留数据
	pricingSet := make(map[string]bool, len(pricing))
	for _, p := range pricing {
		if p.ModelName != "" {
			pricingSet[p.ModelName] = true
		}
	}
	existingMembers, err := common.RDB.SMembers(ctx, modelMonitorModelsKey).Result()
	if err != nil {
		common.SysError(fmt.Sprintf("model monitor smembers failed when cleanup: %v", err))
	} else {
		for _, member := range existingMembers {
			if !pricingSet[member] {
				common.RDB.SRem(ctx, modelMonitorModelsKey, member)
				common.RDB.Del(ctx, modelMonitorSampleKeyPrefix+member)
			}
		}
	}
}

// pickModelMonitorSample 从 logs 表抽取指定模型在时间窗口内的一个随机样本
// 查询最近 60 秒内的成功(type=2)或失败(type=5)请求记录，最多取 modelMonitorSamplePickSize 条，
// 然后在应用层随机抽取一条作为样本。若窗口内无请求，返回 has_data=false 的样本。
func pickModelMonitorSample(modelName string, windowStart, windowEnd int64) (*ModelMonitorSample, error) {
	var logs []model.Log
	err := model.LOG_DB.
		Select("use_time", "type", "created_at").
		Where("model_name = ? AND created_at >= ? AND created_at < ? AND (type = ? OR type = ?)",
			modelName, windowStart, windowEnd, model.LogTypeConsume, model.LogTypeError).
		Limit(modelMonitorSamplePickSize).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	// 该模型在 60 秒内无请求，记录 has_data=false 的样本
	if len(logs) == 0 {
		return &ModelMonitorSample{
			ModelName: modelName,
			UseTimeMs: 0,
			Success:   false,
			CreatedAt: windowEnd,
			HasData:   false,
		}, nil
	}

	// 应用层随机抽取一条
	picked := logs[rand.Intn(len(logs))]
	// logs.UseTime 字段单位为秒（参见 controller/relay.go:950 int(time.Since(startTime).Seconds())），转换为毫秒
	useTimeMs := picked.UseTime * 1000
	return &ModelMonitorSample{
		ModelName: modelName,
		UseTimeMs: useTimeMs,
		Success:   picked.Type == model.LogTypeConsume,
		CreatedAt: picked.CreatedAt,
		HasData:   true,
	}, nil
}

// saveModelMonitorSample 将样本写入 Redis，并维护模型集合
// Key 格式：model_monitor:sample:{model_name}（List 类型）
// 每次 LPUSH 一个新样本到 List 头部，LTRIM 保留最新 modelMonitorSampleMaxKeep 个元素
// 同时维护 model_monitor:models Set，记录所有有采样的模型名
func saveModelMonitorSample(ctx context.Context, modelName string, sample *ModelMonitorSample) error {
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("marshal sample: %w", err)
	}

	key := modelMonitorSampleKeyPrefix + modelName
	// LPUSH 新样本到头部
	if err := common.RDB.LPush(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("lpush sample: %w", err)
	}
	// LTRIM 保留最新 N 个
	if err := common.RDB.LTrim(ctx, key, 0, int64(modelMonitorSampleMaxKeep-1)).Err(); err != nil {
		return fmt.Errorf("ltrim sample: %w", err)
	}
	// 维护模型集合，便于批量查询
	if err := common.RDB.SAdd(ctx, modelMonitorModelsKey, modelName).Err(); err != nil {
		return fmt.Errorf("sadd model set: %w", err)
	}
	return nil
}

// GetModelMonitorSamplesFromRedis 从 Redis 读取指定模型的样本列表
// 按 created_at 升序排列（旧到新），便于前端按时间从左到右绘制柱形图。
func GetModelMonitorSamplesFromRedis(ctx context.Context, modelName string) ([]ModelMonitorSample, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, fmt.Errorf("redis is not enabled")
	}
	key := modelMonitorSampleKeyPrefix + modelName
	values, err := common.RDB.LRange(ctx, key, 0, int64(modelMonitorSampleMaxKeep-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange sample: %w", err)
	}
	samples := make([]ModelMonitorSample, 0, len(values))
	for _, v := range values {
		var s ModelMonitorSample
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

// GetAllModelMonitorSamplesFromRedis 从 Redis 读取所有模型的样本列表
// 返回 map[string][]ModelMonitorSample，key 为模型名。
// 用于前端在模型卡片网格中一次性加载所有模型的监控数据。
func GetAllModelMonitorSamplesFromRedis(ctx context.Context) (map[string][]ModelMonitorSample, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, fmt.Errorf("redis is not enabled")
	}
	modelNames, err := common.RDB.SMembers(ctx, modelMonitorModelsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers model set: %w", err)
	}
	// 防护：只保留当前 pricing 中存在的模型，避免返回已删除模型的残留数据
	pricing := model.GetPricing()
	validSet := make(map[string]bool, len(pricing))
	for _, p := range pricing {
		if p.ModelName != "" {
			validSet[p.ModelName] = true
		}
	}
	result := make(map[string][]ModelMonitorSample, len(modelNames))
	for _, name := range modelNames {
		// 跳过当前 pricing 中已不存在的模型
		if !validSet[name] {
			continue
		}
		samples, err := GetModelMonitorSamplesFromRedis(ctx, name)
		if err != nil {
			common.SysError(fmt.Sprintf("model monitor get samples failed for %s: %v", name, err))
			continue
		}
		result[name] = samples
	}
	return result, nil
}
