package controller

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetAllSubscriptionRedemptions 获取所有订阅兑换码
func GetAllSubscriptionRedemptions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	redemptions, total, err := model.GetAllSubscriptionRedemptions(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
}

// SearchSubscriptionRedemptions 搜索订阅兑换码
func SearchSubscriptionRedemptions(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	redemptions, total, err := model.SearchSubscriptionRedemptions(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
}

// GetSubscriptionRedemption 获取单个订阅兑换码
func GetSubscriptionRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	redemption, err := model.GetSubscriptionRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    redemption,
	})
}

// AddSubscriptionRedemptionRequest 创建订阅兑换码请求
type AddSubscriptionRedemptionRequest struct {
	Name        string `json:"name"`
	PlanId      int    `json:"plan_id"`
	Count       int    `json:"count"`
	ExpiredTime int64  `json:"expired_time"`
}

// AddSubscriptionRedemption 创建订阅兑换码（批量）
func AddSubscriptionRedemption(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}

	var req AddSubscriptionRedemptionRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 验证名称
	if utf8.RuneCountInString(req.Name) == 0 || utf8.RuneCountInString(req.Name) > 20 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
		return
	}

	// 验证数量
	if req.Count <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
		return
	}
	if req.Count > 100 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
		return
	}

	// 验证套餐ID
	if req.PlanId <= 0 {
		common.ApiErrorMsg(c, "请选择订阅套餐")
		return
	}

	// 验证套餐是否存在
	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiErrorMsg(c, "订阅套餐不存在")
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "该订阅套餐已禁用")
		return
	}

	// 验证过期时间
	if valid, msg := validateExpiredTime(c, req.ExpiredTime); !valid {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}

	// 批量创建兑换码
	var keys []string
	for i := 0; i < req.Count; i++ {
		key := common.GetUUID()
		cleanRedemption := model.SubscriptionRedemption{
			PlanId:      req.PlanId,
			Name:        req.Name,
			Key:         key,
			CreatedTime: common.GetTimestamp(),
			ExpiredTime: req.ExpiredTime,
			CreatedBy:   c.GetInt("id"),
			Status:      common.RedemptionCodeStatusEnabled,
		}
		err = cleanRedemption.Insert()
		if err != nil {
			common.SysError("failed to insert subscription redemption: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "创建订阅兑换码失败",
				"data":    keys,
			})
			return
		}
		keys = append(keys, key)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    keys,
	})
}

// DeleteSubscriptionRedemption 删除订阅兑换码
func DeleteSubscriptionRedemption(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteSubscriptionRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// UpdateSubscriptionRedemption 更新订阅兑换码
func UpdateSubscriptionRedemption(c *gin.Context) {
	statusOnly := c.Query("status_only")
	redemption := model.SubscriptionRedemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	cleanRedemption, err := model.GetSubscriptionRedemptionById(redemption.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if statusOnly == "" {
		if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
			return
		}
		cleanRedemption.Name = redemption.Name
		cleanRedemption.ExpiredTime = redemption.ExpiredTime
	}
	if statusOnly != "" {
		// 验证状态值是否合法
		if redemption.Status != common.RedemptionCodeStatusEnabled &&
			redemption.Status != common.RedemptionCodeStatusUsed &&
			redemption.Status != common.RedemptionCodeStatusDisabled {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的状态值"})
			return
		}
		cleanRedemption.Status = redemption.Status
	}

	err = cleanRedemption.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanRedemption,
	})
}

// DeleteInvalidSubscriptionRedemption 删除无效的订阅兑换码
func DeleteInvalidSubscriptionRedemption(c *gin.Context) {
	rows, err := model.DeleteInvalidSubscriptionRedemptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

// DeleteUnusedSubscriptionRedemption 删除未使用的订阅兑换码
func DeleteUnusedSubscriptionRedemption(c *gin.Context) {
	rows, err := model.DeleteUnusedSubscriptionRedemptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

// RedeemSubscriptionRequest 用户兑换订阅请求
type RedeemSubscriptionRequest struct {
	Key string `json:"key"`
}

// RedeemSubscription 用户兑换订阅
func RedeemSubscription(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "用户未登录")
		return
	}

	var req RedeemSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if req.Key == "" {
		common.ApiErrorMsg(c, "请输入兑换码")
		return
	}

	planTitle, err := model.RedeemSubscription(req.Key, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "兑换成功",
		"data": gin.H{
			"plan_title": planTitle,
		},
	})
}