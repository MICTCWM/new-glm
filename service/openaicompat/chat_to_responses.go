package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

func normalizeChatImageURLToString(v any) any {
	switch vv := v.(type) {
	case string:
		return vv
	case map[string]any:
		if url := common.Interface2String(vv["url"]); url != "" {
			return url
		}
		return v
	case dto.MessageImageUrl:
		if vv.Url != "" {
			return vv.Url
		}
		return v
	case *dto.MessageImageUrl:
		if vv != nil && vv.Url != "" {
			return vv.Url
		}
		return v
	default:
		return v
	}
}

func convertChatResponseFormatToResponsesText(reqFormat *dto.ResponseFormat) json.RawMessage {
	if reqFormat == nil || strings.TrimSpace(reqFormat.Type) == "" {
		return nil
	}

	format := map[string]any{
		"type": reqFormat.Type,
	}

	if reqFormat.Type == "json_schema" && len(reqFormat.JsonSchema) > 0 {
		var chatSchema map[string]any
		if err := common.Unmarshal(reqFormat.JsonSchema, &chatSchema); err == nil {
			for key, value := range chatSchema {
				if key == "type" {
					continue
				}
				format[key] = value
			}

			if nested, ok := format["json_schema"].(map[string]any); ok {
				for key, value := range nested {
					if _, exists := format[key]; !exists {
						format[key] = value
					}
				}
				delete(format, "json_schema")
			}
		} else {
			format["json_schema"] = reqFormat.JsonSchema
		}
	}

	textRaw, _ := common.Marshal(map[string]any{
		"format": format,
	})
	return textRaw
}

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	if lo.FromPtrOr(req.N, 1) > 1 {
		return nil, fmt.Errorf("n>1 is not supported in responses compatibility mode")
	}

	var instructionsParts []string
	inputItems := make([]map[string]any, 0, len(req.Messages))

	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}

		if role == "tool" || role == "function" {
			callID := strings.TrimSpace(msg.ToolCallId)

			var output any
			if msg.Content == nil {
				output = ""
			} else if msg.IsStringContent() {
				output = msg.StringContent()
			} else {
				if b, err := common.Marshal(msg.Content); err == nil {
					output = string(b)
				} else {
					output = fmt.Sprintf("%v", msg.Content)
				}
			}

			if callID == "" {
				inputItems = append(inputItems, map[string]any{
					"role":    "user",
					"content": fmt.Sprintf("[tool_output_missing_call_id] %v", output),
				})
				continue
			}

			inputItems = append(inputItems, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  output,
			})
			continue
		}

		// Prefer mapping system/developer messages into `instructions`.
		if role == "system" || role == "developer" {
			if msg.Content == nil {
				continue
			}
			if msg.IsStringContent() {
				if s := strings.TrimSpace(msg.StringContent()); "" != s {
					instructionsParts = append(instructionsParts, s)
				}
				continue
			}
			parts := msg.ParseContent()
			// 若 system 携带 cache_control 断点，则保留为结构化 input item
			// （instructions 是单一字符串，无法承载 cache_control），以便上游命中缓存。
			hasCache := false
			for _, part := range parts {
				if len(part.CacheControl) > 0 {
					hasCache = true
					break
				}
			}
			if hasCache {
				sysParts := make([]map[string]any, 0, len(parts))
				for _, part := range parts {
					if part.Type != dto.ContentTypeText {
						continue
					}
					p := map[string]any{
						"type": "input_text",
						"text": part.Text,
					}
					if len(part.CacheControl) > 0 {
						var cc map[string]any
						if err := common.Unmarshal(part.CacheControl, &cc); err == nil {
							p["cache_control"] = cc
						} else {
							p["cache_control"] = part.CacheControl
						}
					}
					sysParts = append(sysParts, p)
				}
				// 无文本 part 时（极端情况）回退到 instructions，避免 system 内容丢失
				if len(sysParts) == 0 {
					var sb strings.Builder
					for _, part := range parts {
						if part.Type == dto.ContentTypeText && strings.TrimSpace(part.Text) != "" {
							if sb.Len() > 0 {
								sb.WriteString("\n")
							}
							sb.WriteString(part.Text)
						}
					}
					if s := strings.TrimSpace(sb.String()); s != "" {
						instructionsParts = append(instructionsParts, s)
					}
					continue
				}
				// 按原顺序追加到末尾（不前置），以保持多 system 消息的相对顺序与缓存前缀稳定。
				// developer role 归一化为 system，避免部分 Claude 中转/兼容网关不识别 developer。
				inputItems = append(inputItems, map[string]any{
					"role":    "system",
					"content": sysParts,
				})
				continue
			}
			var sb strings.Builder
			for _, part := range parts {
				if part.Type == dto.ContentTypeText && strings.TrimSpace(part.Text) != "" {
					if sb.Len() > 0 {
						sb.WriteString("\n")
					}
					sb.WriteString(part.Text)
				}
			}
			if s := strings.TrimSpace(sb.String()); s != "" {
				instructionsParts = append(instructionsParts, s)
			}
			continue
		}

		item := map[string]any{
			"role": role,
		}

		if msg.Content == nil {
			item["content"] = ""
			inputItems = append(inputItems, item)

			if role == "assistant" {
				for _, tc := range msg.ParseToolCalls() {
					if strings.TrimSpace(tc.ID) == "" {
						continue
					}
					if tc.Type != "" && tc.Type != "function" {
						continue
					}
					name := strings.TrimSpace(tc.Function.Name)
					if name == "" {
						continue
					}
					inputItems = append(inputItems, map[string]any{
						"type":      "function_call",
						"call_id":   tc.ID,
						"name":      name,
						"arguments": tc.Function.Arguments,
					})
				}
			}
			continue
		}

		if msg.IsStringContent() {
			item["content"] = msg.StringContent()
			inputItems = append(inputItems, item)

			if role == "assistant" {
				for _, tc := range msg.ParseToolCalls() {
					if strings.TrimSpace(tc.ID) == "" {
						continue
					}
					if tc.Type != "" && tc.Type != "function" {
						continue
					}
					name := strings.TrimSpace(tc.Function.Name)
					if name == "" {
						continue
					}
					inputItems = append(inputItems, map[string]any{
						"type":      "function_call",
						"call_id":   tc.ID,
						"name":      name,
						"arguments": tc.Function.Arguments,
					})
				}
			}
			continue
		}

		parts := msg.ParseContent()
		contentParts := make([]map[string]any, 0, len(parts))
		for _, part := range parts {
			var part_ map[string]any
			switch part.Type {
			case dto.ContentTypeText:
				textType := "input_text"
				if role == "assistant" {
					textType = "output_text"
				}
				part_ = map[string]any{
					"type": textType,
					"text": part.Text,
				}
			case dto.ContentTypeImageURL:
				part_ = map[string]any{
					"type":      "input_image",
					"image_url": normalizeChatImageURLToString(part.ImageUrl),
				}
				if image := part.GetImageMedia(); image != nil && image.Detail != "" {
					part_["detail"] = image.Detail
				}
			case dto.ContentTypeInputAudio:
				part_ = map[string]any{
					"type":        "input_audio",
					"input_audio": part.InputAudio,
				}
			case dto.ContentTypeFile:
				part_ = map[string]any{"type": "input_file"}
				if file := part.GetFile(); file != nil {
					if file.FileId != "" {
						part_["file_id"] = file.FileId
					}
					if file.FileData != "" {
						part_["file_data"] = file.FileData
					}
					if file.FileName != "" {
						part_["filename"] = file.FileName
					}
				} else if part.File != nil {
					// Preserve custom file fields for compatible providers.
					part_["file"] = part.File
				}
			case dto.ContentTypeVideoUrl:
				part_ = map[string]any{
					"type":      "input_video",
					"video_url": part.VideoUrl,
				}
			default:
				part_ = map[string]any{
					"type": part.Type,
				}
			}
			// 透传 cache_control，使上游（OpenAI Responses / Claude 中转等）能命中 prompt 缓存
			if len(part.CacheControl) > 0 {
				var cc map[string]any
				if err := common.Unmarshal(part.CacheControl, &cc); err == nil {
					part_["cache_control"] = cc
				} else {
					part_["cache_control"] = part.CacheControl
				}
			}
			contentParts = append(contentParts, part_)
		}
		item["content"] = contentParts
		inputItems = append(inputItems, item)

		if role == "assistant" {
			for _, tc := range msg.ParseToolCalls() {
				if strings.TrimSpace(tc.ID) == "" {
					continue
				}
				if tc.Type != "" && tc.Type != "function" {
					continue
				}
				name := strings.TrimSpace(tc.Function.Name)
				if name == "" {
					continue
				}
				inputItems = append(inputItems, map[string]any{
					"type":      "function_call",
					"call_id":   tc.ID,
					"name":      name,
					"arguments": tc.Function.Arguments,
				})
			}
		}
	}

	inputRaw, err := common.Marshal(inputItems)
	if err != nil {
		return nil, err
	}

	var instructionsRaw json.RawMessage
	if len(instructionsParts) > 0 {
		instructions := strings.Join(instructionsParts, "\n\n")
		instructionsRaw, _ = common.Marshal(instructions)
	}

	var toolsRaw json.RawMessage
	if req.Tools != nil {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			switch tool.Type {
			case "function":
				// Build the function tool map conditionally so we respect the
				// `omitempty` semantics of dto.FunctionRequest. The previous
				// implementation always emitted `parameters` and `description`,
				// which produced `"parameters": null` when the caller omitted the
				// field. Upstream Responses APIs reject that with errors such as
				// "null is not of type array" (the meta-schema validates array
				// sub-fields like `required` against the null object).
				functionTool := map[string]any{
					"type": "function",
					"name": tool.Function.Name,
				}
				if tool.Function.Description != "" {
					functionTool["description"] = tool.Function.Description
				}
				if tool.Function.Parameters != nil {
					functionTool["parameters"] = sanitizeFunctionParameters(tool.Function.Parameters)
				}
				if len(tool.Function.Strict) > 0 {
					var strict any
					if err := common.Unmarshal(tool.Function.Strict, &strict); err == nil {
						functionTool["strict"] = strict
					}
				}
				if len(tool.CacheControl) > 0 {
					var cacheControl any
					if err := common.Unmarshal(tool.CacheControl, &cacheControl); err == nil {
						functionTool["cache_control"] = cacheControl
					} else {
						functionTool["cache_control"] = tool.CacheControl
					}
				}
				tools = append(tools, functionTool)
			default:
				// Best-effort: keep original tool shape for unknown types.
				var m map[string]any
				if b, err := common.Marshal(tool); err == nil {
					_ = common.Unmarshal(b, &m)
				}
				if len(m) == 0 {
					m = map[string]any{"type": tool.Type}
				}
				tools = append(tools, m)
			}
		}
		toolsRaw, _ = common.Marshal(tools)
	}

	var toolChoiceRaw json.RawMessage
	if req.ToolChoice != nil {
		switch v := req.ToolChoice.(type) {
		case string:
			toolChoiceRaw, _ = common.Marshal(v)
		default:
			var m map[string]any
			if b, err := common.Marshal(v); err == nil {
				_ = common.Unmarshal(b, &m)
			}
			if m == nil {
				toolChoiceRaw, _ = common.Marshal(v)
			} else if t, _ := m["type"].(string); t == "function" {
				// Chat: {"type":"function","function":{"name":"..."}}
				// Responses: {"type":"function","name":"..."}
				if name, ok := m["name"].(string); ok && name != "" {
					toolChoiceRaw, _ = common.Marshal(map[string]any{
						"type": "function",
						"name": name,
					})
				} else if fn, ok := m["function"].(map[string]any); ok {
					if name, ok := fn["name"].(string); ok && name != "" {
						toolChoiceRaw, _ = common.Marshal(map[string]any{
							"type": "function",
							"name": name,
						})
					} else {
						toolChoiceRaw, _ = common.Marshal(v)
					}
				} else {
					toolChoiceRaw, _ = common.Marshal(v)
				}
			} else {
				toolChoiceRaw, _ = common.Marshal(v)
			}
		}
	}

	var parallelToolCallsRaw json.RawMessage
	if req.ParallelTooCalls != nil {
		parallelToolCallsRaw, _ = common.Marshal(*req.ParallelTooCalls)
	}

	textRaw := convertChatResponseFormatToResponsesText(req.ResponseFormat)

	maxOutputTokens := lo.FromPtrOr(req.MaxTokens, uint(0))
	maxCompletionTokens := lo.FromPtrOr(req.MaxCompletionTokens, uint(0))
	if maxCompletionTokens > maxOutputTokens {
		maxOutputTokens = maxCompletionTokens
	}
	// OpenAI Responses API rejects max_output_tokens < 16 when explicitly provided.
	//if maxOutputTokens > 0 && maxOutputTokens < 16 {
	//	maxOutputTokens = 16
	//}

	var topP *float64
	if req.TopP != nil {
		topP = common.GetPointer(lo.FromPtr(req.TopP))
	}

	out := &dto.OpenAIResponsesRequest{
		Model:                req.Model,
		Input:                inputRaw,
		Instructions:         instructionsRaw,
		Stream:               req.Stream,
		Temperature:          req.Temperature,
		Text:                 textRaw,
		ToolChoice:           toolChoiceRaw,
		Tools:                toolsRaw,
		TopP:                 topP,
		User:                 req.User,
		ParallelToolCalls:    parallelToolCallsRaw,
		Store:                req.Store,
		Metadata:             req.Metadata,
		PromptCacheRetention: req.PromptCacheRetention,
	}
	if req.PromptCacheKey != "" {
		out.PromptCacheKey, _ = common.Marshal(req.PromptCacheKey)
	}
	if req.MaxTokens != nil || req.MaxCompletionTokens != nil {
		out.MaxOutputTokens = lo.ToPtr(maxOutputTokens)
	}

	if req.ReasoningEffort != "" {
		out.Reasoning = &dto.Reasoning{
			Effort:  req.ReasoningEffort,
			Summary: "detailed",
		}
	}

	return out, nil
}

// jsonSchemaArrayFields lists JSON Schema keywords whose value MUST be an
// array. When clients send `null` for these fields (which is invalid per the
// JSON Schema spec), upstream Responses APIs reject the entire function
// definition with errors such as "null is not of type array". We drop these
// null entries defensively so a malformed client schema does not break the
// protocol conversion.
var jsonSchemaArrayFields = map[string]struct{}{
	"required":    {},
	"enum":        {},
	"anyOf":       {},
	"oneOf":       {},
	"allOf":       {},
	"prefixItems": {},
}

// sanitizeFunctionParameters recursively cleans a JSON Schema value before it
// is forwarded to the Responses API. It drops null values for keywords that
// must be arrays (e.g. `required`, `enum`) and recurses into `properties`,
// `items`, and the composition keywords so nested schemas are cleaned too.
// Non-array null values such as `default: null` or `const: null` are
// preserved because they are valid JSON Schema.
func sanitizeFunctionParameters(params any) any {
	switch v := params.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for k, val := range v {
			if val == nil {
				if _, isArrayField := jsonSchemaArrayFields[k]; isArrayField {
					continue
				}
			}
			cleaned[k] = sanitizeFunctionParameters(val)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(v))
		for i, item := range v {
			cleaned[i] = sanitizeFunctionParameters(item)
		}
		return cleaned
	default:
		return params
	}
}
