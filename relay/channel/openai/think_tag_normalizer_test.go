package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestNormalizeTextResponseThinkTags(t *testing.T) {
	response := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:    "assistant",
					Content: "before <think>hidden</think> after",
				},
			},
		},
	}

	if !normalizeTextResponseThinkTags(response) {
		t.Fatal("expected response to be normalized")
	}
	if got := response.Choices[0].Message.StringContent(); got != "before  after" {
		t.Fatalf("content = %q", got)
	}
	if got := response.Choices[0].Message.GetReasoningContent(); got != "hidden" {
		t.Fatalf("reasoning = %q", got)
	}
}

func TestNormalizeTextResponseThinkTagsMergesReasoning(t *testing.T) {
	existing := "api reasoning"
	response := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:             "assistant",
					Content:          "<think>tag reasoning</think>answer",
					ReasoningContent: &existing,
				},
			},
		},
	}

	normalizeTextResponseThinkTags(response)

	if got := response.Choices[0].Message.StringContent(); got != "answer" {
		t.Fatalf("content = %q", got)
	}
	if got := response.Choices[0].Message.GetReasoningContent(); got != "api reasoning\n\ntag reasoning" {
		t.Fatalf("reasoning = %q", got)
	}
}

func TestNormalizeStreamThinkTagsAcrossChunks(t *testing.T) {
	info := &relaycommon.RelayInfo{}

	first := streamChunk("hello <thi")
	if !normalizeStreamThinkTags(info, first) {
		t.Fatal("expected first chunk to change because partial tag is buffered")
	}
	if got := first.Choices[0].Delta.GetContentString(); got != "hello " {
		t.Fatalf("first content = %q", got)
	}
	if got := first.Choices[0].Delta.GetReasoningContent(); got != "" {
		t.Fatalf("first reasoning = %q", got)
	}

	second := streamChunk("nk>secret</thi")
	if !normalizeStreamThinkTags(info, second) {
		t.Fatal("expected second chunk to change")
	}
	if got := second.Choices[0].Delta.GetContentString(); got != "" {
		t.Fatalf("second content = %q", got)
	}
	if got := second.Choices[0].Delta.GetReasoningContent(); got != "secret" {
		t.Fatalf("second reasoning = %q", got)
	}

	third := streamChunk("nk> answer")
	if !normalizeStreamThinkTags(info, third) {
		t.Fatal("expected third chunk to change")
	}
	if got := third.Choices[0].Delta.GetContentString(); got != " answer" {
		t.Fatalf("third content = %q", got)
	}
	if got := third.Choices[0].Delta.GetReasoningContent(); got != "" {
		t.Fatalf("third reasoning = %q", got)
	}
}

func TestNormalizeStreamThinkTagsFlushesPendingOnFinish(t *testing.T) {
	info := &relaycommon.RelayInfo{}

	first := streamChunk("abc <")
	normalizeStreamThinkTags(info, first)
	if got := first.Choices[0].Delta.GetContentString(); got != "abc " {
		t.Fatalf("first content = %q", got)
	}

	finishReason := "stop"
	final := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		},
	}
	normalizeStreamThinkTags(info, final)
	if got := final.Choices[0].Delta.GetContentString(); got != "<" {
		t.Fatalf("final content = %q", got)
	}
}

func streamChunk(content string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: &content,
				},
			},
		},
	}
}
