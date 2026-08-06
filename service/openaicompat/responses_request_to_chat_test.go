package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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

func TestResponsesRequestToChatRejectsUnsupportedNestedNamespaceTool(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Tools: []byte(`[{"type":"namespace","name":"crm","tools":[{"type":"mcp","server_label":"crm"}]}]`),
	}

	_, err := ResponsesRequestToChatCompletionsRequest(req)
	require.EqualError(t, err, `responses namespace tool "crm" contains unsupported tool type "mcp"`)
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return data
}
