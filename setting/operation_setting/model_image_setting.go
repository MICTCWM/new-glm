package operation_setting

import "strings"

// ModelNoImageModels 不支持图片的模型名称列表。
// 默认所有模型都支持图片，在此列表中的模型如果请求中包含图片，
// 会在 relay 阶段被拒绝并返回"模型不支持图片输入"错误。
var ModelNoImageModels = []string{}

func ModelNoImageModelsToString() string {
	return strings.Join(ModelNoImageModels, "\n")
}

func ModelNoImageModelsFromString(s string) {
	ModelNoImageModels = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		if k != "" {
			ModelNoImageModels = append(ModelNoImageModels, k)
		}
	}
}

// IsModelNoImage 判断指定模型是否在不支持图片的列表中。
func IsModelNoImage(modelName string) bool {
	if modelName == "" {
		return false
	}
	for _, m := range ModelNoImageModels {
		if m == modelName {
			return true
		}
	}
	return false
}