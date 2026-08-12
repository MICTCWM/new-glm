package model

import "errors"

// Common errors
var (
	ErrDatabase = errors.New("database error")
)

// User auth errors
var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUserEmptyCredentials = errors.New("empty credentials")
)

// Token auth errors
var (
	ErrTokenNotProvided = errors.New("token not provided")
	ErrTokenInvalid     = errors.New("token invalid")
)

// Redemption errors
var ErrRedeemFailed = errors.New("redeem.failed")

// 2FA errors
var ErrTwoFANotEnabled = errors.New("2fa not enabled")

// ErrSpecialWeeklyPartialWindow 表示当前时刻处于订阅末尾的不完整周窗口内，
// 此时不提供周限额，应回退到小时限额。
var ErrSpecialWeeklyPartialWindow = errors.New("special weekly window is partial, use hourly limit instead")
