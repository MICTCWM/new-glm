package constant

// ForceSystemPrompt 是强制拼接到所有请求 system message 最前面的系统提示词。
// 用于向模型注入身份与知识库截止日期信息，独立于渠道级 SystemPrompt 配置，
// 总是生效且拼接到最前面。最终顺序：强制提示词 + 渠道SystemPrompt + 原请求system。
// 注意：Passthrough 透传模式下不生效（与现有渠道 SystemPrompt 行为一致）。
const ForceSystemPrompt = "你是 GLM-5.2 模型，知识库截止日期为 2025 年 11 月。"
