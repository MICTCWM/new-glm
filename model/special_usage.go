package model

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
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
	ChannelIDs         []int              `json:"channel_ids"`
	ChannelIDsSet      bool               `json:"channel_ids_set"`
	SpecialBilling     bool               `json:"special_billing"`
	ChannelMultipliers map[string]float64 `json:"channel_multipliers"`
	UpdatedAt          int64              `json:"updated_at"`
}

type SpecialUsageRecord struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	// Attempt is the upstream retry/fallback attempt that produced this ledger row.
	// It is optional so rows written before the monitor upgrade remain valid.
	Attempt          int     `json:"attempt" gorm:"default:0;index;uniqueIndex:uk_special_usage_request_attempt;priority:2"`
	UsageSource      string  `json:"usage_source" gorm:"size:32;index"`
	PriceSnapshot    string  `json:"price_snapshot,omitempty" gorm:"type:text"`
	RequestID        string  `json:"request_id" gorm:"size:128;uniqueIndex:uk_special_usage_request_attempt;priority:1"`
	UserID           int     `json:"user_id" gorm:"index"`
	ChannelID        int     `json:"channel_id" gorm:"index;uniqueIndex:uk_special_usage_request_attempt;priority:3"`
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

// SpecialUsagePriceSnapshot captures the mutable pricing inputs used for a
// request. It is stored as JSON in the ledger for historical auditability.
type SpecialUsagePriceSnapshot struct {
	Source           string  `json:"source,omitempty"`
	ModelName        string  `json:"model_name,omitempty"`
	ModelPrice       float64 `json:"model_price,omitempty"`
	ModelRatio       float64 `json:"model_ratio,omitempty"`
	CompletionRatio  float64 `json:"completion_ratio,omitempty"`
	InputPriceUSD    float64 `json:"input_price_usd,omitempty"`
	OutputPriceUSD   float64 `json:"output_price_usd,omitempty"`
	Multiplier       float64 `json:"multiplier,omitempty"`
	UsedSpecialPrice bool    `json:"used_special_price"`
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
	// Optional audit fields. Zero values preserve existing callers and records.
	Attempt       int
	UsageSource   string
	PriceSnapshot string
	// Frozen pricing is supplied by the request path when available.
	FrozenModelPrice           float64
	FrozenModelRatio           float64
	FrozenCompletionRatio      float64
	FrozenUsePrice             bool
	FrozenPriceValid           bool
	FrozenChannelSetting       dto.ChannelSettings
	FrozenChannelSettingValid  bool
	FrozenUsedSpecialPrice     bool
	FrozenSpecialPriceValid    bool
	FrozenSpecialBilling       bool
	FrozenSpecialBillingValid  bool
	FrozenPriceSource          string
	FrozenBillingChannelID     int
	SpecialUsageConfig         SpecialUsageConfig
	SpecialUsageConfigValid    bool
	SpecialUsageSelected       bool
	SpecialUsageSelectionValid bool
	FrozenMultiplier           float64
	FrozenMultiplierValid      bool
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
	StartTime     int64
	EndTime       int64
	GroupNames    []string
	ModelNames    []string
	ChannelIDs    []int
	ChannelIDsSet bool
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
	BaselineCostUSD float64 `json:"baseline_cost_usd,omitempty"`
	AnomalyReason   string  `json:"anomaly_reason,omitempty"`
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
	HistoricalDays      float64 `json:"historical_days,omitempty"`
	DailyTokens         float64 `json:"daily_tokens"`
	DailyCostUSD        float64 `json:"daily_cost_usd"`
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
		ChannelIDs:         []int{},
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

func normalizeIntList(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func normalizeSpecialUsageConfig(config SpecialUsageConfig) SpecialUsageConfig {
	config.GroupNames = normalizeStringList(config.GroupNames)
	config.ModelNames = normalizeStringList(config.ModelNames)
	config.ChannelIDs = normalizeIntList(config.ChannelIDs)
	// Configurations written before channel_ids_set was introduced used an
	// empty list to mean "all matching channels". A non-empty legacy list is
	// unambiguously an explicit channel selection.
	if !config.ChannelIDsSet && len(config.ChannelIDs) > 0 {
		config.ChannelIDsSet = true
	}
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
	if err := UpdateOption(SpecialUsageConfigKey, string(encoded)); err != nil {
		return err
	}
	specialUsageConfigMu.Lock()
	specialUsageConfigCache = specialUsageCostCache{Config: config, At: time.Now()}
	specialUsageConfigMu.Unlock()
	return nil
}

func SpecialUsageChannelMatches(config SpecialUsageConfig, channelID int, modelName, groupName string) bool {
	if DB == nil || channelID <= 0 {
		return false
	}
	var channel Channel
	if err := DB.First(&channel, channelID).Error; err != nil {
		return false
	}
	if groupName != "" {
		matchedGroup := false
		for _, candidate := range channel.GetGroups() {
			if candidate == groupName {
				matchedGroup = true
				break
			}
		}
		if !matchedGroup {
			return false
		}
	}
	return specialUsageChannelSelected(normalizeSpecialUsageConfig(config), &channel, modelName)
}

func specialUsageChannelSelected(config SpecialUsageConfig, channel *Channel, modelName string) bool {
	if channel == nil || !config.Enabled || len(config.GroupNames) == 0 || len(config.ModelNames) == 0 {
		return false
	}
	if config.ChannelIDsSet && len(config.ChannelIDs) == 0 {
		return false
	}
	if len(config.ChannelIDs) > 0 {
		selected := false
		for _, id := range config.ChannelIDs {
			if id == channel.Id {
				selected = true
				break
			}
		}
		if !selected {
			return false
		}
	}
	groupSelected := false
	for _, channelGroup := range channel.GetGroups() {
		if containsString(config.GroupNames, channelGroup) {
			groupSelected = true
			break
		}
	}
	modelSelected := false
	if modelName != "" {
		for _, candidate := range specialUsageModelCandidates(modelName) {
			if containsString(config.ModelNames, candidate) {
				modelSelected = true
				break
			}
		}
	} else {
		for _, channelModel := range channel.GetModels() {
			for _, candidate := range specialUsageModelCandidates(channelModel) {
				if containsString(config.ModelNames, candidate) {
					modelSelected = true
					break
				}
			}
			if modelSelected {
				break
			}
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

// GetSpecialUsageMultiplier exposes the normalized multiplier lookup to relay
// integration code so it can freeze the value before the request settles.
func GetSpecialUsageMultiplier(config SpecialUsageConfig, channelID int, groupName string) float64 {
	return getSpecialUsageMultiplier(normalizeSpecialUsageConfig(config), channelID, groupName)
}

// CalculateSpecialUsageCost returns a USD cost based on the same model pricing
// coefficients used by normal billing. Special channel prices in the existing
// channel editor are per request; they are intentionally used as the request
// cost for this monitoring ledger, while normal prices are token-based.
func CalculateSpecialUsageCost(input SpecialUsageCostInput) (float64, float64, float64, bool) {
	config := GetSpecialUsageConfig()
	if input.SpecialUsageConfigValid {
		config = normalizeSpecialUsageConfig(input.SpecialUsageConfig)
	}
	if DB == nil || input.ChannelID <= 0 || len(config.GroupNames) == 0 || len(config.ModelNames) == 0 {
		return 0, 0, 0, false
	}
	var channel Channel
	if err := DB.First(&channel, input.ChannelID).Error; err != nil {
		return 0, 0, 0, false
	}
	if input.SpecialUsageSelectionValid {
		if !input.SpecialUsageSelected {
			return 0, 0, 0, false
		}
	} else if !specialUsageChannelSelected(config, &channel, input.ModelName) {
		return 0, 0, 0, false
	}
	if input.FrozenChannelSettingValid {
		input.ChannelSetting = input.FrozenChannelSetting
	} else if len(input.ChannelSetting.SpecialBillingPrices) == 0 {
		input.ChannelSetting = channel.GetSetting()
	}
	if input.FrozenSpecialBillingValid {
		config.SpecialBilling = input.FrozenSpecialBilling
	}
	multiplier := getSpecialUsageMultiplier(config, input.ChannelID, input.GroupName)
	if input.FrozenMultiplierValid && input.FrozenMultiplier > 0 {
		multiplier = input.FrozenMultiplier
	}
	if config.SpecialBilling {
		for _, modelName := range specialUsageModelCandidates(input.ModelName) {
			if price, ok := input.ChannelSetting.ResolveSpecialBillingPrice(modelName, input.InputTokens); ok {
				// Special billing is an explicit channel price. It takes
				// precedence over the global price and configured multiplier.
				return price, price, price, true
			}
		}
	}
	inputPrice, usePrice := ratio_setting.GetModelPrice(input.ModelName, false)
	modelRatio, ok, matchedModel := ratio_setting.GetModelRatio(input.ModelName)
	if input.FrozenPriceValid {
		inputPrice = input.FrozenModelPrice
		usePrice = input.FrozenUsePrice
		modelRatio = input.FrozenModelRatio
		ok = modelRatio > 0 || usePrice
		matchedModel = input.ModelName
	}
	if usePrice {
		// ModelPrice is the existing platform's per-request price. Keep it
		// consistent with normal billing instead of treating it as a token rate.
		price := inputPrice * multiplier
		return price, price, price, false
	}
	if !ok {
		return 0, 0, 0, false
	}
	completionRatio := ratio_setting.GetCompletionRatio(matchedModel)
	if input.FrozenPriceValid && input.FrozenCompletionRatio > 0 {
		completionRatio = input.FrozenCompletionRatio
	}
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

func buildSpecialUsagePriceSnapshot(input SpecialUsageCostInput, source string, multiplier, inputPrice, outputPrice float64, usedSpecialPrice bool) string {
	snapshot := SpecialUsagePriceSnapshot{
		Source: source, ModelName: input.ModelName, ModelPrice: input.FrozenModelPrice,
		ModelRatio: input.FrozenModelRatio, CompletionRatio: input.FrozenCompletionRatio,
		InputPriceUSD: inputPrice, OutputPriceUSD: outputPrice, Multiplier: multiplier,
		UsedSpecialPrice: usedSpecialPrice,
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	return string(encoded)
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
	if input.SpecialUsageConfigValid {
		config = normalizeSpecialUsageConfig(input.SpecialUsageConfig)
	}
	var channel Channel
	if input.ChannelID <= 0 || DB.First(&channel, input.ChannelID).Error != nil {
		return
	}
	if input.SpecialUsageSelectionValid {
		if !input.SpecialUsageSelected {
			return
		}
	} else if !specialUsageChannelSelected(config, &channel, input.ModelName) {
		return
	}
	if input.ChannelName == "" {
		input.ChannelName = channel.Name
	}
	cost, inputPrice, outputPrice, usedSpecialPrice := CalculateSpecialUsageCost(input)
	hasRealUsage := input.InputTokens > 0 || input.OutputTokens > 0
	if input.Status != SpecialUsageStatusSuccess && !hasRealUsage {
		// A failed request without upstream usage is an audit event only.
		input.InputTokens = 0
		input.OutputTokens = 0
		cost = 0
	}
	if input.UsageSource == "" {
		if hasRealUsage {
			input.UsageSource = "upstream"
		} else {
			input.UsageSource = "none"
		}
	}
	multiplier := getSpecialUsageMultiplier(config, input.ChannelID, input.GroupName)
	if input.PriceSnapshot == "" {
		source := input.FrozenPriceSource
		if source == "" {
			source = input.UsageSource
		}
		input.PriceSnapshot = buildSpecialUsagePriceSnapshot(input, source, multiplier, inputPrice, outputPrice, usedSpecialPrice)
	}
	record := &SpecialUsageRecord{
		RequestID: input.RequestID, UserID: input.UserID, ChannelID: input.ChannelID,
		ChannelName: input.ChannelName, GroupName: input.GroupName, ModelName: input.ModelName,
		InputTokens: int64(maxInt(input.InputTokens, 0)), OutputTokens: int64(maxInt(input.OutputTokens, 0)),
		UpstreamCostUSD: cost, UserChargeUSD: quotaToUSD(input.UserChargeQuota),
		InputPriceUSD: inputPrice, OutputPriceUSD: outputPrice,
		Multiplier: multiplier, Attempt: input.Attempt, UsageSource: input.UsageSource,
		PriceSnapshot: input.PriceSnapshot, UsedSpecialPrice: usedSpecialPrice, Status: input.Status, RequestTime: input.RequestTime,
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
	query := tx.Where("request_id = ? AND channel_id = ? AND attempt = ?", record.RequestID, record.ChannelID, record.Attempt)
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
				"attempt":            merged.Attempt,
				"usage_source":       merged.UsageSource,
				"price_snapshot":     merged.PriceSnapshot,
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
		merged.Attempt = incoming.Attempt
		merged.UsageSource = incoming.UsageSource
		merged.PriceSnapshot = incoming.PriceSnapshot
		merged.Status = incoming.Status
		merged.ErrorMessage = ""
		return merged, true
	}
	dataImproved := statusImproved || tokensImproved || chargeImproved || costImproved
	if !dataImproved && !metadataImproved && !errorImproved {
		return merged, false
	}
	if incoming.UserID != 0 {
		merged.UserID = incoming.UserID
	}
	if incoming.ChannelName != "" && (dataImproved || existing.ChannelName == "") {
		merged.ChannelName = incoming.ChannelName
	}
	if incoming.GroupName != "" && (dataImproved || existing.GroupName == "") {
		merged.GroupName = incoming.GroupName
	}
	if incoming.ModelName != "" && (dataImproved || existing.ModelName == "") {
		merged.ModelName = incoming.ModelName
	}
	if incoming.Attempt > merged.Attempt || merged.UsageSource == "" {
		merged.Attempt = incoming.Attempt
		if incoming.UsageSource != "" {
			merged.UsageSource = incoming.UsageSource
		}
		if incoming.PriceSnapshot != "" {
			merged.PriceSnapshot = incoming.PriceSnapshot
		}
	}
	if tokensImproved {
		merged.InputTokens = incoming.InputTokens
		merged.OutputTokens = incoming.OutputTokens
		merged.InputPriceUSD = incoming.InputPriceUSD
		merged.OutputPriceUSD = incoming.OutputPriceUSD
		merged.Multiplier = incoming.Multiplier
		merged.UsedSpecialPrice = incoming.UsedSpecialPrice
	}
	if costImproved {
		merged.UpstreamCostUSD = incoming.UpstreamCostUSD
	}
	if chargeImproved {
		merged.UserChargeUSD = incoming.UserChargeUSD
	}
	if statusImproved {
		merged.Status = incoming.Status
		if incoming.Status == SpecialUsageStatusSuccess {
			merged.ErrorMessage = ""
		}
	}
	if errorImproved && incoming.Status != SpecialUsageStatusSuccess {
		merged.ErrorMessage = incoming.ErrorMessage
	}
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
	if filter.ChannelIDsSet || len(filter.ChannelIDs) > 0 {
		if len(filter.ChannelIDs) == 0 {
			return query.Where("1 = 0")
		}
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
	if filter.ChannelIDsSet || len(filter.ChannelIDs) > 0 {
		if len(filter.ChannelIDs) == 0 {
			return query.Where("1 = 0")
		}
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
	if filter.ChannelIDsSet || len(filter.ChannelIDs) > 0 {
		if len(filter.ChannelIDs) == 0 {
			return []specialUsageAggregateRow{}, nil
		}
		query = query.Where("channel_id IN ?", filter.ChannelIDs)
	}
	var rows []specialUsageAggregateRow
	err := query.Group(specialUsageRawBucketExpression() + ", group_name, channel_id, model_name").Find(&rows).Error
	return rows, err
}

func querySpecialUsageRawBucketTimes(filter SpecialUsageFilter, start, end int64) ([]int64, error) {
	if LOG_DB == nil || end <= start {
		return []int64{}, nil
	}
	query := LOG_DB.Model(&SpecialUsageRecord{}).
		Select(specialUsageRawBucketExpression()+" AS bucket_time").
		Where("request_time >= ? AND request_time < ?", start, end)
	if len(filter.GroupNames) > 0 {
		query = query.Where("group_name IN ?", filter.GroupNames)
	}
	if len(filter.ModelNames) > 0 {
		query = query.Where("model_name IN ?", filter.ModelNames)
	}
	if filter.ChannelIDsSet || len(filter.ChannelIDs) > 0 {
		if len(filter.ChannelIDs) == 0 {
			return []int64{}, nil
		}
		query = query.Where("channel_id IN ?", filter.ChannelIDs)
	}
	var rows []struct {
		BucketTime int64 `gorm:"column:bucket_time"`
	}
	if err := query.Group(specialUsageRawBucketExpression()).Find(&rows).Error; err != nil {
		return nil, err
	}
	buckets := make([]int64, 0, len(rows))
	for _, row := range rows {
		buckets = append(buckets, row.BucketTime)
	}
	return buckets, nil
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
		if len(hourlyRows) == 0 {
			rawRows, err := querySpecialUsageRawAggregates(filter, fullStart, fullEnd, nil)
			if err != nil {
				return nil, err
			}
			rows = append(rows, rawRows...)
		} else {
			// Rollups are the normal path. Only inspect bucket presence in the
			// raw ledger, then aggregate raw rows for buckets absent from the
			// rollup. This keeps the large-table fallback bounded to upgrade
			// gaps instead of re-aggregating every complete hour.
			rawBuckets, err := querySpecialUsageRawBucketTimes(filter, fullStart, fullEnd)
			if err != nil {
				return nil, err
			}
			hourlyBuckets := make(map[int64]struct{}, len(hourlyRows))
			for _, row := range hourlyRows {
				hourlyBuckets[row.BucketTime] = struct{}{}
			}
			missingBuckets := make([]int64, 0)
			rawBucketSet := make(map[int64]struct{}, len(rawBuckets))
			for _, bucket := range rawBuckets {
				rawBucketSet[bucket] = struct{}{}
				if _, ok := hourlyBuckets[bucket]; !ok {
					missingBuckets = append(missingBuckets, bucket)
				}
			}
			for _, row := range hourlyRows {
				if _, ok := rawBucketSet[row.BucketTime]; ok {
					rows = append(rows, row)
				}
			}
			if len(missingBuckets) > 0 {
				rawRows, err := querySpecialUsageRawAggregates(filter, fullStart, fullEnd, missingBuckets)
				if err != nil {
					return nil, err
				}
				rows = append(rows, rawRows...)
			}
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
	weightedRequests := int64(0)
	weightedCost := 0.0
	for _, channel := range overview.Channels {
		if channel.RequestCount > 0 {
			weightedRequests += channel.RequestCount
			weightedCost += channel.UpstreamCostUSD
		}
	}
	if weightedRequests == 0 {
		return
	}
	average := weightedCost / float64(weightedRequests)
	for index := range overview.Channels {
		value := overview.Channels[index].AverageCostUSD
		overview.Channels[index].BaselineCostUSD = average
		if value <= 0 {
			continue
		}
		if average <= 0 {
			continue
		}
		relative := (value - average) / average
		if math.Abs(relative) > 0.3 {
			overview.Channels[index].Anomaly = true
			overview.Channels[index].AnomalyReason = fmt.Sprintf(
				"平均 %.6f，当前 %.6f，偏离 %.1f%%",
				average,
				value,
				math.Abs(relative)*100,
			)
		}
	}
}

func PredictSpecialUsageCost(filter SpecialUsageFilter, basis string, days float64, todayRemaining bool) (SpecialUsageForecast, error) {
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
		dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).Unix()
		start = dayStart
		end = localNow.Unix()
	}
	if todayRemaining {
		localNow := time.Now().In(time.Local)
		nextDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day()+1, 0, 0, 0, 0, localNow.Location())
		days = nextDay.Sub(localNow).Hours() / 24
		if days <= 0 {
			days = 1.0 / 86400
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
	dailyCost := totals.UpstreamCostUSD / durationDays
	tokenTotal := float64(totals.InputTokens + totals.OutputTokens)
	averageCostPerToken := 0.0
	if tokenTotal > 0 {
		averageCostPerToken = totals.UpstreamCostUSD / tokenTotal
	}
	forecastTokens := dailyTokens * days
	return SpecialUsageForecast{
		Basis: basis, Days: days, HistoricalDays: durationDays, DailyTokens: dailyTokens,
		DailyCostUSD: dailyCost, ForecastTokens: forecastTokens,
		AverageCostPerToken: averageCostPerToken, ForecastCostUSD: dailyCost * days,
	}, nil
}

func CountSpecialUsageRecords(filter SpecialUsageFilter) (int64, error) {
	if LOG_DB == nil {
		return 0, fmt.Errorf("log database is not initialized")
	}
	query := applySpecialUsageQuery(LOG_DB.Model(&SpecialUsageRecord{}), filter, "special_usage_records")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
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

type specialUsageXLSXCellValue struct {
	value   string
	numeric bool
}

func specialUsageXLSXEscape(value string) string {
	var builder strings.Builder
	_ = xml.EscapeText(&builder, []byte(value))
	return builder.String()
}

func specialUsageXLSXColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
}

func specialUsageXLSXCell(reference string, value specialUsageXLSXCellValue) string {
	if value.numeric {
		return `<c r="` + reference + `"><v>` + specialUsageXLSXEscape(value.value) + `</v></c>`
	}
	return `<c r="` + reference + `" t="inlineStr"><is><t xml:space="preserve">` +
		specialUsageXLSXEscape(value.value) + `</t></is></c>`
}

func specialUsageXLSXRow(rowNumber int, values []specialUsageXLSXCellValue) string {
	var builder strings.Builder
	builder.WriteString(`<row r="`)
	builder.WriteString(strconv.Itoa(rowNumber))
	builder.WriteString(`">`)
	for column, value := range values {
		reference := specialUsageXLSXColumnName(column) + strconv.Itoa(rowNumber)
		builder.WriteString(specialUsageXLSXCell(reference, value))
	}
	builder.WriteString(`</row>`)
	return builder.String()
}

func buildSpecialUsageWorksheet(records []SpecialUsageRecord) []byte {
	const columnCount = 11
	endRow := len(records) + 1
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	builder.WriteString(`<dimension ref="A1:`)
	builder.WriteString(specialUsageXLSXColumnName(columnCount - 1))
	builder.WriteString(strconv.Itoa(endRow))
	builder.WriteString(`"/><sheetData>`)
	builder.WriteString(specialUsageXLSXRow(1, []specialUsageXLSXCellValue{
		{value: "request_id"},
		{value: "channel_id"},
		{value: "channel_name"},
		{value: "group"},
		{value: "model"},
		{value: "input_tokens"},
		{value: "output_tokens"},
		{value: "upstream_cost_usd"},
		{value: "user_charge_usd"},
		{value: "status"},
		{value: "request_time"},
	}))
	for index, record := range records {
		builder.WriteString(specialUsageXLSXRow(index+2, []specialUsageXLSXCellValue{
			{value: record.RequestID},
			{value: strconv.Itoa(record.ChannelID), numeric: true},
			{value: record.ChannelName},
			{value: record.GroupName},
			{value: record.ModelName},
			{value: strconv.FormatInt(record.InputTokens, 10), numeric: true},
			{value: strconv.FormatInt(record.OutputTokens, 10), numeric: true},
			{value: strconv.FormatFloat(record.UpstreamCostUSD, 'f', 10, 64), numeric: true},
			{value: strconv.FormatFloat(record.UserChargeUSD, 'f', 10, 64), numeric: true},
			{value: record.Status},
			{value: time.Unix(record.RequestTime, 0).Format(time.RFC3339)},
		}))
	}
	builder.WriteString(`</sheetData></worksheet>`)
	return []byte(builder.String())
}

func writeSpecialUsageXLSXFile(archive *zip.Writer, name string, content []byte) error {
	file, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	return err
}

// WriteSpecialUsageXLSX writes a minimal OOXML workbook without adding an
// external spreadsheet dependency to the server.
func WriteSpecialUsageXLSX(writer io.Writer, records []SpecialUsageRecord) error {
	if writer == nil {
		return errors.New("xlsx writer is nil")
	}
	archive := zip.NewWriter(writer)
	files := []struct {
		name    string
		content []byte
	}{
		{
			name: "[Content_Types].xml",
			content: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`),
		},
		{
			name: "_rels/.rels",
			content: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`),
		},
		{
			name: "xl/workbook.xml",
			content: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Special Usage" sheetId="1" r:id="rId1"/></sheets>
</workbook>`),
		},
		{
			name: "xl/_rels/workbook.xml.rels",
			content: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`),
		},
		{
			name:    "xl/worksheets/sheet1.xml",
			content: buildSpecialUsageWorksheet(records),
		},
	}
	for _, file := range files {
		if err := writeSpecialUsageXLSXFile(archive, file.name, file.content); err != nil {
			_ = archive.Close()
			return err
		}
	}
	return archive.Close()
}

func ParseSpecialUsageCSVFilter(values map[string][]string) SpecialUsageFilter {
	filter := SpecialUsageFilter{}
	filter.GroupNames = normalizeStringList(values["group"])
	filter.ModelNames = normalizeStringList(values["model"])
	_, filter.ChannelIDsSet = values["channel_id"]
	for _, channelID := range values["channel_id"] {
		if parsed, err := strconv.Atoi(channelID); err == nil && parsed > 0 {
			filter.ChannelIDs = append(filter.ChannelIDs, parsed)
		}
	}
	return filter
}
