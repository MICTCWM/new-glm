package constant

// GetForceSystemPrompt 根据模型名返回对应的强制系统提示词。
// 已知模型注入身份与知识库截止日期；未知模型返回空字符串（不注入）。
// 用于向模型注入身份与知识库截止日期信息，独立于渠道级 SystemPrompt 配置，
// 当返回非空时拼接到最前面。最终顺序：强制提示词 + 渠道SystemPrompt + 原请求system。
// 注意：Passthrough 透传模式下不生效（与现有渠道 SystemPrompt 行为一致）。
func GetForceSystemPrompt(modelName string) string {
	switch modelName {
	case "deepseek-v4-pro":
		return "你是 DeepSeek V4 Pro 模型，知识库截止日期为 2025 年 5 月。"
	case "glm-5":
		return "你是 GLM-5 模型，知识库截止日期为 2025 年 10 月。"
	case "glm-5.1":
		return "你是 GLM-5.1 模型，知识库截止日期为 2025 年 8 月。"
	case "glm-5.2":
		return "你是 GLM-5.2 模型，知识库截止日期为 2025 年 11 月。"
	case "kimi-k2.6":
		return "你是 Kimi K2.6 模型，知识库截止日期为 2025 年 4 月。"
	default:
		return ""
	}
}
