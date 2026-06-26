package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// 渠道配额重置规则类型常量
const (
	ChannelResetRuleTypeDaily          = "daily"          // 每天重置
	ChannelResetRuleTypeWeekly         = "weekly"         // 每周重置
	ChannelResetRuleTypeMonthly        = "monthly"        // 每月重置
	ChannelResetRuleTypeCustomInterval = "custom_interval" // 自定义间隔
	ChannelResetRuleTypeSpecificTime   = "specific_time"   // 一次性定点
)

// ChannelResetRule 渠道配额重置规则
// 为渠道设置一条或多条重置规则，到期后自动把 used_call_count 重置为 0，
// 并可选通过 ResetValue 更新 max_call_count。
type ChannelResetRule struct {
	Id            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelId     int    `json:"channel_id" gorm:"index;not null"`
	RuleType      string `json:"rule_type" gorm:"type:varchar(32);not null"` // daily, weekly, monthly, custom_interval, specific_time
	RuleConfig    string `json:"rule_config" gorm:"type:text"`               // JSON 存配置
	ResetValue    int64  `json:"reset_value" gorm:"bigint;default:0"`        // 重置后的 max_call_count，0/-1=保持不变，>0=更新为新值
	NextResetTime int64  `json:"next_reset_time" gorm:"bigint;index;default:0"` // 下次到期时间（unix秒）
	LastResetTime int64  `json:"last_reset_time" gorm:"bigint;default:0"`
	Enabled       bool   `json:"enabled" gorm:"default:true"`
	CreatedTime   int64  `json:"created_time" gorm:"bigint"`
	Remark        string `json:"remark" gorm:"type:varchar(255)"`
}

// channelResetRuleConfig 重置规则配置（解析 RuleConfig JSON 用）
type channelResetRuleConfig struct {
	Hour            int   `json:"hour"`             // daily/weekly/monthly 使用
	Minute          int   `json:"minute"`           // daily/weekly/monthly 使用
	Weekday         int   `json:"weekday"`          // weekly 使用，0=周日
	DayOfMonth      int   `json:"day_of_month"`     // monthly 使用，1-31
	IntervalSeconds int64 `json:"interval_seconds"` // custom_interval 使用
	SpecificTime    int64 `json:"specific_time"`    // specific_time 使用
}

func (r *ChannelResetRule) TableName() string {
	return "channel_reset_rules"
}

// CreateChannelResetRule 创建一条渠道重置规则
func CreateChannelResetRule(rule *ChannelResetRule) error {
	if rule == nil {
		return errors.New("rule is nil")
	}
	if rule.ChannelId <= 0 {
		return errors.New("channel_id is required")
	}
	if rule.CreatedTime == 0 {
		rule.CreatedTime = common.GetTimestamp()
	}
	// 创建时若未设置 next_reset_time，则根据规则计算一次
	if rule.NextResetTime == 0 && rule.Enabled {
		next, err := CalcNextResetTime(rule.RuleType, rule.RuleConfig, time.Now())
		if err == nil && next > 0 {
			rule.NextResetTime = next
		}
	}
	return DB.Create(rule).Error
}

// GetChannelResetRules 获取某渠道的所有重置规则
func GetChannelResetRules(channelId int) ([]ChannelResetRule, error) {
	var rules []ChannelResetRule
	if channelId <= 0 {
		return rules, errors.New("invalid channel id")
	}
	err := DB.Where("channel_id = ?", channelId).Order("id asc").Find(&rules).Error
	return rules, err
}

// GetAllChannelResetRules 获取所有渠道重置规则（管理用）
// 添加 Limit 避免数据量过大时 OOM；调用方如需全量数据可自行分页。
func GetAllChannelResetRules() ([]ChannelResetRule, error) {
	var rules []ChannelResetRule
	err := DB.Order("next_reset_time asc, id asc").Limit(5000).Find(&rules).Error
	return rules, err
}

// GetChannelResetRuleById 根据 id 查询单条渠道重置规则
func GetChannelResetRuleById(id int64) (*ChannelResetRule, error) {
	if id <= 0 {
		return nil, errors.New("invalid rule id")
	}
	var rule ChannelResetRule
	if err := DB.First(&rule, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateChannelResetRule 更新一条渠道重置规则
// 注意：使用 Updates + map 只更新业务字段，避免 GORM Save 把 NextResetTime/LastResetTime/CreatedTime 等零值字段覆盖为 0
func UpdateChannelResetRule(rule *ChannelResetRule) error {
	if rule == nil || rule.Id <= 0 {
		return errors.New("invalid rule id")
	}
	updates := map[string]interface{}{
		"rule_type":   rule.RuleType,
		"rule_config": rule.RuleConfig,
		"reset_value": rule.ResetValue,
		"enabled":     rule.Enabled,
		"remark":      rule.Remark,
	}
	// 修改规则类型或配置时，无论 next_reset_time 当前是否为零，都重新计算
	// 对于 custom_interval/specific_time 类型，fromTime 使用当前时间确保基准正确
	if rule.Enabled {
		if next, err := CalcNextResetTime(rule.RuleType, rule.RuleConfig, time.Now()); err == nil && next > 0 {
			updates["next_reset_time"] = next
		}
	} else {
		// 禁用规则时清除 next_reset_time，避免启用时沿用旧的过期值
		updates["next_reset_time"] = 0
	}
	return DB.Model(&ChannelResetRule{}).Where("id = ?", rule.Id).Updates(updates).Error
}

// DeleteChannelResetRule 根据 id 删除一条渠道重置规则
func DeleteChannelResetRule(id int64) error {
	if id <= 0 {
		return errors.New("invalid rule id")
	}
	return DB.Where("id = ?", id).Delete(&ChannelResetRule{}).Error
}

// DeleteChannelResetRulesByChannelId 删除某渠道的所有重置规则
func DeleteChannelResetRulesByChannelId(channelId int) error {
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	return DB.Where("channel_id = ?", channelId).Delete(&ChannelResetRule{}).Error
}

// DeleteChannelResetRulesByChannelIdWithTx 在事务中删除某渠道的所有重置规则
func DeleteChannelResetRulesByChannelIdWithTx(tx *gorm.DB, channelId int) error {
	if channelId <= 0 {
		return errors.New("invalid channel id")
	}
	return tx.Where("channel_id = ?", channelId).Delete(&ChannelResetRule{}).Error
}

// BatchSetChannelResetRules 批量给多个渠道创建相同规则（为每个 channelId 插入一条）
// rule 提供规则模板（不含 ChannelId/Id/CreatedTime），channelIds 提供目标渠道列表
// 返回实际插入的规则条数（跳过 channelId <= 0 的无效项）
func BatchSetChannelResetRules(channelIds []int, rule *ChannelResetRule) (int, error) {
	if rule == nil {
		return 0, errors.New("rule is nil")
	}
	if len(channelIds) == 0 {
		return 0, nil
	}
	now := common.GetTimestamp()
	// 预先计算下次重置时间（所有渠道规则相同，共用一次计算结果）
	var nextReset int64
	if rule.Enabled {
		next, err := CalcNextResetTime(rule.RuleType, rule.RuleConfig, time.Now())
		if err == nil && next > 0 {
			nextReset = next
		}
	}
	tx := DB.Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	inserted := 0
	var items []ChannelResetRule
	for _, channelId := range channelIds {
		if channelId <= 0 {
			continue
		}
		items = append(items, ChannelResetRule{
			ChannelId:     channelId,
			RuleType:      rule.RuleType,
			RuleConfig:    rule.RuleConfig,
			ResetValue:    rule.ResetValue,
			NextResetTime: nextReset,
			Enabled:       rule.Enabled,
			CreatedTime:   now,
			Remark:        rule.Remark,
		})
		inserted++
	}
	if len(items) == 0 {
		tx.Rollback()
		return 0, nil
	}
	// 批量插入，减少 SQL 语句数量
	if err := tx.CreateInBatches(items, 200).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return inserted, nil
}

// parseResetRuleConfig 解析 RuleConfig JSON
func parseResetRuleConfig(ruleConfig string) (*channelResetRuleConfig, error) {
	cfg := &channelResetRuleConfig{}
	if ruleConfig == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(ruleConfig), cfg); err != nil {
		return nil, fmt.Errorf("invalid rule_config: %v", err)
	}
	return cfg, nil
}

// CalcNextResetTime 根据规则类型和配置计算下次重置时间
// 时间计算使用 fromTime 的时区（与 model/subscription.go 中 calcNextResetTime 一致），
// 调用方通常传入 time.Now()，因此实际使用本机 Local 时区，便于运维按本地时间理解重置时刻。
func CalcNextResetTime(ruleType string, ruleConfig string, fromTime time.Time) (int64, error) {
	cfg, err := parseResetRuleConfig(ruleConfig)
	if err != nil {
		return 0, err
	}
	loc := fromTime.Location()
	if loc == time.UTC {
		// 与 subscription 模块保持一致，使用 Local 时区表达“每天3点”等概念
		loc = time.Local
	}
	switch ruleType {
	case ChannelResetRuleTypeDaily:
		return calcNextDaily(loc, fromTime, cfg.Hour, cfg.Minute), nil
	case ChannelResetRuleTypeWeekly:
		return calcNextWeekly(loc, fromTime, cfg.Weekday, cfg.Hour, cfg.Minute), nil
	case ChannelResetRuleTypeMonthly:
		return calcNextMonthly(loc, fromTime, cfg.DayOfMonth, cfg.Hour, cfg.Minute), nil
	case ChannelResetRuleTypeCustomInterval:
		if cfg.IntervalSeconds <= 0 {
			return 0, errors.New("interval_seconds must be > 0")
		}
		return fromTime.Add(time.Duration(cfg.IntervalSeconds) * time.Second).Unix(), nil
	case ChannelResetRuleTypeSpecificTime:
		if cfg.SpecificTime <= 0 {
			return 0, errors.New("specific_time must be > 0")
		}
		return cfg.SpecificTime, nil
	default:
		return 0, fmt.Errorf("unknown rule_type: %s", ruleType)
	}
}

// calcNextDaily 计算下一天的 hour:minute（若今日该时刻尚未到达则取今日）
func calcNextDaily(loc *time.Location, from time.Time, hour, minute int) int64 {
	next := time.Date(from.Year(), from.Month(), from.Day(), hour, minute, 0, 0, loc)
	if !next.After(from) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Unix()
}

// calcNextWeekly 计算下个指定 weekday 的 hour:minute
// weekday: 0=周日, 1=周一 ... 6=周六
func calcNextWeekly(loc *time.Location, from time.Time, weekday, hour, minute int) int64 {
	if weekday < 0 || weekday > 6 {
		weekday = 0
	}
	target := time.Date(from.Year(), from.Month(), from.Day(), hour, minute, 0, 0, loc)
	diff := (weekday - int(from.Weekday()) + 7) % 7
	target = target.AddDate(0, 0, diff)
	if !target.After(from) {
		target = target.AddDate(0, 0, 7)
	}
	return target.Unix()
}

// calcNextMonthly 计算下个月（或当月）指定 day_of_month 的 hour:minute
// day_of_month 超过当月天数时，跳到下月同日（AddDate 自动处理跨月）
func calcNextMonthly(loc *time.Location, from time.Time, dayOfMonth, hour, minute int) int64 {
	if dayOfMonth < 1 {
		dayOfMonth = 1
	}
	if dayOfMonth > 31 {
		dayOfMonth = 31
	}
	year, month := from.Year(), from.Month()
	// 先尝试当月
	target := time.Date(year, month, dayOfMonth, hour, minute, 0, 0, loc)
	// 若当月没有该日（如2月30日），Go 会自动溢出到下月，这里直接接受溢出结果
	if !target.After(from) {
		target = target.AddDate(0, 1, 0)
	}
	return target.Unix()
}

// ResetDueChannelRules 找出 next_reset_time <= now 的 enabled 规则，分批处理：
// 对每条规则，重置对应渠道的 used_call_count=0（若 reset_value>0 则同时更新 max_call_count），
// 更新该规则的 last_reset_time=now、next_reset_time=新计算值（specific_time 类型则 enabled=false）。
// 返回本次实际处理的规则条数。
func ResetDueChannelRules(now int64, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	var rules []ChannelResetRule
	if err := DB.Where("enabled = ? AND next_reset_time > 0 AND next_reset_time <= ?", true, now).
		Order("next_reset_time asc, id asc").
		Limit(limit).
		Find(&rules).Error; err != nil {
		return 0, err
	}
	if len(rules) == 0 {
		return 0, nil
	}
	resetCount := 0
	resetFailedCount := 0
	fromTime := time.Unix(now, 0)
	for i := range rules {
		rule := &rules[i]
		if err := applyChannelResetRule(rule, now, fromTime); err != nil {
			common.SysLog(fmt.Sprintf("channel reset rule apply failed: rule_id=%d, channel_id=%d, error=%v", rule.Id, rule.ChannelId, err))
			resetFailedCount++
			continue
		}
		resetCount++
	}
	// 若本轮处理了 limit 条规则但全部失败，next_reset_time 不会更新，会导致死循环。
	// 此时短路返回避免下一次查询到同一批规则反复失败重试。
	if resetCount == 0 && resetFailedCount > 0 {
		return 0, fmt.Errorf("all %d due rules failed to reset, skipping this round", resetFailedCount)
	}
	return resetCount, nil
}

// applyChannelResetRule 执行单条规则的渠道重置，并更新规则自身的下次到期时间
// 使用事务包裹渠道更新和规则更新，对规则行加 FOR UPDATE 锁避免并发冲突；
// 事务成功后同步内存缓存，避免最长 60s 内渠道仍因旧计数不可用。
func applyChannelResetRule(rule *ChannelResetRule, now int64, fromTime time.Time) error {
	// 在事务中完成：锁定规则行 + 验证并更新渠道计数 + 更新规则状态
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 锁定规则行，避免并发重置同一规则
		var lockedRule ChannelResetRule
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&lockedRule, rule.Id).Error; err != nil {
			return err
		}
		// 验证渠道是否存在
		var channel Channel
		if err := tx.First(&channel, "id = ?", rule.ChannelId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("channel not found: id=%d", rule.ChannelId)
			}
			return err
		}
		// 更新渠道的 used_call_count（可选更新 max_call_count）
		channelUpdates := map[string]interface{}{
			"used_call_count": 0,
		}
		if rule.ResetValue > 0 {
			channelUpdates["max_call_count"] = rule.ResetValue
		}
		if err := tx.Model(&Channel{}).Where("id = ?", rule.ChannelId).Updates(channelUpdates).Error; err != nil {
			return err
		}
		// 更新规则自身的状态
		ruleUpdates := map[string]interface{}{
			"last_reset_time": now,
		}
		if rule.RuleType == ChannelResetRuleTypeSpecificTime {
			// 一次性定点规则：触发后禁用
			ruleUpdates["enabled"] = false
			ruleUpdates["next_reset_time"] = 0
		} else {
			next, err := CalcNextResetTime(rule.RuleType, rule.RuleConfig, fromTime)
			if err != nil || next <= 0 {
				// 计算失败则禁用规则，避免反复触发
				ruleUpdates["enabled"] = false
				ruleUpdates["next_reset_time"] = 0
				common.SysLog(fmt.Sprintf("channel reset rule calc next failed, disabled: rule_id=%d, channel_id=%d, error=%v", rule.Id, rule.ChannelId, err))
			} else {
				ruleUpdates["next_reset_time"] = next
			}
		}
		if err := tx.Model(&ChannelResetRule{}).Where("id = ?", rule.Id).Updates(ruleUpdates).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务成功后同步内存缓存（在事务外，避免锁竞争）
	// 若事务提交后、缓存更新前系统崩溃，缓存会在最长 60s 内通过 SyncChannelCache 自动刷新，最终一致性可接受
	var maxCallCount int64 = 0
	if rule.ResetValue > 0 {
		maxCallCount = rule.ResetValue
	}
	UpdateChannelCallCountInCache(rule.ChannelId, 0, maxCallCount)
	return nil
}
