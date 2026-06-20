package openai

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	thinkOpenTag  = "<think>"
	thinkCloseTag = "</think>"
)

func normalizeStreamThinkTags(info *relaycommon.RelayInfo, response *dto.ChatCompletionsStreamResponse) bool {
	if info == nil || response == nil || len(response.Choices) == 0 {
		return false
	}

	changed := false
	for i := range response.Choices {
		choice := &response.Choices[i]
		index := choice.Index
		state := getThinkTagStreamState(info, index)
		content := choice.Delta.GetContentString()

		if content == "" {
			if choice.FinishReason != nil && state.Pending != "" {
				if state.InThinking {
					appendDeltaReasoning(&choice.Delta, state.Pending)
				} else {
					choice.Delta.SetContentString(state.Pending)
				}
				state.Pending = ""
				changed = true
			}
			continue
		}

		visible, reasoning, contentChanged := splitThinkTagsStreaming(content, state)
		if !contentChanged {
			continue
		}

		changed = true
		if visible == "" {
			choice.Delta.Content = nil
		} else {
			choice.Delta.SetContentString(visible)
		}
		if reasoning != "" {
			appendDeltaReasoning(&choice.Delta, reasoning)
		}
	}
	return changed
}

func normalizeTextResponseThinkTags(response *dto.OpenAITextResponse) bool {
	if response == nil || len(response.Choices) == 0 {
		return false
	}

	changed := false
	for i := range response.Choices {
		message := &response.Choices[i].Message
		if !message.IsStringContent() {
			continue
		}

		visible, reasoning, contentChanged := splitThinkTagsComplete(message.StringContent())
		if !contentChanged {
			continue
		}

		changed = true
		message.SetStringContent(visible)
		if reasoning != "" {
			appendMessageReasoning(message, reasoning)
		}
	}
	return changed
}

func getThinkTagStreamState(info *relaycommon.RelayInfo, index int) *relaycommon.ThinkTagStreamState {
	if info.ThinkTagStreamStates == nil {
		info.ThinkTagStreamStates = make(map[int]*relaycommon.ThinkTagStreamState)
	}
	state := info.ThinkTagStreamStates[index]
	if state == nil {
		state = &relaycommon.ThinkTagStreamState{}
		info.ThinkTagStreamStates[index] = state
	}
	return state
}

func splitThinkTagsStreaming(chunk string, state *relaycommon.ThinkTagStreamState) (visible string, reasoning string, changed bool) {
	if state == nil {
		return chunk, "", false
	}

	input := state.Pending + chunk
	if state.Pending != "" {
		changed = true
		state.Pending = ""
	}

	var visibleBuilder strings.Builder
	var reasoningBuilder strings.Builder

	for input != "" {
		if state.InThinking {
			closeIndex := strings.Index(input, thinkCloseTag)
			if closeIndex >= 0 {
				reasoningBuilder.WriteString(input[:closeIndex])
				input = input[closeIndex+len(thinkCloseTag):]
				state.InThinking = false
				changed = true
				continue
			}

			hold := longestTagPrefixSuffix(input, thinkCloseTag)
			reasoningBuilder.WriteString(input[:len(input)-hold])
			state.Pending = input[len(input)-hold:]
			if hold > 0 {
				changed = true
			}
			break
		}

		openIndex := strings.Index(input, thinkOpenTag)
		if openIndex >= 0 {
			visibleBuilder.WriteString(input[:openIndex])
			input = input[openIndex+len(thinkOpenTag):]
			state.InThinking = true
			changed = true
			continue
		}

		hold := longestTagPrefixSuffix(input, thinkOpenTag)
		visibleBuilder.WriteString(input[:len(input)-hold])
		state.Pending = input[len(input)-hold:]
		if hold > 0 {
			changed = true
		}
		break
	}

	return visibleBuilder.String(), reasoningBuilder.String(), changed
}

func splitThinkTagsComplete(content string) (visible string, reasoning string, changed bool) {
	openIndex := strings.Index(content, thinkOpenTag)
	if openIndex < 0 {
		return content, "", false
	}

	var visibleBuilder strings.Builder
	var reasoningBuilder strings.Builder
	remaining := content

	for {
		openIndex = strings.Index(remaining, thinkOpenTag)
		if openIndex < 0 {
			visibleBuilder.WriteString(remaining)
			break
		}

		changed = true
		visibleBuilder.WriteString(remaining[:openIndex])
		remaining = remaining[openIndex+len(thinkOpenTag):]

		closeIndex := strings.Index(remaining, thinkCloseTag)
		if closeIndex < 0 {
			reasoningBuilder.WriteString(remaining)
			break
		}

		reasoningBuilder.WriteString(remaining[:closeIndex])
		remaining = remaining[closeIndex+len(thinkCloseTag):]
	}

	return visibleBuilder.String(), reasoningBuilder.String(), changed
}

func longestTagPrefixSuffix(s string, tag string) int {
	max := len(tag) - 1
	if len(s) < max {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasSuffix(s, tag[:n]) {
			return n
		}
	}
	return 0
}

func appendDeltaReasoning(delta *dto.ChatCompletionsStreamResponseChoiceDelta, reasoning string) {
	if reasoning == "" {
		return
	}
	existing := delta.GetReasoningContent()
	if existing != "" {
		reasoning = existing + reasoning
	}
	delta.SetReasoningContent(reasoning)
	delta.Reasoning = nil
}

func appendMessageReasoning(message *dto.Message, reasoning string) {
	if reasoning == "" {
		return
	}
	existing := message.GetReasoningContent()
	if existing != "" {
		reasoning = existing + "\n\n" + strings.TrimSpace(reasoning)
	}
	message.ReasoningContent = &reasoning
	message.Reasoning = nil
}
