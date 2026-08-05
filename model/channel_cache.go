package model

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var fallbackChannels []*Channel                      // 启用兜底模式的渠道（跨分组）
var channelSyncLock sync.RWMutex

func sortFallbackChannelsByPriority(channels []*Channel) {
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i] == nil || channels[j] == nil {
			return channels[i] != nil
		}
		if fallbackPriority := getFallbackPrioritySetting(channels[i].Setting); fallbackPriority != getFallbackPrioritySetting(channels[j].Setting) {
			return fallbackPriority > getFallbackPrioritySetting(channels[j].Setting)
		}
		if channels[i].GetPriority() != channels[j].GetPriority() {
			return channels[i].GetPriority() > channels[j].GetPriority()
		}
		return channels[i].Id < channels[j].Id
	})
}

// GetFallbackChannels 返回所有启用兜底模式的渠道（跨分组）
func GetFallbackChannels() []*Channel {
	// 当内存缓存未启用时，直接走数据库回退查询，保证兜底功能可用
	if !common.MemoryCacheEnabled {
		channels, err := GetFallbackChannelsFromDB()
		if err != nil {
			common.SysError(fmt.Sprintf("GetFallbackChannels 数据库回退查询失败: %v", err))
			return make([]*Channel, 0)
		}
		return channels
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	result := make([]*Channel, 0, len(fallbackChannels))
	for _, ch := range fallbackChannels {
		if ch.Status == common.ChannelStatusEnabled {
			result = append(result, ch)
		}
	}
	sortFallbackChannelsByPriority(result)
	return result
}

// HasAvailableFallbackChannels 检查是否有可用的兜底渠道（跨分组）
func HasAvailableFallbackChannels() bool {
	// 当内存缓存未启用时，直接走数据库回退查询，保证兜底功能可用
	if !common.MemoryCacheEnabled {
		channels, err := GetFallbackChannelsFromDB()
		if err != nil {
			common.SysError(fmt.Sprintf("HasAvailableFallbackChannels 数据库回退查询失败: %v", err))
			return false
		}
		return len(channels) > 0
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	for _, ch := range fallbackChannels {
		if ch.Status == common.ChannelStatusEnabled {
			return true
		}
	}
	return false
}

// HasAvailableFallbackChannelsExcludingUsed 检查是否存在尚未被使用过的可用兜底渠道。
// 主循环兜底检查与 getChannel 兜底选择使用一致的判断口径（均排除已使用渠道），
// 避免主循环认为有可用兜底渠道但 getChannel 实际无法选出兜底渠道的不一致问题。
func HasAvailableFallbackChannelsExcludingUsed(usedChannelIds []int) bool {
	channels := GetFallbackChannels()
	for _, ch := range channels {
		used := false
		for _, usedId := range usedChannelIds {
			if ch.Id == usedId {
				used = true
				break
			}
		}
		if !used {
			return true
		}
	}
	return false
}

// HasEmergencyChannel reports whether the normal model pool contains an
// enabled emergency channel. Affinity lookup uses this to avoid selecting a
// cached normal channel ahead of the emergency-only selection policy.
func HasEmergencyChannel(group string, modelName string) bool {
	group = strings.TrimSpace(group)
	modelName = strings.TrimSpace(modelName)
	if group == "" || modelName == "" {
		return false
	}

	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()

		channels := group2model2channels[group][modelName]
		if len(channels) == 0 {
			channels = group2model2channels[group][ratio_setting.FormatMatchingModelName(modelName)]
		}
		for _, channelID := range channels {
			if channel, ok := channelsIDM[channelID]; ok && channel.IsEmergencyPlanEnabled() {
				return true
			}
		}
		return false
	}

	if DB == nil {
		return false
	}
	modelNames := channelModelCandidates(modelName)
	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? AND model IN ? AND enabled = ?", group, modelNames, true).Find(&abilities).Error; err != nil {
		return false
	}
	for _, ability := range abilities {
		var channel Channel
		if err := DB.Select("status, setting").First(&channel, ability.ChannelId).Error; err != nil {
			continue
		}
		if channel.Status == common.ChannelStatusEnabled && isEmergencyPlanEnabledSetting(channel.Setting) {
			return true
		}
	}
	return false
}

// GetSpecialBillingChannel returns the highest-priority enabled non-emergency
// channel that has a special price for the requested model. It is used only as
// a billing source when an emergency channel has taken over the route.
func GetSpecialBillingChannel(group string, modelName string) (*Channel, error) {
	if strings.TrimSpace(group) == "" || strings.TrimSpace(modelName) == "" {
		return nil, nil
	}

	modelNames := channelModelCandidates(modelName)
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()

		channelIDs := make([]int, 0)
		seen := make(map[int]struct{})
		for _, candidateModel := range modelNames {
			for _, channelID := range group2model2channels[group][candidateModel] {
				if _, exists := seen[channelID]; exists {
					continue
				}
				seen[channelID] = struct{}{}
				channelIDs = append(channelIDs, channelID)
			}
		}

		candidates := make([]*Channel, 0, len(channelIDs))
		for _, channelID := range channelIDs {
			if channel, ok := channelsIDM[channelID]; ok {
				candidates = append(candidates, channel)
			}
		}
		return findSpecialBillingChannel(candidates, modelName), nil
	}

	if DB == nil {
		return nil, nil
	}

	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? AND model IN ? AND enabled = ?", group, modelNames, true).
		Order("priority DESC, weight DESC, channel_id ASC").Find(&abilities).Error; err != nil {
		return nil, err
	}

	candidates := make([]*Channel, 0, len(abilities))
	seen := make(map[int]struct{})
	for _, ability := range abilities {
		if _, exists := seen[ability.ChannelId]; exists {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channel, err := GetChannelById(ability.ChannelId, true)
		if err != nil {
			continue
		}
		candidates = append(candidates, channel)
	}
	return findSpecialBillingChannel(candidates, modelName), nil
}

func channelModelCandidates(modelName string) []string {
	modelNames := []string{modelName}
	if normalized := ratio_setting.FormatMatchingModelName(modelName); normalized != "" && normalized != modelName {
		modelNames = append(modelNames, normalized)
	}
	if strings.HasSuffix(modelName, ratio_setting.CompactModelSuffix) {
		plainModel := strings.TrimSuffix(modelName, ratio_setting.CompactModelSuffix)
		if plainModel != "" && plainModel != modelName {
			modelNames = append(modelNames, plainModel)
		}
	}
	return modelNames
}

func findSpecialBillingChannel(channels []*Channel, modelName string) *Channel {
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i] == nil || channels[j] == nil {
			return channels[i] != nil
		}
		if channels[i].GetPriority() != channels[j].GetPriority() {
			return channels[i].GetPriority() > channels[j].GetPriority()
		}
		if channels[i].GetWeight() != channels[j].GetWeight() {
			return channels[i].GetWeight() > channels[j].GetWeight()
		}
		return channels[i].Id < channels[j].Id
	})
	for _, channel := range channels {
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		setting := channel.GetSetting()
		if setting.EmergencyPlanEnabled || setting.FallbackModelEnabled || !setting.SpecialBilling {
			continue
		}
		for _, candidateModel := range channelModelCandidates(modelName) {
			if _, ok := setting.SpecialBillingPrices[candidateModel]; ok {
				return channel
			}
		}
	}
	return nil
}

// GetFallbackChannelsFromDB 从数据库查询所有启用兜底模式且状态为启用的渠道。
// 用于 MemoryCacheEnabled=false 时的兜底渠道查询回退路径。
func GetFallbackChannelsFromDB() ([]*Channel, error) {
	var channels []*Channel
	if err := DB.Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	result := make([]*Channel, 0, len(channels))
	for _, ch := range channels {
		// 使用纯函数 isFallbackModelEnabledSetting 替代 ch.GetSetting().FallbackModelEnabled，
		// 避免 GetSetting() 在 JSON 反序列化失败时执行 Save() 写回 DB 引发高并发竞态写。
		if isFallbackModelEnabledSetting(ch.Setting) {
			result = append(result, ch)
		}
	}
	sortFallbackChannelsByPriority(result)
	return result, nil
}

// ErrAllChannelsRpmFull is returned when all matching channels have their RPM at max capacity.
var ErrAllChannelsRpmFull = errors.New("all channels rpm full")

// ErrChannelSpecialUserUnauthorized is returned when matching channels exist,
// but all of them are restricted to other users.
var ErrChannelSpecialUserUnauthorized = errors.New("channel special user permission denied")

// CheckChannelRpmFullFunc is a hook function to check if a channel's RPM is full.
// Registered by service/rpm_tracker.go via init().
var CheckChannelRpmFullFunc func(channelId int) bool

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	newFallbackChannels := make([]*Channel, 0)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.GetSetting().FallbackModelEnabled {
			newFallbackChannels = append(newFallbackChannels, channel)
		}
	}
	sortFallbackChannelsByPriority(newFallbackChannels)
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	fallbackChannels = newFallbackChannels
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

// isChannelUsed checks if a channel ID is in the used channel IDs list
func isChannelUsed(channelId int, usedChannelIds []int) bool {
	for _, usedId := range usedChannelIds {
		if usedId == channelId {
			return true
		}
	}
	return false
}

// GetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// usedChannelIds: list of channel IDs that have been tried and failed, will be excluded from selection.
func GetRandomSatisfiedChannel(group string, model string, retry int, usedChannelIds []int, userId ...int) (*Channel, error) {
	return getSatisfiedChannel(group, model, retry, usedChannelIds, "", userId...)
}

// GetStableSatisfiedChannel selects an eligible channel deterministically from
// the same weighted pool used by GetRandomSatisfiedChannel. It is intended for
// cache-affinity cold starts, where a random first selection would split the
// same upstream cache session across instances.
func GetStableSatisfiedChannel(group string, model string, stableKey string, retry int, usedChannelIds []int, userId ...int) (*Channel, error) {
	return getSatisfiedChannel(group, model, retry, usedChannelIds, stableKey, userId...)
}

func getSatisfiedChannel(group string, model string, retry int, usedChannelIds []int, stableKey string, userId ...int) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		if strings.TrimSpace(stableKey) != "" {
			return GetStableChannel(group, model, stableKey, retry, usedChannelIds, userId...)
		}
		return GetChannel(group, model, retry, usedChannelIds, userId...)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := group2model2channels[group][model]

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
	}

	if len(channels) == 0 {
		return nil, nil
	}

	hasEmergency := false
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok && channel.IsEmergencyPlanEnabled() {
			hasEmergency = true
			break
		}
	}
	if hasEmergency {
		emergencyChannels := make([]int, 0, len(channels))
		for _, channelId := range channels {
			if channel, ok := channelsIDM[channelId]; ok && channel.IsEmergencyPlanEnabled() {
				emergencyChannels = append(emergencyChannels, channelId)
			}
		}
		channels = emergencyChannels
	}

	// Filter out used channels and disabled channels (defensive check)
	var availableChannels []int
	for _, channelId := range channels {
		if isChannelUsed(channelId, usedChannelIds) {
			continue
		}
		// 二次校验渠道状态：即使渠道仍留在 group2model2channels 中，
		// 如果其状态已非启用（可能因多Key全部禁用等路径未及时同步缓存），也跳过
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.Status != common.ChannelStatusEnabled {
				continue
			}
			// 排除兜底渠道，确保兜底渠道只被兜底逻辑使用，不被跨渠道重试消耗
			if isFallbackModelEnabledSetting(channel.Setting) {
				continue
			}
		}
		availableChannels = append(availableChannels, channelId)
	}

	// If all channels have been used, return nil to indicate exhaustion
	if len(availableChannels) == 0 {
		return nil, nil
	}

	// Filter out channels whose RPM is full
	var rpmAvailableChannels []int
	var anyRpmLimited bool
	var anySpecialUserRestricted bool
	var anySpecialUserAllowed bool
	var anyCallCountLimited bool
	requestUserId := firstUserId(userId...)
	// 在循环外获取用户缓存，读取 GPT 模式状态；获取失败时保守视为 false
	userGptMode := false
	if requestUserId > 0 {
		if userCache, cacheErr := GetUserCache(requestUserId); cacheErr == nil && userCache != nil {
			userGptMode = userCache.GetSetting().GptMode
		}
	}
	for _, channelId := range availableChannels {
		if channel, ok := channelsIDM[channelId]; ok {
			// 跳过兜底模式渠道，不参与常规选择
			if channel.GetSetting().FallbackModelEnabled {
				continue
			}
			if !channel.AllowsSpecialUser(requestUserId) {
				anySpecialUserRestricted = true
				continue
			}
			anySpecialUserAllowed = true
			if !channel.AllowsGptMode(userGptMode) {
				continue
			}
			// 检查调用次数配额：max_call_count > 0 且 used_call_count >= max_call_count 时不可选
			if channel.MaxCallCount > 0 && channel.UsedCallCount >= channel.MaxCallCount {
				anyCallCountLimited = true
				continue
			}
			if channel.MaxRPM > 0 {
				anyRpmLimited = true
				if CheckChannelRpmFullFunc != nil && CheckChannelRpmFullFunc(channelId) {
					continue // skip RPM-full channels
				}
			}
			rpmAvailableChannels = append(rpmAvailableChannels, channelId)
		}
	}

	// If all channels were filtered out by RPM, signal for queuing
	if len(rpmAvailableChannels) == 0 {
		if anySpecialUserRestricted && !anySpecialUserAllowed {
			return nil, ErrChannelSpecialUserUnauthorized
		}
		if anyRpmLimited || anyCallCountLimited {
			return nil, ErrAllChannelsRpmFull
		}
		return nil, nil
	}

	// Use RPM-filtered channels for selection
	availableChannels = rpmAvailableChannels

	if len(availableChannels) == 1 {
		if channel, ok := channelsIDM[availableChannels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", availableChannels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range availableChannels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if retry >= len(uniquePriorities) {
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range availableChannels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Use a stable weighted slot for affinity cold starts. The target channel
	// list is sorted by ID so every instance with the same channel snapshot
	// produces the same result. Normal routing keeps its existing randomness.
	randomWeight := 0
	if strings.TrimSpace(stableKey) != "" {
		targetChannels = append([]*Channel(nil), targetChannels...)
		sort.SliceStable(targetChannels, func(i, j int) bool {
			return targetChannels[i].Id < targetChannels[j].Id
		})
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(stableKey))
		randomWeight = int(hasher.Sum32() % uint32(totalWeight))
	} else {
		// Generate a random value in the range [0, totalWeight)
		randomWeight = rand.Intn(totalWeight)
	}

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	} else {
		// 渠道重新启用时，把它加回 group2model2channels 路由表，否则不会被路由
		if channel, ok := channelsIDM[id]; ok && channel.Status == common.ChannelStatusEnabled {
			groups := strings.Split(channel.Group, ",")
			for _, group := range groups {
				if group == "" {
					continue
				}
				if _, ok := group2model2channels[group]; !ok {
					group2model2channels[group] = make(map[string][]int)
				}
				models := strings.Split(channel.Models, ",")
				for _, model := range models {
					if model == "" {
						continue
					}
					channels := group2model2channels[group][model]
					// 避免重复添加
					alreadyExists := false
					for _, cid := range channels {
						if cid == id {
							alreadyExists = true
							break
						}
					}
					if !alreadyExists {
						channels = append(channels, id)
					}
					// 按优先级重新排序
					sort.Slice(channels, func(i, j int) bool {
						ci := channelsIDM[channels[i]]
						cj := channelsIDM[channels[j]]
						if ci == nil || cj == nil {
							return false
						}
						return ci.GetPriority() > cj.GetPriority()
					})
					group2model2channels[group][model] = channels
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	println("CacheUpdateChannel:", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)

	println("before:", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
	channelsIDM[channel.Id] = channel
	println("after :", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
}

// UpdateChannelCallCountInCache 更新缓存中渠道的调用次数计数（线程安全）
// 用于渠道配额重置后同步内存缓存，避免最长 60s 内渠道仍因旧计数不可用。
// 注意：channelSyncLock 只保护 map 结构，不保护 Channel 对象字段。
// 由于 SyncChannelCache 会替换整个 map（channelsIDM = newMap），
// 在 RLock 释放后 channel 指针可能指向已被替换的旧对象。
// 因此这里重新获取一次 RLock 内写入，确保写入的是当前 map 中的对象。
func UpdateChannelCallCountInCache(channelId int, usedCallCount int64, maxCallCount int64) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.RLock()
	channel, ok := channelsIDM[channelId]
	if ok {
		// 在 RLock 保护范围内完成 atomic 写入，确保写入的是当前 map 中的对象
		atomic.StoreInt64(&channel.UsedCallCount, usedCallCount)
		if maxCallCount > 0 {
			atomic.StoreInt64(&channel.MaxCallCount, maxCallCount)
		}
	}
	channelSyncLock.RUnlock()
}
