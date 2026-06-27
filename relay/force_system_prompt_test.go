package relay

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

// TestForceSystemPrompt_NilRequest 验证传入 nil 请求不会 panic。
func TestForceSystemPrompt_NilRequest(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic for nil request, got: %v", r)
		}
	}()
	ApplyForceSystemPromptToMessages(nil, "glm-5.2")
}

// TestForceSystemPrompt_NoSystemMessage 验证请求中没有 system message 时，
// 应在最前面插入一条内容为对应模型强制提示词的 system message。
func TestForceSystemPrompt_NoSystemMessage(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "system" {
		t.Fatalf("expected first message role to be 'system', got %q", request.Messages[0].Role)
	}
	if request.Messages[0].StringContent() != expectedPrompt {
		t.Fatalf("expected first message content to be force prompt, got %q", request.Messages[0].StringContent())
	}
	if request.Messages[1].Role != "user" {
		t.Fatalf("expected second message role to be 'user', got %q", request.Messages[1].Role)
	}
	if request.Messages[1].StringContent() != "你好" {
		t.Fatalf("expected second message content to be '你好', got %q", request.Messages[1].StringContent())
	}
}

// TestForceSystemPrompt_StringContent 验证已有 string content 的 system message
// 会被拼接为强制提示词 + "\n" + 原内容。
func TestForceSystemPrompt_StringContent(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "原系统提示"},
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "system" {
		t.Fatalf("expected first message role to be 'system', got %q", request.Messages[0].Role)
	}
	expected := expectedPrompt + "\n" + "原系统提示"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected system content %q, got %q", expected, request.Messages[0].StringContent())
	}
	if !request.Messages[0].IsStringContent() {
		t.Fatalf("expected system message to remain string content after prepend")
	}
}

// TestForceSystemPrompt_MediaContent 验证已有 media content 数组的 system message
// 会在数组头部插入 type=text、text=强制提示词的内容项。
func TestForceSystemPrompt_MediaContent(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{
				Role: "system",
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "原系统提示",
					},
				},
			},
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	contents, ok := request.Messages[0].Content.([]dto.MediaContent)
	if !ok {
		t.Fatalf("expected Content to be []dto.MediaContent after prepend, got %T", request.Messages[0].Content)
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 media contents, got %d", len(contents))
	}
	if contents[0].Type != dto.ContentTypeText {
		t.Fatalf("expected first content type to be %q, got %q", dto.ContentTypeText, contents[0].Type)
	}
	if contents[0].Text != expectedPrompt {
		t.Fatalf("expected first content text to be force prompt, got %q", contents[0].Text)
	}
	if contents[1].Type != dto.ContentTypeText {
		t.Fatalf("expected second content type to be %q, got %q", dto.ContentTypeText, contents[1].Type)
	}
	if contents[1].Text != "原系统提示" {
		t.Fatalf("expected second content text to be '原系统提示', got %q", contents[1].Text)
	}
}

// TestForceSystemPrompt_DeveloperRole 验证当 model 为 gpt-5 时，
// GetSystemRoleName() 返回 "developer"，developer role 的 message 也会被正确拼接。
func TestForceSystemPrompt_DeveloperRole(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{Role: "developer", Content: "原系统提示"},
			{Role: "user", Content: "你好"},
		},
	}

	if request.GetSystemRoleName() != "developer" {
		t.Fatalf("expected system role name to be 'developer' for gpt-5, got %q", request.GetSystemRoleName())
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "developer" {
		t.Fatalf("expected first message role to be 'developer', got %q", request.Messages[0].Role)
	}
	expected := expectedPrompt + "\n" + "原系统提示"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected developer content %q, got %q", expected, request.Messages[0].StringContent())
	}
}

// TestForceSystemPrompt_DeveloperRoleO1 验证以 "o" 开头（非 o1-mini / o1-preview）的模型
// 也使用 developer role。
func TestForceSystemPrompt_DeveloperRoleO1(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "o3",
		Messages: []dto.Message{
			{Role: "developer", Content: "原系统提示"},
			{Role: "user", Content: "你好"},
		},
	}

	if request.GetSystemRoleName() != "developer" {
		t.Fatalf("expected system role name to be 'developer' for o3, got %q", request.GetSystemRoleName())
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	expected := expectedPrompt + "\n" + "原系统提示"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected developer content %q, got %q", expected, request.Messages[0].StringContent())
	}
}

// TestForceSystemPrompt_MultipleMessages 验证多 message 场景下
// 只拼接 system message，其他 message 保持不变。
func TestForceSystemPrompt_MultipleMessages(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "原系统提示"},
			{Role: "user", Content: "用户问题"},
			{Role: "assistant", Content: "助手回答"},
			{Role: "user", Content: "追问"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	if len(request.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(request.Messages))
	}
	expected := expectedPrompt + "\n" + "原系统提示"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected system content %q, got %q", expected, request.Messages[0].StringContent())
	}
	if request.Messages[1].Role != "user" || request.Messages[1].StringContent() != "用户问题" {
		t.Fatalf("expected user content unchanged, got role=%q content=%q", request.Messages[1].Role, request.Messages[1].StringContent())
	}
	if request.Messages[2].Role != "assistant" || request.Messages[2].StringContent() != "助手回答" {
		t.Fatalf("expected assistant content unchanged, got role=%q content=%q", request.Messages[2].Role, request.Messages[2].StringContent())
	}
	if request.Messages[3].Role != "user" || request.Messages[3].StringContent() != "追问" {
		t.Fatalf("expected user content unchanged, got role=%q content=%q", request.Messages[3].Role, request.Messages[3].StringContent())
	}
}

// TestForceSystemPrompt_StackingOrder 验证叠加顺序为「强制 + 渠道 + 原 system」。
// 先手动将 system 内容设置为 "渠道prompt\n原system"（模拟渠道级 SystemPrompt 已拼接完成的状态），
// 再调用 ApplyForceSystemPromptToMessages，最终 system 应为
// 强制提示词 + "\n" + "渠道prompt" + "\n" + "原system"。
func TestForceSystemPrompt_StackingOrder(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "渠道prompt\n原system"},
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	expected := expectedPrompt + "\n" + "渠道prompt\n原system"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected stacked system content %q, got %q", expected, request.Messages[0].StringContent())
	}
}

// TestForceSystemPrompt_StackingOrderMedia 验证 media content 数组形式下
// 叠加顺序：先在 system 数组前手动插入"渠道prompt"（模拟渠道逻辑），再调用拼接函数，
// 最终数组首元素应为强制提示词，第二个为"渠道prompt"，第三个为"原system"。
func TestForceSystemPrompt_StackingOrderMedia(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{
				Role: "system",
				Content: []any{
					map[string]any{"type": "text", "text": "渠道prompt"},
					map[string]any{"type": "text", "text": "原system"},
				},
			},
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	contents, ok := request.Messages[0].Content.([]dto.MediaContent)
	if !ok {
		t.Fatalf("expected Content to be []dto.MediaContent after prepend, got %T", request.Messages[0].Content)
	}
	if len(contents) != 3 {
		t.Fatalf("expected 3 media contents (force + channel + origin), got %d", len(contents))
	}
	if contents[0].Text != expectedPrompt {
		t.Fatalf("expected contents[0] to be force prompt, got %q", contents[0].Text)
	}
	if contents[1].Text != "渠道prompt" {
		t.Fatalf("expected contents[1] to be '渠道prompt', got %q", contents[1].Text)
	}
	if contents[2].Text != "原system" {
		t.Fatalf("expected contents[2] to be '原system', got %q", contents[2].Text)
	}
}

// TestForceSystemPrompt_OnlyFirstSystemHandled 验证当存在多条 system message 时，
// 只处理第一条匹配的 system message（与生产代码行为一致：匹配到后立即 return）。
func TestForceSystemPrompt_OnlyFirstSystemHandled(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "第一个system"},
			{Role: "system", Content: "第二个system"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "glm-5.2")

	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	expectedFirst := expectedPrompt + "\n" + "第一个system"
	if request.Messages[0].StringContent() != expectedFirst {
		t.Fatalf("expected first system content %q, got %q", expectedFirst, request.Messages[0].StringContent())
	}
	if request.Messages[1].StringContent() != "第二个system" {
		t.Fatalf("expected second system content unchanged, got %q", request.Messages[1].StringContent())
	}
}

// TestForceSystemPrompt_Instructions_NilRequest 验证传入 nil 请求不会 panic 且返回 nil error。
func TestForceSystemPrompt_Instructions_NilRequest(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic for nil request, got: %v", r)
		}
	}()
	if err := ApplyForceSystemPromptToInstructions(nil, "glm-5.2"); err != nil {
		t.Fatalf("expected nil error for nil request, got: %v", err)
	}
}

// TestForceSystemPrompt_Instructions_NoInstructions 验证 Instructions 为空时，
// 被设置为对应模型的强制提示词。
func TestForceSystemPrompt_Instructions_NoInstructions(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5"}

	if err := ApplyForceSystemPromptToInstructions(request, "glm-5.2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	if got != expectedPrompt {
		t.Fatalf("expected instructions to be force prompt, got %q", got)
	}
}

// TestForceSystemPrompt_Instructions_WithExisting 验证已有字符串 Instructions
// 被拼接为强制提示词 + "\n" + 原内容。
func TestForceSystemPrompt_Instructions_WithExisting(t *testing.T) {
	b, _ := json.Marshal("原系统提示")
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: b,
	}

	if err := ApplyForceSystemPromptToInstructions(request, "glm-5.2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	expected := expectedPrompt + "\n" + "原系统提示"
	if got != expected {
		t.Fatalf("expected instructions %q, got %q", expected, got)
	}
}

// TestForceSystemPrompt_Instructions_EmptyString 验证 Instructions 为空格字符串
// （TrimSpace 后为空）时，被设置为强制提示词。
func TestForceSystemPrompt_Instructions_EmptyString(t *testing.T) {
	b, _ := json.Marshal("   ")
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: b,
	}

	if err := ApplyForceSystemPromptToInstructions(request, "glm-5.2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	if got != expectedPrompt {
		t.Fatalf("expected instructions to be force prompt, got %q", got)
	}
}

// TestForceSystemPrompt_Instructions_NonString 验证 Instructions 不是简单字符串
// （而是 JSON 对象）时，直接被替换为强制提示词。
func TestForceSystemPrompt_Instructions_NonString(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: json.RawMessage(`{"foo":"bar"}`),
	}

	if err := ApplyForceSystemPromptToInstructions(request, "glm-5.2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPrompt := constant.GetForceSystemPrompt("glm-5.2")
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	if got != expectedPrompt {
		t.Fatalf("expected instructions to be force prompt, got %q", got)
	}
}

// TestForceSystemPrompt_OtherKnownModels 验证其他已知模型（glm-5、glm-5.1、deepseek-v4-pro、kimi-k2.6）
// 注入对应的强制提示词。
func TestForceSystemPrompt_OtherKnownModels(t *testing.T) {
	models := []string{"deepseek-v4-pro", "glm-5", "glm-5.1", "glm-5.2", "kimi-k2.6"}
	for _, modelName := range models {
		t.Run(modelName, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model: "gpt-4o",
				Messages: []dto.Message{
					{Role: "user", Content: "你好"},
				},
			}

			ApplyForceSystemPromptToMessages(request, modelName)

			expectedPrompt := constant.GetForceSystemPrompt(modelName)
			if expectedPrompt == "" {
				t.Fatalf("expected non-empty prompt for known model %q", modelName)
			}
			if len(request.Messages) != 2 {
				t.Fatalf("expected 2 messages, got %d", len(request.Messages))
			}
			if request.Messages[0].Role != "system" {
				t.Fatalf("expected first message role to be 'system', got %q", request.Messages[0].Role)
			}
			if request.Messages[0].StringContent() != expectedPrompt {
				t.Fatalf("expected first message content to be force prompt for %q, got %q", modelName, request.Messages[0].StringContent())
			}
		})
	}
}

// TestForceSystemPrompt_UnknownModelNoInjection 验证未知模型不注入提示词，
// messages / instructions 保持不变。
func TestForceSystemPrompt_UnknownModelNoInjection(t *testing.T) {
	unknownModels := []string{"gpt-4", "gpt-4o", "unknown-model", "claude-3-opus", ""}
	for _, modelName := range unknownModels {
		t.Run(modelName, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model: "gpt-4o",
				Messages: []dto.Message{
					{Role: "system", Content: "原系统提示"},
					{Role: "user", Content: "你好"},
				},
			}

			originalSystem := request.Messages[0].StringContent()
			ApplyForceSystemPromptToMessages(request, modelName)

			if len(request.Messages) != 2 {
				t.Fatalf("expected 2 messages (no injection), got %d", len(request.Messages))
			}
			if request.Messages[0].StringContent() != originalSystem {
				t.Fatalf("expected system content unchanged for unknown model %q, got %q", modelName, request.Messages[0].StringContent())
			}
			if request.Messages[1].StringContent() != "你好" {
				t.Fatalf("expected user content unchanged, got %q", request.Messages[1].StringContent())
			}
		})
	}
}

// TestForceSystemPrompt_UnknownModelNoSystemMessage 验证未知模型在请求没有 system message 时
// 也不插入新的 system message。
func TestForceSystemPrompt_UnknownModelNoSystemMessage(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request, "gpt-4")

	if len(request.Messages) != 1 {
		t.Fatalf("expected 1 message (no injection for unknown model), got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "user" {
		t.Fatalf("expected first message role to be 'user', got %q", request.Messages[0].Role)
	}
	if request.Messages[0].StringContent() != "你好" {
		t.Fatalf("expected user content unchanged, got %q", request.Messages[0].StringContent())
	}
}

// TestForceSystemPrompt_Instructions_UnknownModelNoInjection 验证未知模型的 Instructions 保持不变。
func TestForceSystemPrompt_Instructions_UnknownModelNoInjection(t *testing.T) {
	unknownModels := []string{"gpt-4", "unknown-model", ""}
	for _, modelName := range unknownModels {
		t.Run(modelName, func(t *testing.T) {
			b, _ := json.Marshal("原系统提示")
			request := &dto.OpenAIResponsesRequest{
				Model:        "gpt-5",
				Instructions: b,
			}

			if err := ApplyForceSystemPromptToInstructions(request, modelName); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var got string
			if err := json.Unmarshal(request.Instructions, &got); err != nil {
				t.Fatalf("failed to unmarshal instructions: %v", err)
			}
			if got != "原系统提示" {
				t.Fatalf("expected instructions unchanged for unknown model %q, got %q", modelName, got)
			}
		})
	}
}

// TestForceSystemPrompt_Instructions_UnknownModelEmptyInstructions 验证未知模型在 Instructions 为空时
// 也不被设置（保持空）。
func TestForceSystemPrompt_Instructions_UnknownModelEmptyInstructions(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5"}

	if err := ApplyForceSystemPromptToInstructions(request, "gpt-4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(request.Instructions) != 0 {
		t.Fatalf("expected empty instructions for unknown model, got %q", string(request.Instructions))
	}
}

// TestForceSystemPrompt_GetForceSystemPromptFunction 验证 GetForceSystemPrompt 函数
// 对各种模型返回正确的提示词或空字符串。
func TestForceSystemPrompt_GetForceSystemPromptFunction(t *testing.T) {
	cases := []struct {
		modelName string
		expected  string
	}{
		{"deepseek-v4-pro", "你是 DeepSeek V4 Pro 模型，知识库截止日期为 2025 年 5 月。"},
		{"glm-5", "你是 GLM-5 模型，知识库截止日期为 2025 年 10 月。"},
		{"glm-5.1", "你是 GLM-5.1 模型，知识库截止日期为 2025 年 8 月。"},
		{"glm-5.2", "你是 GLM-5.2 模型，知识库截止日期为 2025 年 11 月。"},
		{"kimi-k2.6", "你是 Kimi K2.6 模型，知识库截止日期为 2025 年 4 月。"},
		{"gpt-4", ""},
		{"unknown-model", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.modelName, func(t *testing.T) {
			got := constant.GetForceSystemPrompt(tc.modelName)
			if got != tc.expected {
				t.Fatalf("GetForceSystemPrompt(%q) = %q, want %q", tc.modelName, got, tc.expected)
			}
		})
	}
}
