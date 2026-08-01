package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetAbilityList returns a paginated, filterable list of abilities joined with their
// channel info (including the channel name).
//
// Query params:
//   - page, page_size / page_size = -1 to fetch all
//   - group, model, channel_id (filters)
//   - only_enabled (default true)
func GetAbilityList(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	group := c.Query("group")
	modelName := c.Query("model")
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	onlyEnabled := true
	if v := c.Query("only_enabled"); v != "" {
		onlyEnabled, _ = strconv.ParseBool(v)
	}

	abilities, total, err := model.SearchAbilities(
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
		group, modelName, channelId, onlyEnabled,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(abilities)
	common.ApiSuccess(c, pageInfo)
}
