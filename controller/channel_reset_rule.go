package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// validRuleTypes 支持的规则类型白名单（与 model 层常量保持一致）
var validRuleTypes = map[string]bool{
	model.ChannelResetRuleTypeDaily:          true,
	model.ChannelResetRuleTypeWeekly:         true,
	model.ChannelResetRuleTypeMonthly:        true,
	model.ChannelResetRuleTypeCustomInterval: true,
	model.ChannelResetRuleTypeSpecificTime:   true,
}

// validateRuleType 校验规则类型是否在白名单内
func validateRuleType(ruleType string) bool {
	return validRuleTypes[ruleType]
}

// validateRuleConfig 校验 rule_config 是否为合法 JSON
func validateRuleConfig(ruleConfig string) bool {
	if ruleConfig == "" {
		return true // 空字符串允许，model 层会使用默认值
	}
	var js json.RawMessage
	return json.Unmarshal([]byte(ruleConfig), &js) == nil
}

// createChannelResetRuleRequest 创建/更新渠道重置规则的请求体
type createChannelResetRuleRequest struct {
	ChannelId  int    `json:"channel_id"`
	RuleType   string `json:"rule_type"`
	RuleConfig string `json:"rule_config"`
	ResetValue int64  `json:"reset_value"`
	Enabled    *bool  `json:"enabled"`
	Remark     string `json:"remark"`
}

// batchSetChannelResetRuleRequest 批量设置渠道重置规则的请求体
type batchSetChannelResetRuleRequest struct {
	Ids        []int  `json:"ids"`
	RuleType   string `json:"rule_type"`
	RuleConfig string `json:"rule_config"`
	ResetValue int64  `json:"reset_value"`
	Enabled    *bool  `json:"enabled"`
	Remark     string `json:"remark"`
}

// updateChannelResetRuleRequest 更新渠道重置规则的请求体（含 id）
type updateChannelResetRuleRequest struct {
	Id         int64  `json:"id"`
	ChannelId  int    `json:"channel_id"`
	RuleType   string `json:"rule_type"`
	RuleConfig string `json:"rule_config"`
	ResetValue int64  `json:"reset_value"`
	Enabled    *bool  `json:"enabled"`
	Remark     string `json:"remark"`
}

// GetChannelResetRules 获取某渠道的重置规则
// GET /api/channel/:id/reset_rules
func GetChannelResetRules(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的渠道 id"})
		return
	}
	rules, err := model.GetChannelResetRules(id)
	if err != nil {
		common.SysError("failed to get channel reset rules: " + err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

// CreateChannelResetRule 创建渠道重置规则
// POST /api/channel/reset_rule
func CreateChannelResetRule(c *gin.Context) {
	req := createChannelResetRuleRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel_id 不能为空"})
		return
	}
	if req.RuleType == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "rule_type 不能为空"})
		return
	}
	if !validateRuleType(req.RuleType) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 rule_type"})
		return
	}
	if !validateRuleConfig(req.RuleConfig) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 rule_config 格式"})
		return
	}
	// 校验渠道存在
	if _, err := model.GetChannelById(req.ChannelId, false); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel_id 对应的渠道不存在"})
		return
	}
	rule := &model.ChannelResetRule{
		ChannelId:  req.ChannelId,
		RuleType:   req.RuleType,
		RuleConfig: req.RuleConfig,
		ResetValue: req.ResetValue,
		Enabled:    true,
		Remark:     req.Remark,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if err := model.CreateChannelResetRule(rule); err != nil {
		common.SysError("failed to create channel reset rule: " + err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rule)
}

// UpdateChannelResetRule 更新渠道重置规则
// PUT /api/channel/reset_rule
func UpdateChannelResetRule(c *gin.Context) {
	req := updateChannelResetRuleRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的规则 id"})
		return
	}
	if req.ChannelId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel_id 不能为空"})
		return
	}
	if req.RuleType == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "rule_type 不能为空"})
		return
	}
	if !validateRuleType(req.RuleType) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 rule_type"})
		return
	}
	if !validateRuleConfig(req.RuleConfig) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 rule_config 格式"})
		return
	}
	// 查询现有规则，禁止变更 channel_id
	existingRule, err := model.GetChannelResetRuleById(req.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "规则不存在"})
		return
	}
	if req.ChannelId != existingRule.ChannelId {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "禁止变更 channel_id"})
		return
	}
	rule := &model.ChannelResetRule{
		Id:         req.Id,
		ChannelId:  existingRule.ChannelId,
		RuleType:   req.RuleType,
		RuleConfig: req.RuleConfig,
		ResetValue: req.ResetValue,
		Enabled:    true,
		Remark:     req.Remark,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if err := model.UpdateChannelResetRule(rule); err != nil {
		common.SysError("failed to update channel reset rule: " + err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rule)
}

// DeleteChannelResetRule 删除渠道重置规则
// DELETE /api/channel/reset_rule/:id
func DeleteChannelResetRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的规则 id"})
		return
	}
	if err := model.DeleteChannelResetRule(id); err != nil {
		common.SysError("failed to delete channel reset rule: " + err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// BatchSetChannelResetRules 批量给多个渠道创建相同规则
// POST /api/channel/batch/reset_rule
func BatchSetChannelResetRules(c *gin.Context) {
	req := batchSetChannelResetRuleRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "ids 不能为空"})
		return
	}
	if req.RuleType == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "rule_type 不能为空"})
		return
	}
	if !validateRuleType(req.RuleType) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 rule_type"})
		return
	}
	if !validateRuleConfig(req.RuleConfig) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 rule_config 格式"})
		return
	}
	template := &model.ChannelResetRule{
		RuleType:   req.RuleType,
		RuleConfig: req.RuleConfig,
		ResetValue: req.ResetValue,
		Enabled:    true,
		Remark:     req.Remark,
	}
	if req.Enabled != nil {
		template.Enabled = *req.Enabled
	}
	count, err := model.BatchSetChannelResetRules(req.Ids, template)
	if err != nil {
		common.SysError("failed to batch set channel reset rules: " + err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"count": count})
}

// setChannelQuotaConfigRequest 单渠道配额+重置时间配置
type setChannelQuotaConfigRequest struct {
	ChannelId    int   `json:"channel_id"`
	MaxCallCount int64 `json:"max_call_count"` // 总配额，0=不限
	ResetHours   []int `json:"reset_hours"`    // 重置时刻（小时，0-23），空数组=不重置
	ResetMinute  int   `json:"reset_minute"`   // 重置分钟，默认0
}

// batchSetChannelQuotaConfigRequest 批量配置
type batchSetChannelQuotaConfigRequest struct {
	Ids          []int `json:"ids"`
	MaxCallCount int64 `json:"max_call_count"`
	ResetHours   []int `json:"reset_hours"`
	ResetMinute  int   `json:"reset_minute"`
}

// validateQuotaConfigParams 校验配额配置请求参数
func validateQuotaConfigParams(maxCallCount int64, resetHours []int, resetMinute int) error {
	if maxCallCount < 0 {
		return errors.New("max_call_count 必须大于等于 0")
	}
	if resetMinute < 0 || resetMinute > 59 {
		return errors.New("reset_minute 必须在 0-59 之间")
	}
	for _, h := range resetHours {
		if h < 0 || h > 23 {
			return errors.New("reset_hours 元素必须在 0-23 之间")
		}
	}
	return nil
}

// applyChannelQuotaConfig 在事务内更新渠道总配额并重建 remark=quota_config 的 daily 重置规则，事务外同步缓存
func applyChannelQuotaConfig(channelId int, maxCallCount int64, resetHours []int, resetMinute int) error {
	hourSet := make(map[int]struct{}, len(resetHours))
	for _, h := range resetHours {
		hourSet[h] = struct{}{}
	}
	hours := make([]int, 0, len(hourSet))
	for h := range hourSet {
		hours = append(hours, h)
	}
	sort.Ints(hours)

	now := common.GetTimestamp()
	nowTime := time.Now()
	var usedCallCount int64

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var channel model.Channel
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&channel, "id = ?", channelId).Error; err != nil {
			return err
		}
		usedCallCount = channel.UsedCallCount
		if err := tx.Model(&model.Channel{}).Where("id = ?", channelId).
			Update("max_call_count", maxCallCount).Error; err != nil {
			return err
		}
		if err := model.DeleteChannelQuotaConfigRulesWithTx(tx, channelId); err != nil {
			return err
		}
		if len(hours) > 0 {
			items := make([]model.ChannelResetRule, 0, len(hours))
			for _, h := range hours {
				ruleConfig := fmt.Sprintf(`{"hour":%d,"minute":%d}`, h, resetMinute)
				next, calcErr := model.CalcNextResetTime(model.ChannelResetRuleTypeDaily, ruleConfig, nowTime)
				if calcErr != nil || next <= 0 {
					return fmt.Errorf("calc next reset time failed for hour %d: %v", h, calcErr)
				}
				items = append(items, model.ChannelResetRule{
					ChannelId:     channelId,
					RuleType:      model.ChannelResetRuleTypeDaily,
					RuleConfig:    ruleConfig,
					ResetValue:    0,
					NextResetTime: next,
					Enabled:       true,
					CreatedTime:   now,
					Remark:        "quota_config",
				})
			}
			if err := tx.CreateInBatches(items, 200).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	model.UpdateChannelCallCountInCache(channelId, usedCallCount, maxCallCount)
	return nil
}

// GetChannelQuotaConfig 获取渠道配额配置（总配额+已用+每日重置时刻）
// GET /api/channel/:id/quota_config
func GetChannelQuotaConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的渠道 id")
		return
	}
	channel, err := model.GetChannelById(id, false)
	if err != nil {
		common.SysError("failed to get channel: " + err.Error())
		common.ApiError(c, err)
		return
	}
	rules, err := model.GetChannelQuotaConfigRules(id)
	if err != nil {
		common.SysError("failed to get channel quota config rules: " + err.Error())
		common.ApiError(c, err)
		return
	}
	resetHours := make([]int, 0, len(rules))
	var resetMinute int
	for i, r := range rules {
		var cfg struct {
			Hour   int `json:"hour"`
			Minute int `json:"minute"`
		}
		if jsonErr := json.Unmarshal([]byte(r.RuleConfig), &cfg); jsonErr != nil {
			continue
		}
		resetHours = append(resetHours, cfg.Hour)
		if i == 0 {
			resetMinute = cfg.Minute
		}
	}
	common.ApiSuccess(c, gin.H{
		"max_call_count":  channel.MaxCallCount,
		"used_call_count": channel.UsedCallCount,
		"reset_hours":     resetHours,
		"reset_minute":    resetMinute,
	})
}

// SetChannelQuotaConfig 设置渠道配额配置（总配额+每日多时刻重置）
// PUT /api/channel/quota_config
func SetChannelQuotaConfig(c *gin.Context) {
	req := setChannelQuotaConfigRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelId <= 0 {
		common.ApiErrorMsg(c, "channel_id 必须大于 0")
		return
	}
	if err := validateQuotaConfigParams(req.MaxCallCount, req.ResetHours, req.ResetMinute); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if _, err := model.GetChannelById(req.ChannelId, false); err != nil {
		common.ApiErrorMsg(c, "渠道不存在")
		return
	}
	if err := applyChannelQuotaConfig(req.ChannelId, req.MaxCallCount, req.ResetHours, req.ResetMinute); err != nil {
		common.SysError("failed to set channel quota config: " + err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"max_call_count": req.MaxCallCount,
		"reset_hours":    req.ResetHours,
		"reset_minute":   req.ResetMinute,
	})
}

// BatchSetChannelQuotaConfig 批量设置渠道配额配置
// POST /api/channel/batch/quota_config
func BatchSetChannelQuotaConfig(c *gin.Context) {
	req := batchSetChannelQuotaConfigRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.Ids) == 0 {
		common.ApiErrorMsg(c, "ids 不能为空")
		return
	}
	for _, id := range req.Ids {
		if id <= 0 {
			common.ApiErrorMsg(c, "ids 中存在无效的渠道 id")
			return
		}
	}
	if err := validateQuotaConfigParams(req.MaxCallCount, req.ResetHours, req.ResetMinute); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	seen := make(map[int]bool)
	uniqueIds := make([]int, 0, len(req.Ids))
	for _, id := range req.Ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			uniqueIds = append(uniqueIds, id)
		}
	}
	successCount := 0
	for _, id := range uniqueIds {
		if err := applyChannelQuotaConfig(id, req.MaxCallCount, req.ResetHours, req.ResetMinute); err != nil {
			common.SysError(fmt.Sprintf("failed to set channel quota config for channel %d: %s", id, err.Error()))
			continue
		}
		successCount++
	}
	common.ApiSuccess(c, gin.H{"count": successCount})
}
