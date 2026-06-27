package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestStreamResponseOpenAI2ClaudeAppendsReasoningAfterRpmQueueNotice(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RpmQueueThinkingNoticeSent: true,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	reasoning := "upstream thinking"
	response := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningContent: &reasoning,
				},
			},
		},
	}

	converted := StreamResponseOpenAI2Claude(response, info)

	require.Len(t, converted, 1)
	require.Equal(t, "content_block_delta", converted[0].Type)
	require.Equal(t, 0, converted[0].GetIndex())
	require.NotNil(t, converted[0].Delta)
	require.Equal(t, "thinking_delta", converted[0].Delta.Type)
	require.Equal(t, reasoning, *converted[0].Delta.Thinking)
}

func TestStreamResponseOpenAI2ClaudeClosesRpmQueueThinkingBeforeText(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RpmQueueThinkingNoticeSent: true,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	content := "answer"
	response := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: common.GetPointer[string](content),
				},
			},
		},
	}

	converted := StreamResponseOpenAI2Claude(response, info)

	require.Len(t, converted, 3)
	require.Equal(t, "content_block_stop", converted[0].Type)
	require.Equal(t, 0, converted[0].GetIndex())
	require.Equal(t, "content_block_start", converted[1].Type)
	require.Equal(t, 1, converted[1].GetIndex())
	require.Equal(t, "text", converted[1].ContentBlock.Type)
	require.Equal(t, "content_block_delta", converted[2].Type)
	require.Equal(t, 1, converted[2].GetIndex())
	require.Equal(t, "text_delta", converted[2].Delta.Type)
	require.Equal(t, content, *converted[2].Delta.Text)
}

func TestIsPlaceholderContent(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"empty", "", false},
		{"dots only", "..", true},
		{"dots and newlines", ".\n\n", true},
		{"three dots and newlines", "...\n\n", true},
		{"mixed whitespace", "  . \n\t\r", true},
		{"normal text", "Hello world", false},
		{"text with dots", "Hello...", false},
		{"chinese text", "你好...", false},
		{"single dot", ".", true},
		{"newline only", "\n\n", true},
		{"space only", "   ", true},
		{"tab only", "\t", true},
		{"semicolon", ";", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPlaceholderContent(tt.s); got != tt.want {
				t.Errorf("IsPlaceholderContent(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestStreamResponseOpenAI2ClaudeFiltersPlaceholderText(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	// 模型在工具调用前输出的占位/停顿符号，拼接后即 "...\n\n"
	response := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: common.GetPointer[string]("...\n\n"),
				},
			},
		},
	}

	converted := StreamResponseOpenAI2Claude(response, info)

	// 占位符应被完全过滤，不产生任何 content_block_start / text_delta
	require.Empty(t, converted)
}
