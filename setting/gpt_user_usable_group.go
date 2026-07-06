package setting

import (
	"encoding/json"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// gptUserUsableGroups 存储 GPT 专有分组的描述信息
// 仅 GPT 模式用户在编辑密钥时可见，与普通 userUsableGroups 隔离
var gptUserUsableGroups = map[string]string{}
var gptUserUsableGroupsMutex sync.RWMutex

// GetGptUserUsableGroupsCopy 返回 GPT 用户可选分组的拷贝
func GetGptUserUsableGroupsCopy() map[string]string {
	gptUserUsableGroupsMutex.RLock()
	defer gptUserUsableGroupsMutex.RUnlock()

	copyGptUserUsableGroups := make(map[string]string)
	for k, v := range gptUserUsableGroups {
		copyGptUserUsableGroups[k] = v
	}
	return copyGptUserUsableGroups
}

// GptUserUsableGroups2JSONString 将 GPT 用户可选分组序列化为 JSON 字符串
func GptUserUsableGroups2JSONString() string {
	gptUserUsableGroupsMutex.RLock()
	defer gptUserUsableGroupsMutex.RUnlock()

	jsonBytes, err := json.Marshal(gptUserUsableGroups)
	if err != nil {
		common.SysLog("error marshalling gpt user groups: " + err.Error())
	}
	return string(jsonBytes)
}

// UpdateGptUserUsableGroupsByJSONString 通过 JSON 字符串更新 GPT 用户可选分组
func UpdateGptUserUsableGroupsByJSONString(jsonStr string) error {
	gptUserUsableGroupsMutex.Lock()
	defer gptUserUsableGroupsMutex.Unlock()

	gptUserUsableGroups = make(map[string]string)
	return json.Unmarshal([]byte(jsonStr), &gptUserUsableGroups)
}

// GetGptUsableGroupDescription 获取指定 GPT 专有分组的描述
// 若不存在则返回分组名称本身
func GetGptUsableGroupDescription(groupName string) string {
	gptUserUsableGroupsMutex.RLock()
	defer gptUserUsableGroupsMutex.RUnlock()

	if desc, ok := gptUserUsableGroups[groupName]; ok {
		return desc
	}
	return groupName
}
