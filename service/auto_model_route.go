package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const autoRoutePerfHours = 24

type AutoRouteTarget struct {
	IsAuto           bool
	DisplayModelName string
	RoutedModelName  string
}

type autoRoutePerf struct {
	hasMetrics   bool
	avgLatencyMs int64
	successRate  float64
	avgTps       float64
}

func ResolveAutoRouteTarget(c *gin.Context, requestedModel string) (*AutoRouteTarget, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return &AutoRouteTarget{}, nil
	}

	lookupModel, wantsCompact := splitAutoLookupModel(requestedModel)
	if !strings.EqualFold(lookupModel, "Auto") {
		return &AutoRouteTarget{
			DisplayModelName: requestedModel,
			RoutedModelName:  requestedModel,
		}, nil
	}

	autoModel, err := model.GetModelByName("Auto")
	if err != nil {
		return nil, fmt.Errorf("Auto 模型未配置或已被删除")
	}
	if !autoModel.IsAutoModel() || autoModel.Status != 1 {
		return nil, fmt.Errorf("Auto 模型未启用")
	}
	if len(autoModel.AutoRouteModels) == 0 {
		return nil, fmt.Errorf("Auto 模型未配置可路由模型")
	}

	routedModelName, err := selectAutoRouteModel(c, autoModel.AutoRouteModels, wantsCompact)
	if err != nil {
		return nil, err
	}

	return &AutoRouteTarget{
		IsAuto:           true,
		DisplayModelName: "Auto",
		RoutedModelName:  routedModelName,
	}, nil
}

func splitAutoLookupModel(modelName string) (lookupModel string, wantsCompact bool) {
	modelName = strings.TrimSpace(modelName)
	if strings.HasSuffix(modelName, ratio_setting.CompactModelSuffix) {
		return strings.TrimSuffix(modelName, ratio_setting.CompactModelSuffix), true
	}
	return modelName, false
}

func selectAutoRouteModel(c *gin.Context, routeModels []string, wantsCompact bool) (string, error) {
	eligibleGroups := getAutoRouteEligibleGroups(c)
	type candidate struct {
		modelName string
		order     int
		perf      autoRoutePerf
	}

	candidates := make([]candidate, 0, len(routeModels))
	for i, routeModel := range routeModels {
		routeModel = strings.TrimSpace(routeModel)
		if routeModel == "" {
			continue
		}
		if wantsCompact {
			routeModel = ratio_setting.WithCompactModelSuffix(routeModel)
		}
		if !isAutoRouteModelAvailable(routeModel, eligibleGroups) {
			continue
		}
		candidates = append(candidates, candidate{
			modelName: routeModel,
			order:     i,
			perf:      queryAutoRoutePerf(routeModel, eligibleGroups),
		})
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("Auto 模型没有可用的路由目标")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.perf.hasMetrics != right.perf.hasMetrics {
			return left.perf.hasMetrics
		}
		if left.perf.hasMetrics && right.perf.hasMetrics {
			if left.perf.avgLatencyMs != right.perf.avgLatencyMs {
				return left.perf.avgLatencyMs < right.perf.avgLatencyMs
			}
			if left.perf.successRate != right.perf.successRate {
				return left.perf.successRate > right.perf.successRate
			}
			if left.perf.avgTps != right.perf.avgTps {
				return left.perf.avgTps > right.perf.avgTps
			}
		}
		return left.order < right.order
	})

	return candidates[0].modelName, nil
}

func getAutoRouteEligibleGroups(c *gin.Context) []string {
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if usingGroup == "" {
		usingGroup = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}
	if usingGroup == "auto" {
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		return GetUserAutoGroup(userGroup)
	}
	if usingGroup == "" {
		return []string{}
	}
	return []string{usingGroup}
}

func isAutoRouteModelAvailable(modelName string, eligibleGroups []string) bool {
	modelGroups := model.GetModelEnableGroups(modelName)
	if len(modelGroups) == 0 {
		return false
	}
	if len(eligibleGroups) == 0 {
		return true
	}
	for _, group := range eligibleGroups {
		if common.StringsContains(modelGroups, group) {
			return true
		}
	}
	return false
}

func queryAutoRoutePerf(modelName string, eligibleGroups []string) autoRoutePerf {
	best := autoRoutePerf{}
	for _, group := range eligibleGroups {
		if !common.StringsContains(model.GetModelEnableGroups(modelName), group) {
			continue
		}
		result, err := perfmetrics.Query(perfmetrics.QueryParams{
			Model: modelName,
			Group: group,
			Hours: autoRoutePerfHours,
		})
		if err != nil || len(result.Groups) == 0 {
			continue
		}
		groupResult := result.Groups[0]
		if len(groupResult.Series) == 0 {
			continue
		}
		current := autoRoutePerf{
			hasMetrics:   true,
			avgLatencyMs: groupResult.AvgLatencyMs,
			successRate:  groupResult.SuccessRate,
			avgTps:       groupResult.AvgTps,
		}
		if !best.hasMetrics ||
			current.avgLatencyMs < best.avgLatencyMs ||
			(current.avgLatencyMs == best.avgLatencyMs && current.successRate > best.successRate) ||
			(current.avgLatencyMs == best.avgLatencyMs && current.successRate == best.successRate && current.avgTps > best.avgTps) {
			best = current
		}
	}
	return best
}
