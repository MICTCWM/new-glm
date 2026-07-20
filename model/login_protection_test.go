package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func createLoginProtectionTestUser(t *testing.T, username string, failedCount int) *User {
	t.Helper()
	password, err := common.Password2Hash("correct-password")
	require.NoError(t, err)
	user := &User{
		Username:         username,
		Password:         password,
		Status:           common.UserStatusEnabled,
		LoginFailedCount: failedCount,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestRecordLoginFailureLocksEveryFifthAttempt(t *testing.T) {
	truncateTables(t)
	user := createLoginProtectionTestUser(t, "login-protection-threshold", 0)

	for attempt := 1; attempt <= 4; attempt++ {
		status, err := RecordLoginFailure(user.Username, "wrong-password")
		require.NoError(t, err)
		require.Equal(t, attempt, status.FailedCount)
		require.Zero(t, status.LockedUntil)
	}

	status, err := RecordLoginFailure(user.Username, "wrong-password")
	require.NoError(t, err)
	require.Equal(t, LoginFailureLockThreshold, status.FailedCount)
	require.GreaterOrEqual(t, status.LockedUntil, time.Now().Add(9*time.Minute).Unix())
	require.LessOrEqual(t, status.LockedUntil, time.Now().Add(11*time.Minute).Unix())

	lockedStatus, err := RecordLoginFailure(user.Username, "wrong-password")
	require.NoError(t, err)
	require.Equal(t, status.FailedCount, lockedStatus.FailedCount)
	require.Equal(t, status.LockedUntil, lockedStatus.LockedUntil)
}

func TestRecordLoginFailureAutoBansAtFiftyAndResetPreservesPassword(t *testing.T) {
	truncateTables(t)
	user := createLoginProtectionTestUser(t, "login-protection-auto-ban", LoginFailureAutoBanLimit-1)
	originalPassword := user.Password

	status, err := RecordLoginFailure(user.Username, "wrong-password")
	require.NoError(t, err)
	require.True(t, status.AutoBanned)
	require.Equal(t, LoginFailureAutoBanLimit, status.FailedCount)

	var banned User
	require.NoError(t, DB.First(&banned, user.Id).Error)
	require.Equal(t, common.UserStatusDisabled, banned.Status)
	require.True(t, banned.LoginAutoBanned)
	require.Equal(t, originalPassword, banned.Password)

	require.NoError(t, ResetLoginProtection(user.Id))
	var reset User
	require.NoError(t, DB.First(&reset, user.Id).Error)
	require.Equal(t, common.UserStatusDisabled, reset.Status)
	require.Zero(t, reset.LoginFailedCount)
	require.Zero(t, reset.LoginLockedUntil)
	require.False(t, reset.LoginAutoBanned)
	require.Equal(t, originalPassword, reset.Password)
}

func TestRecordLoginFailureSkipsUnknownAndManuallyDisabledUsers(t *testing.T) {
	truncateTables(t)

	status, err := RecordLoginFailure("unknown-login-protection-user", "wrong-password")
	require.NoError(t, err)
	require.False(t, status.Found)

	user := createLoginProtectionTestUser(t, "login-protection-disabled", 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)

	status, err = RecordLoginFailure(user.Username, "wrong-password")
	require.NoError(t, err)
	require.True(t, status.Found)
	require.Zero(t, status.FailedCount)

	var unchanged User
	require.NoError(t, DB.First(&unchanged, user.Id).Error)
	require.Zero(t, unchanged.LoginFailedCount)
}
