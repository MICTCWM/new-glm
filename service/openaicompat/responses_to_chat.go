package openaicompat

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	responsesIncompleteReasonContentFilter = "content_filter"
	responsesOutputTypeFunctionCall        = "function_call"
	responsesOutputTypeCustomToolCall      = "custom_tool_call"
	responsesOutputTypeMessage             = "message"
	responsesOutputTypeReasoning           = "reasoning"
)

func ResponsesFinishReasonFromStatus(resp *dto.OpenAIResponsesResponse) (string, bool) {
	if resp == nil || responseStatusString(resp) != "incomplete" {
		return "", false
	}
	if resp.IncompleteDetails != nil {
		reason := strings.TrimSpace(resp.IncompleteDetails.Reason)
		if reason == "" {
			reason = strings.TrimSpace(resp.IncompleteDetails.Reasoning)
		}
		if reason == responsesIncompleteReasonContentFilter {
			return "content_filter", true
		}
	}
	return "length", true
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}

	usage := UsageFromResponsesUsage(resp.Usage)
	text := ExtractOutputTextFromResponses(resp)
	reasoning := ExtractReasoningTextFromResponses(resp)

	toolCalls := make([]dto.ToolCallResponse, 0)
	for _, out := range resp.Output {
		if out.Type != responsesOutputTypeFunctionCall && out.Type != responsesOutputTypeCustomToolCall {
			continue
		}
		name := strings.TrimSpace(out.Name)
		if name == "" {
			continue
		}
		callID := strings.TrimSpace(out.CallId)
		if callID == "" {
			callID = strings.TrimSpace(out.ID)
		}
		toolCalls = append(toolCalls, dto.ToolCallResponse{
			ID:   callID,
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      name,
				Arguments: out.ArgumentsString(),
			},
		})
	}

	finishReason := "stop"
	if mapped, ok := ResponsesFinishReasonFromStatus(resp); ok {
		finishReason = mapped
	} else if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	message := dto.Message{Role: "assistant", Content: text}
	if reasoning != "" {
		message.ReasoningContent = &reasoning
	}
	if len(toolCalls) > 0 {
		message.SetToolCalls(toolCalls)
	}

	out := &dto.OpenAITextResponse{
		Id:      id,
		Object:  "chat.completion",
		Created: resp.CreatedAt,
		Model:   resp.Model,
		Choices: []dto.OpenAITextResponseChoice{{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		}},
		Usage: *usage,
	}
	return out, usage, nil
}

func UsageFromResponsesUsage(src *dto.Usage) *dto.Usage {
	usage := &dto.Usage{}
	if src == nil {
		return usage
	}
	if src.InputTokens != 0 {
		usage.PromptTokens = src.InputTokens
		usage.InputTokens = src.InputTokens
	}
	if src.OutputTokens != 0 {
		usage.CompletionTokens = src.OutputTokens
		usage.OutputTokens = src.OutputTokens
	}
	if src.TotalTokens != 0 {
		usage.TotalTokens = src.TotalTokens
	} else {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if src.InputTokensDetails != nil {
		usage.PromptTokensDetails.CachedTokens = src.InputTokensDetails.CachedTokens
		usage.PromptTokensDetails.CachedCreationTokens = src.InputTokensDetails.CachedCreationTokens
		usage.PromptTokensDetails.TextTokens = src.InputTokensDetails.TextTokens
		usage.PromptTokensDetails.ImageTokens = src.InputTokensDetails.ImageTokens
		usage.PromptTokensDetails.AudioTokens = src.InputTokensDetails.AudioTokens
	}
	usage.CompletionTokenDetails = src.CompletionTokenDetails
	return usage
}

func responseStatusString(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Status) == 0 {
		return ""
	}
	var status string
	if err := common.Unmarshal(resp.Status, &status); err != nil {
		return ""
	}
	return strings.TrimSpace(status)
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Output) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, out := range resp.Output {
		if out.Type != responsesOutputTypeMessage {
			continue
		}
		if out.Role != "" && out.Role != "assistant" {
			continue
		}
		for _, content := range out.Content {
			if content.Type == "output_text" && content.Text != "" {
				sb.WriteString(content.Text)
			}
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	for _, out := range resp.Output {
		for _, content := range out.Content {
			if content.Text != "" {
				sb.WriteString(content.Text)
			}
		}
	}
	return sb.String()
}

func ExtractReasoningTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil {
		return ""
	}
	var sb strings.Builder
	for _, out := range resp.Output {
		if out.Type != responsesOutputTypeReasoning {
			continue
		}
		for _, content := range out.Content {
			if content.Text != "" {
				sb.WriteString(content.Text)
			}
		}
	}
	return sb.String()
}
