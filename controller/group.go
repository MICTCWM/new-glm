package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	seen := make(map[string]bool)
	// 普通分组
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
		seen[groupName] = true
	}
	// GPT 专有分组（避免与普通分组重名）
	for groupName := range ratio_setting.GetGptGroupRatioCopy() {
		if !seen[groupName] {
			groupNames = append(groupNames, groupName)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	// GPT 模式用户合并 GPT 专有分组
	// 仅当用户开启 GPT 模式时，才追加 GPT 专有分组，并避免与普通分组重名覆盖
	userGptMode := false
	if userId > 0 {
		if userCache, err := model.GetUserCache(userId); err == nil && userCache != nil {
			userGptMode = userCache.GetSetting().GptMode
		}
	}
	if userGptMode {
		for groupName, ratio := range ratio_setting.GetGptGroupRatioCopy() {
			if _, exists := usableGroups[groupName]; exists {
				continue // 避免与普通分组重名覆盖
			}
			usableGroups[groupName] = map[string]interface{}{
				"ratio":    ratio,
				"desc":     setting.GetGptUsableGroupDescription(groupName),
				"gpt_only": true, // 标识 GPT 专有分组（前端可用于显示 badge）
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
