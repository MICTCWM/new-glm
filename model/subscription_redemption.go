package model

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// SubscriptionRedemption 订阅兑换码模型
type SubscriptionRedemption struct {
	Id           int            `json:"id"`
	PlanId       int            `json:"plan_id" gorm:"index"`                 // 关联的订阅套餐ID
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"` // 兑换码
	Name         string         `json:"name" gorm:"index"`                    // 兑换码名称
	Status       int            `json:"status" gorm:"default:1"`              // 状态：1-可用，2-已用，3-禁用
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	UsedUserId   int            `json:"used_user_id" gorm:"index"` // 使用者ID
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
	CreatedBy    int            `json:"created_by"`                // 创建者（管理员ID）
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// GetAllSubscriptionRedemptions 获取所有订阅兑换码（分页）
func GetAllSubscriptionRedemptions(startIdx int, num int) (redemptions []*SubscriptionRedemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	err = tx.Model(&SubscriptionRedemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

// SearchSubscriptionRedemptions 搜索订阅兑换码
func SearchSubscriptionRedemptions(keyword string, startIdx int, num int) (redemptions []*SubscriptionRedemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&SubscriptionRedemption{})

	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}

	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

// GetSubscriptionRedemptionById 根据ID获取订阅兑换码
func GetSubscriptionRedemptionById(id int) (*SubscriptionRedemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := SubscriptionRedemption{Id: id}
	err := DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

// RedeemSubscription 兑换订阅
func RedeemSubscription(key string, userId int) (planTitle string, err error) {
	if key == "" {
		return "", errors.New("未提供兑换码")
	}
	if userId == 0 {
		return "", errors.New("无效的 user id")
	}

	redemption := &SubscriptionRedemption{}
	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}

	common.RandomSleep()
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 锁定兑换码记录
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}

		// 检查状态
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用或已禁用")
		}

		// 检查过期时间
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}

		// 获取订阅套餐
		plan, err := GetSubscriptionPlanById(redemption.PlanId)
		if err != nil {
			return errors.New("订阅套餐不存在")
		}

		if !plan.Enabled {
			return errors.New("该订阅套餐已禁用")
		}

		// 检查用户购买限制
		if plan.MaxPurchasePerUser > 0 {
			var count int64
			if err := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND plan_id = ?", userId, plan.Id).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(plan.MaxPurchasePerUser) {
				return errors.New("已达到该套餐购买上限")
			}
		}

		// 创建用户订阅
		_, err = CreateUserSubscriptionFromPlanTx(tx, userId, plan, "redemption")
		if err != nil {
			return err
		}

		// 更新兑换码状态
		redemption.RedeemedTime = common.GetTimestamp()
		redemption.Status = common.RedemptionCodeStatusUsed
		redemption.UsedUserId = userId
		err = tx.Save(redemption).Error
		if err != nil {
			return err
		}

		planTitle = plan.Title
		return nil
	})

	if err != nil {
		common.SysError("subscription redemption failed: " + err.Error())
		return "", err
	}

	// 记录日志
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过订阅兑换码兑换套餐: %s，兑换码ID: %d", planTitle, redemption.Id))
	return planTitle, nil
}

// Insert 插入订阅兑换码
func (redemption *SubscriptionRedemption) Insert() error {
	return DB.Create(redemption).Error
}

// Update 更新订阅兑换码
func (redemption *SubscriptionRedemption) Update() error {
	return DB.Model(redemption).Select("name", "status", "expired_time").Updates(redemption).Error
}

// Delete 删除订阅兑换码
func (redemption *SubscriptionRedemption) Delete() error {
	return DB.Delete(redemption).Error
}

// DeleteSubscriptionRedemptionById 根据ID删除订阅兑换码
func DeleteSubscriptionRedemptionById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := SubscriptionRedemption{Id: id}
	err := DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

// DeleteInvalidSubscriptionRedemptions 删除无效的订阅兑换码
func DeleteInvalidSubscriptionRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)",
		[]int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled},
		common.RedemptionCodeStatusEnabled, now).Delete(&SubscriptionRedemption{})
	return result.RowsAffected, result.Error
}

// DeleteUnusedSubscriptionRedemptions 删除未使用的订阅兑换码
func DeleteUnusedSubscriptionRedemptions() (int64, error) {
	result := DB.Where("status = ?", common.RedemptionCodeStatusEnabled).Delete(&SubscriptionRedemption{})
	return result.RowsAffected, result.Error
}