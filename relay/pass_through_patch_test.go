package relay

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestPatchChangedJSONFieldsPreservesUnknownFields(t *testing.T) {
	original := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hello","provider_extension":{"keep":true}}],"unknown_top_level":{"keep":true}}`)
	before := &dto.GeneralOpenAIRequest{
		Model: "client-model",
		Messages: []dto.Message{{
			Role:    "user",
			Content: "hello",
		}},
	}
	after := &dto.GeneralOpenAIRequest{
		Model: "mapped-model",
		Messages: []dto.Message{
			{Role: "system", Content: "policy"},
			{Role: "user", Content: "hello"},
		},
	}

	patched, err := patchChangedJSONFields(original, before, after)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(patched, &payload))
	require.Equal(t, "mapped-model", payload["model"])
	require.Equal(t, true, payload["unknown_top_level"].(map[string]any)["keep"])
	messages := payload["messages"].([]any)
	require.Equal(t, "policy", messages[0].(map[string]any)["content"])
	require.Equal(t, true, messages[1].(map[string]any)["provider_extension"].(map[string]any)["keep"])
}

func TestPatchChangedJSONFieldsPreservesUnknownNestedArrayFields(t *testing.T) {
	original := []byte(`{"systemInstruction":{"parts":[{"text":"hello","provider_extension":{"keep":true}}]}}`)
	before := &dto.GeminiChatRequest{
		SystemInstructions: &dto.GeminiChatContent{Parts: []dto.GeminiPart{{Text: "hello"}}},
	}
	after := &dto.GeminiChatRequest{
		SystemInstructions: &dto.GeminiChatContent{Parts: []dto.GeminiPart{{Text: "policy\nhello"}}},
	}

	patched, err := patchChangedJSONFields(original, before, after)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(patched, &payload))
	parts := payload["systemInstruction"].(map[string]any)["parts"].([]any)
	require.Equal(t, "policy\nhello", parts[0].(map[string]any)["text"])
	require.Equal(t, true, parts[0].(map[string]any)["provider_extension"].(map[string]any)["keep"])
}
