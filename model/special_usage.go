package model

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SpecialUsageConfigKey     = "special_usage_monitor_config"
	SpecialUsageStatusSuccess = "success"
	SpecialUsageStatusFailed  = "failed"

	specialUsageDefaultMultiplier = 1.0
)

// SpecialUsageConfig is stored independently from the order and usage log
// tables. It controls which traffic is copied into the monitoring ledger.
type SpecialUsageConfig struct {
	Enabled            bool               `json:"enabled"`
	GroupNames         []string           `json:"group_names"`
	ModelNames         []string           `json:"model_names"`
	SpecialBilling     bool               `json:"special_billing"`
	ChannelMultipliers map[string]float64 `json:"channel_multipliers"`
	UpdatedAt          int64              `json:"updated_at"`
}

type SpecialUsageRecord struct {
	ID               int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestID        string  `json:"request_id" gorm:"size:128;uniqueIndex:uk_special_usage_request_channel;priority:1"`
	UserID           int     `json:"user_id" gorm:"index"`
	ChannelID        int     `json:"channel_id" gorm:"index;uniqueIndex:uk_special_usage_request_channel;priority:2"`
	ChannelName      string  `json:"channel_name" gorm:"size:255;index"`
	GroupName        string  `json:"group_name" gorm:"size:64;index"`
	ModelName        string  `json:"model_name" gorm:"size:255;index"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	UpstreamCostUSD  float64 `json:"upstream_cost_usd" gorm:"type:decimal(20,10);default:0"`
	UserChargeUSD    float64 `json:"user_charge_usd" gorm:"type:decimal(20,10);default:0"`
	InputPriceUSD    float64 `json:"input_price_usd" gorm:"type:decimal(20,10);default:0"`
	OutputPriceUSD   float64 `json:"output_price_usd" gorm:"type:decimal(20,10);default:0"`
	Multiplier       float64 `json:"multiplier" gorm:"type:decimal(12,6);default:1"`
	UsedSpecialPrice bool    `json:"used_special_price"`
	Status           string  `json:"status" gorm:"size:16;index"`
	RequestTime      int64   `json:"request_time" gorm:"bigint;index:idx_special_usage_time"`
	ErrorMessage     string  `json:"error_message,omitempty" gorm:"type:text"`
}

// SpecialUsageHourly stores the durable hourly rollup used by the dashboard.
type SpecialUsageHourly struct {
	ID              int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	BucketTime      int64   `json:"bucket_time" gorm:"bigint;uniqueIndex:uk_special_usage_hour;priority:1"`
	GroupName       string  `json:"group_name" gorm:"size:64;uniqueIndex:uk_special_usage_hour;priority:2"`
	ChannelID       int     `json:"channel_id" gorm:"uniqueIndex:uk_special_usage_hour;priority:3"`
	ModelName       string  `json:"model_name" gorm:"size:255;uniqueIndex:uk_special_usage_hour;priority:4"`
	ChannelName     string  `json:"channel_name" gorm:"size:255"`
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	UpstreamCostUSD float64 `json:"upstream_cost_usd" gorm:"type:decimal(20,10);default:0"`
	UserChargeUSD   float64 `json:"user_charge_usd" gorm:"type:decimal(20,10);default:0"`
}

type SpecialUsageCostInput struct {
	RequestID       string
	UserID          int
	ChannelID       int
	ChannelName     string
	GroupName       string
	ModelName       string
	InputTokens     int
	OutputTokens    int
	UserChargeQuota int
	Status          string
	ErrorMessage    string
	RequestTime     int64
	ChannelSetting  dto.ChannelSettings
}

type SpecialUsageChannel struct {
	ID                  int      `json:"id"`
	Name                string   `json:"name"`
	Groups              []string `json:"groups"`
	Models              []string `json:"models"`
	Multiplier          float64  `json:"multiplier"`
	SpecialBilling      bool     `json:"special_billing"`
	SpecialBillingPrice bool     `json:"has_special_price"`
}

type SpecialUsageMetadata struct {
	Groups   []string              `json:"groups"`
	Models   []string              `json:"models"`
	Channels []SpecialUsageChannel `json:"channels"`
	Config   SpecialUsageConfig    `json:"config"`
}

type SpecialUsageFilter struct {
	StartTime  int64
	EndTime    int64
	GroupNames []string
	ModelNames []string
	ChannelIDs []int
}

type SpecialUsageTotals struct {
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	UpstreamCostUSD float64 `json:"upstream_cost_usd"`
	UserChargeUSD   float64 `json:"user_charge_usd"`
}

type SpecialUsageSeriesPoint struct {
	Time            int64   `json:"time"`
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	UpstreamCostUSD float64 `json:"upstream_cost_usd"`
	UserChargeUSD   float64 `json:"user_charge_usd"`
}

type SpecialUsageNamedValue struct {
	Name            string  `json:"name"`
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	UpstreamCostUSD float64 `json:"upstream_cost_usd"`
	UserChargeUSD   float64 `json:"user_charge_usd"`
}

type SpecialUsageTreeNode struct {
	Name     string                 `json:"name"`
	Value    float64                `json:"value"`
	Children []SpecialUsageTreeNode `json:"children,omitempty"`
}

type SpecialUsageChannelStat struct {
	ChannelID       int     `json:"channel_id"`
	ChannelName     string  `json:"channel_name"`
	RequestCount    int64   `json:"request_count"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	UpstreamCostUSD float64 `json:"upstream_cost_usd"`
	UserChargeUSD   float64 `json:"user_charge_usd"`
	AverageCostUSD  float64 `json:"average_cost_usd"`
	Anomaly         bool    `json:"anomaly"`
}

type SpecialUsageOverview struct {
	Totals          SpecialUsageTotals        `json:"totals"`
	Series          []SpecialUsageSeriesPoint `json:"series"`
	GroupCosts      []SpecialUsageNamedValue  `json:"group_costs"`
	ModelTokens     []SpecialUsageNamedValue  `json:"model_tokens"`
	Channels        []SpecialUsageChannelStat `json:"channels"`
	InputOutput     []SpecialUsageNamedValue  `json:"input_output"`
	GroupProfit     []SpecialUsageNamedValue  `json:"group_profit"`
	ChannelCostTree []SpecialUsageTreeNode    `json:"channel_cost_tree"`
	LastUpdatedAt   int64                     `json:"last_updated_at"`
}

type SpecialUsageForecast struct {
	Basis               string  `json:"basis"`
	Days                float64 `json:"days"`
	DailyTokens         float64 `json:"daily_tokens"`
	ForecastTokens      float64 `json:"forecast_tokens"`
	AverageCostPerToken float64 `json:"average_cost_per_token"`
	ForecastCostUSD     float64 `json:"forecast_cost_usd"`
}

type specialUsageCostCache struct {
	Config SpecialUsageConfig
	At     time.Time
}

var (
	specialUsageConfigCache specialUsageCostCache
	specialUsageConfigMu    sync.RWMutex
	// Serializes rollup backfills within this process so concurrent retries do
	// not both create a bucket from overlapping snapshots and then add deltas.
	specialUsageHourlyBackfillMu sync.Mutex
)

func defaultSpecialUsageConfig() SpecialUsageConfig {
	return SpecialUsageConfig{
		Enabled:            false,
		GroupNames:         []string{},
		ModelNames:         []string{},
		SpecialBilling:     false,
		ChannelMultipliers: map[string]float64{},
	}
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
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
	sort.Strings(result)
	return result
}

func normalizeSpecialUsageConfig(config SpecialUsageConfig) SpecialUsageConfig {
	config.GroupNames = normalizeStringList(config.GroupNames)
	config.ModelNames = normalizeStringList(config.ModelNames)
	if config.ChannelMultipliers == nil {
		config.ChannelMultipliers = map[string]float64{}
	}
	for channelID, multiplier := range config.ChannelMultipliers {
		if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
			config.ChannelMultipliers[channelID] = specialUsageDefaultMultiplier
		}
	}
	if config.UpdatedAt == 0 {
		config.UpdatedAt = common.GetTimestamp()
	}
	return config
}

func GetSpecialUsageConfig() SpecialUsageConfig {
	specialUsageConfigMu.RLock()
	if !specialUsageConfigCache.At.IsZero() && time.Since(specialUsageConfigCache.At) < time.Minute {
		config := specialUsageConfigCache.Config
		specialUsageConfigMu.RUnlock()
		return config
	}
	specialUsageConfigMu.RUnlock()

	config := defaultSpecialUsageConfig()
	if DB != nil {
		var option Option
		if err := DB.Where("key = ?", SpecialUsageConfigKey).First(&option).Error; err == nil {
			_ = json.Unmarshal([]byte(option.Value), &config)
		}
	}
	config = normalizeSpecialUsageConfig(config)
	specialUsageConfigMu.Lock()
	specialUsageConfigCache = specialUsageCostCache{Config: config, At: time.Now()}
	specialUsageConfigMu.Unlock()
	return config
}

func SaveSpecialUsageConfig(config SpecialUsageConfig) error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	config = normalizeSpecialUsageConfig(config)
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	option := Option{Key: SpecialUsageConfigKey, Value: string(encoded)}
	if err := DB.Save(&option).Error; err != nil {
		return err
	}
	specialUsageConfigMu.Lock()
	specialUsageConfigCache = specialUsageCostCache{Config: config, At: time.Now()}
	specialUsageConfigMu.Unlock()
	return nil
}

func specialUsageChannelSelected(config SpecialUsageConfig, channel *Channel, modelName string) bool {
	if channel == nil || !config.Enabled || len(config.GroupNames) == 0 || len(config.ModelNames) == 0 {
		return false
	}
	groupSelected := false
	for _, channelGroup := range channel.GetGroups() {
		if containsString(config.GroupNames, channelGroup) {
			groupSelected = true
			break
		}
	}
	modelSelected := false
	for _, candidate := range specialUsageModelCandidates(modelName) {
		if containsString(config.ModelNames, candidate) {
			modelSelected = true
			break
		}
	}
	if !groupSelected || !modelSelected {
		return false
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func getSpecialUsageMultiplier(config SpecialUsageConfig, channelID int, groupName string) float64 {
	if value, ok := config.ChannelMultipliers[strconv.Itoa(channelID)]; ok && value > 0 {
		return value
	}
	if value, ok := config.ChannelMultipliers["group:"+groupName]; ok && value > 0 {
		return value
	}
	return specialUsageDefaultMultiplier
}

// CalculateSpecialUsageCost returns a USD cost based on the same model pricing
// coefficients used by normal billing. Special channel prices in the existing
// channel editor are per request; they are intentionally used as the request
// cost for this monitoring ledger, while normal prices are token-based.
func CalculateSpecialUsageCost(input SpecialUsageCostInput) (float64, float64, float64, bool) {
	config := GetSpecialUsageConfig()
	if DB == nil || input.ChannelID <= 0 || len(config.GroupNames) == 0 || len(config.ModelNames) == 0 {
		return 0, 0, 0, false
	}
	var channel Channel
	if err := DB.First(&channel, input.ChannelID).Error; err != nil || !specialUsageChannelSelected(config, &channel, input.ModelName) {
		return 0, 0, 0, false
	}
	if len(input.ChannelSetting.SpecialBillingPrices) == 0 {
		input.ChannelSetting = channel.GetSetting()
	}
	multiplier := getSpecialUsageMultiplier(config, input.ChannelID, input.GroupName)
	if config.SpecialBilling {
		for _, modelName := range specialUsageModelCandidates(input.ModelName) {
			if price, ok := input.ChannelSetting.ResolveSpecialBillingPrice(modelName, input.InputTokens); ok {
				price *= multiplier
				return price, price, price, true
			}
		}
	}
	inputPrice, usePrice := ratio_setting.GetModelPrice(input.ModelName, false)
	if usePrice {
		// ModelPrice is the existing platform's per-request price. Keep it
		// consistent with normal billing instead of treating it as a token rate.
		price := inputPrice * multiplier
		return price, price, price, false
	}
	modelRatio, ok, matchedModel := ratio_setting.GetModelRatio(input.ModelName)
	if !ok {
		return 0, 0, 0, false
	}
	completionRatio := ratio_setting.GetCompletionRatio(matchedModel)
	inputPrice = modelRatio * 2.0
	outputPrice := inputPrice * completionRatio
	cost := (float64(input.InputTokens)*inputPrice + float64(input.OutputTokens)*outputPrice) / 1_000_000 * multiplier
	return cost, inputPrice * multiplier, outputPrice * multiplier, false
}

func quotaToUSD(quota int) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func bucketHour(timestamp int64) int64 {
	return timestamp - timestamp%3600
}

func specialUsageModelCandidates(modelName string) []string {
	candidates := []string{modelName}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		candidates = append(candidates, normalized)
	}
	if strings.HasSuffix(modelName, ratio_setting.CompactModelSuffix) {
		plain := strings.TrimSuffix(modelName, ratio_setting.CompactModelSuffix)
		if plain != "" && plain != modelName {
			candidates = append(candidates, plain)
		}
	}
	return candidates
}

func RecordSpecialUsage(input SpecialUsageCostInput) {
	if DB == nil || LOG_DB == nil {
		return
	}
	if input.RequestTime == 0 {
		input.RequestTime = common.GetTimestamp()
	}
	if input.Status == "" {
		input.Status = SpecialUsageStatusSuccess
	}
	if input.RequestID == "" {
		return
	}
	config := GetSpecialUsageConfig()
	var channel Channel
	if input.ChannelID <= 0 || DB.First(&channel, input.ChannelID).Error != nil || !specialUsageChannelSelected(config, &channel, input.ModelName) {
		return
	}
	if input.ChannelName == "" {
		input.ChannelName = channel.Name
	}
	cost, inputPrice, outputPrice, usedSpecialPrice := CalculateSpecialUsageCost(input)
	record := &SpecialUsageRecord{
		RequestID: input.RequestID, UserID: input.UserID, ChannelID: input.ChannelID,
		ChannelName: input.ChannelName, GroupName: input.GroupName, ModelName: input.ModelName,
		InputTokens: int64(maxInt(input.InputTokens, 0)), OutputTokens: int64(maxInt(input.OutputTokens, 0)),
		UpstreamCostUSD: cost, UserChargeUSD: quotaToUSD(input.UserChargeQuota),
		InputPriceUSD: inputPrice, OutputPriceUSD: outputPrice,
		Multiplier:       getSpecialUsageMultiplier(config, input.ChannelID, input.GroupName),
		UsedSpecialPrice: usedSpecialPrice, Status: input.Status, RequestTime: input.RequestTime,
		ErrorMessage: input.ErrorMessage,
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := recordSpecialUsageOnce(record); err == nil {
			return
		} else if !isSpecialUsageRetryableError(err) {
			common.SysLog("failed to record special usage: " + err.Error())
			return
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	common.SysLog("failed to record special usage after retries")
}

func maxInt(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}

func recordSpecialUsageOnce(record *SpecialUsageRecord) error {
	tx := LOG_DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			tx.Rollback()
			panic(recovered)
		}
	}()

	var existing SpecialUsageRecord
	query := tx.Where("request_id = ? AND channel_id = ?", record.RequestID, record.ChannelID)
	if tx.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		record.ID = 0
		if err := tx.Create(record).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := updateSpecialUsageHourly(tx, nil, record); err != nil {
			tx.Rollback()
			return err
		}
	} else {
		merged, improved := mergeSpecialUsageRecord(existing, *record)
		if improved {
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"user_id":            merged.UserID,
				"channel_name":       merged.ChannelName,
				"group_name":         merged.GroupName,
				"model_name":         merged.ModelName,
				"input_tokens":       merged.InputTokens,
				"output_tokens":      merged.OutputTokens,
				"upstream_cost_usd":  merged.UpstreamCostUSD,
				"user_charge_usd":    merged.UserChargeUSD,
				"input_price_usd":    merged.InputPriceUSD,
				"output_price_usd":   merged.OutputPriceUSD,
				"multiplier":         merged.Multiplier,
				"used_special_price": merged.UsedSpecialPrice,
				"status":             merged.Status,
				"error_message":      merged.ErrorMessage,
			}).Error; err != nil {
				tx.Rollback()
				return err
			}
			if err := updateSpecialUsageHourly(tx, &existing, &merged); err != nil {
				tx.Rollback()
				return err
			}
		} else if err := updateSpecialUsageHourly(tx, &existing, &existing); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func isSpecialUsageRetryableError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate") ||
		strings.Contains(message, "locked") || strings.Contains(message, "busy") ||
		strings.Contains(message, "deadlock") || strings.Contains(message, "serialization")
}

func specialUsageStatusRank(status string) int {
	if status == SpecialUsageStatusSuccess {
		return 2
	}
	if status == SpecialUsageStatusFailed {
		return 1
	}
	return 0
}

func specialUsageTokenTotal(record SpecialUsageRecord) int64 {
	return record.InputTokens + record.OutputTokens
}

func mergeSpecialUsageRecord(existing, incoming SpecialUsageRecord) (SpecialUsageRecord, bool) {
	merged := existing
	statusImproved := specialUsageStatusRank(incoming.Status) > specialUsageStatusRank(existing.Status)
	finalUsage := incoming.Status == SpecialUsageStatusSuccess && existing.Status != SpecialUsageStatusSuccess
	canImproveUsage := existing.Status != SpecialUsageStatusSuccess || incoming.Status == SpecialUsageStatusSuccess
	tokensImproved := canImproveUsage && (specialUsageTokenTotal(incoming) > specialUsageTokenTotal(existing) ||
		(finalUsage && specialUsageTokenTotal(incoming) >= 0))
	chargeImproved := canImproveUsage && (incoming.UserChargeUSD > existing.UserChargeUSD || finalUsage)
	costImproved := canImproveUsage && (incoming.UpstreamCostUSD > existing.UpstreamCostUSD || finalUsage)
	metadataImproved := (existing.ChannelName == "" && incoming.ChannelName != "") ||
		(existing.GroupName == "" && incoming.GroupName != "") || (existing.ModelName == "" && incoming.ModelName != "")
	errorImproved := existing.Status != SpecialUsageStatusSuccess && incoming.Status != SpecialUsageStatusSuccess &&
		incoming.ErrorMessage != "" && (existing.ErrorMessage == "" || tokensImproved || statusImproved)
	if finalUsage {
		// A failed entry is only an estimate. Once the request succeeds, replace
		// every measured/pricing field instead of retaining any estimate that is
		// larger than the final response.
		merged.UserID = incoming.UserID
		merged.ChannelName = incoming.ChannelName
		merged.GroupName = incoming.GroupName
		merged.ModelName = incoming.ModelName
		merged.InputTokens = incoming.InputTokens
		merged.OutputTokens = incoming.OutputTokens
		merged.UpstreamCostUSD = incoming.UpstreamCostUSD
		merged.UserChargeUSD = incoming.UserChargeUSD
		merged.InputPriceUSD = incoming.InputPriceUSD
		merged.OutputPriceUSD = incoming.OutputPriceUSD
		merged.Multiplier = incoming.Multiplier
		merged.UsedSpecialPrice = incoming.UsedSpecialPrice
		merged.Status = incoming.Status
		merged.ErrorMessage = ""
		return merged, true
	}
	dataImproved := statusImproved || tokensImproved || chargeImproved || costImproved
	if !dataImproved && !metadataImproved && !errorImproved {
		return merged, false
	}
	if incoming.UserID != 0 { merged.UserID = incoming.UserID }
	if incoming.ChannelName != "" && (dataImproved || existing.ChannelName == "") { merged.ChannelName = incoming.ChannelName }
	if incoming.GroupName != "" && (dataImproved || existing.GroupName == "") { merged.GroupName = incoming.GroupName }
	if incoming.ModelName != "" && (dataImproved || existing.ModelName == "") { merged.ModelName = incoming.ModelName }
	if tokensImproved {
		merged.InputTokens = incoming.InputTokens
		merged.OutputTokens = incoming.OutputTokens
		merged.InputPriceUSD = incoming.InputPriceUSD
		merged.OutputPriceUSD = incoming.OutputPriceUSD
		merged.Multiplier = incoming.Multiplier
		merged.UsedSpecialPrice = incoming.UsedSpecialPrice
	}
	if costImproved { merged.UpstreamCostUSD = incoming.UpstreamCostUSD }
	if chargeImproved { merged.UserChargeUSD = incoming.UserChargeUSD }
	if statusImproved {
		merged.Status = incoming.Status
		if incoming.Status == SpecialUsageStatusSuccess { merged.ErrorMessage = "" }
	}
	if errorImproved && incoming.Status != SpecialUsageStatusSuccess { merged.ErrorMessage = incoming.ErrorMessage }
	return merged, true
}
func specialUsageHourlyKey(record *SpecialUsageRecord) string {
	return fmt.Sprintf("%d:%s:%d:%s", bucketHour(record.RequestTime), record.GroupName, record.ChannelID, record.ModelName)
}

func updateSpecialUsageHourly(tx *gorm.DB, previous, record *SpecialUsageRecord) error {
	if tx == nil || record == nil {
		return nil
	}
	if previous != nil && specialUsageHourlyKey(previous) != specialUsageHourlyKey(record) {
		if err := incrementSpecialUsageHourly(tx, previous, SpecialUsageTotals{
			RequestCount: -1, InputTokens: -previous.InputTokens, OutputTokens: -previous.OutputTokens,
			UpstreamCostUSD: -previous.UpstreamCostUSD, UserChargeUSD: -previous.UserChargeUSD,
		}, false); err != nil {
			return err
		}
		return incrementSpecialUsageHourly(tx, record, SpecialUsageTotals{
			RequestCount: 1, InputTokens: record.InputTokens, OutputTokens: record.OutputTokens,
			UpstreamCostUSD: record.UpstreamCostUSD, UserChargeUSD: record.UserChargeUSD,
		}, true)
	}

	requestCount := int64(1)
	inputTokens := record.InputTokens
	outputTokens := record.OutputTokens
	upstreamCost := record.UpstreamCostUSD
	userCharge := record.UserChargeUSD
	if previous != nil {
		requestCount = 0
		inputTokens -= previous.InputTokens
		outputTokens -= previous.OutputTokens
		upstreamCost -= previous.UpstreamCostUSD
		userCharge -= previous.UserChargeUSD
	}
	return incrementSpecialUsageHourly(tx, record, SpecialUsageTotals{
		RequestCount: requestCount, InputTokens: inputTokens, OutputTokens: outputTokens,
		UpstreamCostUSD: upstreamCost, UserChargeUSD: userCharge,
	}, true)
}

func incrementSpecialUsageHourly(tx *gorm.DB, record *SpecialUsageRecord, delta SpecialUsageTotals, allowCreate bool) error {
	bucketTime := bucketHour(record.RequestTime)
	updates := map[string]interface{}{
		"request_count":     gorm.Expr("request_count + ?", delta.RequestCount),
		"input_tokens":      gorm.Expr("input_tokens + ?", delta.InputTokens),
		"output_tokens":     gorm.Expr("output_tokens + ?", delta.OutputTokens),
		"upstream_cost_usd": gorm.Expr("upstream_cost_usd + ?", delta.UpstreamCostUSD),
		"user_charge_usd":   gorm.Expr("user_charge_usd + ?", delta.UserChargeUSD),
	}
	if record.ChannelName != "" {
		updates["channel_name"] = record.ChannelName
	}
	if !allowCreate {
		return tx.Model(&SpecialUsageHourly{}).
			Where("bucket_time = ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketTime, record.GroupName, record.ChannelID, record.ModelName).
			Updates(updates).Error
	}
	result := tx.Model(&SpecialUsageHourly{}).
		Where("bucket_time = ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketTime, record.GroupName, record.ChannelID, record.ModelName).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	if delta.RequestCount == 0 && delta.InputTokens == 0 && delta.OutputTokens == 0 && delta.UpstreamCostUSD == 0 && delta.UserChargeUSD == 0 {
		var rowCount int64
		if err := tx.Model(&SpecialUsageHourly{}).
			Where("bucket_time = ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketTime, record.GroupName, record.ChannelID, record.ModelName).
			Count(&rowCount).Error; err != nil {
			return err
		}
		if rowCount > 0 {
			return nil
		}
	}
	return ensureSpecialUsageHourly(tx, record, delta)
}

func ensureSpecialUsageHourly(tx *gorm.DB, record *SpecialUsageRecord, delta SpecialUsageTotals) error {
	if tx == nil || record == nil {
		return nil
	}
	specialUsageHourlyBackfillMu.Lock()
	defer specialUsageHourlyBackfillMu.Unlock()
	var totals SpecialUsageTotals
	if err := tx.Model(&SpecialUsageRecord{}).
		Select("COUNT(*) AS request_count, COALESCE(SUM(input_tokens), 0) AS input_tokens, COALESCE(SUM(output_tokens), 0) AS output_tokens, COALESCE(SUM(upstream_cost_usd), 0) AS upstream_cost_usd, COALESCE(SUM(user_charge_usd), 0) AS user_charge_usd").
		Where("request_time >= ? AND request_time < ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketHour(record.RequestTime), bucketHour(record.RequestTime)+3600, record.GroupName, record.ChannelID, record.ModelName).
		Scan(&totals).Error; err != nil {
		return err
	}
	bucketTime := bucketHour(record.RequestTime)
	hourly := &SpecialUsageHourly{
		BucketTime: bucketTime, GroupName: record.GroupName, ChannelID: record.ChannelID,
		ModelName: record.ModelName, ChannelName: record.ChannelName,
		RequestCount: totals.RequestCount, InputTokens: totals.InputTokens, OutputTokens: totals.OutputTokens,
		UpstreamCostUSD: totals.UpstreamCostUSD, UserChargeUSD: totals.UserChargeUSD,
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(hourly)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 || (delta.RequestCount == 0 && delta.InputTokens == 0 && delta.OutputTokens == 0 && delta.UpstreamCostUSD == 0 && delta.UserChargeUSD == 0) {
		return nil
	}
	return tx.Model(&SpecialUsageHourly{}).
		Where("bucket_time = ? AND group_name = ? AND channel_id = ? AND model_name = ?", bucketTime, record.GroupName, record.ChannelID, record.ModelName).
		Updates(map[string]interface{}{
			"request_count":     gorm.Expr("request_count + ?", delta.RequestCount),
			"input_tokens":      gorm.Expr("input_tokens + ?", delta.InputTokens),
			"output_tokens":     gorm.Expr("output_tokens + ?", delta.OutputTokens),
			"upstream_cost_usd": gorm.Expr("upstream_cost_usd + ?", delta.UpstreamCostUSD),
			"user_charge_usd":   gorm.Expr("user_charge_usd + ?", delta.UserChargeUSD),
		}).Error
}

func applySpecialUsageQuery(query *gorm.DB, filter SpecialUsageFilter, table string) *gorm.DB {
	if filter.StartTime > 0 {
		query = query.Where(table+".request_time >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where(table+".request_time < ?", filter.EndTime)
	}
	if len(filter.GroupNames) > 0 {
		query = query.Where(table+".group_name IN ?", filter.GroupNames)
	}
	if len(filter.ModelNames) > 0 {
		query = query.Where(table+".model_name IN ?", filter.ModelNames)
	}
	if len(filter.ChannelIDs) > 0 {
		query = query.Where(table+".channel_id IN ?", filter.ChannelIDs)
	}
	return query
}

func querySpecialUsageRecords(filter SpecialUsageFilter) ([]SpecialUsageRecord, error) {
	if LOG_DB == nil {
		return nil, fmt.Errorf("log database is not initialized")
	}
	var records []SpecialUsageRecord
	query := applySpecialUsageQuery(LOG_DB.Model(&SpecialUsageRecord{}), filter, "special_usage_records")
	err := query.Order("request_time ASC, id ASC").Find(&records).Error
	return records, err
}

type specialUsageAggregateRow struct {
	BucketTime      int64   `gorm:"column:bucket_time"`
	GroupName       string  `gorm:"column:group_name"`
	ChannelID       int     `gorm:"column:channel_id"`
	ChannelName     string  `gorm:"column:channel_name"`
	ModelName       string  `gorm:"column:model_name"`
	RequestCount    int64   `gorm:"column:request_count"`
	InputTokens     int64   `gorm:"column:input_tokens"`
	OutputTokens    int64   `gorm:"column:output_tokens"`
	UpstreamCostUSD float64 `gorm:"column:upstream_cost_usd"`
	UserChargeUSD   float64 `gorm:"column:user_charge_usd"`
}

func normalizeSpecialUsageRange(filter SpecialUsageFilter) (int64, int64) {
	end := filter.EndTime
	if end <= 0 {
		end = common.GetTimestamp()
	}
	start := filter.StartTime
	if start <= 0 {
		start = end - 30*86400
	}
	return start, end
}

func specialUsageCeilHour(timestamp int64) int64 {
	bucket := bucketHour(timestamp)
	if timestamp > bucket {
		return bucket + 3600
	}
	return bucket
}

func applySpecialUsageHourlyFilter(query *gorm.DB, filter SpecialUsageFilter) *gorm.DB {
	if len(filter.GroupNames) > 0 {
		query = query.Where("group_name IN ?", filter.GroupNames)
	}
	if len(filter.ModelNames) > 0 {
		query = query.Where("model_name IN ?", filter.ModelNames)
	}
	if len(filter.ChannelIDs) > 0 {
		query = query.Where("channel_id IN ?", filter.ChannelIDs)
	}
	return query
}

func specialUsageRawBucketExpression() string {
	return "request_time - (request_time % 3600)"
}

func querySpecialUsageRawAggregates(filter SpecialUsageFilter, start, end int64, buckets []int64) ([]specialUsageAggregateRow, error) {
	if LOG_DB == nil || end <= start {
		return []specialUsageAggregateRow{}, nil
	}
	query := LOG_DB.Model(&SpecialUsageRecord{}).
		Select(specialUsageRawBucketExpression()+" AS bucket_time, group_name, channel_id, MAX(channel_name) AS channel_name, model_name, COUNT(*) AS request_count, COALESCE(SUM(input_tokens), 0) AS input_tokens, COALESCE(SUM(output_tokens), 0) AS output_tokens, COALESCE(SUM(upstream_cost_usd), 0) AS upstream_cost_usd, COALESCE(SUM(user_charge_usd), 0) AS user_charge_usd").
		Where("request_time >= ? AND request_time < ?", start, end)
	if len(buckets) > 0 {
		query = query.Where(specialUsageRawBucketExpression()+" IN ?", buckets)
	}
	if len(filter.GroupNames) > 0 {
		query = query.Where("group_name IN ?", filter.GroupNames)
	}
	if len(filter.ModelNames) > 0 {
		query = query.Where("model_name IN ?", filter.ModelNames)
	}
	if len(filter.ChannelIDs) > 0 {
		query = query.Where("channel_id IN ?", filter.ChannelIDs)
	}
	var rows []specialUsageAggregateRow
	err := query.Group(specialUsageRawBucketExpression() + ", group_name, channel_id, model_name").Find(&rows).Error
	return rows, err
}

func querySpecialUsageHourlyAggregates(filter SpecialUsageFilter, start, end int64) ([]specialUsageAggregateRow, error) {
	if LOG_DB == nil || end <= start {
		return []specialUsageAggregateRow{}, nil
	}
	query := applySpecialUsageHourlyFilter(LOG_DB.Model(&SpecialUsageHourly{}).
		Select("bucket_time, group_name, channel_id, MAX(channel_name) AS channel_name, model_name, SUM(request_count) AS request_count, SUM(input_tokens) AS input_tokens, SUM(output_tokens) AS output_tokens, SUM(upstream_cost_usd) AS upstream_cost_usd, SUM(user_charge_usd) AS user_charge_usd").
		Where("bucket_time >= ? AND bucket_time < ?", start, end), filter)
	var rows []specialUsageAggregateRow
	err := query.Group("bucket_time, group_name, channel_id, model_name").Find(&rows).Error
	return rows, err
}

func specialUsageAggregateKey(row specialUsageAggregateRow) string {
	return fmt.Sprintf("%d\x00%s\x00%d\x00%s", row.BucketTime, row.GroupName, row.ChannelID, row.ModelName)
}

func specialUsageAggregateRowsMatch(left, right []specialUsageAggregateRow) bool {
	if len(left) != len(right) {
		return false
	}
	rows := make(map[string]specialUsageAggregateRow, len(left))
	for _, row := range left {
		rows[specialUsageAggregateKey(row)] = row
	}
	for _, row := range right {
		other, ok := rows[specialUsageAggregateKey(row)]
		if !ok || other.RequestCount != row.RequestCount || other.InputTokens != row.InputTokens || other.OutputTokens != row.OutputTokens ||
			math.Abs(other.UpstreamCostUSD-row.UpstreamCostUSD) > 1e-9 || math.Abs(other.UserChargeUSD-row.UserChargeUSD) > 1e-9 {
			return false
		}
	}
	return true
}

func querySpecialUsageAggregateRows(filter SpecialUsageFilter) ([]specialUsageAggregateRow, error) {
	if LOG_DB == nil {
		return nil, fmt.Errorf("log database is not initialized")
	}
	start, end := normalizeSpecialUsageRange(filter)
	if end <= start {
		return []specialUsageAggregateRow{}, nil
	}
	fullStart := specialUsageCeilHour(start)
	fullEnd := bucketHour(end)
	rows := make([]specialUsageAggregateRow, 0)

	if fullEnd > fullStart {
		hourlyRows, err := querySpecialUsageHourlyAggregates(filter, fullStart, fullEnd)
		if err != nil {
			return nil, err
		}
		rawRows, err := querySpecialUsageRawAggregates(filter, fullStart, fullEnd, nil)
		if err != nil {
			return nil, err
		}
		if specialUsageAggregateRowsMatch(hourlyRows, rawRows) {
			rows = append(rows, hourlyRows...)
		} else {
			// Databases upgraded from before the rollup existed, or buckets
			// changed by a late retry, fall back to bounded SQL aggregates.
			rows = append(rows, rawRows...)
		}
	}

	if fullEnd <= fullStart {
		rawRows, err := querySpecialUsageRawAggregates(filter, start, end, nil)
		if err != nil {
			return nil, err
		}
		return rawRows, nil
	}
	if start < minInt64(end, fullStart) {
		rawRows, err := querySpecialUsageRawAggregates(filter, start, minInt64(end, fullStart), nil)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rawRows...)
	}
	if fullEnd < end && maxInt64(start, fullEnd) < end {
		rawRows, err := querySpecialUsageRawAggregates(filter, maxInt64(start, fullEnd), end, nil)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rawRows...)
	}
	return rows, nil
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func aggregateSpecialUsage(records []SpecialUsageRecord) SpecialUsageTotals {
	result := SpecialUsageTotals{}
	for _, record := range records {
		result.RequestCount++
		result.InputTokens += record.InputTokens
		result.OutputTokens += record.OutputTokens
		result.UpstreamCostUSD += record.UpstreamCostUSD
		result.UserChargeUSD += record.UserChargeUSD
	}
	return result
}

func aggregateSpecialUsageRows(rows []specialUsageAggregateRow) SpecialUsageTotals {
	result := SpecialUsageTotals{}
	for _, row := range rows {
		result.RequestCount += row.RequestCount
		result.InputTokens += row.InputTokens
		result.OutputTokens += row.OutputTokens
		result.UpstreamCostUSD += row.UpstreamCostUSD
		result.UserChargeUSD += row.UserChargeUSD
	}
	return result
}

func AggregateSpecialUsageOverview(filter SpecialUsageFilter) (SpecialUsageOverview, error) {
	rows, err := querySpecialUsageAggregateRows(filter)
	if err != nil {
		return SpecialUsageOverview{}, err
	}
	overview := SpecialUsageOverview{
		Series: make([]SpecialUsageSeriesPoint, 0), GroupCosts: make([]SpecialUsageNamedValue, 0),
		ModelTokens: make([]SpecialUsageNamedValue, 0), Channels: make([]SpecialUsageChannelStat, 0),
		InputOutput: make([]SpecialUsageNamedValue, 0), GroupProfit: make([]SpecialUsageNamedValue, 0),
		ChannelCostTree: make([]SpecialUsageTreeNode, 0),
		LastUpdatedAt:   common.GetTimestamp(),
	}
	overview.Totals = aggregateSpecialUsageRows(rows)
	timeMap := map[int64]*SpecialUsageSeriesPoint{}
	groupMap := map[string]*SpecialUsageNamedValue{}
	modelMap := map[string]*SpecialUsageNamedValue{}
	channelMap := map[int]*SpecialUsageChannelStat{}
	treeMap := map[string]map[string]float64{}
	for _, row := range rows {
		hour := row.BucketTime
		series := timeMap[hour]
		if series == nil {
			series = &SpecialUsageSeriesPoint{Time: hour}
			timeMap[hour] = series
		}
		series.RequestCount += row.RequestCount
		series.InputTokens += row.InputTokens
		series.OutputTokens += row.OutputTokens
		series.UpstreamCostUSD += row.UpstreamCostUSD
		series.UserChargeUSD += row.UserChargeUSD
		group := groupMap[row.GroupName]
		if group == nil {
			group = &SpecialUsageNamedValue{Name: row.GroupName}
			groupMap[row.GroupName] = group
		}
		group.RequestCount += row.RequestCount
		group.InputTokens += row.InputTokens
		group.OutputTokens += row.OutputTokens
		group.UpstreamCostUSD += row.UpstreamCostUSD
		group.UserChargeUSD += row.UserChargeUSD
		modelValue := modelMap[row.ModelName]
		if modelValue == nil {
			modelValue = &SpecialUsageNamedValue{Name: row.ModelName}
			modelMap[row.ModelName] = modelValue
		}
		modelValue.RequestCount += row.RequestCount
		modelValue.InputTokens += row.InputTokens
		modelValue.OutputTokens += row.OutputTokens
		modelValue.UpstreamCostUSD += row.UpstreamCostUSD
		modelValue.UserChargeUSD += row.UserChargeUSD
		channel := channelMap[row.ChannelID]
		if channel == nil {
			channel = &SpecialUsageChannelStat{ChannelID: row.ChannelID, ChannelName: row.ChannelName}
			channelMap[row.ChannelID] = channel
		}
		channel.RequestCount += row.RequestCount
		channel.InputTokens += row.InputTokens
		channel.OutputTokens += row.OutputTokens
		channel.UpstreamCostUSD += row.UpstreamCostUSD
		channel.UserChargeUSD += row.UserChargeUSD
		if treeMap[row.GroupName] == nil {
			treeMap[row.GroupName] = make(map[string]float64)
		}
		treeMap[row.GroupName][row.ChannelName] += row.UpstreamCostUSD
	}
	for _, value := range timeMap {
		overview.Series = append(overview.Series, *value)
	}
	for _, value := range groupMap {
		overview.GroupCosts = append(overview.GroupCosts, *value)
		profit := *value
		profit.UserChargeUSD = value.UserChargeUSD
		overview.GroupProfit = append(overview.GroupProfit, profit)
	}
	for _, value := range modelMap {
		overview.ModelTokens = append(overview.ModelTokens, *value)
	}
	for _, value := range channelMap {
		if value.RequestCount > 0 {
			value.AverageCostUSD = value.UpstreamCostUSD / float64(value.RequestCount)
		}
		overview.Channels = append(overview.Channels, *value)
	}
	for groupName, channels := range treeMap {
		node := SpecialUsageTreeNode{Name: groupName, Children: make([]SpecialUsageTreeNode, 0)}
		for channelName, cost := range channels {
			node.Children = append(node.Children, SpecialUsageTreeNode{Name: channelName, Value: cost})
			node.Value += cost
		}
		sort.Slice(node.Children, func(i, j int) bool { return node.Children[i].Value > node.Children[j].Value })
		overview.ChannelCostTree = append(overview.ChannelCostTree, node)
	}
	overview.InputOutput = []SpecialUsageNamedValue{
		{Name: "input", InputTokens: overview.Totals.InputTokens},
		{Name: "output", OutputTokens: overview.Totals.OutputTokens},
	}
	sort.Slice(overview.Series, func(i, j int) bool { return overview.Series[i].Time < overview.Series[j].Time })
	sort.Slice(overview.GroupCosts, func(i, j int) bool {
		return overview.GroupCosts[i].UpstreamCostUSD > overview.GroupCosts[j].UpstreamCostUSD
	})
	sort.Slice(overview.ModelTokens, func(i, j int) bool {
		return overview.ModelTokens[i].InputTokens+overview.ModelTokens[i].OutputTokens > overview.ModelTokens[j].InputTokens+overview.ModelTokens[j].OutputTokens
	})
	sort.Slice(overview.Channels, func(i, j int) bool {
		return overview.Channels[i].UpstreamCostUSD > overview.Channels[j].UpstreamCostUSD
	})
	sort.Slice(overview.ChannelCostTree, func(i, j int) bool { return overview.ChannelCostTree[i].Value > overview.ChannelCostTree[j].Value })
	markSpecialUsageAnomalies(&overview)
	return overview, nil
}

func markSpecialUsageAnomalies(overview *SpecialUsageOverview) {
	if overview == nil || len(overview.Channels) == 0 {
		return
	}
	average := 0.0
	for _, channel := range overview.Channels {
		average += channel.AverageCostUSD
	}
	average /= float64(len(overview.Channels))
	if average <= 0 {
		return
	}
	for index := range overview.Channels {
		value := overview.Channels[index].AverageCostUSD
		overview.Channels[index].Anomaly = value > average*1.3 || value < average*0.7
	}
}

func PredictSpecialUsageCost(filter SpecialUsageFilter, basis string, days float64) (SpecialUsageForecast, error) {
	if days <= 0 {
		return SpecialUsageForecast{}, fmt.Errorf("forecast days must be positive")
	}
	end := filter.EndTime
	if end <= 0 {
		end = common.GetTimestamp()
	}
	start := filter.StartTime
	if start <= 0 {
		start = end - 24*3600
	}
	if basis == "today_current" {
		localNow := time.Now().In(time.Local)
		nextDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day()+1, 0, 0, 0, 0, localNow.Location())
		days = nextDay.Sub(localNow).Hours() / 24
		if days <= 0 {
			days = 1.0 / 86400
		}
		dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).Unix()
		if dayStart > start {
			start = dayStart
		}
	}
	if end <= start {
		return SpecialUsageForecast{Basis: basis, Days: days}, nil
	}
	forecastFilter := filter
	forecastFilter.StartTime = start
	forecastFilter.EndTime = end
	rows, err := querySpecialUsageAggregateRows(forecastFilter)
	if err != nil {
		return SpecialUsageForecast{}, err
	}
	totals := aggregateSpecialUsageRows(rows)
	if totals.RequestCount == 0 {
		return SpecialUsageForecast{Basis: basis, Days: days}, nil
	}
	durationDays := float64(end-start) / 86400
	if durationDays <= 0 {
		durationDays = 1.0 / 24
	}
	dailyTokens := float64(totals.InputTokens+totals.OutputTokens) / durationDays
	tokenTotal := float64(totals.InputTokens + totals.OutputTokens)
	averageCostPerToken := 0.0
	if tokenTotal > 0 {
		averageCostPerToken = totals.UpstreamCostUSD / tokenTotal
	}
	forecastTokens := dailyTokens * days
	return SpecialUsageForecast{
		Basis: basis, Days: days, DailyTokens: dailyTokens, ForecastTokens: forecastTokens,
		AverageCostPerToken: averageCostPerToken, ForecastCostUSD: forecastTokens * averageCostPerToken,
	}, nil
}

func CountSpecialUsageRecords(filter SpecialUsageFilter) (int64, error) {
	if LOG_DB == nil { return 0, fmt.Errorf("log database is not initialized") }
	query := applySpecialUsageQuery(LOG_DB.Model(&SpecialUsageRecord{}), filter, "special_usage_records")
	var total int64
	if err := query.Count(&total).Error; err != nil { return 0, err }
	return total, nil
}

func ListSpecialUsageRecords(filter SpecialUsageFilter, page, pageSize int) ([]SpecialUsageRecord, int64, error) {
	if LOG_DB == nil {
		return nil, 0, fmt.Errorf("log database is not initialized")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100000 {
		pageSize = 100
	}
	query := applySpecialUsageQuery(LOG_DB.Model(&SpecialUsageRecord{}), filter, "special_usage_records")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []SpecialUsageRecord
	err := query.Order("request_time desc, id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&records).Error
	return records, total, err
}

func sanitizeCSVCell(value string) string {
	if value == "" {
		return value
	}
	// Excel treats cells beginning with these characters as formulas. Prefixing
	// a tab keeps the displayed value while preventing formula execution.
	switch value[0] {
	case '=', '+', '-', '@':
		return "\t" + value
	default:
		return value
	}
}

func WriteSpecialUsageCSV(writer io.Writer, records []SpecialUsageRecord) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"request_id", "channel_id", "channel_name", "group", "model", "input_tokens", "output_tokens", "upstream_cost_usd", "user_charge_usd", "status", "request_time"}); err != nil {
		return err
	}
	for _, record := range records {
		if err := csvWriter.Write([]string{
			sanitizeCSVCell(record.RequestID), strconv.Itoa(record.ChannelID), sanitizeCSVCell(record.ChannelName), sanitizeCSVCell(record.GroupName), sanitizeCSVCell(record.ModelName),
			strconv.FormatInt(record.InputTokens, 10), strconv.FormatInt(record.OutputTokens, 10),
			strconv.FormatFloat(record.UpstreamCostUSD, 'f', 10, 64), strconv.FormatFloat(record.UserChargeUSD, 'f', 10, 64),
			sanitizeCSVCell(record.Status), time.Unix(record.RequestTime, 0).Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func ParseSpecialUsageCSVFilter(values map[string][]string) SpecialUsageFilter {
	filter := SpecialUsageFilter{}
	filter.GroupNames = normalizeStringList(values["group"])
	filter.ModelNames = normalizeStringList(values["model"])
	for _, channelID := range values["channel_id"] {
		if parsed, err := strconv.Atoi(channelID); err == nil && parsed > 0 {
			filter.ChannelIDs = append(filter.ChannelIDs, parsed)
		}
	}
	return filter
}
