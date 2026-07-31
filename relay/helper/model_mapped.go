package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}
	// A retry can switch between channels with different mappings. Do not carry
	// the previous channel's mapping-specific reasoning level into this request.
	info.MappedReasoningEffort = ""

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	originModelName := info.OriginModelName
	mappingModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]json.RawMessage)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := mappingModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			if rawMappedModel, exists := modelMap[currentModel]; exists {
				mappedModel, reasoningEffort, parseErr := parseModelMappingTarget(rawMappedModel)
				if parseErr != nil {
					return parseErr
				}
				if mappedModel == "" {
					break
				}
				if reasoningEffort != "" {
					info.MappedReasoningEffort = reasoningEffort
				}
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							return nil
						} else {
							info.IsModelMapped = true
							break
						}
					}
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.OriginModelName = ratio_setting.WithCompactModelSuffix(finalUpstreamModelName)
	}
	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}

// parseModelMappingTarget accepts the legacy string target and the extended
// object form used by emergency mappings:
// {"model":"gpt-5","reasoning_effort":"high"}
func parseModelMappingTarget(raw json.RawMessage) (string, string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", "", nil
	}

	var stringTarget string
	if err := json.Unmarshal(raw, &stringTarget); err == nil {
		return stringTarget, "", nil
	}

	var objectTarget struct {
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
	}
	if err := json.Unmarshal(raw, &objectTarget); err != nil || strings.TrimSpace(objectTarget.Model) == "" {
		return "", "", fmt.Errorf("model_mapping_target_invalid")
	}

	effort := strings.ToLower(strings.TrimSpace(objectTarget.ReasoningEffort))
	if effort == "inherit" {
		effort = ""
	} else if effort != "" {
		effort = common.NormalizeFallbackReasoningEffort(effort)
		if effort == "" {
			return "", "", fmt.Errorf("model_mapping_reasoning_effort_invalid")
		}
	}
	return strings.TrimSpace(objectTarget.Model), effort, nil
}
