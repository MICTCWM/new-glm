package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChatCompletionsRequestToResponsesPreservesCoreMappings(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:           "gpt-test",
		N:               lo.ToPtr(1),
		ReasoningEffort: "high",
		ResponseFormat: &dto.ResponseFormat{
			Type:       "json_schema",
			JsonSchema: []byte(`{"name":"answer","strict":true,"schema":{"type":"object"}}`),
		},
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "lookup",
				Description: "Look something up",
				Parameters:  map[string]any{"type": "object"},
				Strict:      []byte(`true`),
			},
		}},
		Messages: []dto.Message{
			{Role: "system", Content: "system rules"},
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "find it"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.test/a.png", "detail": "high"}},
				map[string]any{"type": "file", "file": map[string]any{"file_id": "file-1", "filename": "answer.pdf"}},
			}},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)
	require.Equal(t, `"system rules"`, string(got.Instructions))
	require.Equal(t, "input_image", jsonPath(t, got.Input, "0.content.1.type"))
	require.Equal(t, "high", jsonPath(t, got.Input, "0.content.1.detail"))
	require.Equal(t, "file-1", jsonPath(t, got.Input, "0.content.2.file_id"))
	require.Equal(t, "answer.pdf", jsonPath(t, got.Input, "0.content.2.filename"))
	require.Equal(t, "json_schema", jsonPath(t, got.Text, "format.type"))
	require.Equal(t, "true", jsonPath(t, got.Tools, "0.strict"))
	require.Equal(t, "high", got.Reasoning.Effort)
}

func TestResponsesResponseToChatPreservesTextReasoningToolsAndIncompleteStatus(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:                "resp_1",
		Model:             "gpt-test",
		Status:            []byte(`"incomplete"`),
		IncompleteDetails: &dto.IncompleteDetails{Reason: "max_output_tokens"},
		Output: []dto.ResponsesOutput{
			{Type: "reasoning", Content: []dto.ResponsesOutputContent{{Type: "summary_text", Text: "thinking"}}},
			{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "answer"}}},
			{Type: "function_call", ID: "fc_1", CallId: "call_1", Name: "lookup", Arguments: []byte(`{"q":"x"}`)},
		},
		Usage: &dto.Usage{InputTokens: 2, OutputTokens: 3, TotalTokens: 5},
	}

	chat, usage, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	require.Equal(t, 5, usage.TotalTokens)
	require.Equal(t, "length", chat.Choices[0].FinishReason)
	require.Equal(t, "answer", chat.Choices[0].Message.StringContent())
	require.Equal(t, "thinking", chat.Choices[0].Message.GetReasoningContent())
	require.Len(t, chat.Choices[0].Message.ParseToolCalls(), 1)
	require.Equal(t, "call_1", chat.Choices[0].Message.ParseToolCalls()[0].ID)
}

func TestResponsesFinishReasonFromStatusUsesStandardReasonField(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Status:            []byte(`"incomplete"`),
		IncompleteDetails: &dto.IncompleteDetails{Reason: "content_filter"},
	}
	reason, ok := ResponsesFinishReasonFromStatus(resp)
	require.True(t, ok)
	require.Equal(t, "content_filter", reason)
}

func TestChatCompletionsResponseToResponsesPreservesTextToolsAndUsage(t *testing.T) {
	finish := "tool_calls"
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Created: 1710000000,
		Model:   "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{{
			Message:      dto.Message{Role: "assistant", Content: "answer", ReasoningContent: lo.ToPtr("thinking")},
			FinishReason: finish,
		}},
		Usage: dto.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
	}
	chat.Choices[0].Message.SetToolCalls([]dto.ToolCallRequest{{
		ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "lookup", Arguments: `{"q":"x"}`},
	}})

	resp, usage, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1")
	require.NoError(t, err)
	require.Equal(t, "completed", string(resp.Status[1:len(resp.Status)-1]))
	require.Equal(t, 5, usage.TotalTokens)
	require.Equal(t, "output_text", resp.Output[0].Content[0].Type)
	require.Equal(t, "summary_text", resp.Output[1].Content[0].Type)
	require.Equal(t, "function_call", resp.Output[2].Type)
	require.Equal(t, "call_1", resp.Output[2].CallId)
}

func TestChatCompletionsStreamChunkToResponsesEventsFinalizesToolCall(t *testing.T) {
	state := NewChatToResponsesStreamState("resp_1", "gpt-test")
	content := "hello"
	toolArgs := `{"q":"x"}`
	index := 0
	chunk := &dto.ChatCompletionsStreamResponse{
		Id: "chatcmpl_1", Created: 1710000000, Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				Content:   &content,
				ToolCalls: []dto.ToolCallResponse{{ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "lookup", Arguments: toolArgs}, Index: &index}},
			},
		}},
	}
	events, err := ChatCompletionsStreamChunkToResponsesEvents(chunk, state)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	final := FinalizeChatCompletionsStreamToResponses(state)
	require.NotEmpty(t, final)
	require.Equal(t, "response.function_call_arguments.done", final[len(final)-3].Type)
	require.Equal(t, `{"q":"x"}`, gjson.ParseBytes(final[len(final)-3].Payload.Arguments).String())
	require.Equal(t, "response.completed", final[len(final)-1].Type)
	require.Equal(t, "function_call", final[len(final)-1].Payload.Response.Output[1].Type)
}

func jsonPath(t *testing.T, raw []byte, path string) string {
	t.Helper()
	return gjson.GetBytes(raw, path).String()
}
