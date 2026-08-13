package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"gorm.io/gorm"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

var (
	ErrSubscriptionOrderNotFound      = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid = errors.New("subscription order status invalid")
)

const (
	subscriptionPlanCacheNamespace     = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace = "new-api:subscription_plan_info:v1"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	StripePriceId  string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// AccessibleGroups restricts this plan's quota to the listed request groups.
	// An empty list keeps the plan global for backwards compatibility.
	AccessibleGroups JSONValue `json:"accessible_groups" gorm:"type:json"`

	// RestrictedGroups excludes request groups from this plan. It takes
	// precedence over AccessibleGroups when evaluating a request.
	RestrictedGroups JSONValue `json:"restricted_groups" gorm:"type:json"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// Weekly amount limit (amount in quota units, 0 = no weekly limit)
	// 与quota_reset_period独立叠加，按订阅起始日计算每7天一个周期
	WeeklyAmountLimit int64 `json:"weekly_amount_limit" gorm:"type:bigint;not null;default:0"`

	// Special quota mode. When enabled, users choose whether the hourly or
	// special-weekly bucket is the active enforcement bucket.
	SpecialQuotaEnabled      bool  `json:"special_quota_enabled" gorm:"default:false"`
	HourlyResetHours         int   `json:"hourly_reset_hours" gorm:"type:int;default:5"`
	HourlyAmountLimit        int64 `json:"hourly_amount_limit" gorm:"type:bigint;not null;default:0"`
	SpecialWeeklyResetWeeks  int   `json:"special_weekly_reset_weeks" gorm:"type:int;default:1"`
	SpecialWeeklyAmountLimit int64 `json:"special_weekly_amount_limit" gorm:"type:bigint;not null;default:0"`
	SpecialConfigUpdatedAt   int64 `json:"special_config_updated_at" gorm:"type:bigint;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

// NormalizeSubscriptionAccessibleGroups validates, trims, and de-duplicates
// the JSON array persisted on a subscription plan. Empty or null values mean
// the plan is global and are normalized to an empty array.
func normalizeSubscriptionPlanGroups(groups JSONValue, fieldName string) (JSONValue, error) {
	if len(groups) == 0 {
		return JSONValue([]byte("[]")), nil
	}
	var values []string
	if err := json.Unmarshal(groups, &values); err != nil {
		return nil, fmt.Errorf("%s必须是字符串数组", fieldName)
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		group := strings.TrimSpace(value)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return JSONValue(data), nil
}

func NormalizeSubscriptionAccessibleGroups(groups JSONValue) (JSONValue, error) {
	return normalizeSubscriptionPlanGroups(groups, "可访问分组")
}

func NormalizeSubscriptionRestrictedGroups(groups JSONValue) (JSONValue, error) {
	return normalizeSubscriptionPlanGroups(groups, "限制访问分组")
}

// SubscriptionPlanAccessibleGroups returns the normalized group list for a
// plan. A nil or empty list identifies a global subscription plan.
func SubscriptionPlanAccessibleGroups(plan *SubscriptionPlan) ([]string, error) {
	if plan == nil {
		return nil, errors.New("subscription plan is nil")
	}
	normalized, err := NormalizeSubscriptionAccessibleGroups(plan.AccessibleGroups)
	if err != nil {
		return nil, err
	}
	var groups []string
	if err := json.Unmarshal(normalized, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func SubscriptionPlanRestrictedGroups(plan *SubscriptionPlan) ([]string, error) {
	if plan == nil {
		return nil, errors.New("subscription plan is nil")
	}
	normalized, err := NormalizeSubscriptionRestrictedGroups(plan.RestrictedGroups)
	if err != nil {
		return nil, err
	}
	var groups []string
	if err := json.Unmarshal(normalized, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func subscriptionPlanMatchesGroup(plan *SubscriptionPlan, targetGroup string) (matches bool, restricted bool, err error) {
	accessibleGroups, err := SubscriptionPlanAccessibleGroups(plan)
	if err != nil {
		return false, false, err
	}
	restrictedGroups, err := SubscriptionPlanRestrictedGroups(plan)
	if err != nil {
		return false, false, err
	}
	targetGroup = strings.TrimSpace(targetGroup)
	for _, group := range restrictedGroups {
		if group == targetGroup {
			return false, len(accessibleGroups) > 0, nil
		}
	}
	if len(accessibleGroups) == 0 {
		return true, false, nil
	}
	for _, group := range accessibleGroups {
		if group == targetGroup {
			return true, true, nil
		}
	}
	return false, true, nil
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.HourlyResetHours <= 0 {
		p.HourlyResetHours = 5
	}
	if p.SpecialWeeklyResetWeeks <= 0 {
		p.SpecialWeeklyResetWeeks = 1
	}
	if p.SpecialConfigUpdatedAt == 0 {
		p.SpecialConfigUpdatedAt = now
	}
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id     int     `json:"id"`
	UserId int     `json:"user_id" gorm:"index"`
	PlanId int     `json:"plan_id" gorm:"index"`
	Money  float64 `json:"money"`

	TradeNo         string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status          string `json:"status"`
	CreateTime      int64  `json:"create_time"`
	CompleteTime    int64  `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	return DB.Create(o).Error
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	// 周限额相关字段（与quota_reset_period独立叠加）
	// WeeklyAmountLimit: 周限额快照（0=不限制），创建订阅时从plan快照
	WeeklyAmountLimit int64 `json:"weekly_amount_limit" gorm:"type:bigint;not null;default:0"`
	// WeeklyAmountUsed: 本周已用额度
	WeeklyAmountUsed int64 `json:"weekly_amount_used" gorm:"type:bigint;not null;default:0"`
	// WeeklyPeriodStart: 当前周周期起点（unix秒）
	WeeklyPeriodStart int64 `json:"weekly_period_start" gorm:"type:bigint;default:0"`
	// WeeklyPeriodEnd: 当前周周期终点（unix秒），定时任务用索引
	WeeklyPeriodEnd int64 `json:"weekly_period_end" gorm:"type:bigint;default:0;index"`

	// Special mode preference. It is deliberately stored on the subscription,
	// while the plan's feature flag and limits remain live configuration.
	HourlyLimitEnabled bool `json:"hourly_limit_enabled" gorm:"default:true"`

	// The following fields are calculated from the current plan and usage
	// buckets for API responses; they are not persisted on user_subscriptions.
	SpecialQuotaEnabled      bool   `json:"special_quota_enabled" gorm:"-"`
	SpecialConfigUpdatedAt   int64  `json:"special_config_updated_at" gorm:"-"`
	HourlyResetHours         int    `json:"hourly_reset_hours" gorm:"-"`
	HourlyAmountLimit        int64  `json:"hourly_amount_limit" gorm:"-"`
	HourlyAmountUsed         int64  `json:"hourly_amount_used" gorm:"-"`
	HourlyPeriodStart        int64  `json:"hourly_period_start" gorm:"-"`
	HourlyPeriodEnd          int64  `json:"hourly_period_end" gorm:"-"`
	SpecialWeeklyResetWeeks  int    `json:"special_weekly_reset_weeks" gorm:"-"`
	SpecialWeeklyAmountLimit int64  `json:"special_weekly_amount_limit" gorm:"-"`
	SpecialWeeklyAmountUsed  int64  `json:"special_weekly_amount_used" gorm:"-"`
	SpecialWeeklyPeriodStart int64  `json:"special_weekly_period_start" gorm:"-"`
	SpecialWeeklyPeriodEnd   int64  `json:"special_weekly_period_end" gorm:"-"`
	EffectiveQuotaMode       string `json:"effective_quota_mode" gorm:"-"`

	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

type SubscriptionSummary struct {
	Subscription *UserSubscription `json:"subscription"`
}

const (
	SubscriptionUsageBucketHourly = "hourly"
	SubscriptionUsageBucketWeekly = "weekly"
)

// SubscriptionUsageBucket is an immutable time-window identity with a mutable
// usage total. Keeping one row per window preserves usage across mode switches
// and lets a plan change start a new bucket without rewriting history.
type SubscriptionUsageBucket struct {
	Id                 int    `json:"id"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index;uniqueIndex:idx_subscription_usage_bucket,priority:1"`
	BucketType         string `json:"bucket_type" gorm:"type:varchar(16);uniqueIndex:idx_subscription_usage_bucket,priority:2"`
	PeriodStart        int64  `json:"period_start" gorm:"bigint;uniqueIndex:idx_subscription_usage_bucket,priority:3"`
	PeriodEnd          int64  `json:"period_end" gorm:"bigint"`
	AmountUsed         int64  `json:"amount_used" gorm:"type:bigint;not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (b *SubscriptionUsageBucket) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	b.CreatedAt = now
	b.UpdatedAt = now
	return nil
}

func (b *SubscriptionUsageBucket) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = common.GetTimestamp()
	return nil
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// Align to next Monday 00:00
		weekday := int(base.Weekday()) // Sunday=0
		// Convert to Monday=1..Sunday=7
		if weekday == 0 {
			weekday = 7
		}
		daysUntil := 8 - weekday
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, daysUntil)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	if key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			return &cached, nil
		}
	}
	var plan SubscriptionPlan
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	_ = getSubscriptionPlanCache().SetWithTTL(key, plan, subscriptionPlanCacheTTL())
	return &plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var group string
	if err := tx.Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}
	var activeSub UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}

func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := GetDBTimestamp()
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" {
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}
	// 周限额初始化：从plan快照WeeklyAmountLimit，启用时设置首个周期[StartTime, StartTime+7天)
	weeklyPeriodStart := int64(0)
	weeklyPeriodEnd := int64(0)
	if plan.WeeklyAmountLimit > 0 {
		weeklyPeriodStart = now.Unix()
		weeklyPeriodEnd = weeklyPeriodStart + 7*24*3600
	}
	sub := &UserSubscription{
		UserId:             userId,
		PlanId:             plan.Id,
		AmountTotal:        plan.TotalAmount,
		AmountUsed:         0,
		StartTime:          now.Unix(),
		EndTime:            endUnix,
		Status:             "active",
		Source:             source,
		LastResetTime:      lastReset,
		NextResetTime:      nextReset,
		WeeklyAmountLimit:  plan.WeeklyAmountLimit,
		WeeklyAmountUsed:   0,
		WeeklyPeriodStart:  weeklyPeriodStart,
		WeeklyPeriodEnd:    weeklyPeriodEnd,
		HourlyLimitEnabled: true,
		UpgradeGroup:       upgradeGroup,
		PrevUserGroup:      prevGroup,
		CreatedAt:          common.GetTimestamp(),
		UpdatedAt:          common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
// expectedPaymentProvider guards against cross-gateway callback attacks (empty skips the check).
// actualPaymentMethod updates the order's PaymentMethod to reflect the real payment type used (empty skips update).
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := GetSubscriptionPlanById(order.PlanId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		_, err = CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order")
		if err != nil {
			return err
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, upgradeGroup)
	}
	if logUserId > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	var topup TopUp
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId:        order.UserId,
				Amount:        0,
				Money:         order.Money,
				TradeNo:       order.TradeNo,
				PaymentMethod: order.PaymentMethod,
				CreateTime:    order.CreateTime,
				CompleteTime:  now,
				Status:        common.TopUpStatusSuccess,
			}
			return tx.Create(&topup).Error
		}
		return err
	}
	topup.Money = order.Money
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return ErrPaymentMethodMismatch
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = common.TopUpStatusSuccess
	return tx.Save(&topup).Error
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

// Admin bind (no payment). Creates a UserSubscription from a plan.
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plan.UpgradeGroup) != "" {
		_ = UpdateUserGroupCache(userId, plan.UpgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
	}
	return "", nil
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		decorateSubscriptionWithSpecialUsage(&subCopy)
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
		})
	}
	return result
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":     "cancelled",
			"end_time":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
	TargetGroup        string
	IsGroupRestricted  bool
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":     "expired",
					"updated_at": common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's an active upgraded subscription, keep current group.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// No active upgraded subscription, downgrade to previous group if needed.
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> ''",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userId, cacheGroup)
		}
	}
	return expiredCount, nil
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	HourlyBucketId     int    `json:"hourly_bucket_id" gorm:"index;default:0"`
	WeeklyBucketId     int    `json:"weekly_bucket_id" gorm:"index;default:0"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTime(base, plan, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTime(base, plan, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

// maybeResetWeeklyQuotaTx 在事务内检查并重置周限额。
// 如果当前时间 >= WeeklyPeriodEnd，清零 WeeklyAmountUsed 并推进周期。
// 循环推进以处理服务停机导致的多次周期已过。
// 与 quota_reset_period 独立，按订阅起始日计算每7天一个周期。
func maybeResetWeeklyQuotaTx(tx *gorm.DB, sub *UserSubscription, now int64) error {
	if tx == nil || sub == nil {
		return errors.New("invalid weekly reset args")
	}
	if sub.WeeklyAmountLimit <= 0 {
		return nil // 未启用周限额
	}
	// 循环推进周期直到 WeeklyPeriodEnd > now
	changed := false
	for sub.WeeklyPeriodEnd > 0 && sub.WeeklyPeriodEnd <= now {
		sub.WeeklyPeriodStart = sub.WeeklyPeriodEnd
		sub.WeeklyPeriodEnd = sub.WeeklyPeriodStart + 7*24*3600
		sub.WeeklyAmountUsed = 0
		changed = true
	}
	if !changed {
		return nil
	}
	// 保存到数据库
	return tx.Model(sub).Updates(map[string]interface{}{
		"weekly_period_start": sub.WeeklyPeriodStart,
		"weekly_period_end":   sub.WeeklyPeriodEnd,
		"weekly_amount_used":  sub.WeeklyAmountUsed,
	}).Error
}

func ValidateSpecialQuotaPlan(plan *SubscriptionPlan) error {
	if plan == nil || !plan.SpecialQuotaEnabled {
		return nil
	}
	if plan.HourlyResetHours <= 0 {
		return errors.New("小时重置周期必须大于0")
	}
	if plan.HourlyAmountLimit <= 0 {
		return errors.New("小时额度必须大于0")
	}
	if plan.SpecialWeeklyResetWeeks != 1 && plan.SpecialWeeklyResetWeeks != 2 {
		return errors.New("特殊周重置周期只能为1或2周")
	}
	if plan.SpecialWeeklyAmountLimit <= 0 {
		return errors.New("特殊周额度必须大于0")
	}
	return nil
}

func specialBucketWindow(sub *UserSubscription, plan *SubscriptionPlan, bucketType string, now int64) (int64, int64, error) {
	if sub == nil || plan == nil {
		return 0, 0, errors.New("invalid special bucket args")
	}
	duration := int64(0)
	switch bucketType {
	case SubscriptionUsageBucketHourly:
		if plan.HourlyResetHours <= 0 {
			return 0, 0, errors.New("invalid hourly reset period")
		}
		duration = int64(plan.HourlyResetHours) * 3600
	case SubscriptionUsageBucketWeekly:
		if plan.SpecialWeeklyResetWeeks != 1 && plan.SpecialWeeklyResetWeeks != 2 {
			return 0, 0, errors.New("invalid special weekly reset period")
		}
		duration = int64(plan.SpecialWeeklyResetWeeks) * 7 * 24 * 3600
	default:
		return 0, 0, errors.New("invalid special bucket type")
	}
	base := sub.StartTime
	if plan.SpecialConfigUpdatedAt > base {
		base = plan.SpecialConfigUpdatedAt
	}
	if base <= 0 {
		base = now
	}
	if now < base {
		now = base
	}
	index := (now - base) / duration
	start := base + index*duration
	end := start + duration
	if sub.EndTime > 0 && bucketType == SubscriptionUsageBucketWeekly && sub.EndTime-now < duration {
		// 订阅剩余时间不足一个完整周窗口（如 30 天订阅只剩 5 天）时，
		// 周限额不可用，回退到小时限额。即使当前周窗口恰好完整
		// （窗口终点未越过订阅结束时间），也一律不提供周限额。
		return 0, 0, ErrSpecialWeeklyPartialWindow
	}
	if sub.EndTime > 0 && end > sub.EndTime {
		// 周限额只允许完整周窗口：当前窗口若越过订阅结束时间（如 30 天订阅
		// 末尾不足一周的天数），这些天不提供周限额，回退到小时限额。
		if bucketType == SubscriptionUsageBucketWeekly {
			return 0, 0, ErrSpecialWeeklyPartialWindow
		}
		end = sub.EndTime
	}
	return start, end, nil
}

func getSpecialUsageBucketTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, bucketType string, now int64) (*SubscriptionUsageBucket, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	start, end, err := specialBucketWindow(sub, plan, bucketType, now)
	if err != nil {
		// 订阅末尾不足一个完整周窗口时，周限额不可用，返回 nil bucket 由调用方回退到小时限额
		if bucketType == SubscriptionUsageBucketWeekly && errors.Is(err, ErrSpecialWeeklyPartialWindow) {
			return nil, nil
		}
		return nil, err
	}
	var bucket SubscriptionUsageBucket
	query := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("user_subscription_id = ? AND bucket_type = ? AND period_start = ?", sub.Id, bucketType, start).
		First(&bucket)
	if query.Error == nil {
		return &bucket, nil
	}
	if !errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return nil, query.Error
	}
	bucket = SubscriptionUsageBucket{
		UserSubscriptionId: sub.Id,
		BucketType:         bucketType,
		PeriodStart:        start,
		PeriodEnd:          end,
	}
	if err := tx.Create(&bucket).Error; err != nil {
		return nil, err
	}
	return &bucket, nil
}

func getCurrentSpecialUsageBucket(sub *UserSubscription, plan *SubscriptionPlan, bucketType string, now int64) (*SubscriptionUsageBucket, error) {
	start, end, err := specialBucketWindow(sub, plan, bucketType, now)
	if err != nil {
		// 订阅末尾不足一个完整周窗口时，周限额不可用，返回 nil bucket 由调用方回退到小时限额
		if bucketType == SubscriptionUsageBucketWeekly && errors.Is(err, ErrSpecialWeeklyPartialWindow) {
			return nil, nil
		}
		return nil, err
	}
	var bucket SubscriptionUsageBucket
	query := DB.Where("user_subscription_id = ? AND bucket_type = ? AND period_start = ?", sub.Id, bucketType, start).
		First(&bucket)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return &SubscriptionUsageBucket{
			UserSubscriptionId: sub.Id,
			BucketType:         bucketType,
			PeriodStart:        start,
			PeriodEnd:          end,
		}, nil
	}
	if query.Error != nil {
		return nil, query.Error
	}
	return &bucket, nil
}

func decorateSubscriptionWithSpecialUsage(sub *UserSubscription) {
	if sub == nil {
		return
	}
	plan, err := GetSubscriptionPlanById(sub.PlanId)
	if err != nil || plan == nil {
		return
	}
	sub.SpecialQuotaEnabled = plan.SpecialQuotaEnabled
	sub.SpecialConfigUpdatedAt = plan.SpecialConfigUpdatedAt
	sub.HourlyResetHours = plan.HourlyResetHours
	sub.HourlyAmountLimit = plan.HourlyAmountLimit
	sub.SpecialWeeklyResetWeeks = plan.SpecialWeeklyResetWeeks
	sub.SpecialWeeklyAmountLimit = plan.SpecialWeeklyAmountLimit
	if !plan.SpecialQuotaEnabled {
		sub.EffectiveQuotaMode = "normal"
		return
	}
	now := GetDBTimestamp()
	hourly, hourlyErr := getCurrentSpecialUsageBucket(sub, plan, SubscriptionUsageBucketHourly, now)
	weekly, weeklyErr := getCurrentSpecialUsageBucket(sub, plan, SubscriptionUsageBucketWeekly, now)
	if hourlyErr == nil && hourly != nil {
		sub.HourlyAmountUsed = hourly.AmountUsed
		sub.HourlyPeriodStart = hourly.PeriodStart
		sub.HourlyPeriodEnd = hourly.PeriodEnd
	}
	if weeklyErr == nil && weekly != nil {
		sub.SpecialWeeklyAmountUsed = weekly.AmountUsed
		sub.SpecialWeeklyPeriodStart = weekly.PeriodStart
		sub.SpecialWeeklyPeriodEnd = weekly.PeriodEnd
	}
	if sub.HourlyLimitEnabled {
		sub.EffectiveQuotaMode = SubscriptionUsageBucketHourly
	} else if weekly != nil {
		sub.EffectiveQuotaMode = SubscriptionUsageBucketWeekly
	} else {
		// 订阅末尾不足一个完整周窗口，周限额不可用，回退到小时限额
		sub.EffectiveQuotaMode = SubscriptionUsageBucketHourly
	}
}

// PreConsumeUserSubscription pre-consumes quota from an active subscription
// that can fund the target request group. Matching restricted subscriptions
// are selected before global subscriptions.
func PreConsumeUserSubscription(requestId string, userId int, modelName string, targetGroup string, quotaType int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	targetGroup = strings.TrimSpace(targetGroup)
	now := GetDBTimestamp()

	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
			if err != nil {
				return err
			}
			_, restricted, err := subscriptionPlanMatchesGroup(plan, targetGroup)
			if err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = existing.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			returnValue.TargetGroup = targetGroup
			returnValue.IsGroupRestricted = restricted
			return nil
		}

		var subs []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}
		for _, restrictedPass := range []bool{true, false} {
			for _, candidate := range subs {
				sub := candidate
				plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
				if err != nil {
					return err
				}
				matches, isRestricted, err := subscriptionPlanMatchesGroup(plan, targetGroup)
				if err != nil {
					return err
				}
				if isRestricted != restrictedPass || !matches {
					continue
				}
				if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
					return err
				}
				// 检查并重置周限额（与quota_reset_period独立叠加）
				if err := maybeResetWeeklyQuotaTx(tx, &sub, now); err != nil {
					return err
				}

				var hourlyBucket *SubscriptionUsageBucket
				var weeklyBucket *SubscriptionUsageBucket
				usedBefore := sub.AmountUsed
				effectiveLimit := sub.AmountTotal
				if plan.SpecialQuotaEnabled {
					// Both buckets receive every real consumption. Only the user's
					// selected bucket is enforced, so toggling cannot reset either one.
					hourlyBucket, err = getSpecialUsageBucketTx(tx, &sub, plan, SubscriptionUsageBucketHourly, now)
					if err != nil {
						return err
					}
					weeklyBucket, err = getSpecialUsageBucketTx(tx, &sub, plan, SubscriptionUsageBucketWeekly, now)
					if err != nil {
						return err
					}
					if sub.HourlyLimitEnabled || weeklyBucket == nil {
						// 用户选择小时限额，或订阅末尾不足一个完整周窗口（周限额不可用）时，
						// 一律按小时限额校验
						usedBefore = hourlyBucket.AmountUsed
						effectiveLimit = plan.HourlyAmountLimit
						if effectiveLimit > 0 && effectiveLimit-usedBefore < amount {
							continue
						}
					} else {
						usedBefore = weeklyBucket.AmountUsed
						effectiveLimit = plan.SpecialWeeklyAmountLimit
						if effectiveLimit > 0 && effectiveLimit-usedBefore < amount {
							continue
						}
					}
				} else {
					// Existing subscription behavior remains unchanged outside the
					// special mode.
					if sub.AmountTotal > 0 && sub.AmountTotal-sub.AmountUsed < amount {
						continue
					}
					if sub.WeeklyAmountLimit > 0 && sub.WeeklyAmountLimit-sub.WeeklyAmountUsed < amount {
						continue
					}
				}
				record := &SubscriptionPreConsumeRecord{
					RequestId:          requestId,
					UserId:             userId,
					UserSubscriptionId: sub.Id,
					PreConsumed:        amount,
					Status:             "consumed",
				}
				if hourlyBucket != nil {
					record.HourlyBucketId = hourlyBucket.Id
				}
				if weeklyBucket != nil {
					record.WeeklyBucketId = weeklyBucket.Id
				}
				if err := tx.Create(record).Error; err != nil {
					var dup SubscriptionPreConsumeRecord
					if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
						if dup.Status == "refunded" {
							return errors.New("subscription pre-consume already refunded")
						}
						returnValue.UserSubscriptionId = sub.Id
						returnValue.PreConsumed = dup.PreConsumed
						returnValue.AmountTotal = sub.AmountTotal
						returnValue.AmountUsedBefore = sub.AmountUsed
						returnValue.AmountUsedAfter = sub.AmountUsed
						return nil
					}
					return err
				}
				sub.AmountUsed += amount
				if !plan.SpecialQuotaEnabled && sub.WeeklyAmountLimit > 0 {
					sub.WeeklyAmountUsed += amount
				}
				if err := tx.Save(&sub).Error; err != nil {
					return err
				}
				if hourlyBucket != nil {
					hourlyBucket.AmountUsed += amount
					if err := tx.Save(hourlyBucket).Error; err != nil {
						return err
					}
				}
				if weeklyBucket != nil {
					weeklyBucket.AmountUsed += amount
					if err := tx.Save(weeklyBucket).Error; err != nil {
						return err
					}
				}
				returnValue.UserSubscriptionId = sub.Id
				returnValue.PreConsumed = amount
				returnValue.AmountTotal = effectiveLimit
				returnValue.AmountUsedBefore = usedBefore
				returnValue.AmountUsedAfter = usedBefore + amount
				returnValue.TargetGroup = targetGroup
				returnValue.IsGroupRestricted = isRestricted
				return nil
			}
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed <= 0 {
			record.Status = "refunded"
			return tx.Save(&record).Error
		}
		if err := adjustSubscriptionUsageByRecordTx(tx, &record, -record.PreConsumed); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

func adjustUsageBucketByIdTx(tx *gorm.DB, bucketId int, delta int64) error {
	if bucketId <= 0 || delta == 0 {
		return nil
	}
	var bucket SubscriptionUsageBucket
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", bucketId).First(&bucket).Error; err != nil {
		return err
	}
	bucket.AmountUsed += delta
	if bucket.AmountUsed < 0 {
		bucket.AmountUsed = 0
	}
	return tx.Save(&bucket).Error
}

func adjustSubscriptionUsageByRecordTx(tx *gorm.DB, record *SubscriptionPreConsumeRecord, delta int64) error {
	if tx == nil || record == nil {
		return errors.New("invalid subscription usage record")
	}
	if err := adjustUsageBucketByIdTx(tx, record.HourlyBucketId, delta); err != nil {
		return err
	}
	if err := adjustUsageBucketByIdTx(tx, record.WeeklyBucketId, delta); err != nil {
		return err
	}
	var sub UserSubscription
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
		return err
	}
	sub.AmountUsed += delta
	if sub.AmountUsed < 0 {
		sub.AmountUsed = 0
	}
	return tx.Save(&sub).Error
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// ResetDueWeeklyQuotas resets weekly quotas whose weekly_period_end has passed.
// 查询启用周限额且当前周期已到期的订阅，分批重置WeeklyAmountUsed并推进周期。
// 复用与ResetDueSubscriptions相同的FOR UPDATE事务模式，保证并发安全。
func ResetDueWeeklyQuotas(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("weekly_period_end > 0 AND weekly_period_end <= ? AND weekly_amount_limit > 0 AND status = ?", now, "active").
		Order("weekly_period_end asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		err := DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND weekly_period_end > 0 AND weekly_period_end <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil // 可能已被其他进程处理，跳过
			}
			if err := maybeResetWeeklyQuotaTx(tx, &locked, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// PostConsumeUserSubscriptionDelta 调整订阅已用额度（delta>0补扣，delta<0退还）。
// Special mode updates both usage buckets so later mode switches inherit the
// complete settled amount.
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		newUsed := sub.AmountUsed + delta
		if newUsed < 0 {
			newUsed = 0
		}
		if !plan.SpecialQuotaEnabled && sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
			return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
		}
		sub.AmountUsed = newUsed
		if plan.SpecialQuotaEnabled {
			hourly, err := getSpecialUsageBucketTx(tx, &sub, plan, SubscriptionUsageBucketHourly, GetDBTimestamp())
			if err != nil {
				return err
			}
			weekly, err := getSpecialUsageBucketTx(tx, &sub, plan, SubscriptionUsageBucketWeekly, GetDBTimestamp())
			if err != nil {
				return err
			}
			hourly.AmountUsed += delta
			if hourly.AmountUsed < 0 {
				hourly.AmountUsed = 0
			}
			if err := tx.Save(hourly).Error; err != nil {
				return err
			}
			if weekly != nil {
				weekly.AmountUsed += delta
				if weekly.AmountUsed < 0 {
					weekly.AmountUsed = 0
				}
				if err := tx.Save(weekly).Error; err != nil {
					return err
				}
			}
		} else if sub.WeeklyAmountLimit > 0 {
			newWeeklyUsed := sub.WeeklyAmountUsed + delta
			if newWeeklyUsed < 0 {
				newWeeklyUsed = 0
			}
			sub.WeeklyAmountUsed = newWeeklyUsed
		}
		return tx.Save(&sub).Error
	})
}

// ReduceSubscriptionDays reduces the end_time of a user subscription by the given number of days
// and returns the new end_time. It validates that the subscription is active and the reduction
// doesn't make end_time earlier than now.
// The caller must handle the transaction.
func ReduceSubscriptionDays(tx *gorm.DB, subscriptionId, userId int, days int) (int64, error) {
	if tx == nil {
		return 0, errors.New("tx is nil")
	}
	if days <= 0 {
		return 0, errors.New("days must be > 0")
	}
	now := common.GetTimestamp()
	var sub UserSubscription
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ? AND user_id = ?", subscriptionId, userId).First(&sub).Error; err != nil {
		return 0, errors.New("subscription not found")
	}
	if sub.Status != "active" {
		return 0, errors.New("subscription is not active")
	}
	if sub.EndTime <= now {
		return 0, errors.New("subscription has expired")
	}
	reduceSeconds := int64(days) * 86400
	newEndTime := sub.EndTime - reduceSeconds
	if newEndTime < now {
		return 0, errors.New("reduction exceeds remaining valid days")
	}
	if err := tx.Model(&sub).Update("end_time", newEndTime).Error; err != nil {
		return 0, err
	}
	return newEndTime, nil
}

// PostponeUserSubscriptions postpones all active subscriptions of a user by the given number of days.
// It extends end_time, next_reset_time and weekly_period_end by days*86400 seconds.
// Returns the number of affected subscriptions.
func PostponeUserSubscriptions(userId int, days int) (int, error) {
	if userId <= 0 {
		return 0, errors.New("invalid userId")
	}
	if days <= 0 {
		return 0, errors.New("days must be > 0")
	}
	postponeSeconds := int64(days) * 86400
	affected := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Lock active subscriptions for this user (FOR UPDATE)
		var subs []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ?", userId, "active").
			Find(&subs).Error; err != nil {
			return err
		}
		if len(subs) == 0 {
			return nil
		}
		now := common.GetTimestamp()
		result := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND status = ?", userId, "active").
			Updates(map[string]interface{}{
				"end_time":            gorm.Expr("end_time + ?", postponeSeconds),
				"next_reset_time":     gorm.Expr("next_reset_time + ?", postponeSeconds),
				"weekly_period_start": gorm.Expr("weekly_period_start + ?", postponeSeconds),
				"weekly_period_end":   gorm.Expr("weekly_period_end + ?", postponeSeconds),
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		affected = int(result.RowsAffected)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// PostponeSubscriptionsForUsers postpones subscriptions for multiple users.
// Each user is handled in its own transaction so a single failure does not roll back others.
// Returns a map of userId -> affected subscription count (0 means no active subscription or error).
func PostponeSubscriptionsForUsers(userIds []int, days int) (map[int]int, error) {
	results := make(map[int]int, len(userIds))
	if len(userIds) == 0 {
		return results, errors.New("no user ids provided")
	}
	if days <= 0 {
		return results, errors.New("days must be > 0")
	}
	for _, userId := range userIds {
		if userId <= 0 {
			results[userId] = 0
			continue
		}
		count, err := PostponeUserSubscriptions(userId, days)
		if err != nil {
			results[userId] = 0
			continue
		}
		results[userId] = count
	}
	return results, nil
}
