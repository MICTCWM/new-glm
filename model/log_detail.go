package model

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
)

// LogDetail 记录每个请求的详细日志数据，保留1天后自动删除。
// 关联 Log.Id / Log.RequestId，仅记录成功请求（错误日志不记录详情）。
type LogDetail struct {
	Id                     int    `json:"id" gorm:"primaryKey"`
	LogId                  int    `json:"log_id" gorm:"index"`           // 关联 Log.Id
	RequestId              string `json:"request_id" gorm:"index"`       // 关联 Log.RequestId
	UserRequestBody        string `json:"user_request_body" gorm:"type:longtext"`        // 数据点1：用户原始请求 JSON
	UpstreamRequestBody    string `json:"upstream_request_body" gorm:"type:longtext"`    // 数据点2：转换后发给上游的请求体（无转换时为空）
	UpstreamResponseBody   string `json:"upstream_response_body" gorm:"type:longtext"`   // 数据点3：上游返回的原始响应体
	DownstreamResponseBody string `json:"downstream_response_body" gorm:"type:longtext"` // 数据点4：系统最终返回给用户的响应体（=数据点5）
	HasConversion          bool   `json:"has_conversion"`                                // 是否发生协议转换
	CreatedAt              int64  `json:"created_at" gorm:"bigint"`
}

// RecordLogDetail 写入一条日志详情记录
func RecordLogDetail(logId int, requestId string, userReq, upstreamReq, upstreamResp, downstreamResp string, hasConversion bool) error {
	if logId == 0 {
		return nil
	}
	detail := &LogDetail{
		LogId:                  logId,
		RequestId:              requestId,
		UserRequestBody:        userReq,
		UpstreamRequestBody:    upstreamReq,
		UpstreamResponseBody:   upstreamResp,
		DownstreamResponseBody: downstreamResp,
		HasConversion:          hasConversion,
		CreatedAt:              common.GetTimestamp(),
	}
	return LOG_DB.Create(detail).Error
}

// GetLogDetailByLogId 按关联的 Log.Id 查询日志详情
func GetLogDetailByLogId(logId int) (*LogDetail, error) {
	var detail LogDetail
	err := LOG_DB.Where("log_id = ?", logId).First(&detail).Error
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

// CleanupExpiredLogDetails 删除1天前的日志详情记录
func CleanupExpiredLogDetails() error {
	threshold := time.Now().Unix() - 86400
	return LOG_DB.Where("created_at < ?", threshold).Delete(&LogDetail{}).Error
}

// logDetailCleanupInterval 定时清理间隔：每小时执行一次
const logDetailCleanupInterval = 1 * time.Hour

var logDetailCleanupOnce sync.Once

// StartLogDetailCleanupTask 启动定时清理过期 LogDetail 的后台任务（每小时一次）。
// 仅在主节点运行，避免多节点重复清理。
func StartLogDetailCleanupTask() {
	logDetailCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog("log detail cleanup task started: tick=" + logDetailCleanupInterval.String())
			ticker := time.NewTicker(logDetailCleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				if err := CleanupExpiredLogDetails(); err != nil {
					common.SysError("failed to cleanup expired log details: " + err.Error())
				}
			}
		})
	})
}
