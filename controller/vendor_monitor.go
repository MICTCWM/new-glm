package controller

import (
	"context"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetVendorMonitorSamples 获取单个供应商的监控采样数据
// 查询参数：vendor_id（供应商 ID）
// 返回：[]VendorMonitorSample，按 created_at 升序排列（旧到新），便于前端按时间从左到右绘制柱形图
// 权限：需管理员登录（路由组已应用 AdminAuth）
func GetVendorMonitorSamples(c *gin.Context) {
	// 校验 vendor_id：必填且必须为正整数，避免直接透传 Atoi 的不友好错误
	vendorIDStr := c.Query("vendor_id")
	if vendorIDStr == "" {
		common.ApiErrorMsg(c, "vendor_id 参数为必填")
		return
	}
	vendorID, err := strconv.Atoi(vendorIDStr)
	if err != nil || vendorID <= 0 {
		common.ApiErrorMsg(c, "vendor_id 参数无效")
		return
	}
	ctx := context.Background()
	samples, err := service.GetVendorMonitorSamplesFromRedis(ctx, vendorID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if samples == nil {
		samples = []service.VendorMonitorSample{}
	}
	common.ApiSuccess(c, samples)
}

// GetAllVendorMonitorSamples 批量获取所有供应商的监控采样数据
// 返回：map[int][]VendorMonitorSample，key 为供应商 ID
// 用于前端在供应商卡片网格中一次性加载所有供应商的监控数据
// 权限：需管理员登录（路由组已应用 AdminAuth）
func GetAllVendorMonitorSamples(c *gin.Context) {
	ctx := context.Background()
	samples, err := service.GetAllVendorMonitorSamplesFromRedis(ctx)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if samples == nil {
		samples = map[int][]service.VendorMonitorSample{}
	}
	common.ApiSuccess(c, samples)
}
