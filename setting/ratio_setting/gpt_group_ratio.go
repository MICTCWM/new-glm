package ratio_setting

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// gptGroupRatioMap 存储 GPT 专有分组倍率
// 仅 GPT 模式用户可使用这些分组，与普通分组隔离
// 注意：不注册到 config.GlobalConfig，采用扁平 key 模式（与 GroupRatio 一致）
var gptGroupRatioMap = types.NewRWMap[string, float64]()

// GetGptGroupRatioCopy 返回 GPT 分组倍率的拷贝
func GetGptGroupRatioCopy() map[string]float64 {
	return gptGroupRatioMap.ReadAll()
}

// ContainsGptGroupRatio 判断指定分组是否为 GPT 专有分组
func ContainsGptGroupRatio(name string) bool {
	_, ok := gptGroupRatioMap.Get(name)
	return ok
}

// GptGroupRatio2JSONString 将 GPT 分组倍率序列化为 JSON 字符串
func GptGroupRatio2JSONString() string {
	return gptGroupRatioMap.MarshalJSONString()
}

// UpdateGptGroupRatioByJSONString 通过 JSON 字符串更新 GPT 分组倍率
func UpdateGptGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(gptGroupRatioMap, jsonStr)
}

// GetGptGroupRatio 获取指定 GPT 专有分组的倍率
// 若不存在则记录日志并返回 1
func GetGptGroupRatio(name string) float64 {
	ratio, ok := gptGroupRatioMap.Get(name)
	if !ok {
		common.SysLog("gpt group ratio not found: " + name)
		return 1
	}
	return ratio
}

// CheckGptGroupRatio 校验 GPT 分组倍率 JSON 字符串
// 要求所有倍率值不小于 0
func CheckGptGroupRatio(jsonStr string) error {
	checkGptGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGptGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGptGroupRatio {
		if ratio < 0 {
			return errors.New("gpt group ratio must be not less than 0: " + name)
		}
	}
	return nil
}
