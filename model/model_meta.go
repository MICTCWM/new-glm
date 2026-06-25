package model

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

const (
	ModelTypeRegular = iota
	ModelTypeAuto
)

type BoundChannel struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type Model struct {
	Id                  int            `json:"id"`
	ModelName           string         `json:"model_name" gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description         string         `json:"description,omitempty" gorm:"type:text"`
	Icon                string         `json:"icon,omitempty" gorm:"type:varchar(128)"`
	Tags                string         `json:"tags,omitempty" gorm:"type:varchar(255)"`
	VendorID            int            `json:"vendor_id,omitempty" gorm:"index"`
	Endpoints           string         `json:"endpoints,omitempty" gorm:"type:text"`
	ModelType           int            `json:"model_type" gorm:"default:0;index"`
	ContextLength       int            `json:"context_length,omitempty" gorm:"default:0"`
	AutoRouteModelsJSON string         `json:"-" gorm:"column:auto_route_models;type:text"`
	Status              int            `json:"status" gorm:"default:1"`
	SyncOfficial        int            `json:"sync_official" gorm:"default:1"`
	CreatedTime         int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt           gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`

	BoundChannels   []BoundChannel `json:"bound_channels,omitempty" gorm:"-"`
	EnableGroups    []string       `json:"enable_groups,omitempty" gorm:"-"`
	QuotaTypes      []int          `json:"quota_types,omitempty" gorm:"-"`
	NameRule        int            `json:"name_rule" gorm:"default:0"`
	AutoRouteModels []string     `json:"auto_route_models,omitempty" gorm:"-"`

	MatchedModels []string `json:"matched_models,omitempty" gorm:"-"`
	MatchedCount  int      `json:"matched_count,omitempty" gorm:"-"`
}

func normalizeModelNameList(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		normalized = append(normalized, modelName)
	}
	return normalized
}

func (mi *Model) IsAutoModel() bool {
	return mi != nil && mi.ModelType == ModelTypeAuto
}

func (mi *Model) PrepareForSave() error {
	if mi == nil {
		return nil
	}
	mi.AutoRouteModels = normalizeModelNameList(mi.AutoRouteModels)
	if mi.IsAutoModel() {
		mi.ModelName = "Auto"
		mi.NameRule = NameRuleExact
		mi.SyncOfficial = 0
	} else {
		mi.AutoRouteModels = nil
	}
	if len(mi.AutoRouteModels) == 0 {
		mi.AutoRouteModelsJSON = "[]"
		return nil
	}
	data, err := json.Marshal(mi.AutoRouteModels)
	if err != nil {
		return err
	}
	mi.AutoRouteModelsJSON = string(data)
	return nil
}

func (mi *Model) LoadDerivedFields() {
	if mi == nil {
		return
	}
	mi.AutoRouteModels = []string{}
	if strings.TrimSpace(mi.AutoRouteModelsJSON) == "" {
		return
	}
	var routeModels []string
	if err := json.Unmarshal([]byte(mi.AutoRouteModelsJSON), &routeModels); err != nil {
		return
	}
	mi.AutoRouteModels = normalizeModelNameList(routeModels)
}

func (mi *Model) Insert() error {
	now := common.GetTimestamp()
	mi.CreatedTime = now
	mi.UpdatedTime = now
	if err := mi.PrepareForSave(); err != nil {
		return err
	}

	// 保存原始值（因为 Create 后可能被 GORM 的 default 标签覆盖为 1）
	originalStatus := mi.Status
	originalSyncOfficial := mi.SyncOfficial

	// 先创建记录（GORM 会对零值字段应用默认值）
	if err := DB.Create(mi).Error; err != nil {
		return err
	}

	// 使用保存的原始值进行更新，确保零值能正确保存
	return DB.Model(&Model{}).Where("id = ?", mi.Id).Updates(map[string]interface{}{
		"status":            originalStatus,
		"sync_official":     originalSyncOfficial,
		"model_type":        mi.ModelType,
		"context_length":    mi.ContextLength,
		"auto_route_models": mi.AutoRouteModelsJSON,
	}).Error
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

func (mi *Model) Update() error {
	mi.UpdatedTime = common.GetTimestamp()
	if err := mi.PrepareForSave(); err != nil {
		return err
	}
	// 使用 Select 强制更新所有字段，包括零值
	return DB.Model(&Model{}).Where("id = ?", mi.Id).
		Select("model_name", "description", "icon", "tags", "vendor_id", "endpoints", "model_type", "context_length", "auto_route_models", "status", "sync_official", "name_rule", "updated_time").
		Updates(mi).Error
}

func (mi *Model) Delete() error {
	return DB.Delete(mi).Error
}

func GetVendorModelCounts() (map[int64]int64, error) {
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := DB.Model(&Model{}).
		Select("vendor_id as vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(stats))
	for _, s := range stats {
		m[s.VendorID] = s.Count
	}
	return m, nil
}

func GetAllModels(offset int, limit int) ([]*Model, error) {
	var models []*Model
	err := DB.Order("id DESC").Offset(offset).Limit(limit).Find(&models).Error
	for _, m := range models {
		m.LoadDerivedFields()
	}
	return models, err
}

func GetModelByName(name string) (*Model, error) {
	var m Model
	if err := DB.Where("model_name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	m.LoadDerivedFields()
	return &m, nil
}

func GetBoundChannelsByModelsMap(modelNames []string) (map[string][]BoundChannel, error) {
	result := make(map[string][]BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Name  string
		Type  int
	}
	var rows []row
	err := DB.Table("channels").
		Select("abilities.model as model, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = append(result[r.Model], BoundChannel{Name: r.Name, Type: r.Type})
	}
	return result, nil
}

func SearchModels(keyword string, vendor string, offset int, limit int) ([]*Model, int64, error) {
	var models []*Model
	db := DB.Model(&Model{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("model_name LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like)
	}
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			db = db.Where("models.vendor_id = ?", vid)
		} else {
			db = db.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	for _, m := range models {
		m.LoadDerivedFields()
	}
	return models, total, nil
}
