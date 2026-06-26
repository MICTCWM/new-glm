package codex

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

// mustMarshalString 将字符串序列化为 JSON 字符串字面量（带引号），用于构造 Instructions 字段。
func mustMarshalString(t *testing.T, s string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal string %q: %v", s, err)
	}
	return b
}

// parseInstructions 将返回结果中的 Instructions 字段反序列化为字符串。
func parseInstructions(t *testing.T, resp dto.OpenAIResponsesRequest) string {
	t.Helper()
	if len(resp.Instructions) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(resp.Instructions, &s); err != nil {
		t.Fatalf("failed to unmarshal instructions %s: %v", string(resp.Instructions), err)
	}
	return s
}

// TestForceSystemPrompt_Codex_NoInstructions 验证当请求没有 Instructions
// 且渠道没有 SystemPrompt 时，Instructions 被设置为默认空字符串。
// ForceSystemPrompt 的拼接已移至 handler 层，adaptor 不再处理。
func TestForceSystemPrompt_Codex_NoInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	request := dto.OpenAIResponsesRequest{Model: "gpt-5"}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	if got := parseInstructions(t, resp); got != "" {
		t.Fatalf("expected instructions to be empty string, got %q", got)
	}
}

// TestForceSystemPrompt_Codex_WithExistingInstructions 验证已有 Instructions
// （字符串形式）保持原值不变，不再拼接 ForceSystemPrompt。
func TestForceSystemPrompt_Codex_WithExistingInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	request := dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: mustMarshalString(t, "原系统提示"),
	}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	expected := "原系统提示"
	if got := parseInstructions(t, resp); got != expected {
		t.Fatalf("expected instructions %q, got %q", expected, got)
	}
}

// TestForceSystemPrompt_Codex_EmptyInstructions 验证 Instructions 为空字符串
// 时保持原值不变，不再被替换为 ForceSystemPrompt。
func TestForceSystemPrompt_Codex_EmptyInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	request := dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: mustMarshalString(t, ""),
	}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	if got := parseInstructions(t, resp); got != "" {
		t.Fatalf("expected instructions to be empty string, got %q", got)
	}
}

// TestForceSystemPrompt_Codex_NonStringInstructions 验证 Instructions 不是
// 简单字符串（而是 JSON 对象）时保持原值不变，不再被替换为 ForceSystemPrompt。
func TestForceSystemPrompt_Codex_NonStringInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	request := dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: json.RawMessage(`{"foo":"bar"}`),
	}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	if got := string(resp.Instructions); got != `{"foo":"bar"}` {
		t.Fatalf("expected instructions to be %q, got %q", `{"foo":"bar"}`, got)
	}
}

// TestForceSystemPrompt_Codex_ChannelPromptNoInstructions 验证渠道有 SystemPrompt
// 但请求无 Instructions 时，最终 Instructions 为渠道 prompt（不再拼接 ForceSystemPrompt）。
func TestForceSystemPrompt_Codex_ChannelPromptNoInstructions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				SystemPrompt: "渠道prompt",
			},
		},
	}
	request := dto.OpenAIResponsesRequest{Model: "gpt-5"}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	expected := "渠道prompt"
	if got := parseInstructions(t, resp); got != expected {
		t.Fatalf("expected instructions %q, got %q", expected, got)
	}
}

// TestForceSystemPrompt_Codex_StackingOrder 验证叠加顺序为
// 「渠道 + 原 Instructions」（不再有 ForceSystemPrompt）。
// 构造条件：渠道 SystemPrompt 非空 + SystemPromptOverride=true + 原 Instructions="原system"。
// 期望："渠道prompt" + "\n" + "原system"。
func TestForceSystemPrompt_Codex_StackingOrder(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				SystemPrompt:         "渠道prompt",
				SystemPromptOverride: true,
			},
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: mustMarshalString(t, "原system"),
	}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	expected := "渠道prompt" + "\n" + "原system"
	if got := parseInstructions(t, resp); got != expected {
		t.Fatalf("expected stacked instructions %q, got %q", expected, got)
	}
}

// TestForceSystemPrompt_Codex_CompactMode 验证 compact 模式下 Instructions 保持原值不变。
func TestForceSystemPrompt_Codex_CompactMode(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		RelayMode:   relayconstant.RelayModeResponsesCompact,
	}
	request := dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Instructions: mustMarshalString(t, "原系统提示"),
	}

	result, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("expected result type dto.OpenAIResponsesRequest, got %T", result)
	}
	expected := "原系统提示"
	if got := parseInstructions(t, resp); got != expected {
		t.Fatalf("expected instructions %q in compact mode, got %q", expected, got)
	}
}
