package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- Shared types ----

type SubscriptionPlanDTO struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

type BillingPreferenceRequest struct {
	BillingPreference string `json:"billing_preference"`
}

// ---- User APIs ----

func GetSubscriptionPlans(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiSuccess(c, []SubscriptionPlanDTO{})
		return
	}

	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

func GetSubscriptionSelf(c *gin.Context) {
	userId := c.GetInt("id")
	settingMap, _ := model.GetUserSetting(userId, false)
	pref := common.NormalizeBillingPreference(settingMap.BillingPreference)

	// Get all subscriptions (including expired)
	allSubscriptions, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		allSubscriptions = []model.SubscriptionSummary{}
	}

	// Get active subscriptions for backward compatibility
	activeSubscriptions, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		activeSubscriptions = []model.SubscriptionSummary{}
	}

	common.ApiSuccess(c, gin.H{
		"billing_preference": pref,
		"subscriptions":      activeSubscriptions, // all active subscriptions
		"all_subscriptions":  allSubscriptions,    // all subscriptions including expired
	})
}

func UpdateSubscriptionPreference(c *gin.Context) {
	userId := c.GetInt("id")
	var req BillingPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	pref := common.NormalizeBillingPreference(req.BillingPreference)

	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	current := user.GetSetting()
	current.BillingPreference = pref
	user.SetSetting(current)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"billing_preference": pref})
}

type HourlyLimitPreferenceRequest struct {
	Enabled *bool `json:"enabled"`
}

// UpdateSubscriptionHourlyLimit changes only the user's active enforcement
// bucket. The usage buckets themselves are never reset by this operation.
func UpdateSubscriptionHourlyLimit(c *gin.Context) {
	userId := c.GetInt("id")
	subscriptionId, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	var req HourlyLimitPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	now := common.GetTimestamp()
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var sub model.UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", subscriptionId, userId, "active", now).
			First(&sub).Error; err != nil {
			return err
		}
		plan, err := model.GetSubscriptionPlanById(sub.PlanId)
		if err != nil {
			return err
		}
		if !plan.SpecialQuotaEnabled {
			return fmt.Errorf("该订阅当前未启用小时限制模式")
		}
		return tx.Model(&sub).Update("hourly_limit_enabled", *req.Enabled).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"enabled": *req.Enabled})
}

// ---- Admin APIs ----

func AdminListSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

type AdminUpsertSubscriptionPlanRequest struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

func normalizeSubscriptionPlanAccessibleGroups(plan *model.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("套餐不能为空")
	}
	accessibleGroups, err := model.NormalizeSubscriptionAccessibleGroups(plan.AccessibleGroups)
	if err != nil {
		return err
	}
	plan.AccessibleGroups = accessibleGroups
	restrictedGroups, err := model.NormalizeSubscriptionRestrictedGroups(plan.RestrictedGroups)
	if err != nil {
		return err
	}
	plan.RestrictedGroups = restrictedGroups
	normalizedAccessibleGroups, err := model.SubscriptionPlanAccessibleGroups(plan)
	if err != nil {
		return err
	}
	normalizedRestrictedGroups, err := model.SubscriptionPlanRestrictedGroups(plan)
	if err != nil {
		return err
	}
	groupRatios := ratio_setting.GetGroupRatioCopy()
	gptGroupRatios := ratio_setting.GetGptGroupRatioCopy()
	validateGroups := func(groups []string, fieldName string) error {
		for _, group := range groups {
			if _, ok := groupRatios[group]; ok {
				continue
			}
			if _, ok := gptGroupRatios[group]; !ok {
				return fmt.Errorf("%s不存在: %s", fieldName, group)
			}
		}
		return nil
	}
	if err := validateGroups(normalizedAccessibleGroups, "可访问分组"); err != nil {
		return err
	}
	if err := validateGroups(normalizedRestrictedGroups, "限制访问分组"); err != nil {
		return err
	}
	accessibleSet := make(map[string]struct{}, len(normalizedAccessibleGroups))
	for _, group := range normalizedAccessibleGroups {
		accessibleSet[group] = struct{}{}
	}
	for _, group := range normalizedRestrictedGroups {
		if _, ok := accessibleSet[group]; ok {
			return fmt.Errorf("可访问分组和限制访问分组不能包含相同分组: %s", group)
		}
	}
	return nil
}

func AdminCreateSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Plan.Id = 0
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	req.Plan.Currency = "USD"
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return
	}
	if req.Plan.WeeklyAmountLimit < 0 {
		common.ApiErrorMsg(c, "周限额不能为负数")
		return
	}
	if req.Plan.HourlyResetHours == 0 {
		req.Plan.HourlyResetHours = 5
	}
	if req.Plan.SpecialWeeklyResetWeeks == 0 {
		req.Plan.SpecialWeeklyResetWeeks = 1
	}
	if err := model.ValidateSpecialQuotaPlan(&req.Plan); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := normalizeSubscriptionPlanAccessibleGroups(&req.Plan); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}
	err := model.DB.Create(&req.Plan).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(req.Plan.Id)
	common.ApiSuccess(c, req.Plan)
}

func AdminUpdateSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	req.Plan.Id = id
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	req.Plan.Currency = "USD"
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return
	}
	if req.Plan.WeeklyAmountLimit < 0 {
		common.ApiErrorMsg(c, "周限额不能为负数")
		return
	}
	if req.Plan.HourlyResetHours == 0 {
		req.Plan.HourlyResetHours = 5
	}
	if req.Plan.SpecialWeeklyResetWeeks == 0 {
		req.Plan.SpecialWeeklyResetWeeks = 1
	}
	if err := model.ValidateSpecialQuotaPlan(&req.Plan); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := normalizeSubscriptionPlanAccessibleGroups(&req.Plan); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}

	previousPlan, err := model.GetSubscriptionPlanById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	specialConfigChanged := previousPlan.SpecialQuotaEnabled != req.Plan.SpecialQuotaEnabled ||
		previousPlan.HourlyResetHours != req.Plan.HourlyResetHours ||
		previousPlan.HourlyAmountLimit != req.Plan.HourlyAmountLimit ||
		previousPlan.SpecialWeeklyResetWeeks != req.Plan.SpecialWeeklyResetWeeks ||
		previousPlan.SpecialWeeklyAmountLimit != req.Plan.SpecialWeeklyAmountLimit
	specialConfigUpdatedAt := previousPlan.SpecialConfigUpdatedAt
	if specialConfigChanged || specialConfigUpdatedAt == 0 {
		specialConfigUpdatedAt = common.GetTimestamp()
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// update plan (allow zero values updates with map)
		updateMap := map[string]interface{}{
			"title":                       req.Plan.Title,
			"subtitle":                    req.Plan.Subtitle,
			"price_amount":                req.Plan.PriceAmount,
			"currency":                    req.Plan.Currency,
			"duration_unit":               req.Plan.DurationUnit,
			"duration_value":              req.Plan.DurationValue,
			"custom_seconds":              req.Plan.CustomSeconds,
			"enabled":                     req.Plan.Enabled,
			"sort_order":                  req.Plan.SortOrder,
			"stripe_price_id":             req.Plan.StripePriceId,
			"creem_product_id":            req.Plan.CreemProductId,
			"max_purchase_per_user":       req.Plan.MaxPurchasePerUser,
			"total_amount":                req.Plan.TotalAmount,
			"weekly_amount_limit":         req.Plan.WeeklyAmountLimit,
			"special_quota_enabled":       req.Plan.SpecialQuotaEnabled,
			"hourly_reset_hours":          req.Plan.HourlyResetHours,
			"hourly_amount_limit":         req.Plan.HourlyAmountLimit,
			"special_weekly_reset_weeks":  req.Plan.SpecialWeeklyResetWeeks,
			"special_weekly_amount_limit": req.Plan.SpecialWeeklyAmountLimit,
			"special_config_updated_at":   specialConfigUpdatedAt,
			"upgrade_group":               req.Plan.UpgradeGroup,
			"accessible_groups":           req.Plan.AccessibleGroups,
			"restricted_groups":           req.Plan.RestrictedGroups,
			"quota_reset_period":          req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds":  req.Plan.QuotaResetCustomSeconds,
			"updated_at":                  common.GetTimestamp(),
		}
		if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminUpdateSubscriptionPlanStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

func AdminUpdateSubscriptionPlanStatus(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Update("enabled", *req.Enabled).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminBindSubscriptionRequest struct {
	UserId int `json:"user_id"`
	PlanId int `json:"plan_id"`
}

func AdminBindSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req AdminBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(req.UserId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// ---- Admin: user subscription management ----

func AdminListUserSubscriptions(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	subs, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, subs)
}

type AdminCreateUserSubscriptionRequest struct {
	PlanId int `json:"plan_id"`
}

// AdminCreateUserSubscription creates a new user subscription from a plan (no payment).
func AdminCreateUserSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req AdminCreateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(userId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminInvalidateUserSubscription cancels a user subscription immediately.
func AdminInvalidateUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminInvalidateUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminDeleteUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

type AdminPostponeSubscriptionsRequest struct {
	UserIds []int `json:"user_ids" binding:"required"`
	Days    int   `json:"days" binding:"required,min=1"`
}

// AdminPostponeUserSubscriptions postpones all active subscriptions for the given users.
func AdminPostponeUserSubscriptions(c *gin.Context) {
	var req AdminPostponeSubscriptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.UserIds) == 0 || req.Days <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	results, err := model.PostponeSubscriptionsForUsers(req.UserIds, req.Days)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	skipped := make([]int, 0)
	for _, userId := range req.UserIds {
		if results[userId] == 0 {
			skipped = append(skipped, userId)
		}
	}
	common.ApiSuccess(c, gin.H{
		"results": results,
		"skipped": skipped,
	})
}
