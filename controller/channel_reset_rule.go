package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
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
