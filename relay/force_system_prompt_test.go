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
	ApplyForceSystemPromptToMessages(nil)
}

// TestForceSystemPrompt_NoSystemMessage 验证请求中没有 system message 时，
// 应在最前面插入一条内容为 ForceSystemPrompt 的 system message。
func TestForceSystemPrompt_NoSystemMessage(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request)

	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "system" {
		t.Fatalf("expected first message role to be 'system', got %q", request.Messages[0].Role)
	}
	if request.Messages[0].StringContent() != constant.ForceSystemPrompt {
		t.Fatalf("expected first message content to be ForceSystemPrompt, got %q", request.Messages[0].StringContent())
	}
	if request.Messages[1].Role != "user" {
		t.Fatalf("expected second message role to be 'user', got %q", request.Messages[1].Role)
	}
	if request.Messages[1].StringContent() != "你好" {
		t.Fatalf("expected second message content to be '你好', got %q", request.Messages[1].StringContent())
	}
}

// TestForceSystemPrompt_StringContent 验证已有 string content 的 system message
// 会被拼接为 ForceSystemPrompt + "\n" + 原内容。
func TestForceSystemPrompt_StringContent(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "原系统提示"},
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request)

	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "system" {
		t.Fatalf("expected first message role to be 'system', got %q", request.Messages[0].Role)
	}
	expected := constant.ForceSystemPrompt + "\n" + "原系统提示"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected system content %q, got %q", expected, request.Messages[0].StringContent())
	}
	if !request.Messages[0].IsStringContent() {
		t.Fatalf("expected system message to remain string content after prepend")
	}
}

// TestForceSystemPrompt_MediaContent 验证已有 media content 数组的 system message
// 会在数组头部插入 type=text、text=ForceSystemPrompt 的内容项。
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

	ApplyForceSystemPromptToMessages(request)

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
	if contents[0].Text != constant.ForceSystemPrompt {
		t.Fatalf("expected first content text to be ForceSystemPrompt, got %q", contents[0].Text)
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

	ApplyForceSystemPromptToMessages(request)

	if len(request.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(request.Messages))
	}
	if request.Messages[0].Role != "developer" {
		t.Fatalf("expected first message role to be 'developer', got %q", request.Messages[0].Role)
	}
	expected := constant.ForceSystemPrompt + "\n" + "原系统提示"
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

	ApplyForceSystemPromptToMessages(request)

	expected := constant.ForceSystemPrompt + "\n" + "原系统提示"
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

	ApplyForceSystemPromptToMessages(request)

	if len(request.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(request.Messages))
	}
	expected := constant.ForceSystemPrompt + "\n" + "原系统提示"
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
// ForceSystemPrompt + "\n" + "渠道prompt" + "\n" + "原system"。
func TestForceSystemPrompt_StackingOrder(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "渠道prompt\n原system"},
			{Role: "user", Content: "你好"},
		},
	}

	ApplyForceSystemPromptToMessages(request)

	expected := constant.ForceSystemPrompt + "\n" + "渠道prompt\n原system"
	if request.Messages[0].StringContent() != expected {
		t.Fatalf("expected stacked system content %q, got %q", expected, request.Messages[0].StringContent())
	}
}

// TestForceSystemPrompt_StackingOrderMedia 验证 media content 数组形式下
// 叠加顺序：先在 system 数组前手动插入"渠道prompt"（模拟渠道逻辑），再调用拼接函数，
// 最终数组首元素应为 ForceSystemPrompt，第二个为"渠道prompt"，第三个为"原system"。
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

	ApplyForceSystemPromptToMessages(request)

	contents, ok := request.Messages[0].Content.([]dto.MediaContent)
	if !ok {
		t.Fatalf("expected Content to be []dto.MediaContent after prepend, got %T", request.Messages[0].Content)
	}
	if len(contents) != 3 {
		t.Fatalf("expected 3 media contents (force + channel + origin), got %d", len(contents))
	}
	if contents[0].Text != constant.ForceSystemPrompt {
		t.Fatalf("expected contents[0] to be ForceSystemPrompt, got %q", contents[0].Text)
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

	ApplyForceSystemPromptToMessages(request)

	expectedFirst := constant.ForceSystemPrompt + "\n" + "第一个system"
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
	if err := ApplyForceSystemPromptToInstructions(nil); err != nil {
		t.Fatalf("expected nil error for nil request, got: %v", err)
	}
}

// TestForceSystemPrompt_Instructions_NoInstructions 验证 Instructions 为空时，
// 被设置为 ForceSystemPrompt。
func TestForceSystemPrompt_Instructions_NoInstructions(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5"}

	if err := ApplyForceSystemPromptToInstructions(request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	if got != constant.ForceSystemPrompt {
		t.Fatalf("expected instructions to be ForceSystemPrompt, got %q", got)
	}
}

// TestForceSystemPrompt_Instructions_WithExisting 验证已有字符串 Instructions
// 被拼接为 ForceSystemPrompt + "\n" + 原内容。
func TestForceSystemPrompt_Instructions_WithExisting(t *testing.T) {
	b, _ := json.Marshal("原系统提示")
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: b,
	}

	if err := ApplyForceSystemPromptToInstructions(request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	expected := constant.ForceSystemPrompt + "\n" + "原系统提示"
	if got != expected {
		t.Fatalf("expected instructions %q, got %q", expected, got)
	}
}

// TestForceSystemPrompt_Instructions_EmptyString 验证 Instructions 为空格字符串
// （TrimSpace 后为空）时，被设置为 ForceSystemPrompt。
func TestForceSystemPrompt_Instructions_EmptyString(t *testing.T) {
	b, _ := json.Marshal("   ")
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: b,
	}

	if err := ApplyForceSystemPromptToInstructions(request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	if got != constant.ForceSystemPrompt {
		t.Fatalf("expected instructions to be ForceSystemPrompt, got %q", got)
	}
}

// TestForceSystemPrompt_Instructions_NonString 验证 Instructions 不是简单字符串
// （而是 JSON 对象）时，直接被替换为 ForceSystemPrompt。
func TestForceSystemPrompt_Instructions_NonString(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: json.RawMessage(`{"foo":"bar"}`),
	}

	if err := ApplyForceSystemPromptToInstructions(request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got string
	if err := json.Unmarshal(request.Instructions, &got); err != nil {
		t.Fatalf("failed to unmarshal instructions: %v", err)
	}
	if got != constant.ForceSystemPrompt {
		t.Fatalf("expected instructions to be ForceSystemPrompt, got %q", got)
	}
}
