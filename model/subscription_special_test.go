package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSpecialWeeklyLimitPartialWindow(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 902, Username: "weekly-partial-test", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)

	plan := &SubscriptionPlan{
		Id:                       902,
		Title:                    "Special plan",
		DurationUnit:             SubscriptionDurationDay,
		DurationValue:            30,
		Enabled:                  true,
		SpecialQuotaEnabled:      true,
		HourlyResetHours:         5,
		HourlyAmountLimit:        100,
		SpecialWeeklyResetWeeks:  1,
		SpecialWeeklyAmountLimit: 1000,
	}
	require.NoError(t, DB.Create(plan).Error)

	start := time.Now().Unix() - 28*24*3600 // 28 days ago
	subscription := &UserSubscription{
		UserId:             user.Id,
		PlanId:             plan.Id,
		AmountTotal:        plan.TotalAmount,
		StartTime:          start,
		EndTime:            start + 30*24*3600, // 30 days from start = 2 days from now
		Status:             "active",
		Source:             "test",
		HourlyLimitEnabled: false, // user chose weekly mode
		CreatedAt:          start,
		UpdatedAt:          start,
	}
	require.NoError(t, DB.Create(subscription).Error)

	// Now is within the last 2 days of the 30-day subscription.
	// The last 2 days are a partial weekly window (not a complete 7-day week).
	// Weekly limit should NOT be available. Hourly limit should be used instead.

	// Try to consume 60 quota — should succeed via hourly limit (100 > 60)
	first, err := PreConsumeUserSubscription("partial-weekly-1", user.Id, "test", "", 0, 60)
	require.NoError(t, err)
	require.Equal(t, int64(60), first.AmountUsedAfter)
	// Effective limit should be hourly limit (100), not weekly limit (1000)
	require.Equal(t, int64(100), first.AmountTotal)

	// Try to consume 50 more — would exceed hourly limit (60+50=110 > 100)
	_, err = PreConsumeUserSubscription("partial-weekly-2", user.Id, "test", "", 0, 50)
	require.Error(t, err, "should exceed hourly limit (100) in partial weekly window")

	// Verify the weekly bucket for the partial window does NOT exist
	var weeklyBuckets []SubscriptionUsageBucket
	require.NoError(t, DB.Where("user_subscription_id = ? AND bucket_type = ?", subscription.Id, SubscriptionUsageBucketWeekly).Find(&weeklyBuckets).Error)
	// The consumption went to hourly only, so no weekly bucket was created for the partial window
	require.Len(t, weeklyBuckets, 0, "no weekly bucket should exist for a partial window")
}

func TestSpecialSubscriptionUsageSurvivesModeSwitch(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 901, Username: "special-mode-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Id:                       901,
		Title:                    "Special plan",
		DurationUnit:             SubscriptionDurationDay,
		DurationValue:            30,
		Enabled:                  true,
		SpecialQuotaEnabled:      true,
		HourlyResetHours:         5,
		HourlyAmountLimit:        100,
		SpecialWeeklyResetWeeks:  1,
		SpecialWeeklyAmountLimit: 1000,
	}
	require.NoError(t, DB.Create(plan).Error)
	start := time.Now().Unix()
	subscription := &UserSubscription{
		UserId:             user.Id,
		PlanId:             plan.Id,
		AmountTotal:        plan.TotalAmount,
		StartTime:          start,
		EndTime:            start + 30*24*3600,
		Status:             "active",
		Source:             "test",
		HourlyLimitEnabled: true,
		CreatedAt:          start,
		UpdatedAt:          start,
	}
	require.NoError(t, DB.Create(subscription).Error)

	first, err := PreConsumeUserSubscription("special-request-1", user.Id, "test", "", 0, 60)
	require.NoError(t, err)
	require.Equal(t, int64(60), first.AmountUsedAfter)

	var sub UserSubscription
	sub = *subscription
	require.NoError(t, DB.Model(&sub).Update("hourly_limit_enabled", false).Error)

	_, err = PreConsumeUserSubscription("special-request-2", user.Id, "test", "", 0, 940)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("special-request-3", user.Id, "test", "", 0, 1)
	require.Error(t, err)

	require.NoError(t, DB.Model(&sub).Update("hourly_limit_enabled", true).Error)
	_, err = PreConsumeUserSubscription("special-request-4", user.Id, "test", "", 0, 40)
	require.Error(t, err)
	_, err = PreConsumeUserSubscription("special-request-5", user.Id, "test", "", 0, 1)
	require.Error(t, err)

	var hourly, weekly SubscriptionUsageBucket
	require.NoError(t, DB.Where("user_subscription_id = ? AND bucket_type = ?", sub.Id, SubscriptionUsageBucketHourly).First(&hourly).Error)
	require.NoError(t, DB.Where("user_subscription_id = ? AND bucket_type = ?", sub.Id, SubscriptionUsageBucketWeekly).First(&weekly).Error)
	require.Equal(t, int64(1000), hourly.AmountUsed)
	require.Equal(t, int64(1000), weekly.AmountUsed)
}
