package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type channelProbeBatchRequest struct {
	Ids          []int   `json:"ids"`
	ProbeEnabled bool    `json:"probe_enabled"`
	TestModel    *string `json:"test_model"`
}

// BatchSetChannelProbe updates probe settings without replacing each channel's
// other routing settings. A blank test_model is omitted by the UI and keeps the
// existing channel-specific test model.
func BatchSetChannelProbe(c *gin.Context) {
	var req channelProbeBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数错误"})
		return
	}

	updated := 0
	for _, id := range req.Ids {
		channel, err := model.GetChannelById(id, true)
		if err != nil || channel == nil {
			continue
		}
		setting := channel.GetSetting()
		setting.ProbeEnabled = req.ProbeEnabled
		channel.SetSetting(setting)
		if req.TestModel != nil {
			modelName := strings.TrimSpace(*req.TestModel)
			if modelName == "" {
				channel.TestModel = nil
			} else {
				channel.TestModel = common.GetPointer(modelName)
			}
		}
		if err := channel.Update(); err != nil {
			continue
		}
		updated++
	}

	if updated == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "没有渠道更新成功"})
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    updated,
	})
}
