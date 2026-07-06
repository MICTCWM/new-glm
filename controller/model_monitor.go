package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// getUserGptMode 从上下文获取当前用户的 GPT 专有模式状态
// 返回 true 表示用户已开启 GPT 模式，可查看 GPT 专用渠道提供的模型数据
func getUserGptMode(c *gin.Context) bool {
	userId := c.GetInt("id")
	if userId <= 0 {
		return false
	}
	userCache, err := model.GetUserCache(userId)
	if err != nil || userCache == nil {
		return false
	}
	return userCache.GetSetting().GptMode
}

// GetModelMonitorSamples 获取单个模型的监控采样数据
// 查询参数：model（模型名）
// 返回：[]ModelMonitorSample，按 created_at 升序排列（旧到新），便于前端按时间从左到右绘制柱形图
// 权限：需登录，未开启 GPT 模式的用户无法查看仅 GPT 专用渠道提供的模型数据
func GetModelMonitorSamples(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		common.ApiError(c, fmt.Errorf("model parameter is required"))
		return
	}

	// GPT 专有模式过滤：未开启 GPT 模式的用户无法查看 GPT 专用模型
	userGptMode := getUserGptMode(c)
	if !userGptMode && model.IsModelGptOnly(modelName) {
		common.ApiSuccess(c, []service.ModelMonitorSample{})
		return
	}

	ctx := c.Request.Context()
	samples, err := service.GetModelMonitorSamplesFromRedis(ctx, modelName)
	if err != nil {
		common.SysError(fmt.Sprintf("get model monitor samples failed: %v", err))
		common.ApiError(c, err)
		return
	}
	if samples == nil {
		samples = []service.ModelMonitorSample{}
	}
	common.ApiSuccess(c, samples)
}

// GetAllModelMonitorSamples 批量获取所有模型的监控采样数据
// 返回：map[string][]ModelMonitorSample，key 为模型名
// 用于前端在模型卡片网格中一次性加载所有模型的监控数据
// 权限：需登录，未开启 GPT 模式的用户无法查看仅 GPT 专用渠道提供的模型数据
func GetAllModelMonitorSamples(c *gin.Context) {
	ctx := c.Request.Context()
	allSamples, err := service.GetAllModelMonitorSamplesFromRedis(ctx)
	if err != nil {
		common.SysError(fmt.Sprintf("get all model monitor samples failed: %v", err))
		common.ApiError(c, err)
		return
	}

	// GPT 专有模式过滤：未开启 GPT 模式的用户过滤掉 GPT 专用模型
	userGptMode := getUserGptMode(c)
	if !userGptMode {
		for name := range allSamples {
			if model.IsModelGptOnly(name) {
				delete(allSamples, name)
			}
		}
	}

	common.ApiSuccess(c, allSamples)
}
