package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LoginFailureLockThreshold = 5
	LoginFailureAutoBanLimit  = 50
	LoginFailureLockDuration  = 10 * time.Minute
)

// LoginProtectionStatus contains only the state needed by the login handler.
// It deliberately does not expose password-related data.
type LoginProtectionStatus struct {
	Found       bool
	FailedCount int
	LockedUntil int64
	AutoBanned  bool
}

func findLoginUser(tx *gorm.DB, identifier string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var user User
	err := tx.Where("username = ? OR email = ?", identifier, identifier).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetLoginProtection returns the persisted lock state for a username/email.
// Missing users are intentionally represented as Found=false so callers can
// keep returning the same generic credential error.
func GetLoginProtection(identifier string) (LoginProtectionStatus, error) {
	user, err := findLoginUser(DB, identifier)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LoginProtectionStatus{}, nil
		}
		return LoginProtectionStatus{}, err
	}
	return LoginProtectionStatus{
		Found:       true,
		FailedCount: user.LoginFailedCount,
		LockedUntil: user.LoginLockedUntil,
		AutoBanned:  user.LoginAutoBanned,
	}, nil
}

// RecordLoginFailure atomically records a wrong password for an existing,
// enabled account. It does not count unknown usernames, manually disabled
// accounts, correct passwords, or attempts made while the temporary lock is
// active.
func RecordLoginFailure(identifier, password string) (LoginProtectionStatus, error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return LoginProtectionStatus{}, tx.Error
	}
	defer tx.Rollback()

	query := tx
	// SQLite does not support SELECT ... FOR UPDATE. The transaction still
	// keeps tests and single-node deployments consistent; other databases use
	// a row lock so concurrent failures cannot skip a threshold.
	if tx.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	user, err := findLoginUser(query, identifier)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LoginProtectionStatus{}, nil
		}
		return LoginProtectionStatus{}, err
	}

	now := time.Now().Unix()
	result := LoginProtectionStatus{
		Found:       true,
		FailedCount: user.LoginFailedCount,
		LockedUntil: user.LoginLockedUntil,
		AutoBanned:  user.LoginAutoBanned,
	}
	if user.Status != common.UserStatusEnabled || user.LoginAutoBanned || user.LoginLockedUntil > now {
		return result, tx.Commit().Error
	}
	if common.ValidatePasswordAndHash(password, user.Password) {
		return result, tx.Commit().Error
	}
	failedCount := user.LoginFailedCount + 1
	updates := map[string]any{"login_failed_count": failedCount}
	result.FailedCount = failedCount
	if failedCount >= LoginFailureAutoBanLimit {
		updates["status"] = common.UserStatusDisabled
		updates["login_auto_banned"] = true
		updates["login_locked_until"] = 0
		result.AutoBanned = true
		result.LockedUntil = 0
	} else if failedCount%LoginFailureLockThreshold == 0 {
		lockedUntil := time.Now().Add(LoginFailureLockDuration).Unix()
		updates["login_locked_until"] = lockedUntil
		result.LockedUntil = lockedUntil
	}

	if err = tx.Model(&User{}).Where("id = ?", user.Id).Updates(updates).Error; err != nil {
		return LoginProtectionStatus{}, err
	}
	if err = tx.Commit().Error; err != nil {
		return LoginProtectionStatus{}, err
	}
	if result.AutoBanned {
		if err = invalidateUserCache(user.Id); err != nil {
			return result, err
		}
		if err = InvalidateUserTokensCache(user.Id); err != nil {

			return result, err
		}
	}
	return result, nil
}

// LoginLockRemainingMinutes returns the user-facing remaining lock time,
// rounded up so a positive lock is never reported as zero minutes.
func LoginLockRemainingMinutes(lockedUntil int64) int {
	remaining := lockedUntil - time.Now().Unix()
	if remaining <= 0 {
		return 0
	}
	return int((remaining + int64(time.Minute.Seconds()) - 1) / int64(time.Minute.Seconds()))
}

// ResetLoginProtection is called by an administrator when unlocking an
// automatically banned account. It never changes the user's password.
func ResetLoginProtection(userID int) error {
	if userID == 0 {
		return errors.New("user id is empty")
	}
	if err := DB.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"login_failed_count": 0,
		"login_locked_until": 0,
		"login_auto_banned":  false,
	}).Error; err != nil {
		return err
	}
	return invalidateUserCache(userID)
}
