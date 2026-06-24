package common

import (
	"strings"

	"github.com/QuantumNous/new-api/types"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OverrideResponseModel 强制将响应体中的 model/modelVersion 字段
// 覆盖为用户原始请求的 model ID。
//
// 背景：部分上游（聚合/中转）会在响应里返回它们内部的 model 名字
// （如 "GM 5025.1 deep deepseek" / "千问 GPT" 等），与用户实际请求的
// model ID 不一致。客户端依赖响应里的 model 字段做日志、计费核对、
// 路由分发，因此这里统一改写为 info.OriginModelName。
//
// 该函数在协议转换完成、序列化后、写回客户端前调用，与协议转换逻辑
// 完全解耦：
//   - OpenAI / Anthropic / OpenAI Responses 协议 → 改顶层 model
//   - Gemini 协议 → 改 modelVersion
//   - OpenAI Responses 流式事件（如 response.completed）→ 改嵌套
//     response.model（其他不包含 response.model 的 chunk 自动跳过）
//
// 入参 data 通常为完整 JSON 字节，函数在原 bytes 上做最小修改并返回
// 新切片；如无需修改则原样返回。
func OverrideResponseModel(data []byte, info *RelayInfo) []byte {
	if info == nil || len(data) == 0 {
		return data
	}
	target := strings.TrimSpace(info.OriginModelName)
	if target == "" {
		return data
	}

	path, ok := modelFieldPath(info.RelayFormat, false)
	if !ok {
		return data
	}

	if gjson.GetBytes(data, path).String() == target {
		return data
	}

	out, err := sjson.SetBytes(data, path, target)
	if err != nil {
		return data
	}
	return out
}

// OverrideStreamChunkModel 对流式 chunk（单个 SSE data 行内容）做
// 同样的覆盖。流式 chunk 通常形如 {"id":...,"model":"xxx",...}。
func OverrideStreamChunkModel(data string, info *RelayInfo) string {
	if info == nil || data == "" {
		return data
	}
	target := strings.TrimSpace(info.OriginModelName)
	if target == "" {
		return data
	}

	path, ok := modelFieldPath(info.RelayFormat, true)
	if !ok {
		return data
	}

	if gjson.Get(data, path).String() == target {
		return data
	}

	out, err := sjson.Set(data, path, target)
	if err != nil {
		return data
	}
	return out
}

// modelFieldPath 根据客户端的 RelayFormat 决定 model 字段在 JSON 中
// 的路径。非流式与流式在 OpenAIResponses 下路径不同（流式事件
// response.completed 等的 model 嵌套在 response.model 中）。
func modelFieldPath(format types.RelayFormat, isStream bool) (string, bool) {
	switch format {
	case types.RelayFormatOpenAI,
		types.RelayFormatClaude,
		types.RelayFormatOpenAIAudio,
		types.RelayFormatEmbedding,
		types.RelayFormatRerank,
		types.RelayFormatOpenAIImage,
		types.RelayFormatOpenAIRealtime,
		types.RelayFormatOpenAIResponsesCompaction:
		return "model", true
	case types.RelayFormatGemini:
		return "modelVersion", true
	case types.RelayFormatOpenAIResponses:
		if isStream {
			return "response.model", true
		}
		return "model", true
	default:
		return "", false
	}
}
