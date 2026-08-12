package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesRequestToChatPreservesMessagesToolsAndReasoning(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:        "gpt-test",
		Instructions: []byte(`"follow the rules"`),
		Input: []byte(`[
      {"role":"user","content":[
        {"type":"input_text","text":"find it"},
        {"type":"input_image","image_url":"https://example.test/a.png","detail":"high"}
      ]},
      {"type":"function_call","call_id":"call_1","name":"lookup","arguments":{"q":"x"}},
      {"type":"function_call_output","call_id":"call_1","output":"result"}
    ]`),
		Reasoning:         &dto.Reasoning{Effort: "high"},
		Tools:             []byte(`[{"type":"function","name":"lookup","description":"look up","parameters":{"type":"object"},"strict":true}]`),
		ToolChoice:        []byte(`{"type":"function","name":"lookup"}`),
		ParallelToolCalls: []byte(`true`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 4)
	require.Equal(t, "system", got.Messages[0].Role)
	require.Equal(t, "follow the rules", got.Messages[0].StringContent())
	require.Equal(t, "user", got.Messages[1].Role)
	require.Equal(t, "find it", got.Messages[1].ParseContent()[0].Text)
	require.Equal(t, "high", got.Messages[1].ParseContent()[1].GetImageMedia().Detail)
	require.Equal(t, "high", got.ReasoningEffort)
	require.Len(t, got.Tools, 1)
	require.Equal(t, "lookup", got.Tools[0].Function.Name)
	require.Equal(t, "lookup", gjson.GetBytes(mustMarshal(t, got.ToolChoice), "function.name").String())
	require.Equal(t, "call_1", got.Messages[2].ParseToolCalls()[0].ID)
	require.Equal(t, "tool", got.Messages[3].Role)
	require.True(t, *got.ParallelTooCalls)
}

func TestResponsesRequestToChatPreservesCacheControl(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[{"role":"user","content":[{"type":"input_text","text":"stable prefix","cache_control":{"type":"ephemeral"}}]}]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)

	content := got.Messages[0].ParseContent()
	require.Len(t, content, 1)
	require.Equal(t, "text", content[0].Type)
	require.JSONEq(t, `{"type":"ephemeral"}`, string(content[0].CacheControl))
}

func TestResponsesRequestToChatConvertsVideoInput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[{
			"role":"user",
			"content":[{"type":"input_video","video_url":"https://example.test/video.mp4"}]
		}]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	content := got.Messages[0].ParseContent()
	require.Len(t, content, 1)
	require.Equal(t, dto.ContentTypeVideoUrl, content[0].Type)
	require.Equal(t, "https://example.test/video.mp4", content[0].GetVideoUrl().Url)
}

func TestResponsesRequestToChatDoesNotSerializeUnknownContentAsText(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[{"role":"user","content":[{"type":"provider_payload","payload":{"secret":"internal"}}]}]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	require.Empty(t, got.Messages[0].ParseContent())
}

func TestResponsesRequestToChatConvertsResponseFormat(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Text:  []byte(`{"format":{"type":"json_schema","name":"answer","strict":true,"schema":{"type":"object"}}}`),
		Input: []byte(`"hello"`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, got.ResponseFormat)
	require.Equal(t, "json_schema", got.ResponseFormat.Type)
	require.Equal(t, "answer", gjson.GetBytes(got.ResponseFormat.JsonSchema, "name").String())
	require.Equal(t, "object", gjson.GetBytes(got.ResponseFormat.JsonSchema, "schema.type").String())
}

func TestResponsesRequestToChatFlattensNamespaceTools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Tools: []byte(`[
		  {
		    "type":"namespace",
		    "name":"crm",
		    "description":"CRM tools",
		    "tools":[
		      {"type":"function","name":"get_customer_profile","description":"get profile","parameters":{"type":"object"}},
		      {"type":"function","name":"list_open_orders","parameters":{"type":"object","properties":{}}}
		    ]
		  }
		]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Tools, 2)
	require.Equal(t, "function", got.Tools[0].Type)
	require.Equal(t, "get_customer_profile", got.Tools[0].Function.Name)
	require.Equal(t, "get profile", got.Tools[0].Function.Description)
	require.Equal(t, "list_open_orders", got.Tools[1].Function.Name)
}

func TestResponsesRequestToChatConvertsWebSearchTool(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-search",
		Tools: []byte(`[{
			"type":"web_search",
			"search_context_size":"high",
			"user_location":{"type":"approximate","country":"US"}
		}]`),
		ToolChoice: []byte(`{"type":"web_search"}`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, got.Tools)
	require.NotNil(t, got.WebSearchOptions)
	require.Equal(t, "high", got.WebSearchOptions.SearchContextSize)
	require.JSONEq(t, `{"type":"approximate","country":"US"}`, string(got.WebSearchOptions.UserLocation))
	// Chat Completions has no web-search tool_choice equivalent. The
	// web_search_options field enables the provider search path directly.
	require.Nil(t, got.ToolChoice)
}

func TestResponsesRequestToChatRejectsUnsupportedNestedNamespaceTool(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Tools: []byte(`[{"type":"namespace","name":"crm","tools":[{"type":"mcp","server_label":"crm"}]}]`),
	}

	_, err := ResponsesRequestToChatCompletionsRequest(req)
	require.EqualError(t, err, `responses namespace tool "crm" contains unsupported tool type "mcp"`)
}

func TestResponsesRequestToChatAttachesReasoningToToolCallTurn(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[
			{"role":"user","content":"find it"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"searching..."}]},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
		]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 3)

	// reasoning 与 function_call 必须合并到同一条 assistant 消息,
	// 而不是拆成两条独立的 assistant 消息。
	assistant := got.Messages[1]
	require.Equal(t, "assistant", assistant.Role)
	require.Equal(t, "searching...", assistant.GetReasoningContent())
	calls := assistant.ParseToolCalls()
	require.Len(t, calls, 1)
	require.Equal(t, "call_1", calls[0].ID)
	require.Equal(t, "lookup", calls[0].Function.Name)

	toolMsg := got.Messages[2]
	require.Equal(t, "tool", toolMsg.Role)
	require.Equal(t, "call_1", toolMsg.ToolCallId)
	require.Equal(t, "ok", toolMsg.StringContent())
}

func TestResponsesRequestToChatAccumulatesParallelToolCalls(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[
			{"role":"user","content":"both"},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},
			{"type":"function_call","call_id":"call_2","name":"search","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"a"},
			{"type":"function_call_output","call_id":"call_2","output":"b"}
		]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 4)

	// 连续的 function_call 必须合并为同一条 assistant 消息(并行工具调用),
	// 且该消息必须携带 reasoning_content 占位,thinking 模型才接受。
	assistant := got.Messages[1]
	calls := assistant.ParseToolCalls()
	require.Len(t, calls, 2)
	require.Equal(t, "call_1", calls[0].ID)
	require.Equal(t, "call_2", calls[1].ID)
	require.Equal(t, "tool call", assistant.GetReasoningContent())

	require.Equal(t, "tool", got.Messages[2].Role)
	require.Equal(t, "tool", got.Messages[3].Role)
}

func TestResponsesRequestToChatDoesNotLeakReasoningAcrossUserTurn(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[
			{"role":"user","content":"q1"},
			{"role":"assistant","content":"a1"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking for turn two"}]},
			{"role":"user","content":"q2"},
			{"role":"assistant","content":"a2"}
		]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 4)

	// reasoning 必须回溯附挂到 turn1 的 assistant,不能泄漏到 a2。
	require.Equal(t, "a1", got.Messages[1].StringContent())
	require.Equal(t, "thinking for turn two", got.Messages[1].GetReasoningContent())
	require.Empty(t, got.Messages[3].GetReasoningContent())
}

func TestResponsesRequestToChatBackfillsTrailingReasoning(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[
			{"role":"user","content":"q"},
			{"role":"assistant","content":"a"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"trailing think"}]}
		]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 2)
	require.Equal(t, "trailing think", got.Messages[1].GetReasoningContent())
}

func TestResponsesRequestToChatInjectsReasoningPlaceholderForToolCalls(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: []byte(`[
			{"role":"user","content":"go"},
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}
			]}
		]`),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, got.Messages, 2)

	assistant := got.Messages[1]
	require.Len(t, assistant.ParseToolCalls(), 1)
	require.Equal(t, "tool call", assistant.GetReasoningContent())
}

func TestResponsesRequestToChatInjectsStreamIncludeUsage(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:  "gpt-test",
		Input:  []byte(`"hello"`),
		Stream: lo.ToPtr(true),
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, got.StreamOptions)
	require.True(t, got.StreamOptions.IncludeUsage)
}

func TestResponsesRequestToChatPreservesExplicitStreamOptions(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:         "gpt-test",
		Input:         []byte(`"hello"`),
		Stream:        lo.ToPtr(true),
		StreamOptions: &dto.StreamOptions{IncludeUsage: false},
	}

	got, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, got.StreamOptions)
	require.False(t, got.StreamOptions.IncludeUsage)
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return data
}
