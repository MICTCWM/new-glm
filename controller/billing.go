package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// GetQueueStatus returns the current RPM queue length for the admin dashboard.
func GetQueueStatus(c *gin.Context) {
	queueLen := service.GetQueueLength()
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"queue_count": queueLen,
		"queue_items": service.GetQueueSnapshot(),
	})
}

func GetSubscription(c *gin.Context) {
	var remainQuota int
	var usedQuota int
	var err error
	var token *model.Token
	var expiredTime int64
	if common.DisplayTokenStatEnabled {
		tokenId := c.GetInt("token_id")
		token, err = model.GetTokenById(tokenId)
		expiredTime = token.ExpiredTime
		remainQuota = token.RemainQuota
		usedQuota = token.UsedQuota
	} else {
		userId := c.GetInt("id")
		remainQuota, err = model.GetUserQuota(userId, false)
		usedQuota, err = model.GetUserUsedQuota(userId)
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	if err != nil {
		openAIError := types.OpenAIError{
			Message: err.Error(),
			Type:    "upstream_error",
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
		return
	}
	quota := remainQuota + usedQuota
	amount := float64(quota)
	// OpenAI 兼容接口中的 *_USD 字段含义保持“额度单位”对应值：
	// 我们将其解释为以“站点展示类型”为准：
	// - USD: 直接除以 QuotaPerUnit
	// - CNY: 先转 USD 再乘汇率
	// - TOKENS: 直接使用 tokens 数量
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		// amount 保持 tokens 数值
	default:
		amount = amount / common.QuotaPerUnit
	}
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}
	subscription := OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
	return
}

func GetUsage(c *gin.Context) {
	var quota int
	var err error
	var token *model.Token
	if common.DisplayTokenStatEnabled {
		tokenId := c.GetInt("token_id")
		token, err = model.GetTokenById(tokenId)
		quota = token.UsedQuota
	} else {
		userId := c.GetInt("id")
		quota, err = model.GetUserUsedQuota(userId)
	}
	if err != nil {
		openAIError := types.OpenAIError{
			Message: err.Error(),
			Type:    "new_api_error",
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
		return
	}
	amount := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		// tokens 保持原值
	default:
		amount = amount / common.QuotaPerUnit
	}
	usage := OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
	return
}

// GetV1UsageQuota returns the 5-hour quota usage for the cc-switch client (Kimi format).
// It picks the active subscription with the highest used ratio (AmountUsed/AmountTotal),
// and falls back to the wallet quota when no usable active subscription exists.
func GetV1UsageQuota(c *gin.Context) {
	userId := c.GetInt("id")

	// 1. 查询所有 active 订阅
	subs, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 2. 选取已用比例最高的订阅（AmountUsed/AmountTotal 最大，且 AmountTotal > 0）
	var selectedLimit, selectedRemaining, selectedResetTime int64
	foundSubscription := false

	if len(subs) > 0 {
		maxRatio := -1.0
		for _, sub := range subs {
			if sub.Subscription == nil {
				continue
			}
			// 跳过无限额度订阅（AmountTotal <= 0）
			if sub.Subscription.AmountTotal <= 0 {
				continue
			}
			ratio := float64(sub.Subscription.AmountUsed) / float64(sub.Subscription.AmountTotal)
			if ratio > maxRatio {
				maxRatio = ratio
				selectedLimit = sub.Subscription.AmountTotal
				selectedRemaining = sub.Subscription.AmountTotal - sub.Subscription.AmountUsed
				selectedResetTime = sub.Subscription.NextResetTime
				foundSubscription = true
			}
		}
	}

	// 3. 如果没有可用的订阅，回退到钱包额度
	if !foundSubscription {
		userQuota, err := model.GetUserQuota(userId, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		userUsedQuota, err := model.GetUserUsedQuota(userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		selectedLimit = int64(userQuota + userUsedQuota)
		selectedRemaining = int64(userQuota)
		selectedResetTime = 0 // 钱包无重置周期
	}

	// 4. 确保剩余额度不为负
	if selectedRemaining < 0 {
		selectedRemaining = 0
	}

	// 5. 返回 Kimi 格式（resetTime 转毫秒）
	c.JSON(http.StatusOK, gin.H{
		"limits": []gin.H{
			{
				"detail": gin.H{
					"limit":     selectedLimit,
					"remaining": selectedRemaining,
					"resetTime": selectedResetTime * 1000, // unix 秒 → 毫秒
				},
			},
		},
	})
}
