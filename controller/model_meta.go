package controller

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func validateModelPayload(c *gin.Context, m *model.Model) bool {
	if m == nil {
		common.ApiErrorMsg(c, "模型数据不能为空")
		return false
	}
	if m.ContextLength < 0 {
		common.ApiErrorMsg(c, "上下文长度不能为负数")
		return false
	}
	if m.IsAutoModel() {
		m.ModelName = "Auto"
		m.NameRule = model.NameRuleExact
		m.SyncOfficial = 0
		if len(m.AutoRouteModels) == 0 {
			common.ApiErrorMsg(c, "Auto 模型至少需要配置一个自动路由模型")
			return false
		}
		for _, routeModel := range m.AutoRouteModels {
			if strings.EqualFold(strings.TrimSpace(routeModel), "Auto") {
				common.ApiErrorMsg(c, "Auto 模型不能把自己加入自动路由列表")
				return false
			}
		}
		return true
	}
	if strings.TrimSpace(m.ModelName) == "" {
		common.ApiErrorMsg(c, "模型名称不能为空")
		return false
	}
	return true
}

// GetAllModelsMeta 获取模型列表（分页）
func GetAllModelsMeta(c *gin.Context) {
	// 如果是选择器场景（select=all），返回全量模型列表（仅选择所需字段）
	// 用于规避分页上限（100），保证 Auto 模型路由多选能获取全部可选模型
	if c.Query("select") == "all" {
		var allModels []*model.Model
		err := model.DB.Select("id", "model_name", "model_type", "status", "vendor_id", "name_rule", "sync_official").Order("id DESC").Find(&allModels).Error
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{
			"items":     allModels,
			"total":     len(allModels),
			"page":      1,
			"page_size": len(allModels),
		})
		return
	}

	pageInfo := common.GetPageQuery(c)
	// Allow larger page sizes for model management to support auto route model selection
	if requestedSize := c.Query("page_size"); requestedSize != "" {
		if size, err := strconv.Atoi(requestedSize); err == nil && size > 100 && size <= 10000 {
			pageInfo.PageSize = size
		}
	}
	modelsMeta, err := model.GetAllModels(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 批量填充附加字段，提升列表接口性能
	enrichModels(modelsMeta)
	var total int64
	model.DB.Model(&model.Model{}).Count(&total)

	// 统计供应商计数（全部数据，不受分页影响）
	vendorCounts, _ := model.GetVendorModelCounts()

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(modelsMeta)
	common.ApiSuccess(c, gin.H{
		"items":         modelsMeta,
		"total":         total,
		"page":          pageInfo.GetPage(),
		"page_size":     pageInfo.GetPageSize(),
		"vendor_counts": vendorCounts,
	})
}

// SearchModelsMeta 搜索模型列表
func SearchModelsMeta(c *gin.Context) {

	keyword := c.Query("keyword")
	vendor := c.Query("vendor")
	pageInfo := common.GetPageQuery(c)

	modelsMeta, total, err := model.SearchModels(keyword, vendor, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 批量填充附加字段，提升列表接口性能
	enrichModels(modelsMeta)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(modelsMeta)
	common.ApiSuccess(c, pageInfo)
}

// GetModelMeta 根据 ID 获取单条模型信息
func GetModelMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var m model.Model
	if err := model.DB.First(&m, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	m.LoadDerivedFields()
	enrichModels([]*model.Model{&m})
	common.ApiSuccess(c, &m)
}

// CreateModelMeta 新建模型
func CreateModelMeta(c *gin.Context) {
	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ApiError(c, err)
		return
	}
	if !validateModelPayload(c, &m) {
		return
	}
	// 名称冲突检查
	if dup, err := model.IsModelNameDuplicated(0, m.ModelName); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "模型名称已存在")
		return
	}

	if err := m.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RefreshPricing()
	common.ApiSuccess(c, &m)
}

// UpdateModelMeta 更新模型
func UpdateModelMeta(c *gin.Context) {
	statusOnly := c.Query("status_only") == "true"

	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ApiError(c, err)
		return
	}
	if m.Id == 0 {
		common.ApiErrorMsg(c, "缺少模型 ID")
		return
	}

	if statusOnly {
		// 只更新状态，防止误清空其他字段
		if err := model.DB.Model(&model.Model{}).Where("id = ?", m.Id).Update("status", m.Status).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		if !validateModelPayload(c, &m) {
			return
		}
		// 名称冲突检查
		if dup, err := model.IsModelNameDuplicated(m.Id, m.ModelName); err != nil {
			common.ApiError(c, err)
			return
		} else if dup {
			common.ApiErrorMsg(c, "模型名称已存在")
			return
		}

		if err := m.Update(); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	model.RefreshPricing()
	common.ApiSuccess(c, &m)
}

// DeleteModelMeta 删除模型
func DeleteModelMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Delete(&model.Model{}, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.RefreshPricing()
	common.ApiSuccess(c, nil)
}

// enrichModels 批量填充附加信息：端点、渠道、分组、计费类型，避免 N+1 查询
func enrichModels(models []*model.Model) {
	if len(models) == 0 {
		return
	}
	for _, m := range models {
		if m != nil {
			m.LoadDerivedFields()
		}
	}

	// 1) 拆分精确与规则匹配
	exactNames := make([]string, 0)
	exactIdx := make(map[string][]int) // modelName -> indices in models
	ruleIndices := make([]int, 0)
	autoIndices := make([]int, 0)
	for i, m := range models {
		if m == nil {
			continue
		}
		if m.IsAutoModel() {
			autoIndices = append(autoIndices, i)
			continue
		}
		if m.NameRule == model.NameRuleExact {
			exactNames = append(exactNames, m.ModelName)
			exactIdx[m.ModelName] = append(exactIdx[m.ModelName], i)
		} else {
			ruleIndices = append(ruleIndices, i)
		}
	}

	for _, idx := range autoIndices {
		mm := models[idx]
		if len(mm.AutoRouteModels) == 0 {
			continue
		}

		endpointSet := make(map[constant.EndpointType]struct{})
		groupSet := make(map[string]struct{})
		quotaSet := make(map[int]struct{})
		channelSet := make(map[string]model.BoundChannel)

		for _, routeModel := range mm.AutoRouteModels {
			for _, endpointType := range model.GetModelSupportEndpointTypes(routeModel) {
				endpointSet[endpointType] = struct{}{}
			}
			for _, group := range model.GetModelEnableGroups(routeModel) {
				groupSet[group] = struct{}{}
			}
			for _, quotaType := range model.GetModelQuotaTypes(routeModel) {
				quotaSet[quotaType] = struct{}{}
			}
		}

		channelsByModel, _ := model.GetBoundChannelsByModelsMap(mm.AutoRouteModels)
		for _, routeModel := range mm.AutoRouteModels {
			for _, ch := range channelsByModel[routeModel] {
				key := fmt.Sprintf("%s_%d", ch.Name, ch.Type)
				channelSet[key] = ch
			}
		}

		if len(endpointSet) > 0 {
			eps := make([]constant.EndpointType, 0, len(endpointSet))
			for endpointType := range endpointSet {
				eps = append(eps, endpointType)
			}
			if b, err := json.Marshal(eps); err == nil {
				mm.Endpoints = string(b)
			}
		}
		if len(groupSet) > 0 {
			groups := make([]string, 0, len(groupSet))
			for group := range groupSet {
				groups = append(groups, group)
			}
			sort.Strings(groups)
			mm.EnableGroups = groups
		}
		if len(quotaSet) > 0 {
			quotaTypes := make([]int, 0, len(quotaSet))
			for quotaType := range quotaSet {
				quotaTypes = append(quotaTypes, quotaType)
			}
			sort.Ints(quotaTypes)
			mm.QuotaTypes = quotaTypes
		}
		if len(channelSet) > 0 {
			channels := make([]model.BoundChannel, 0, len(channelSet))
			for _, ch := range channelSet {
				channels = append(channels, ch)
			}
			mm.BoundChannels = channels
		}
	}

	// 2) 批量查询精确模型的绑定渠道
	channelsByModel, _ := model.GetBoundChannelsByModelsMap(exactNames)

	// 3) 精确模型：端点从缓存、渠道批量映射、分组/计费类型从缓存
	for name, indices := range exactIdx {
		chs := channelsByModel[name]
		for _, idx := range indices {
			mm := models[idx]
			if mm.Endpoints == "" {
				eps := model.GetModelSupportEndpointTypes(mm.ModelName)
				if b, err := json.Marshal(eps); err == nil {
					mm.Endpoints = string(b)
				}
			}
			mm.BoundChannels = chs
			mm.EnableGroups = model.GetModelEnableGroups(mm.ModelName)
			mm.QuotaTypes = model.GetModelQuotaTypes(mm.ModelName)
		}
	}

	if len(ruleIndices) == 0 {
		return
	}

	// 4) 一次性读取定价缓存，内存匹配所有规则模型
	pricings := model.GetPricing()

	// 为全部规则模型收集匹配名集合、端点并集、分组并集、配额集合
	matchedNamesByIdx := make(map[int][]string)
	endpointSetByIdx := make(map[int]map[constant.EndpointType]struct{})
	groupSetByIdx := make(map[int]map[string]struct{})
	quotaSetByIdx := make(map[int]map[int]struct{})

	for _, p := range pricings {
		for _, idx := range ruleIndices {
			mm := models[idx]
			var matched bool
			switch mm.NameRule {
			case model.NameRulePrefix:
				matched = strings.HasPrefix(p.ModelName, mm.ModelName)
			case model.NameRuleSuffix:
				matched = strings.HasSuffix(p.ModelName, mm.ModelName)
			case model.NameRuleContains:
				matched = strings.Contains(p.ModelName, mm.ModelName)
			}
			if !matched {
				continue
			}
			matchedNamesByIdx[idx] = append(matchedNamesByIdx[idx], p.ModelName)

			es := endpointSetByIdx[idx]
			if es == nil {
				es = make(map[constant.EndpointType]struct{})
				endpointSetByIdx[idx] = es
			}
			for _, et := range p.SupportedEndpointTypes {
				es[et] = struct{}{}
			}

			gs := groupSetByIdx[idx]
			if gs == nil {
				gs = make(map[string]struct{})
				groupSetByIdx[idx] = gs
			}
			for _, g := range p.EnableGroup {
				gs[g] = struct{}{}
			}

			qs := quotaSetByIdx[idx]
			if qs == nil {
				qs = make(map[int]struct{})
				quotaSetByIdx[idx] = qs
			}
			qs[p.QuotaType] = struct{}{}
		}
	}

	// 5) 汇总所有匹配到的模型名称，批量查询一次渠道
	allMatchedSet := make(map[string]struct{})
	for _, names := range matchedNamesByIdx {
		for _, n := range names {
			allMatchedSet[n] = struct{}{}
		}
	}
	allMatched := make([]string, 0, len(allMatchedSet))
	for n := range allMatchedSet {
		allMatched = append(allMatched, n)
	}
	matchedChannelsByModel, _ := model.GetBoundChannelsByModelsMap(allMatched)

	// 6) 回填每个规则模型的并集信息
	for _, idx := range ruleIndices {
		mm := models[idx]

		// 端点并集 -> 序列化
		if es, ok := endpointSetByIdx[idx]; ok && mm.Endpoints == "" {
			eps := make([]constant.EndpointType, 0, len(es))
			for et := range es {
				eps = append(eps, et)
			}
			if b, err := json.Marshal(eps); err == nil {
				mm.Endpoints = string(b)
			}
		}

		// 分组并集
		if gs, ok := groupSetByIdx[idx]; ok {
			groups := make([]string, 0, len(gs))
			for g := range gs {
				groups = append(groups, g)
			}
			mm.EnableGroups = groups
		}

		// 配额类型集合（保持去重并排序）
		if qs, ok := quotaSetByIdx[idx]; ok {
			arr := make([]int, 0, len(qs))
			for k := range qs {
				arr = append(arr, k)
			}
			sort.Ints(arr)
			mm.QuotaTypes = arr
		}

		// 渠道并集
		names := matchedNamesByIdx[idx]
		channelSet := make(map[string]model.BoundChannel)
		for _, n := range names {
			for _, ch := range matchedChannelsByModel[n] {
				key := ch.Name + "_" + strconv.Itoa(ch.Type)
				channelSet[key] = ch
			}
		}
		if len(channelSet) > 0 {
			chs := make([]model.BoundChannel, 0, len(channelSet))
			for _, ch := range channelSet {
				chs = append(chs, ch)
			}
			mm.BoundChannels = chs
		}

		// 匹配信息
		mm.MatchedModels = names
		mm.MatchedCount = len(names)
	}
}
