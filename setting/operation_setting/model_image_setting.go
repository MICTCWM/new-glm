package operation_setting

import (
	"encoding/json"
	"strings"
)

// ModelNoImageModels 不支持视觉/图片的模型名称列表。
// 默认所有模型都支持视觉；在此列表中的模型如果请求中包含图片，
// 会触发视觉路由：把图片交给 VisionRouteModels 中的视觉模型描述后缓存，
// 再将图片替换为文字描述转发给上游。
var ModelNoImageModels = []string{}

// VisionRouteModels 可被路由到的视觉模型名称列表。
// 当一个"不支持视觉的模型"收到含图片的请求时，系统会从该列表中挑选
// 一个视觉模型描述图片，并将描述结果缓存以便后续命中复用。
var VisionRouteModels = []string{}

func ModelNoImageModelsToString() string {
	return modelsToJSONString(ModelNoImageModels)
}

func VisionRouteModelsToString() string {
	return modelsToJSONString(VisionRouteModels)
}

func ModelNoImageModelsFromString(s string) {
	ModelNoImageModels = parseModelList(s)
}

func VisionRouteModelsFromString(s string) {
	VisionRouteModels = parseModelList(s)
}

func modelsToJSONString(models []string) string {
	if models == nil {
		models = []string{}
	}
	b, err := json.Marshal(models)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// parseModelList 解析模型列表：优先按 JSON 数组解析（前端多选组件写入），
// 兼容旧的按换行分隔的格式。
func parseModelList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		return normalizeModelList(arr)
	}
	return normalizeModelList(strings.Split(s, "\n"))
}

func normalizeModelList(items []string) []string {
	result := make([]string, 0, len(items))
	for _, v := range items {
		v = strings.TrimSpace(v)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

// IsModelNoImage 判断指定模型是否在不支持视觉/图片的列表中。
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
