package model

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const UserNameMaxLength = 20

const (
	userGptQuotaColumnType = "decimal(36,18)"
	userGptQuotaSQLCast    = "DECIMAL(36,18)"
	userGptQuotaScale      = 18
)

// User if you add sensitive fields, don't forget to clean them in setupLogin function.
// Otherwise, the sensitive information will be saved on local storage in plain text!
type User struct {
	Id               int            `json:"id"`
	Username         string         `json:"username" gorm:"unique;index" validate:"max=20"`
	Password         string         `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	OriginalPassword string         `json:"original_password" gorm:"-:all"` // this field is only for Password change verification, don't save it to database!
	DisplayName      string         `json:"display_name" gorm:"index" validate:"max=20"`
	Role             int            `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status           int            `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email            string         `json:"email" gorm:"index" validate:"max=50"`
	GitHubId         string         `json:"github_id" gorm:"column:github_id;index"`
	DiscordId        string         `json:"discord_id" gorm:"column:discord_id;index"`
	OidcId           string         `json:"oidc_id" gorm:"column:oidc_id;index"`
	WeChatId         string         `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId       string         `json:"telegram_id" gorm:"column:telegram_id;index"`
	VerificationCode string         `json:"verification_code" gorm:"-:all"`                                    // this field is only for Email verification, don't save it to database!
	EntryCode        string         `json:"entry_code" gorm:"-:all"`                                           // 注册进入码，仅用于注册校验，不写入数据库
	AccessToken      *string        `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota            int            `json:"quota" gorm:"type:int;default:0"`
	GptQuota         float64        `json:"gpt_quota" gorm:"type:decimal;default:0"`         // GPT 专属额度（用户开启 GPT 模式后将基础余额转换得到）
	UsedQuota        int            `json:"used_quota" gorm:"type:int;default:0;column:used_quota"` // used quota
	RequestCount     int            `json:"request_count" gorm:"type:int;default:0;"`               // request number
	Group            string         `json:"group" gorm:"type:varchar(64);default:'default'"`
	AffCode          string         `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount         int            `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota         int            `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`           // 邀请剩余额度
	AffHistoryQuota  int            `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"` // 邀请历史额度
	InviterId        int            `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	LinuxDOId        string         `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	Setting          string         `json:"setting" gorm:"type:text;column:setting"`
	Remark           string         `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	RpmLimit         int            `json:"rpm_limit" gorm:"type:int;default:0;column:rpm_limit" validate:"gte=0"` // 用户级每分钟请求上限,0 = 沿用分组/全局配置,>0 时覆盖分组与全局限流
	StripeCustomer   string         `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
	CreatedAt        int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	LastLoginAt      int64          `json:"last_login_at" gorm:"default:0;column:last_login_at"`
	LoginFailedCount int            `json:"login_failed_count" gorm:"type:int;default:0;column:login_failed_count"`
	LoginLockedUntil int64          `json:"login_locked_until" gorm:"type:bigint;default:0;column:login_locked_until"`
	LoginAutoBanned  bool           `json:"login_auto_banned" gorm:"default:false;column:login_auto_banned"`
}

func (user *User) ToBaseUser() *UserBase {
	cache := &UserBase{
		Id:       user.Id,
		Group:    user.Group,
		Quota:    user.Quota,
		Status:   user.Status,
		Username: user.Username,
		Setting:  user.Setting,
		Email:    user.Email,
		RpmLimit: user.RpmLimit,
	}
	return cache
}

func (user *User) GetAccessToken() string {
	if user.AccessToken == nil {
		return ""
	}
	return *user.AccessToken
}

func (user *User) SetAccessToken(token string) {
	user.AccessToken = &token
}

func (user *User) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := json.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

func (user *User) SetSetting(setting dto.UserSetting) {
	settingBytes, err := json.Marshal(setting)
	if err != nil {
		common.SysLog("failed to marshal setting: " + err.Error())
		return
	}
	user.Setting = string(settingBytes)
}

// 根据用户角色生成默认的边栏配置
func generateDefaultSidebarConfigForRole(userRole int) string {
	defaultConfig := map[string]interface{}{}

	// 聊天区域 - 所有用户都可以访问
	defaultConfig["chat"] = map[string]interface{}{
		"enabled":    true,
		"playground": true,
		"chat":       true,
	}

	// 控制台区域 - 所有用户都可以访问
	defaultConfig["console"] = map[string]interface{}{
		"enabled":    true,
		"detail":     true,
		"token":      true,
		"log":        true,
		"midjourney": true,
		"task":       true,
	}

	// 个人中心区域 - 所有用户都可以访问
	defaultConfig["personal"] = map[string]interface{}{
		"enabled":  true,
		"topup":    true,
		"personal": true,
	}

	// 管理员区域 - 根据角色决定
	if userRole == common.RoleAdminUser {
		// 管理员可以访问管理员区域，但不能访问系统设置
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    false, // 管理员不能访问系统设置
		}
	} else if userRole == common.RoleRootUser {
		// 超级管理员可以访问所有功能
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    true,
		}
	}
	// 普通用户不包含admin区域

	// 转换为JSON字符串
	configBytes, err := json.Marshal(defaultConfig)
	if err != nil {
		common.SysLog("生成默认边栏配置失败: " + err.Error())
		return ""
	}

	return string(configBytes)
}

// CheckUserExistOrDeleted check if user exist or deleted, if not exist, return false, nil, if deleted or exist, return true, nil
func CheckUserExistOrDeleted(username string, email string) (bool, error) {
	var user User

	// err := DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	// check email if empty
	var err error
	if email == "" {
		err = DB.Unscoped().First(&user, "username = ?", username).Error
	} else {
		err = DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not exist, return false, nil
			return false, nil
		}
		// other error, return false, err
		return false, err
	}
	// exist, return true, nil
	return true, nil
}

func GetMaxUserId() int {
	var user User
	DB.Unscoped().Last(&user)
	return user.Id
}

func GetAllUsers(pageInfo *common.PageInfo) (users []*User, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get total count within transaction
	err = tx.Unscoped().Model(&User{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated users within same transaction
	err = tx.Unscoped().Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Omit("password").Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func SearchUsers(keyword string, group string, startIdx int, num int) ([]*User, int64, error) {
	var users []*User
	var total int64
	var err error

	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 构建基础查询
	query := tx.Unscoped().Model(&User{})

	// 构建搜索条件
	likeCondition := "username LIKE ? OR email LIKE ? OR display_name LIKE ?"

	// 尝试将关键字转换为整数ID
	keywordInt, err := strconv.Atoi(keyword)
	if err == nil {
		// 如果是数字，同时搜索ID和其他字段
		likeCondition = "id = ? OR " + likeCondition
		if group != "" {
			query = query.Where("("+likeCondition+") AND "+commonGroupCol+" = ?",
				keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", group)
		} else {
			query = query.Where(likeCondition,
				keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	} else {
		// 非数字关键字，只搜索字符串字段
		if group != "" {
			query = query.Where("("+likeCondition+") AND "+commonGroupCol+" = ?",
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", group)
		} else {
			query = query.Where(likeCondition,
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	}

	// 获取总数
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = query.Omit("password").Order("id desc").Limit(num).Offset(startIdx).Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func GetUserById(id int, selectAll bool) (*User, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	user := User{Id: id}
	var err error = nil
	if selectAll {
		err = DB.First(&user, "id = ?", id).Error
	} else {
		err = DB.Omit("password").First(&user, "id = ?", id).Error
	}
	return &user, err
}

func GetUserIdByAffCode(affCode string) (int, error) {
	if affCode == "" {
		return 0, errors.New("affCode 为空！")
	}
	var user User
	err := DB.Select("id").First(&user, "aff_code = ?", affCode).Error
	return user.Id, err
}

func DeleteUserById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	user := User{Id: id}
	return user.Delete()
}

func HardDeleteUserById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	err := DB.Unscoped().Delete(&User{}, "id = ?", id).Error
	return err
}

func inviteUser(inviterId int) (err error) {
	user, err := GetUserById(inviterId, true)
	if err != nil {
		return err
	}
	user.AffCount++
	user.AffQuota += common.QuotaForInviter
	user.AffHistoryQuota += common.QuotaForInviter
	return DB.Save(user).Error
}

func (user *User) TransferAffQuotaToQuota(quota int) error {
	// 检查quota是否小于最小额度
	if float64(quota) < common.QuotaPerUnit {
		return fmt.Errorf("转移额度最小为%s！", logger.LogQuota(int(common.QuotaPerUnit)))
	}

	// 开始数据库事务
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback() // 确保在函数退出时事务能回滚

	// 加锁查询用户以确保数据一致性
	err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, user.Id).Error
	if err != nil {
		return err
	}

	// 再次检查用户的AffQuota是否足够
	if user.AffQuota < quota {
		return errors.New("邀请额度不足！")
	}

	// 更新用户额度
	user.AffQuota -= quota
	user.Quota += quota

	// 保存用户状态
	if err := tx.Save(user).Error; err != nil {
		return err
	}

	// 提交事务
	return tx.Commit().Error
}

func (user *User) Insert(inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = common.QuotaForNewUser
	//user.SetAccessToken(common.GetUUID())
	user.AffCode = common.GetRandomString(4)

	// 初始化用户设置，包括默认的边栏配置
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		// 这里暂时不设置SidebarModules，因为需要在用户创建后根据角色设置
		user.SetSetting(defaultSetting)
	}

	result := DB.Create(user)
	if result.Error != nil {
		return result.Error
	}

	// 用户创建成功后，根据角色初始化边栏配置
	// 需要重新获取用户以确保有正确的ID和Role
	var createdUser User
	if err := DB.Where("username = ?", user.Username).First(&createdUser).Error; err == nil {
		// 生成基于角色的默认边栏配置
		defaultSidebarConfig := generateDefaultSidebarConfigForRole(createdUser.Role)
		if defaultSidebarConfig != "" {
			currentSetting := createdUser.GetSetting()
			currentSetting.SidebarModules = defaultSidebarConfig
			createdUser.SetSetting(currentSetting)
			createdUser.Update(false)
			common.SysLog(fmt.Sprintf("为新用户 %s (角色: %d) 初始化边栏配置", createdUser.Username, createdUser.Role))
		}
	}

	if common.QuotaForNewUser > 0 {
		RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", logger.LogQuota(common.QuotaForNewUser)))
	}
	if inviterId != 0 && operation_setting.IsPaymentComplianceConfirmed() {
		if common.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, common.QuotaForInvitee, true)
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", logger.LogQuota(common.QuotaForInvitee)))
		}
		if common.QuotaForInviter > 0 {
			//_ = IncreaseUserQuota(inviterId, common.QuotaForInviter)
			RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", logger.LogQuota(common.QuotaForInviter)))
			_ = inviteUser(inviterId)
		}
	}
	return nil
}

// InsertWithTx inserts a new user within an existing transaction.
// This is used for OAuth registration where user creation and binding need to be atomic.
// Post-creation tasks (sidebar config, logs, inviter rewards) are handled after the transaction commits.
func (user *User) InsertWithTx(tx *gorm.DB, inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = common.QuotaForNewUser
	user.AffCode = common.GetRandomString(4)

	// 初始化用户设置
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		user.SetSetting(defaultSetting)
	}

	result := tx.Create(user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// FinalizeOAuthUserCreation performs post-transaction tasks for OAuth user creation.
// This should be called after the transaction commits successfully.
func (user *User) FinalizeOAuthUserCreation(inviterId int) {
	// 用户创建成功后，根据角色初始化边栏配置
	var createdUser User
	if err := DB.Where("id = ?", user.Id).First(&createdUser).Error; err == nil {
		defaultSidebarConfig := generateDefaultSidebarConfigForRole(createdUser.Role)
		if defaultSidebarConfig != "" {
			currentSetting := createdUser.GetSetting()
			currentSetting.SidebarModules = defaultSidebarConfig
			createdUser.SetSetting(currentSetting)
			createdUser.Update(false)
			common.SysLog(fmt.Sprintf("为新用户 %s (角色: %d) 初始化边栏配置", createdUser.Username, createdUser.Role))
		}
	}

	if common.QuotaForNewUser > 0 {
		RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", logger.LogQuota(common.QuotaForNewUser)))
	}
	if inviterId != 0 && operation_setting.IsPaymentComplianceConfirmed() {
		if common.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, common.QuotaForInvitee, true)
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", logger.LogQuota(common.QuotaForInvitee)))
		}
		if common.QuotaForInviter > 0 {
			RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", logger.LogQuota(common.QuotaForInviter)))
			_ = inviteUser(inviterId)
		}
	}
}

func (user *User) Update(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	newUser := *user
	DB.First(&user, user.Id)
	if err = DB.Model(user).Updates(newUser).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) Edit(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}

	newUser := *user
	updates := map[string]interface{}{
		"username":     newUser.Username,
		"display_name": newUser.DisplayName,
		"group":        newUser.Group,
		"remark":       newUser.Remark,
		"rpm_limit":    newUser.RpmLimit,
	}
	if updatePassword {
		updates["password"] = newUser.Password
	}

	DB.First(&user, user.Id)
	if err = DB.Model(user).Updates(updates).Error; err != nil {
		return err
	}

	// 重新加载用户,确保缓存里的字段(包括 RpmLimit)是最新值。
	// 上面 DB.First(&user, user.Id) 把 user 重载成了旧值,这里必须再读一次。
	if err = DB.First(&user, user.Id).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) ClearBinding(bindingType string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}

	bindingColumnMap := map[string]string{
		"email":    "email",
		"github":   "github_id",
		"discord":  "discord_id",
		"oidc":     "oidc_id",
		"wechat":   "wechat_id",
		"telegram": "telegram_id",
		"linuxdo":  "linux_do_id",
	}

	column, ok := bindingColumnMap[bindingType]
	if !ok {
		return errors.New("invalid binding type")
	}

	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update(column, "").Error; err != nil {
		return err
	}

	if err := DB.Where("id = ?", user.Id).First(user).Error; err != nil {
		return err
	}

	return updateUserCache(*user)
}

func (user *User) Delete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	if err := DB.Delete(user).Error; err != nil {
		return err
	}

	// 清除缓存
	return invalidateUserCache(user.Id)
}

func (user *User) HardDelete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	err := DB.Unscoped().Delete(user).Error
	return err
}

// ValidateAndFill check password & user status
func (user *User) ValidateAndFill() (err error) {
	// When querying with struct, GORM will only query with non-zero fields,
	// that means if your field's value is 0, '', false or other zero values,
	// it won't be used to build query conditions
	password := user.Password
	username := strings.TrimSpace(user.Username)
	if username == "" || password == "" {
		return ErrUserEmptyCredentials
	}
	// find by username or email
	err = DB.Where("username = ? OR email = ?", username, username).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	okay := common.ValidatePasswordAndHash(password, user.Password)
	if !okay || user.Status != common.UserStatusEnabled || user.LoginAutoBanned || user.LoginLockedUntil > time.Now().Unix() {
		return ErrInvalidCredentials
	}
	return nil
}

func (user *User) FillUserById() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	DB.Where(User{Id: user.Id}).First(user)
	return nil
}

func (user *User) FillUserByEmail() error {
	if user.Email == "" {
		return errors.New("email 为空！")
	}
	DB.Where(User{Email: user.Email}).First(user)
	return nil
}

func (user *User) FillUserByGitHubId() error {
	if user.GitHubId == "" {
		return errors.New("GitHub id 为空！")
	}
	DB.Where(User{GitHubId: user.GitHubId}).First(user)
	return nil
}

// UpdateGitHubId updates the user's GitHub ID (used for migration from login to numeric ID)
func (user *User) UpdateGitHubId(newGitHubId string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}
	return DB.Model(user).Update("github_id", newGitHubId).Error
}

func (user *User) FillUserByDiscordId() error {
	if user.DiscordId == "" {
		return errors.New("discord id 为空！")
	}
	DB.Where(User{DiscordId: user.DiscordId}).First(user)
	return nil
}

func (user *User) FillUserByOidcId() error {
	if user.OidcId == "" {
		return errors.New("oidc id 为空！")
	}
	DB.Where(User{OidcId: user.OidcId}).First(user)
	return nil
}

func (user *User) FillUserByWeChatId() error {
	if user.WeChatId == "" {
		return errors.New("WeChat id 为空！")
	}
	DB.Where(User{WeChatId: user.WeChatId}).First(user)
	return nil
}

func (user *User) FillUserByTelegramId() error {
	if user.TelegramId == "" {
		return errors.New("Telegram id 为空！")
	}
	err := DB.Where(User{TelegramId: user.TelegramId}).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("该 Telegram 账户未绑定")
	}
	return nil
}

func IsEmailAlreadyTaken(email string) bool {
	return DB.Unscoped().Where("email = ?", email).Find(&User{}).RowsAffected == 1
}

func IsWeChatIdAlreadyTaken(wechatId string) bool {
	return DB.Unscoped().Where("wechat_id = ?", wechatId).Find(&User{}).RowsAffected == 1
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return DB.Unscoped().Where("github_id = ?", githubId).Find(&User{}).RowsAffected == 1
}

func IsDiscordIdAlreadyTaken(discordId string) bool {
	return DB.Unscoped().Where("discord_id = ?", discordId).Find(&User{}).RowsAffected == 1
}

func IsOidcIdAlreadyTaken(oidcId string) bool {
	return DB.Where("oidc_id = ?", oidcId).Find(&User{}).RowsAffected == 1
}

func IsTelegramIdAlreadyTaken(telegramId string) bool {
	return DB.Unscoped().Where("telegram_id = ?", telegramId).Find(&User{}).RowsAffected == 1
}

func ResetUserPasswordByEmail(email string, password string) error {
	if email == "" || password == "" {
		return errors.New("邮箱地址或密码为空！")
	}
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return err
	}
	err = DB.Model(&User{}).Where("email = ?", email).Update("password", hashedPassword).Error
	return err
}

func IsAdmin(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	err := DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		common.SysLog("no such user " + err.Error())
		return false
	}
	return user.Role >= common.RoleAdminUser
}

//// IsUserEnabled checks user status from Redis first, falls back to DB if needed
//func IsUserEnabled(id int, fromDB bool) (status bool, err error) {
//	defer func() {
//		// Update Redis cache asynchronously on successful DB read
//		if shouldUpdateRedis(fromDB, err) {
//			gopool.Go(func() {
//				if err := updateUserStatusCache(id, status); err != nil {
//					common.SysError("failed to update user status cache: " + err.Error())
//				}
//			})
//		}
//	}()
//	if !fromDB && common.RedisEnabled {
//		// Try Redis first
//		status, err := getUserStatusCache(id)
//		if err == nil {
//			return status == common.UserStatusEnabled, nil
//		}
//		// Don't return error - fall through to DB
//	}
//	fromDB = true
//	var user User
//	err = DB.Where("id = ?", id).Select("status").Find(&user).Error
//	if err != nil {
//		return false, err
//	}
//
//	return user.Status == common.UserStatusEnabled, nil
//}

func ValidateAccessToken(token string) (*User, error) {
	if token == "" {
		return nil, nil
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	user := &User{}
	err := DB.Where("access_token = ?", token).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return user, nil
}

// GetUserQuota gets quota from Redis first, falls back to DB if needed
func GetUserQuota(id int, fromDB bool) (quota int, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserQuotaCache(id, quota); err != nil {
					common.SysLog("failed to update user quota cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		quota, err := getUserQuotaCache(id)
		if err == nil {
			return quota, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select("quota").Find(&quota).Error
	if err != nil {
		return 0, err
	}

	return quota, nil
}

func GetUserUsedQuota(id int) (quota int, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("used_quota").Find(&quota).Error
	return quota, err
}

func GetUserEmail(id int) (email string, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("email").Find(&email).Error
	return email, err
}

// GetUserGroup gets group from Redis first, falls back to DB if needed
func GetUserGroup(id int, fromDB bool) (group string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserGroupCache(id, group); err != nil {
					common.SysLog("failed to update user group cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		group, err := getUserGroupCache(id)
		if err == nil {
			return group, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select(commonGroupCol).Find(&group).Error
	if err != nil {
		return "", err
	}

	return group, nil
}

// GetUserSetting gets setting from Redis first, falls back to DB if needed
func GetUserSetting(id int, fromDB bool) (settingMap dto.UserSetting, err error) {
	var setting string
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserSettingCache(id, setting); err != nil {
					common.SysLog("failed to update user setting cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		setting, err := getUserSettingCache(id)
		if err == nil {
			return setting, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	// can be nil setting
	var safeSetting sql.NullString
	err = DB.Model(&User{}).Where("id = ?", id).Select("setting").Find(&safeSetting).Error
	if err != nil {
		return settingMap, err
	}
	if safeSetting.Valid {
		setting = safeSetting.String
	} else {
		setting = ""
	}
	userBase := &UserBase{
		Setting: setting,
	}
	return userBase.GetSetting(), nil
}

func IncreaseUserQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheIncrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return increaseUserQuota(id, quota)
}

func increaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

func DecreaseUserQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheDecrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to decrease user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return decreaseUserQuota(id, quota)
}

func decreaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota - ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

func DeltaUpdateUserQuota(id int, delta int) (err error) {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return IncreaseUserQuota(id, delta, false)
	} else {
		return DecreaseUserQuota(id, -delta, false)
	}
}

func gptQuotaAmount(quota float64) (decimal.Decimal, error) {
	if math.IsNaN(quota) || math.IsInf(quota, 0) {
		return decimal.Zero, errors.New("gpt_quota 不是有效数字")
	}
	amount := decimal.NewFromFloat(quota).Round(userGptQuotaScale)
	if amount.IsNegative() {
		return decimal.Zero, errors.New("gpt_quota 不能为负数！")
	}
	return amount, nil
}

func gptQuotaSQLValue(amount decimal.Decimal) string {
	return amount.StringFixed(userGptQuotaScale)
}

func gptBillingQuotaFromBaseQuotaDecimal(baseQuota int) decimal.Decimal {
	// GPT 请求扣费沿用日志中的数值语义：500000 内部额度 = 1 GPT 扣费单位。
	return decimal.NewFromInt(int64(baseQuota)).
		Div(decimal.NewFromInt(500000)).
		Round(userGptQuotaScale)
}

func gptTransferQuotaFromBaseQuotaDecimal(baseQuota int) decimal.Decimal {
	// GPT 钱包互转规则：500 基础余额 = 1.5 GPT 余额。
	return decimal.NewFromInt(int64(baseQuota)).
		Mul(decimal.NewFromInt(3)).
		Div(decimal.NewFromInt(500000000)).
		Round(userGptQuotaScale)
}

// GptQuotaFromBaseQuota converts internal quota units to GPT billing units.
func GptQuotaFromBaseQuota(baseQuota int) float64 {
	value, _ := gptBillingQuotaFromBaseQuotaDecimal(baseQuota).Float64()
	return value
}

// GetUserGptQuota 从数据库读取用户的 GPT 专属额度
// GPT 额度充值是低频操作，暂不引入 Redis 缓存，直接操作 DB
func GetUserGptQuota(id int, fromDB bool) (gptQuota float64, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("gpt_quota").Find(&gptQuota).Error
	if err != nil {
		return 0, err
	}
	return gptQuota, nil
}

// IncreaseUserGptQuota 增加用户的 GPT 专属额度
func IncreaseUserGptQuota(id int, quota float64) error {
	amount, err := gptQuotaAmount(quota)
	if err != nil {
		return err
	}
	if amount.IsZero() {
		return nil
	}
	return DB.Model(&User{}).
		Where("id = ?", id).
		Update("gpt_quota", gorm.Expr("gpt_quota + CAST(? AS "+userGptQuotaSQLCast+")", gptQuotaSQLValue(amount))).Error
}

// DecreaseUserGptQuota 扣减用户的 GPT 专属额度
// 余额不足时返回错误，避免额度变成负数
func DecreaseUserGptQuota(id int, quota float64) error {
	amount, err := gptQuotaAmount(quota)
	if err != nil {
		return err
	}
	if amount.IsZero() {
		return nil
	}
	sqlAmount := gptQuotaSQLValue(amount)
	result := DB.Model(&User{}).
		Where("id = ? AND gpt_quota >= CAST(? AS "+userGptQuotaSQLCast+")", id, sqlAmount).
		Update("gpt_quota", gorm.Expr("gpt_quota - CAST(? AS "+userGptQuotaSQLCast+")", sqlAmount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("gpt_quota 余额不足")
	}
	return nil
}

// ForceDecreaseUserGptQuota 扣减用户的 GPT 专属额度，允许余额变成负数。
// 仅应用于请求完成后的补扣场景，避免上游已成功响应但补扣失败导致漏计费。
func ForceDecreaseUserGptQuota(id int, quota float64) error {
	amount, err := gptQuotaAmount(quota)
	if err != nil {
		return err
	}
	if amount.IsZero() {
		return nil
	}
	return DB.Model(&User{}).
		Where("id = ?", id).
		Update("gpt_quota", gorm.Expr("gpt_quota - CAST(? AS "+userGptQuotaSQLCast+")", gptQuotaSQLValue(amount))).Error
}

// CalcDailyPriceFromPlan calculates the daily price in USD from a SubscriptionPlan.
// It converts the plan's duration to months, then divides PriceAmount by total days.
func CalcDailyPriceFromPlan(plan *SubscriptionPlan) float64 {
	if plan == nil || plan.PriceAmount <= 0 {
		return 0
	}
	var totalMonths float64
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		totalMonths = float64(plan.DurationValue) * 12
	case SubscriptionDurationMonth:
		totalMonths = float64(plan.DurationValue)
	case SubscriptionDurationDay:
		totalMonths = float64(plan.DurationValue) / 30.0
	case SubscriptionDurationHour:
		totalMonths = float64(plan.DurationValue) / (30.0 * 24.0)
	case SubscriptionDurationCustom:
		if plan.CustomSeconds > 0 {
			totalMonths = float64(plan.CustomSeconds) / (30.0 * 24.0 * 3600.0)
		}
	default:
		totalMonths = 1
	}
	if totalMonths <= 0 {
		return 0
	}
	totalDays := totalMonths * 30.0
	if totalDays <= 0 {
		return 0
	}
	return plan.PriceAmount / totalDays
}

// TransferQuotaToGptQuota 将基础余额转换为 GPT 专属额度
// 规则：500 基础余额 = 1.5 GPT 余额（250000000 内部额度 = 1.5 GPT）。
// 事务内 FOR UPDATE 锁定用户行，保证扣减与增加的原子性
func TransferQuotaToGptQuota(userId int, baseQuota int) (float64, error) {
	if baseQuota <= 0 {
		return 0, errors.New("转换额度必须大于 0！")
	}

	tx := DB.Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer tx.Rollback() // 确保在函数退出时事务能回滚

	// 加锁查询用户以确保数据一致性
	user := &User{}
	err := tx.Set("gorm:query_option", "FOR UPDATE").First(user, userId).Error
	if err != nil {
		return 0, err
	}

	// 检查用户的基础余额是否充足
	if user.Quota < baseQuota {
		return 0, errors.New("基础余额不足！")
	}

	// 计算可获得的 GPT 额度（钱包互转规则）
	gptQuotaDecimal := gptTransferQuotaFromBaseQuotaDecimal(baseQuota)
	gptQuota, _ := gptQuotaDecimal.Float64()

	// 更新用户额度
	result := tx.Model(&User{}).
		Where("id = ? AND quota >= ?", userId, baseQuota).
		Updates(map[string]interface{}{
			"quota":     gorm.Expr("quota - ?", baseQuota),
			"gpt_quota": gorm.Expr("gpt_quota + CAST(? AS "+userGptQuotaSQLCast+")", gptQuotaSQLValue(gptQuotaDecimal)),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, errors.New("基础余额不足！")
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	// 记录转换日志
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("转换 %d 基础额度为 %.9f GPT 额度", baseQuota, gptQuota))

	return gptQuota, nil
}

// TransferGptQuotaToQuota 将 GPT 专属额度转换回基础余额
// 规则：按钱包互转汇率反向转换，1 GPT = 500000000 / 3 内部额度。
// 事务内 FOR UPDATE 锁定用户行，保证扣减与增加的原子性
func TransferGptQuotaToQuota(userId int, gptQuota float64) (int, error) {
	if gptQuota <= 0 {
		return 0, errors.New("转换额度必须大于 0！")
	}
	gptQuotaDecimal, err := gptQuotaAmount(gptQuota)
	if err != nil {
		return 0, err
	}
	if gptQuotaDecimal.IsZero() {
		return 0, errors.New("转换金额过小，无法转换为有效基础额度！")
	}

	tx := DB.Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer tx.Rollback()

	user := &User{}
	err = tx.Set("gorm:query_option", "FOR UPDATE").First(user, userId).Error
	if err != nil {
		return 0, err
	}

	// 检查用户的 GPT 额度是否充足
	currentGptQuota := decimal.NewFromFloat(user.GptQuota).Round(userGptQuotaScale)
	if currentGptQuota.LessThan(gptQuotaDecimal) {
		return 0, errors.New("GPT 额度不足！")
	}

	// 反向计算可获得的内部额度。内部额度是整数，向下取整避免小数向上取整套利。
	baseQuota := int(gptQuotaDecimal.
		Mul(decimal.NewFromInt(500000000)).
		Div(decimal.NewFromInt(3)).
		IntPart())
	if baseQuota <= 0 {
		return 0, errors.New("转换金额过小，无法转换为有效基础额度！")
	}

	sqlAmount := gptQuotaSQLValue(gptQuotaDecimal)
	result := tx.Model(&User{}).
		Where("id = ? AND gpt_quota >= CAST(? AS "+userGptQuotaSQLCast+")", userId, sqlAmount).
		Updates(map[string]interface{}{
			"quota":     gorm.Expr("quota + ?", baseQuota),
			"gpt_quota": gorm.Expr("gpt_quota - CAST(? AS "+userGptQuotaSQLCast+")", sqlAmount),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, errors.New("GPT 额度不足！")
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	RecordLog(userId, LogTypeTopup, fmt.Sprintf("转换 %.9f GPT 额度为 %d 基础额度", gptQuota, baseQuota))

	return baseQuota, nil
}

// DisableGptModeForAllUsers 管理员关闭 GPT 模式时，强制退出所有 GPT 模式用户，
// 并将其 GPT 额度全部转为基础额度。
// 同时设置通知标志（GptModeDisabledAt 时间戳），供前端登录后弹窗提示。
// 单个用户处理失败不会中断整体流程，仅记录日志，保证批量处理的健壮性。
func DisableGptModeForAllUsers() error {
	// 1. 查询所有 Setting 中包含 "gpt_mode":true 的用户（LIKE 粗筛）
	var users []User
	if err := DB.Where("setting LIKE ?", "%\"gpt_mode\":true%").Find(&users).Error; err != nil {
		return err
	}

	// 2. 逐个处理：关闭 GptMode + GPT 额度转基础额度
	for i := range users {
		u := &users[i]
		setting := u.GetSetting()
		// LIKE 查询可能误判，二次确认确实为 GPT 模式用户
		if !setting.GptMode {
			continue
		}

		// 2.1 关闭 GptMode 并保存设置到数据库
		setting.GptMode = false
		u.SetSetting(setting)
		if err := DB.Model(u).Updates(map[string]interface{}{
			"setting": u.Setting,
		}).Error; err != nil {
			common.SysError(fmt.Sprintf("保存用户 %d 设置失败: %v", u.Id, err))
			// 设置保存失败则跳过后续额度转换，避免数据不一致
			continue
		}

		// 2.2 GPT 额度转基础额度（如果有 GPT 额度）
		if u.GptQuota > 0 {
			if _, err := TransferGptQuotaToQuota(u.Id, u.GptQuota); err != nil {
				common.SysError(fmt.Sprintf("用户 %d GPT 额度转换失败: %v", u.Id, err))
				// 额度转换失败不影响 GptMode 关闭
			}
		}

		// 2.3 清除用户缓存，避免 userCache.GetSetting().GptMode 仍返回 true
		if err := InvalidateUserCache(u.Id); err != nil {
			common.SysError(fmt.Sprintf("清除用户 %d 缓存失败: %v", u.Id, err))
		}
	}

	// 3. 设置通知标志（记录关闭时间戳，供前端判断是否需要弹窗）
	disabledAt := strconv.FormatInt(time.Now().Unix(), 10)
	if err := UpdateOption("GptModeDisabledAt", disabledAt); err != nil {
		common.SysError(fmt.Sprintf("保存 GptModeDisabledAt 失败: %v", err))
	}

	return nil
}

// AddGptQuota increases the user's GPT quota by the given amount.
// This function operates on an existing transaction.
func AddGptQuota(tx *gorm.DB, userId int, gptQuota float64) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	amount, err := gptQuotaAmount(gptQuota)
	if err != nil {
		return err
	}
	if !amount.IsPositive() {
		return errors.New("gpt quota must be > 0")
	}
	return tx.Model(&User{}).
		Where("id = ?", userId).
		Update("gpt_quota", gorm.Expr("gpt_quota + CAST(? AS "+userGptQuotaSQLCast+")", gptQuotaSQLValue(amount))).Error
}

//func GetRootUserEmail() (email string) {
//	DB.Model(&User{}).Where("role = ?", common.RoleRootUser).Select("email").Find(&email)
//	return email
//}

func GetRootUser() (user *User) {
	DB.Where("role = ?", common.RoleRootUser).First(&user)
	return user
}

func UpdateUserLastLoginAt(id int) {
	if err := DB.Model(&User{}).Where("id = ?", id).Update("last_login_at", common.GetTimestamp()).Error; err != nil {
		common.SysLog("failed to update user last_login_at: " + err.Error())
	}
}

func UpdateUserUsedQuotaAndRequestCount(id int, quota int) {
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUsedQuota, id, quota)
		addNewRecord(BatchUpdateTypeRequestCount, id, 1)
		return
	}
	updateUserUsedQuotaAndRequestCount(id, quota, 1)
}

func updateUserUsedQuotaAndRequestCount(id int, quota int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota and request count: " + err.Error())
		return
	}

	//// 更新缓存
	//if err := invalidateUserCache(id); err != nil {
	//	common.SysError("failed to invalidate user cache: " + err.Error())
	//}
}

func updateUserUsedQuota(id int, quota int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota: " + err.Error())
	}
}

func updateUserRequestCount(id int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Update("request_count", gorm.Expr("request_count + ?", count)).Error
	if err != nil {
		common.SysLog("failed to update user request count: " + err.Error())
	}
}

// GetUsernameById gets username from Redis first, falls back to DB if needed
func GetUsernameById(id int, fromDB bool) (username string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserNameCache(id, username); err != nil {
					common.SysLog("failed to update user name cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		username, err := getUserNameCache(id)
		if err == nil {
			return username, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select("username").Find(&username).Error
	if err != nil {
		return "", err
	}

	return username, nil
}

func IsLinuxDOIdAlreadyTaken(linuxDOId string) bool {
	var user User
	err := DB.Unscoped().Where("linux_do_id = ?", linuxDOId).First(&user).Error
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

func (user *User) FillUserByLinuxDOId() error {
	if user.LinuxDOId == "" {
		return errors.New("linux do id is empty")
	}
	err := DB.Where("linux_do_id = ?", user.LinuxDOId).First(user).Error
	return err
}

func RootUserExists() bool {
	var user User
	err := DB.Where("role = ?", common.RoleRootUser).First(&user).Error
	if err != nil {
		return false
	}
	return true
}

// UserSubscriptionBrief 用户订阅精简信息
type UserSubscriptionBrief struct {
	PlanId      int    `json:"plan_id"`
	PlanTitle   string `json:"plan_title"`
	AmountTotal int64  `json:"amount_total"`
	AmountUsed  int64  `json:"amount_used"`
	EndTime     int64  `json:"end_time"`
}

// UserBalanceInfo 用户余额精简信息
type UserBalanceInfo struct {
	Id            int                     `json:"id"`
	Username      string                  `json:"username"`
	DisplayName   string                  `json:"display_name"`
	Quota         int                     `json:"quota"`
	GptQuota      float64                 `json:"gpt_quota"`
	UsedQuota     int                     `json:"used_quota"`
	Subscriptions []UserSubscriptionBrief `json:"subscriptions"`
	RenewScore            int                      `json:"renew_score"`           // 续费潜力评分 0-100
	RenewLevel            RenewPotentialLevel      `json:"renew_level"`           // 续费潜力等级
	RegressionLevel       RegressionPotentialLevel `json:"regression_level"`      // 回归潜力等级
	DailyConsume          float64                  `json:"daily_consume"`         // 近N天日均消耗
	QuotaRemainingRatio   float64                  `json:"quota_remaining_ratio"` // 剩余额度占比 (0-1)
	LastLoginAt           int64                    `json:"last_login_at"`         // 最后登录时间
	LastSubEndTime        int64                    `json:"last_sub_end_time"`     // 最后订阅过期时间
}

// GetAllUserBalances 获取所有用户的余额信息（仅返回余额相关字段，强制上限防止 OOM）
func GetAllUserBalances() ([]UserBalanceInfo, error) {
	var balances []UserBalanceInfo
	err := DB.Model(&User{}).
		Select("id, username, display_name, quota, gpt_quota, used_quota").
		Order("id desc").
		Limit(10000).
		Find(&balances).Error
	if err != nil {
		return nil, err
	}

	// 提前构建 userIds，供后续订阅查询、消费统计等复用
	userIds := make([]int, len(balances))
	for i, b := range balances {
		userIds[i] = b.Id
	}

	// 批量查询活跃订阅，避免 N+1 问题
	if len(balances) > 0 {
		now := common.GetTimestamp()
		var subs []UserSubscription
		err = DB.Where("user_id IN ? AND status = ? AND end_time > ?", userIds, "active", now).
			Find(&subs).Error
		if err != nil {
			return balances, nil // 订阅查询失败不影响余额返回
		}
		// 批量获取 plan title
		planIds := make(map[int]bool)
		for _, s := range subs {
			planIds[s.PlanId] = true
		}
		planTitles := make(map[int]string)
		for planId := range planIds {
			if plan, err := GetSubscriptionPlanById(planId); err == nil && plan != nil {
				planTitles[planId] = plan.Title
			}
		}
		// 按用户分组
		subsByUser := make(map[int][]UserSubscriptionBrief)
		for _, s := range subs {
			subsByUser[s.UserId] = append(subsByUser[s.UserId], UserSubscriptionBrief{
				PlanId:      s.PlanId,
				PlanTitle:   planTitles[s.PlanId],
				AmountTotal: s.AmountTotal,
				AmountUsed:  s.AmountUsed,
				EndTime:     s.EndTime,
			})
		}
		for i := range balances {
			balances[i].Subscriptions = subsByUser[balances[i].Id]
		}
	}

	// 批量查询近N天消费记录
	now := common.GetTimestamp()
	periodDays := common.ConsumeStatPeriodDays
	if periodDays <= 0 {
		periodDays = 30
	}
	startTime := now - int64(periodDays)*86400
	consumeMap, err := SumUsedQuotaByUserIds(userIds, startTime)
	if err != nil {
		// 消费统计失败不影响余额返回
		consumeMap = make(map[int]int)
	}

	// 批量查询 last_login_at（在评分之前查询，确保回归潜力计算可用）
	loginMap := make(map[int]int64)
	if len(userIds) > 0 {
		var users []User
		DB.Select("id, last_login_at").Where("id IN ?", userIds).Find(&users)
		for _, u := range users {
			loginMap[u.Id] = u.LastLoginAt
		}
	}

	// 查询所有用户的订阅（含已过期，用于回归潜力判定）
	var allSubs []UserSubscription
	if len(userIds) > 0 {
		DB.Where("user_id IN ?", userIds).Find(&allSubs)
	}

	// 按用户分组所有订阅
	allSubsByUser := make(map[int][]UserSubscription)
	for _, s := range allSubs {
		allSubsByUser[s.UserId] = append(allSubsByUser[s.UserId], s)
	}

	// 计算每个用户的评分
	for i := range balances {
		b := &balances[i]
		userConsume := consumeMap[b.Id]
		b.DailyConsume = float64(userConsume) / float64(periodDays)
		b.LastLoginAt = loginMap[b.Id]

		// 获取该用户的所有订阅（含已过期）
		userAllSubs := allSubsByUser[b.Id]

		// 计算续费潜力评分
		b.RenewScore, b.RenewLevel = calculateRenewPotential(b, userAllSubs, userConsume, periodDays, now)
		// 计算回归潜力
		b.RegressionLevel = calculateRegressionPotential(b, userAllSubs, now)
	}

	return balances, nil
}

// RenewPotentialLevel 续费潜力等级
type RenewPotentialLevel string

const (
	RenewPotentialHigh   RenewPotentialLevel = "high"   // 高潜力 80-100
	RenewPotentialMedium RenewPotentialLevel = "medium" // 中潜力 60-79
	RenewPotentialLow    RenewPotentialLevel = "low"    // 低潜力 30-59
	RenewPotentialNone   RenewPotentialLevel = "none"   // 无潜力 0-29
)

// RegressionPotentialLevel 回归潜力等级
type RegressionPotentialLevel string

const (
	RegressionPotentialHigh   RegressionPotentialLevel = "high"   // 高回归潜力
	RegressionPotentialMedium RegressionPotentialLevel = "medium" // 中回归潜力
	RegressionPotentialLow    RegressionPotentialLevel = "low"    // 低回归潜力
)

// calculateRenewPotential 计算续费潜力评分和等级
func calculateRenewPotential(b *UserBalanceInfo, allSubs []UserSubscription, consumeTotal int, periodDays int, now int64) (int, RenewPotentialLevel) {
	// 无套餐或套餐过期 → 无潜力
	activeSubs := make([]UserSubscription, 0)
	for _, s := range allSubs {
		if s.Status == "active" && s.EndTime > now {
			activeSubs = append(activeSubs, s)
		}
	}
	if len(activeSubs) == 0 {
		return 0, RenewPotentialNone
	}
	// 零余额 → 无潜力
	if b.Quota <= 0 && b.GptQuota <= 0 {
		return 0, RenewPotentialNone
	}

	// 1. 消耗速率评分 (40分)
	// 日均消耗越高分数越高，用对数缩放防止极端值
	dailyConsume := float64(consumeTotal) / float64(periodDays)
	// 假设日均消耗达到 1000000（约$2）为满分
	consumeScore := 0.0
	if dailyConsume > 0 {
		consumeScore = math.Min(math.Log10(dailyConsume+1)/math.Log10(1000001)*40, 40)
	}

	// 2. 套餐剩余额度占比评分 (30分)
	// 计算所有活跃订阅的总额度和剩余额度
	var totalAmount, usedAmount int64
	for _, s := range activeSubs {
		totalAmount += s.AmountTotal
		usedAmount += s.AmountUsed
	}
	remainingRatio := 1.0
	if totalAmount > 0 {
		remainingRatio = float64(totalAmount-usedAmount) / float64(totalAmount)
	}
	if remainingRatio < 0 {
		remainingRatio = 0
	}
	b.QuotaRemainingRatio = remainingRatio
	// 剩余比例越高分数越高
	ratioScore := remainingRatio * 30

	// 3. 订阅剩余有效天数评分 (30分)
	// 取所有活跃订阅中最大的 end_time
	var maxEndTime int64
	for _, s := range activeSubs {
		if s.EndTime > maxEndTime {
			maxEndTime = s.EndTime
		}
	}
	remainSeconds := maxEndTime - now
	remainDays := float64(remainSeconds) / 86400.0
	// 剩余天数越长分数越高，30天为满分
	daysScore := math.Min(remainDays/30.0*30, 30)
	if daysScore < 0 {
		daysScore = 0
	}

	totalScore := int(consumeScore + ratioScore + daysScore)
	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	// 判定等级
	var level RenewPotentialLevel
	switch {
	case totalScore >= 80:
		level = RenewPotentialHigh
	case totalScore >= 60:
		level = RenewPotentialMedium
	case totalScore >= 30:
		level = RenewPotentialLow
	default:
		level = RenewPotentialNone
	}

	return totalScore, level
}

// calculateRegressionPotential 计算回归潜力
// 判定有过订阅但当前无活跃订阅的用户，根据上次登录和订阅过期时间推断回归潜力
func calculateRegressionPotential(b *UserBalanceInfo, allSubs []UserSubscription, now int64) RegressionPotentialLevel {
	// 有活跃订阅 → 不适用回归潜力
	hasActive := false
	var lastExpiredTime int64
	for _, s := range allSubs {
		if s.Status == "active" && s.EndTime > now {
			hasActive = true
		}
		if s.Status != "active" || s.EndTime <= now {
			if s.EndTime > lastExpiredTime {
				lastExpiredTime = s.EndTime
			}
		}
	}
	b.LastSubEndTime = lastExpiredTime

	if hasActive {
		return ""
	}
	if lastExpiredTime == 0 {
		return "" // 从未有过订阅
	}

	// 距订阅过期的时间（天）
	daysSinceExpired := float64(now-lastExpiredTime) / 86400.0
	// 距上次登录的时间（天）
	daysSinceLogin := float64(0)
	if b.LastLoginAt > 0 {
		daysSinceLogin = float64(now-b.LastLoginAt) / 86400.0
	}

	// 综合评分：过期时间越近 + 最近登录过 → 回归潜力越高
	// 过期7天内且30天内登录过 → 高
	// 过期30天内且90天内登录过 → 中
	// 其他 → 低
	if daysSinceExpired <= 7 && daysSinceLogin <= 30 {
		return RegressionPotentialHigh
	}
	if daysSinceExpired <= 30 && daysSinceLogin <= 90 {
		return RegressionPotentialMedium
	}
	return RegressionPotentialLow
}
