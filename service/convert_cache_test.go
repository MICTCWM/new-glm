package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestClaudeToOpenAIRequestDropsCacheControlForStrictOpenAIChannel(t *testing.T) {
	cacheControl := json.RawMessage(`{"type":"ephemeral"}`)
	request := dto.ClaudeRequest{
		Model: "claude-sonnet",
		System: []dto.ClaudeMediaMessage{{
			Type:         dto.ContentTypeText,
			Text:         ptrString("stable system"),
			CacheControl: cacheControl,
		}},
		Messages: []dto.ClaudeMessage{{
			Role: "user",
			Content: []dto.ClaudeMediaMessage{{
				Type:         dto.ContentTypeText,
				Text:         ptrString("question"),
				CacheControl: cacheControl,
			}},
		}},
	}

	openAI, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, openAI)

	system := openAI.Messages[0].ParseContent()
	require.Len(t, system, 1)
	require.Empty(t, system[0].CacheControl)

	user := openAI.Messages[1].ParseContent()
	require.Len(t, user, 1)
	require.Empty(t, user[0].CacheControl)
}

func TestClaudeToOpenAIRequestPreservesCacheControlForAnthropicChannel(t *testing.T) {
	cacheControl := json.RawMessage(`{"type":"ephemeral"}`)
	request := dto.ClaudeRequest{
		Model: "claude-sonnet",
		Messages: []dto.ClaudeMessage{{
			Role: "user",
			Content: []dto.ClaudeMediaMessage{{
				Type:         dto.ContentTypeText,
				Text:         ptrString("question"),
				CacheControl: cacheControl,
			}},
		}},
	}

	openAI, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAnthropic,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, openAI)

	user := openAI.Messages[0].ParseContent()
	require.Len(t, user, 1)
	require.JSONEq(t, string(cacheControl), string(user[0].CacheControl))
}

func TestClaudeToOpenAIRequestPreservesCacheControlForOpenRouter(t *testing.T) {
	cacheControl := json.RawMessage(`{"type":"ephemeral"}`)
	request := dto.ClaudeRequest{
		Model: "claude-sonnet",
		Messages: []dto.ClaudeMessage{{
			Role: "user",
			Content: []dto.ClaudeMediaMessage{{
				Type:         dto.ContentTypeText,
				Text:         ptrString("stable prefix"),
				CacheControl: cacheControl,
			}},
		}},
	}

	openAI, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
	})
	require.NoError(t, err)

	user := openAI.Messages[0].ParseContent()
	require.Len(t, user, 1)
	require.JSONEq(t, string(cacheControl), string(user[0].CacheControl))
}

func TestClaudeToOpenAIRequestPreservesToolAndImageCacheControlForOpenRouter(t *testing.T) {
	cacheControl := json.RawMessage(`{"type":"ephemeral"}`)
	request := dto.ClaudeRequest{
		Model: "claude-sonnet",
		Tools: []dto.Tool{{
			Name:         "lookup",
			InputSchema:  map[string]interface{}{"type": "object"},
			CacheControl: cacheControl,
		}},
		Messages: []dto.ClaudeMessage{{
			Role: "user",
			Content: []dto.ClaudeMediaMessage{{
				Type:         "image",
				Source:       &dto.ClaudeMessageSource{MediaType: "image/png", Data: "ZmFrZQ=="},
				CacheControl: cacheControl,
			}},
		}},
	}

	openAI, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
	})
	require.NoError(t, err)
	require.Len(t, openAI.Tools, 1)
	require.JSONEq(t, string(cacheControl), string(openAI.Tools[0].CacheControl))

	content := openAI.Messages[0].ParseContent()
	require.Len(t, content, 1)
	require.JSONEq(t, string(cacheControl), string(content[0].CacheControl))
}

func TestClaudeToOpenAIRequestPreservesThinkingEffort(t *testing.T) {
	budget := 4096
	request := dto.ClaudeRequest{
		Model:    "gpt-5",
		Thinking: &dto.Thinking{Type: "enabled", BudgetTokens: &budget},
	}

	openAI, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "high", openAI.ReasoningEffort)
}

func ptrString(value string) *string {
	return &value
}
