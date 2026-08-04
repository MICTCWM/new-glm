package controller

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func parseSpecialUsageList(c *gin.Context, key string) []string {
	values := c.QueryArray(key)
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, rawValue := range values {
		for _, value := range strings.Split(rawValue, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func parseSpecialUsageFilter(c *gin.Context) (model.SpecialUsageFilter, error) {
	start, err := parseSpecialUsageInt64Query(c, "start_time")
	if err != nil {
		return model.SpecialUsageFilter{}, fmt.Errorf("invalid start_time")
	}
	end, err := parseSpecialUsageInt64Query(c, "end_time")
	if err != nil {
		return model.SpecialUsageFilter{}, fmt.Errorf("invalid end_time")
	}
	if end == 0 {
		end = time.Now().Unix()
	}
	channelIDs, err := parseSpecialUsageChannelIDs(c)
	if err != nil {
		return model.SpecialUsageFilter{}, err
	}
	return model.SpecialUsageFilter{
		StartTime:     start,
		EndTime:       end,
		GroupNames:    parseSpecialUsageList(c, "group"),
		ModelNames:    parseSpecialUsageList(c, "model"),
		ChannelIDs:    channelIDs,
		ChannelIDsSet: hasSpecialUsageQueryParameter(c, "channel_id"),
	}, nil
}

func parseSpecialUsageInt64Query(c *gin.Context, key string) (int64, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}

func hasSpecialUsageQueryParameter(c *gin.Context, key string) bool {
	_, exists := c.GetQuery(key)
	return exists
}

func parseSpecialUsageChannelIDs(c *gin.Context) ([]int, error) {
	values := c.QueryArray("channel_id")
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{})
	for _, rawValue := range values {
		for _, value := range strings.Split(rawValue, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			id, err := strconv.Atoi(value)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid channel_id")
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result, nil
}

func validateSpecialUsageFilter(c *gin.Context, filter model.SpecialUsageFilter) bool {
	if filter.StartTime > 0 && filter.EndTime > 0 && filter.StartTime >= filter.EndTime {
		common.ApiErrorMsg(c, "invalid time range")
		return false
	}
	return true
}
func GetSpecialUsageMetadata(c *gin.Context) {
	config := model.GetSpecialUsageConfig()
	groupSet := make(map[string]struct{})
	modelSet := make(map[string]struct{})
	var channels []model.Channel
	if err := model.DB.Omit("key").Order("id asc").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	metadata := model.SpecialUsageMetadata{Groups: []string{}, Models: []string{}, Channels: []model.SpecialUsageChannel{}, Config: config}
	for _, channel := range channels {
		setting := channel.GetSetting()
		models := channel.GetModels()
		groups := channel.GetGroups()
		for _, group := range groups {
			groupSet[group] = struct{}{}
		}
		for _, modelName := range models {
			modelSet[modelName] = struct{}{}
		}
		hasSpecialPrice := false
		for _, prices := range setting.SpecialBillingPrices {
			if len(prices) > 0 {
				hasSpecialPrice = true
				break
			}
		}
		metadata.Channels = append(metadata.Channels, model.SpecialUsageChannel{
			ID: channel.Id, Name: channel.Name, Groups: groups, Models: models,
			Multiplier:     specialUsageConfigMultiplier(config, channel.Id, groups),
			SpecialBilling: setting.SpecialBilling, SpecialBillingPrice: hasSpecialPrice,
		})
	}
	for group := range groupSet {
		metadata.Groups = append(metadata.Groups, group)
	}
	for modelName := range modelSet {
		metadata.Models = append(metadata.Models, modelName)
	}
	// Include configured pricing models even when no channel is currently enabled.
	for modelName := range ratio_setting.GetModelRatioCopy() {
		modelSet[modelName] = struct{}{}
	}
	for modelName := range ratio_setting.GetModelPriceCopy() {
		modelSet[modelName] = struct{}{}
	}
	metadata.Models = metadata.Models[:0]
	for modelName := range modelSet {
		metadata.Models = append(metadata.Models, modelName)
	}
	sortStrings(metadata.Groups)
	sortStrings(metadata.Models)
	common.ApiSuccess(c, metadata)
}

func specialUsageConfigMultiplier(config model.SpecialUsageConfig, channelID int, groups []string) float64 {
	if value, ok := config.ChannelMultipliers[strconv.Itoa(channelID)]; ok && value > 0 {
		return value
	}
	for _, group := range groups {
		if value, ok := config.ChannelMultipliers["group:"+group]; ok && value > 0 {
			return value
		}
	}
	return 1
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

type saveSpecialUsageConfigRequest struct {
	Enabled            bool               `json:"enabled"`
	GroupNames         []string           `json:"group_names"`
	ModelNames         []string           `json:"model_names"`
	ChannelIDs         *[]int             `json:"channel_ids"`
	SpecialBilling     bool               `json:"special_billing"`
	ChannelMultipliers map[string]float64 `json:"channel_multipliers"`
}

func SaveSpecialUsageMonitorConfig(c *gin.Context) {
	var request saveSpecialUsageConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Enabled && (!hasSpecialUsageValue(request.GroupNames) || !hasSpecialUsageValue(request.ModelNames)) {
		common.ApiErrorMsg(c, "启用监测时至少选择一个监测分组和模型")
		return
	}
	for key, multiplier := range request.ChannelMultipliers {
		if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
			common.ApiErrorMsg(c, fmt.Sprintf("倍率必须是正数：%s", key))
			return
		}
	}
	channelIDs := model.GetSpecialUsageConfig().ChannelIDs
	channelIDsSet := model.GetSpecialUsageConfig().ChannelIDsSet
	if request.ChannelIDs != nil {
		channelIDs = *request.ChannelIDs
		channelIDsSet = true
	}
	config := model.SpecialUsageConfig{
		Enabled: request.Enabled, GroupNames: request.GroupNames, ModelNames: request.ModelNames,
		ChannelIDs: channelIDs, ChannelIDsSet: channelIDsSet, SpecialBilling: request.SpecialBilling, ChannelMultipliers: request.ChannelMultipliers,
		UpdatedAt: time.Now().Unix(),
	}
	if request.Enabled && channelIDsSet {
		for _, channelID := range channelIDs {
			if !model.SpecialUsageChannelMatches(config, channelID, "", "") {
				common.ApiErrorMsg(c, fmt.Sprintf("渠道 %d 不属于已选分组和模型范围", channelID))
				return
			}
		}
	}
	if err := model.SaveSpecialUsageConfig(config); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, model.GetSpecialUsageConfig())
}

func hasSpecialUsageValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func GetSpecialUsageOverview(c *gin.Context) {
	filter, err := parseSpecialUsageFilter(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !validateSpecialUsageFilter(c, filter) {
		return
	}
	overview, err := model.AggregateSpecialUsageOverview(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}

func GetSpecialUsageForecast(c *gin.Context) {
	filter, err := parseSpecialUsageFilter(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !validateSpecialUsageFilter(c, filter) {
		return
	}
	basis := c.DefaultQuery("basis", "today_current")
	if basis != "today_current" && basis != "historical_daily" {
		common.ApiErrorMsg(c, "无效的预测基准")
		return
	}
	days, err := strconv.ParseFloat(c.DefaultQuery("days", "1"), 64)
	if err != nil || math.IsNaN(days) || math.IsInf(days, 0) || days <= 0 || days > 3650 {
		common.ApiErrorMsg(c, "预测天数必须在 0 到 3650 之间")
		return
	}
	todayRemaining := c.DefaultQuery("today_remaining", "false") == "true"
	forecast, err := model.PredictSpecialUsageCost(filter, basis, days, todayRemaining)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, forecast)
}

func GetSpecialUsageRecords(c *gin.Context) {
	filter, err := parseSpecialUsageFilter(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !validateSpecialUsageFilter(c, filter) {
		return
	}
	page, err := parseSpecialUsagePositiveInt(c, "page", 1)
	if err != nil || page < 1 {
		common.ApiErrorMsg(c, "页码必须是正整数")
		return
	}
	pageSize, err := parseSpecialUsagePositiveInt(c, "page_size", 100)
	if err != nil || pageSize < 1 || pageSize > 1000 {
		common.ApiErrorMsg(c, "每页数量必须在 1 到 1000 之间")
		return
	}
	records, total, err := model.ListSpecialUsageRecords(filter, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": records, "total": total, "page": page, "page_size": pageSize})
}

func parseSpecialUsagePositiveInt(c *gin.Context, key string, defaultValue int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
func ExportSpecialUsage(c *gin.Context) {
	filter, err := parseSpecialUsageFilter(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !validateSpecialUsageFilter(c, filter) {
		return
	}
	const exportLimit int64 = 100000
	total, err := model.CountSpecialUsageRecords(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if total > exportLimit {
		common.ApiErrorMsg(c, "export limit exceeded")
		return
	}
	records, _, err := model.ListSpecialUsageRecords(filter, 1, int(exportLimit))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "xlsx")))
	if format != "xlsx" && format != "csv" {
		common.ApiErrorMsg(c, "export format must be xlsx or csv")
		return
	}
	extension := format
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if format == "csv" {
		contentType = "text/csv; charset=utf-8"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=special-usage-%d.%s", time.Now().Unix(), extension))
	if format == "csv" {
		_, _ = c.Writer.WriteString("\ufeff")
	}
	var writeErr error
	if format == "xlsx" {
		writeErr = model.WriteSpecialUsageXLSX(c.Writer, records)
	} else {
		writeErr = model.WriteSpecialUsageCSV(c.Writer, records)
	}
	if writeErr != nil {
		common.SysError("failed to export special usage: " + writeErr.Error())
	}
}

func GetSpecialUsageProfit(c *gin.Context) {
	if !model.GetSpecialUsageConfig().Enabled {
		common.ApiErrorMsg(c, "监测未启用")
		return
	}
	filter, err := parseSpecialUsageFilter(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !validateSpecialUsageFilter(c, filter) {
		return
	}
	mode := c.DefaultQuery("mode", "auto")
	if mode != "auto" && mode != "manual" {
		common.ApiErrorMsg(c, "无效的利润计算模式")
		return
	}
	manualRevenue := 0.0
	if mode == "manual" {
		revenueValue := strings.TrimSpace(c.Query("revenue"))
		if revenueValue == "" {
			common.ApiErrorMsg(c, "手动模式必须提供营业收入")
			return
		}
		manualRevenue, err := strconv.ParseFloat(revenueValue, 64)
		if err != nil || math.IsNaN(manualRevenue) || math.IsInf(manualRevenue, 0) || manualRevenue < 0 {
			common.ApiErrorMsg(c, "营业收入必须是非负有限数字")
			return
		}
	}
	overview, err := model.AggregateSpecialUsageOverview(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	revenue := overview.Totals.UserChargeUSD
	if mode == "manual" {
		revenue = manualRevenue
	}
	profit := revenue - overview.Totals.UpstreamCostUSD
	margin := 0.0
	if revenue != 0 {
		margin = profit / revenue * 100
	}
	common.ApiSuccess(c, gin.H{"mode": mode, "revenue": revenue, "cost": overview.Totals.UpstreamCostUSD, "profit": profit, "margin": margin})
}
func ValidateSpecialUsageConfig(c *gin.Context) {
	config := model.GetSpecialUsageConfig()
	if len(config.GroupNames) == 0 || len(config.ModelNames) == 0 {
		common.ApiErrorMsg(c, "监测范围尚未配置")
		return
	}
	common.ApiSuccess(c, gin.H{"valid": true})
}
