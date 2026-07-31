package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

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

	first, err := PreConsumeUserSubscription("special-request-1", user.Id, "test", 0, 60)
	require.NoError(t, err)
	require.Equal(t, int64(60), first.AmountUsedAfter)

	var sub UserSubscription
	sub = *subscription
	require.NoError(t, DB.Model(&sub).Update("hourly_limit_enabled", false).Error)

	_, err = PreConsumeUserSubscription("special-request-2", user.Id, "test", 0, 940)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("special-request-3", user.Id, "test", 0, 1)
	require.Error(t, err)

	require.NoError(t, DB.Model(&sub).Update("hourly_limit_enabled", true).Error)
	_, err = PreConsumeUserSubscription("special-request-4", user.Id, "test", 0, 40)
	require.Error(t, err)
	_, err = PreConsumeUserSubscription("special-request-5", user.Id, "test", 0, 1)
	require.Error(t, err)

	var hourly, weekly SubscriptionUsageBucket
	require.NoError(t, DB.Where("user_subscription_id = ? AND bucket_type = ?", sub.Id, SubscriptionUsageBucketHourly).First(&hourly).Error)
	require.NoError(t, DB.Where("user_subscription_id = ? AND bucket_type = ?", sub.Id, SubscriptionUsageBucketWeekly).First(&weekly).Error)
	require.Equal(t, int64(1000), hourly.AmountUsed)
	require.Equal(t, int64(1000), weekly.AmountUsed)
}
