package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType    int     `json:"channel_type"`
	ChannelSetting *string `json:"channel_setting"`
	ChannelName    string  `json:"channel_name"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type, channels.setting as channel_setting, channels.name as channel_name").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	if err != nil {
		return nil, err
	}
	filtered := make([]AbilityWithChannel, 0, len(abilities))
	for _, ability := range abilities {
		if isEmergencyPlanEnabledSetting(ability.ChannelSetting) {
			continue
		}
		if isFallbackModelEnabledSetting(ability.ChannelSetting) {
			continue
		}
		filtered = append(filtered, ability)
	}
	return filtered, nil
}

func GetGroupEnabledModels(group string) []string {
	if group == "" {
		return []string{}
	}
	pricing := GetPricing()
	models := make([]string, 0, len(pricing))
	for _, item := range pricing {
		for _, enableGroup := range item.EnableGroup {
			if enableGroup == group {
				models = append(models, item.ModelName)
				break
			}
		}
	}
	return models
}

func GetEnabledModels() []string {
	pricing := GetPricing()
	models := make([]string, 0, len(pricing))
	for _, item := range pricing {
		models = append(models, item.ModelName)
	}
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

// SearchAbilities returns a paginated and filtered list of abilities joined with their channel info,
// including the channel name so the frontend can display it.
func SearchAbilities(offset int, limit int, group string, model string, channelId int, onlyEnabled bool) ([]AbilityWithChannel, int64, error) {
	query := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type, channels.setting as channel_setting, channels.name as channel_name").
		Joins("left join channels on abilities.channel_id = channels.id")

	if group != "" {
		query = query.Where("abilities.group = ?", group)
	}
	if model != "" {
		query = query.Where("abilities.model = ?", model)
	}
	if channelId != 0 {
		query = query.Where("abilities.channel_id = ?", channelId)
	}
	if onlyEnabled {
		query = query.Where("abilities.enabled = ?", true)
	}

	var total int64
	if err := query.Distinct("abilities.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var abilities []AbilityWithChannel
	if err := query.Order("abilities.group asc, abilities.model asc, abilities.channel_id asc").
		Offset(offset).Limit(limit).Scan(&abilities).Error; err != nil {
		return nil, 0, err
	}
	return abilities, total, nil
}

func getPriority(group string, model string, retry int) (int, error) {

	var priorities []int
	err := DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Order("priority DESC").              // 按优先级降序排序
		Pluck("priority", &priorities).Error // Pluck用于将查询的结果直接扫描到一个切片中

	if err != nil {
		// 处理错误
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

func getChannelQuery(group string, model string, retry int) (*gorm.DB, error) {
	maxPrioritySubQuery := DB.Model(&Ability{}).Select("MAX(priority)").Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true)
	channelQuery := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = (?)", group, model, true, maxPrioritySubQuery)
	if retry != 0 {
		priority, err := getPriority(group, model, retry)
		if err != nil {
			return nil, err
		} else {
			channelQuery = DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priority)
		}
	}

	return channelQuery, nil
}

// isAbilityChannelUsed checks if a channel ID is in the used channel IDs list
func isAbilityChannelUsed(channelId int, usedChannelIds []int) bool {
	for _, usedId := range usedChannelIds {
		if usedId == channelId {
			return true
		}
	}
	return false
}

func GetChannel(group string, model string, retry int, usedChannelIds []int, userId ...int) (*Channel, error) {
	var abilities []Ability

	var err error = nil
	channelQuery, err := getChannelQuery(group, model, retry)
	if err != nil {
		return nil, err
	}
	if common.UsingSQLite || common.UsingPostgreSQL {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	} else {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	}
	if err != nil {
		return nil, err
	}

	type abilityCandidate struct {
		ability Ability
		channel Channel
	}
	emergencyCandidates := make([]abilityCandidate, 0, len(abilities))
	normalCandidates := make([]abilityCandidate, 0, len(abilities))
	anyRpmLimited := false
	anySpecialUserRestricted := false
	anySpecialUserAllowed := false
	anyCallCountLimited := false
	requestUserId := firstUserId(userId...)
	// 在循环外获取用户缓存，读取 GPT 模式状态；获取失败时保守视为 false
	userGptMode := false
	if requestUserId > 0 {
		if userCache, cacheErr := GetUserCache(requestUserId); cacheErr == nil && userCache != nil {
			userGptMode = userCache.GetSetting().GptMode
		}
	}
	for _, ability_ := range abilities {
		if len(usedChannelIds) > 0 && isAbilityChannelUsed(ability_.ChannelId, usedChannelIds) {
			continue
		}
		candidateChannel := Channel{}
		if err = DB.First(&candidateChannel, "id = ?", ability_.ChannelId).Error; err != nil {
			return nil, err
		}
		// 排除兜底渠道，确保兜底渠道只被兜底逻辑使用，不被跨渠道重试消耗
		if isFallbackModelEnabledSetting(candidateChannel.Setting) {
			continue
		}
		if !candidateChannel.AllowsSpecialUser(requestUserId) {
			anySpecialUserRestricted = true
			continue
		}
		anySpecialUserAllowed = true
		if !candidateChannel.AllowsGptMode(userGptMode) {
			continue
		}
		// 检查调用次数配额：max_call_count > 0 且 used_call_count >= max_call_count 时不可选
		if candidateChannel.MaxCallCount > 0 && candidateChannel.UsedCallCount >= candidateChannel.MaxCallCount {
			anyCallCountLimited = true
			continue
		}
		if candidateChannel.MaxRPM > 0 {
			anyRpmLimited = true
			if CheckChannelRpmFullFunc != nil && CheckChannelRpmFullFunc(candidateChannel.Id) {
				continue
			}
		}
		candidate := abilityCandidate{
			ability: ability_,
			channel: candidateChannel,
		}
		if candidateChannel.IsEmergencyPlanEnabled() {
			emergencyCandidates = append(emergencyCandidates, candidate)
		} else {
			normalCandidates = append(normalCandidates, candidate)
		}
	}

	candidates := normalCandidates
	if len(emergencyCandidates) > 0 {
		candidates = emergencyCandidates
	}

	if len(candidates) > 0 {
		weightSum := uint(0)
		for _, candidate := range candidates {
			weightSum += candidate.ability.Weight + 10
		}
		if weightSum == 0 {
			if anySpecialUserRestricted && !anySpecialUserAllowed {
				return nil, ErrChannelSpecialUserUnauthorized
			}
			if anyRpmLimited || anyCallCountLimited {
				return nil, ErrAllChannelsRpmFull
			}
			return nil, nil
		}
		weight := common.GetRandomInt(int(weightSum))
		for _, candidate := range candidates {
			weight -= int(candidate.ability.Weight) + 10
			if weight <= 0 {
				return &candidate.channel, nil
			}
		}
	} else {
		return nil, nil
	}
	return nil, nil
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// DeleteAbilitiesWithTx 在事务中删除渠道的能力记录
func (channel *Channel) DeleteAbilitiesWithTx(tx *gorm.DB) error {
	return tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingSQLite {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
