package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subscriptionAccessibleGroups(t *testing.T, groups ...string) JSONValue {
	t.Helper()
	data, err := json.Marshal(groups)
	require.NoError(t, err)
	normalized, err := NormalizeSubscriptionAccessibleGroups(JSONValue(data))
	require.NoError(t, err)
	return normalized
}

func subscriptionRestrictedGroups(t *testing.T, groups ...string) JSONValue {
	t.Helper()
	data, err := json.Marshal(groups)
	require.NoError(t, err)
	normalized, err := NormalizeSubscriptionRestrictedGroups(JSONValue(data))
	require.NoError(t, err)
	return normalized
}

func TestNormalizeSubscriptionAccessibleGroups(t *testing.T) {
	normalized, err := NormalizeSubscriptionAccessibleGroups(JSONValue(` [" premium ", "basic", "premium", ""] `))
	require.NoError(t, err)
	assert.JSONEq(t, `["premium","basic"]`, string(normalized))

	global, err := NormalizeSubscriptionAccessibleGroups(nil)
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(global))

	_, err = NormalizeSubscriptionAccessibleGroups(JSONValue(`{"group":"premium"}`))
	require.Error(t, err)
}

func TestSubscriptionPlanMatchesGroupHonorsRestrictedGroups(t *testing.T) {
	plan := &SubscriptionPlan{
		AccessibleGroups: subscriptionAccessibleGroups(t, "premium", "standard"),
		RestrictedGroups: subscriptionRestrictedGroups(t, "blocked"),
	}

	matches, isRestricted, err := subscriptionPlanMatchesGroup(plan, "premium")
	require.NoError(t, err)
	assert.True(t, matches)
	assert.True(t, isRestricted)

	matches, isRestricted, err = subscriptionPlanMatchesGroup(plan, "blocked")
	require.NoError(t, err)
	assert.False(t, matches)
	assert.True(t, isRestricted)

	// With no accessible list, every group except the explicitly restricted
	// ones can use the plan.
	plan.AccessibleGroups = nil
	matches, isRestricted, err = subscriptionPlanMatchesGroup(plan, "standard")
	require.NoError(t, err)
	assert.True(t, matches)
	assert.False(t, isRestricted)

	matches, isRestricted, err = subscriptionPlanMatchesGroup(plan, "blocked")
	require.NoError(t, err)
	assert.False(t, matches)
	assert.False(t, isRestricted)
}

func TestPreConsumeUserSubscriptionPrefersMatchingRestrictedPlan(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 940, Username: "group-restricted-subscription", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)

	globalPlan := &SubscriptionPlan{
		Id:            9401,
		Title:         "global",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 30,
		Enabled:       true,
		TotalAmount:   200,
	}
	premiumPlan := &SubscriptionPlan{
		Id:               9402,
		Title:            "premium only",
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    30,
		Enabled:          true,
		TotalAmount:      100,
		AccessibleGroups: subscriptionAccessibleGroups(t, "premium"),
	}
	standardPlan := &SubscriptionPlan{
		Id:               9403,
		Title:            "standard only",
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    30,
		Enabled:          true,
		TotalAmount:      100,
		AccessibleGroups: subscriptionAccessibleGroups(t, "standard"),
	}
	require.NoError(t, DB.Create(globalPlan).Error)
	require.NoError(t, DB.Create(premiumPlan).Error)
	require.NoError(t, DB.Create(standardPlan).Error)

	now := time.Now().Unix()
	globalSubscription := &UserSubscription{
		UserId: user.Id, PlanId: globalPlan.Id, AmountTotal: globalPlan.TotalAmount,
		StartTime: now, EndTime: now + 10*24*3600, Status: "active", Source: "test",
	}
	premiumSubscription := &UserSubscription{
		UserId: user.Id, PlanId: premiumPlan.Id, AmountTotal: premiumPlan.TotalAmount,
		StartTime: now, EndTime: now + 20*24*3600, Status: "active", Source: "test",
	}
	standardSubscription := &UserSubscription{
		UserId: user.Id, PlanId: standardPlan.Id, AmountTotal: standardPlan.TotalAmount,
		StartTime: now, EndTime: now + 30*24*3600, Status: "active", Source: "test",
	}
	require.NoError(t, DB.Create(globalSubscription).Error)
	require.NoError(t, DB.Create(premiumSubscription).Error)
	require.NoError(t, DB.Create(standardSubscription).Error)

	// The global subscription expires first, but matching restricted plans must
	// still be selected before it.
	premium, err := PreConsumeUserSubscription("subscription-group-premium", user.Id, "test", "premium", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, premiumSubscription.Id, premium.UserSubscriptionId)
	assert.True(t, premium.IsGroupRestricted)
	assert.Equal(t, "premium", premium.TargetGroup)

	// Once the matching restricted plan is exhausted, the global plan is used.
	premiumFallback, err := PreConsumeUserSubscription("subscription-group-premium-fallback", user.Id, "test", "premium", 0, 1)
	require.NoError(t, err)
	assert.Equal(t, globalSubscription.Id, premiumFallback.UserSubscriptionId)
	assert.False(t, premiumFallback.IsGroupRestricted)

	// Different restricted plans consume only their own matching group.
	standard, err := PreConsumeUserSubscription("subscription-group-standard", user.Id, "test", "standard", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, standardSubscription.Id, standard.UserSubscriptionId)
	assert.True(t, standard.IsGroupRestricted)

	// A group with no matching restricted plan must not consume either one.
	other, err := PreConsumeUserSubscription("subscription-group-other", user.Id, "test", "other", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, globalSubscription.Id, other.UserSubscriptionId)
	assert.False(t, other.IsGroupRestricted)

	var reloadedPremium, reloadedStandard, reloadedGlobal UserSubscription
	require.NoError(t, DB.First(&reloadedPremium, premiumSubscription.Id).Error)
	require.NoError(t, DB.First(&reloadedStandard, standardSubscription.Id).Error)
	require.NoError(t, DB.First(&reloadedGlobal, globalSubscription.Id).Error)
	assert.Equal(t, int64(100), reloadedPremium.AmountUsed)
	assert.Equal(t, int64(10), reloadedStandard.AmountUsed)
	assert.Equal(t, int64(11), reloadedGlobal.AmountUsed)
}

func TestPreConsumeUserSubscriptionSkipsExpiredAndExcludedPlans(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 950, Username: "subscription-group-fallback", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)

	expiredPlan := &SubscriptionPlan{
		Id:               9501,
		Title:            "expired allowed",
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    30,
		Enabled:          true,
		TotalAmount:      100,
		AccessibleGroups: subscriptionAccessibleGroups(t, "blocked"),
	}
	excludedPlan := &SubscriptionPlan{
		Id:               9502,
		Title:            "global except blocked",
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    30,
		Enabled:          true,
		TotalAmount:      100,
		RestrictedGroups: subscriptionRestrictedGroups(t, "blocked"),
	}
	fallbackPlan := &SubscriptionPlan{
		Id:               9503,
		Title:            "blocked allowed",
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    30,
		Enabled:          true,
		TotalAmount:      100,
		AccessibleGroups: subscriptionAccessibleGroups(t, "blocked"),
	}
	require.NoError(t, DB.Create(expiredPlan).Error)
	require.NoError(t, DB.Create(excludedPlan).Error)
	require.NoError(t, DB.Create(fallbackPlan).Error)

	now := time.Now().Unix()
	expiredSubscription := &UserSubscription{
		UserId: user.Id, PlanId: expiredPlan.Id, AmountTotal: expiredPlan.TotalAmount,
		StartTime: now - 31*24*3600, EndTime: now - 1, Status: "active", Source: "test",
	}
	excludedSubscription := &UserSubscription{
		UserId: user.Id, PlanId: excludedPlan.Id, AmountTotal: excludedPlan.TotalAmount,
		StartTime: now, EndTime: now + 10*24*3600, Status: "active", Source: "test",
	}
	fallbackSubscription := &UserSubscription{
		UserId: user.Id, PlanId: fallbackPlan.Id, AmountTotal: fallbackPlan.TotalAmount,
		StartTime: now, EndTime: now + 20*24*3600, Status: "active", Source: "test",
	}
	require.NoError(t, DB.Create(expiredSubscription).Error)
	require.NoError(t, DB.Create(excludedSubscription).Error)
	require.NoError(t, DB.Create(fallbackSubscription).Error)

	result, err := PreConsumeUserSubscription("subscription-group-excluded-fallback", user.Id, "test", "blocked", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, fallbackSubscription.Id, result.UserSubscriptionId)

	var reloadedExpired, reloadedExcluded, reloadedFallback UserSubscription
	require.NoError(t, DB.First(&reloadedExpired, expiredSubscription.Id).Error)
	require.NoError(t, DB.First(&reloadedExcluded, excludedSubscription.Id).Error)
	require.NoError(t, DB.First(&reloadedFallback, fallbackSubscription.Id).Error)
	assert.Equal(t, int64(0), reloadedExpired.AmountUsed)
	assert.Equal(t, int64(0), reloadedExcluded.AmountUsed)
	assert.Equal(t, int64(10), reloadedFallback.AmountUsed)
}
