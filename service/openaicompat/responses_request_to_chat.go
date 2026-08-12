package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

// ResponsesRequestToChatCompletionsRequest converts the common subset of a
// Responses request into the Chat Completions representation. Fields that
// have no Chat equivalent, such as previous_response_id, are intentionally
// omitted because forwarding them would produce an invalid Chat request.
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New("model is required")
	}

	out := &dto.GeneralOpenAIRequest{
		Model:                req.Model,
		Stream:               req.Stream,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		TopLogProbs:          req.TopLogProbs,
		Metadata:             req.Metadata,
		Store:                req.Store,
		User:                 req.User,
		SafetyIdentifier:     req.SafetyIdentifier,
		PromptCacheKey:       responsesStringValue(req.PromptCacheKey),
		PromptCacheRetention: req.PromptCacheRetention,
		MaxCompletionTokens:  req.MaxOutputTokens,
	}
	if req.ServiceTier != "" {
		out.ServiceTier, _ = common.Marshal(req.ServiceTier)
	}
	if req.StreamOptions != nil {
		out.StreamOptions = &dto.StreamOptions{IncludeUsage: req.StreamOptions.IncludeUsage}
	}
	// Streaming Requests clients usually omit stream_options, while OpenAI
	// compatible upstreams only append a usage chunk when include_usage is
	// requested explicitly. Without it token accounting and billing miss the
	// stream usage, so inject the flag unless the caller set stream_options.
	if lo.FromPtrOr(req.Stream, false) && out.StreamOptions == nil {
		out.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	if req.ParallelToolCalls != nil {
		var parallel bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallel); err == nil {
			out.ParallelTooCalls = lo.ToPtr(parallel)
		}
	}

	if responseFormat := responsesTextToChatResponseFormat(req.Text); responseFormat != nil {
		out.ResponseFormat = responseFormat
	}

	tools, webSearchOptions, err := responsesToolsToChatTools(req.Tools)
	if err != nil {
		return nil, err
	}
	out.Tools = tools
	out.WebSearchOptions = webSearchOptions
	if req.ToolChoice != nil {
		out.ToolChoice = responsesToolChoiceToChat(req.ToolChoice)
	}

	messages, err := responsesInputToChatMessages(req.Input)
	if err != nil {
		return nil, err
	}
	if instruction := responsesTextValue(req.Instructions); instruction != "" {
		messages = append([]dto.Message{{Role: "system", Content: instruction}}, messages...)
	}
	if len(messages) == 0 && len(req.Prompt) > 0 {
		var prompt any
		if err := common.Unmarshal(req.Prompt, &prompt); err != nil {
			return nil, fmt.Errorf("invalid prompt: %w", err)
		}
		message := dto.Message{Role: "user"}
		setChatMessageContent(&message, prompt)
		messages = append(messages, message)
	}
	out.Messages = messages

	return out, nil
}

// responsesInputState folds the flat Responses input item list into Chat
// messages. Responses models reasoning and tool calls as top-level items,
// while Chat attaches them to the assistant message. A single turn may
// interleave several reasoning/function_call items, so fragments are
// accumulated in the state until the next message boundary decides where they
// belong.
type responsesInputState struct {
	pendingReasoning   strings.Builder
	pendingToolCalls   []dto.ToolCallRequest
	lastAssistantIndex int // index of the last assistant message; -1 = none
}

func newResponsesInputState() *responsesInputState {
	return &responsesInputState{lastAssistantIndex: -1}
}

func (s *responsesInputState) hasPendingReasoning() bool {
	return strings.TrimSpace(s.pendingReasoning.String()) != ""
}

func (s *responsesInputState) takePendingReasoning() string {
	reasoning := strings.TrimSpace(s.pendingReasoning.String())
	s.pendingReasoning.Reset()
	return reasoning
}

func (s *responsesInputState) appendReasoning(reasoning string) {
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" {
		return
	}
	if s.pendingReasoning.Len() > 0 {
		s.pendingReasoning.WriteString("\n\n")
	}
	s.pendingReasoning.WriteString(reasoning)
}

// attachPendingReasoning moves pending reasoning into an assistant message,
// appending to reasoning content that is already present.
func (s *responsesInputState) attachPendingReasoning(message *dto.Message) {
	reasoning := s.takePendingReasoning()
	if reasoning == "" {
		return
	}
	if existing := message.GetReasoningContent(); existing != "" {
		message.ReasoningContent = lo.ToPtr(existing + "\n\n" + reasoning)
	} else {
		message.ReasoningContent = lo.ToPtr(reasoning)
	}
}

// flushPendingToolCalls emits the accumulated function calls as one assistant
// message, attaching any pending reasoning to it. Consecutive function_call
// items belong to the same parallel tool-call batch in Chat, so they must not
// be split into separate assistant messages.
func (s *responsesInputState) flushPendingToolCalls(messages *[]dto.Message) {
	if len(s.pendingToolCalls) == 0 {
		return
	}
	message := dto.Message{Role: "assistant"}
	message.SetToolCalls(s.pendingToolCalls)
	s.pendingToolCalls = nil
	s.attachPendingReasoning(&message)
	*messages = append(*messages, message)
	s.lastAssistantIndex = len(*messages) - 1
}

// backfillReasoningToLastAssistant attaches remaining pending reasoning to the
// previous assistant message. Reasoning must not leak across a user turn, so
// non-assistant boundaries call this before appending a message.
func (s *responsesInputState) backfillReasoningToLastAssistant(messages []dto.Message) {
	if !s.hasPendingReasoning() {
		return
	}
	if s.lastAssistantIndex < 0 || s.lastAssistantIndex >= len(messages) {
		return
	}
	message := &messages[s.lastAssistantIndex]
	if message.Role != "assistant" {
		return
	}
	s.attachPendingReasoning(message)
}

// finish emits whatever is still pending at the end of the input list.
func (s *responsesInputState) finish(messages *[]dto.Message) {
	s.flushPendingToolCalls(messages)
	s.backfillReasoningToLastAssistant(*messages)
}

func responsesInputToChatMessages(raw json.RawMessage) ([]dto.Message, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var input any
	if err := common.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	if text, ok := input.(string); ok {
		return []dto.Message{{Role: "user", Content: text}}, nil
	}

	items, ok := input.([]any)
	if !ok {
		return nil, fmt.Errorf("unsupported responses input type %T", input)
	}

	messages := make([]dto.Message, 0, len(items))
	state := newResponsesInputState()
	for _, item := range items {
		if text, ok := item.(string); ok {
			state.flushPendingToolCalls(&messages)
			state.backfillReasoningToLastAssistant(messages)
			messages = append(messages, dto.Message{Role: "user", Content: text})
			state.lastAssistantIndex = -1
			continue
		}
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if err := state.appendItem(itemMap, &messages); err != nil {
			return nil, err
		}
	}
	state.finish(&messages)
	backfillToolCallReasoningPlaceholders(messages)
	return messages, nil
}

// appendItem folds one Responses input item into the message list according to
// its type. reasoning/function_call fragments are accumulated in the state;
// everything else acts as a message boundary.
func (s *responsesInputState) appendItem(item map[string]any, messages *[]dto.Message) error {
	switch strings.TrimSpace(common.Interface2String(item["type"])) {
	case "reasoning":
		s.appendReasoning(responsesReasoningItemText(item))
		return nil
	case "function_call":
		s.pendingToolCalls = append(s.pendingToolCalls, responsesFunctionCallToChatToolCall(item))
		return nil
	case "function_call_output":
		return s.appendFunctionCallOutput(item, messages)
	case "input_text", "output_text", "text", "input_image", "image_url", "input_audio", "audio", "input_file", "file":
		return s.appendContentItem(item, messages)
	default:
		// "message" type and any item carrying a role are mapped by role.
		return s.appendRoleMessage(item, strings.TrimSpace(common.Interface2String(item["role"])), messages)
	}
}

func (s *responsesInputState) appendFunctionCallOutput(item map[string]any, messages *[]dto.Message) error {
	s.flushPendingToolCalls(messages)
	message := &dto.Message{
		Role:       "tool",
		ToolCallId: common.Interface2String(item["call_id"]),
	}
	message.SetStringContent(responsesValueString(item["output"]))
	*messages = append(*messages, *message)
	return nil
}

func (s *responsesInputState) appendContentItem(item map[string]any, messages *[]dto.Message) error {
	s.flushPendingToolCalls(messages)

	role := "user"
	if itemRole := strings.TrimSpace(common.Interface2String(item["role"])); itemRole != "" {
		role = responsesRoleToChatRole(itemRole)
	}
	message := &dto.Message{Role: role}
	content, err := responsesMessageContent([]any{item})
	if err != nil {
		return err
	}
	setChatMessageContent(message, content)
	if role == "assistant" {
		// Reasoning that precedes this assistant content belongs to it.
		s.attachPendingReasoning(message)
	} else {
		// Non-assistant content is a turn boundary; leftover reasoning goes
		// back to the previous assistant message.
		s.backfillReasoningToLastAssistant(*messages)
	}
	*messages = append(*messages, *message)
	if role == "assistant" {
		s.lastAssistantIndex = len(*messages) - 1
	} else {
		s.lastAssistantIndex = -1
	}
	return nil
}

func (s *responsesInputState) appendRoleMessage(item map[string]any, role string, messages *[]dto.Message) error {
	if role == "" {
		role = "user"
	}
	chatRole := responsesRoleToChatRole(role)

	s.flushPendingToolCalls(messages)

	message := &dto.Message{Role: chatRole}
	content, err := responsesMessageContent(item["content"])
	if err != nil {
		return err
	}
	setChatMessageContent(message, content)
	if toolCalls := responsesMessageToolCalls(item["tool_calls"]); len(toolCalls) > 0 {
		message.SetToolCalls(toolCalls)
	}

	if chatRole == "assistant" {
		// Assistant messages may carry reasoning inline (round-tripped from a
		// previous Chat response); fold it in front of any pending reasoning.
		if reasoning := responsesMessageReasoningText(item); reasoning != "" {
			s.appendReasoning(reasoning)
		}
		s.attachPendingReasoning(message)
		*messages = append(*messages, *message)
		s.lastAssistantIndex = len(*messages) - 1
		return nil
	}

	// Turn boundary: reasoning must not leak into the next assistant message.
	s.backfillReasoningToLastAssistant(*messages)
	*messages = append(*messages, *message)
	if chatRole != "tool" {
		s.lastAssistantIndex = -1
	}
	return nil
}

func responsesReasoningItemText(item map[string]any) string {
	reasoning := responsesOutputContentText(item["summary"])
	if reasoning == "" {
		reasoning = responsesOutputContentText(item["content"])
	}
	return reasoning
}

func responsesMessageReasoningText(item map[string]any) string {
	for _, key := range []string{"reasoning", "reasoning_content"} {
		if text, ok := item[key].(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func responsesFunctionCallToChatToolCall(item map[string]any) dto.ToolCallRequest {
	callID := common.Interface2String(item["call_id"])
	if callID == "" {
		callID = common.Interface2String(item["id"])
	}
	return dto.ToolCallRequest{
		ID:   callID,
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      common.Interface2String(item["name"]),
			Arguments: responsesArgumentsString(item["arguments"]),
		},
	}
}

func responsesRoleToChatRole(role string) string {
	switch role {
	case "system", "developer":
		return "system"
	case "assistant":
		return "assistant"
	case "tool":
		return "tool"
	default:
		return "user"
	}
}

// backfillToolCallReasoningPlaceholders ensures every assistant message that
// carries tool calls also has a non-empty reasoning_content. Thinking models
// (kimi, DeepSeek) reject assistant tool-call messages without
// reasoning_content, so a placeholder is injected when no real reasoning text
// could be recovered from the Responses input.
func backfillToolCallReasoningPlaceholders(messages []dto.Message) {
	for i := range messages {
		message := &messages[i]
		if message.Role != "assistant" || len(message.ParseToolCalls()) == 0 {
			continue
		}
		if strings.TrimSpace(message.GetReasoningContent()) != "" {
			continue
		}
		placeholder := "tool call"
		message.ReasoningContent = &placeholder
	}
}

func responsesMessageContent(raw any) (any, error) {
	if raw == nil {
		return "", nil
	}
	if text, ok := raw.(string); ok {
		return text, nil
	}
	parts, ok := raw.([]any)
	if !ok {
		if part, isObject := raw.(map[string]any); isObject {
			// A single content part is valid in several compatible Responses
			// implementations. Normalize it through the same typed-part path
			// instead of serializing the whole protocol object as user text.
			return responsesMessageContent([]any{part})
		}
		return nil, fmt.Errorf("unsupported Responses message content type %T", raw)
	}

	media := make([]dto.MediaContent, 0, len(parts))
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			continue
		}
		var cacheControl json.RawMessage
		if rawCacheControl, exists := partMap["cache_control"]; exists && rawCacheControl != nil {
			cacheControl, _ = common.Marshal(rawCacheControl)
		}
		partType := strings.TrimSpace(common.Interface2String(partMap["type"]))
		switch partType {
		case "input_text", "output_text", "text":
			media = append(media, dto.MediaContent{
				Type:         dto.ContentTypeText,
				Text:         common.Interface2String(partMap["text"]),
				CacheControl: cacheControl,
			})
		case "input_image", "image_url":
			imageURL := partMap["image_url"]
			if imageURL == nil {
				imageURL = partMap["url"]
			}
			image := &dto.MessageImageUrl{Url: responsesURLString(imageURL)}
			image.Detail = common.Interface2String(partMap["detail"])
			media = append(media, dto.MediaContent{
				Type:         dto.ContentTypeImageURL,
				ImageUrl:     image,
				CacheControl: cacheControl,
			})
		case "input_audio", "audio":
			audio := partMap["input_audio"]
			if audio == nil {
				audio = partMap
			}
			media = append(media, dto.MediaContent{
				Type:         dto.ContentTypeInputAudio,
				InputAudio:   audio,
				CacheControl: cacheControl,
			})
		case "input_video", "video_url", "video":
			video := partMap["video_url"]
			if video == nil {
				video = partMap["video"]
			}
			media = append(media, dto.MediaContent{
				Type: dto.ContentTypeVideoUrl,
				VideoUrl: &dto.MessageVideoUrl{
					Url: responsesURLString(video),
				},
				CacheControl: cacheControl,
			})
		case "input_file", "file":
			file := map[string]any{}
			for _, key := range []string{"file_id", "file_data", "filename"} {
				if value, exists := partMap[key]; exists {
					file[key] = value
				}
			}
			if len(file) == 0 {
				if nested, ok := partMap["file"].(map[string]any); ok {
					file = nested
				}
			}
			media = append(media, dto.MediaContent{
				Type:         dto.ContentTypeFile,
				File:         file,
				CacheControl: cacheControl,
			})
		default:
			// Do not marshal an unknown protocol block into visible prompt text.
			// Keep a plain text field when a compatible provider adds a new block
			// type, but drop opaque structured data rather than leaking its JSON.
			if text, ok := partMap["text"].(string); ok && text != "" {
				media = append(media, dto.MediaContent{
					Type:         dto.ContentTypeText,
					Text:         text,
					CacheControl: cacheControl,
				})
			}
		}
	}
	return media, nil
}

func responsesMessageToolCalls(raw any) []dto.ToolCallRequest {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	toolCalls := make([]dto.ToolCallRequest, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		function, _ := m["function"].(map[string]any)
		toolCalls = append(toolCalls, dto.ToolCallRequest{
			ID:   common.Interface2String(m["id"]),
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      common.Interface2String(function["name"]),
				Arguments: common.Interface2String(function["arguments"]),
			},
		})
	}
	return toolCalls
}

func responsesToolsToChatTools(raw json.RawMessage) ([]dto.ToolCallRequest, *dto.WebSearchOptions, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil, nil
	}
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, nil, fmt.Errorf("invalid tools: %w", err)
	}
	tools := make([]dto.ToolCallRequest, 0, len(items))
	var webSearchOptions *dto.WebSearchOptions
	for _, item := range items {
		toolType := common.Interface2String(item["type"])
		switch toolType {
		case "function":
			tool, err := responsesFunctionToolToChatTool(item)
			if err != nil {
				return nil, nil, err
			}
			tools = append(tools, tool)
		case "web_search", "web_search_preview", "web_search_preview_2025_03_11":
			// Chat Completions exposes web search as a top-level option rather
			// than as an entry in tools. Keep the search settings while removing
			// the Responses-only tool wrapper.
			if webSearchOptions == nil {
				webSearchOptions = responsesWebSearchToolToChatOptions(item)
			} else {
				mergeResponsesWebSearchToolOptions(webSearchOptions, item)
			}
		case "namespace":
			// Chat Completions has no namespace tool type. A Responses
			// namespace is a container for function tools, so flatten its
			// functions while preserving their names. Keeping the names is
			// important because the Chat-to-Responses response converter has
			// no request-local namespace metadata with which to reconstruct a
			// renamed function call.
			nested, err := responsesNamespaceTools(item["tools"])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid Responses namespace tool %q: %w", common.Interface2String(item["name"]), err)
			}
			for _, nestedItem := range nested {
				if nestedType := common.Interface2String(nestedItem["type"]); nestedType != "function" {
					return nil, nil, fmt.Errorf(
						"responses namespace tool %q contains unsupported tool type %q",
						common.Interface2String(item["name"]),
						nestedType,
					)
				}
				tool, err := responsesFunctionToolToChatTool(nestedItem)
				if err != nil {
					return nil, nil, err
				}
				tools = append(tools, tool)
			}
		default:
			// Built-in Responses tools have no generic Chat Completions
			// equivalent. Failing explicitly is safer than silently removing
			// the user's tool and changing the request's behavior.
			return nil, nil, fmt.Errorf("responses tool type %q cannot be converted to Chat Completions", toolType)
		}
	}
	return tools, webSearchOptions, nil
}

func responsesWebSearchToolToChatOptions(item map[string]any) *dto.WebSearchOptions {
	options := &dto.WebSearchOptions{}
	mergeResponsesWebSearchToolOptions(options, item)
	return options
}

func mergeResponsesWebSearchToolOptions(options *dto.WebSearchOptions, item map[string]any) {
	if options == nil {
		return
	}
	if options.SearchContextSize == "" {
		options.SearchContextSize = common.Interface2String(item["search_context_size"])
	}
	if len(options.UserLocation) == 0 {
		if userLocation, exists := item["user_location"]; exists && userLocation != nil {
			options.UserLocation, _ = common.Marshal(userLocation)
		}
	}
}

func responsesNamespaceTools(raw any) ([]map[string]any, error) {
	if raw == nil {
		return nil, errors.New("tools must be an array")
	}
	rawTools, err := common.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("tools must be an array: %w", err)
	}
	var tools []map[string]any
	if err := common.Unmarshal(rawTools, &tools); err != nil {
		return nil, fmt.Errorf("tools must be an array: %w", err)
	}
	return tools, nil
}

func responsesFunctionToolToChatTool(item map[string]any) (dto.ToolCallRequest, error) {
	var strict json.RawMessage
	if strictValue, ok := item["strict"]; ok {
		strict, _ = common.Marshal(strictValue)
	}
	return dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        common.Interface2String(item["name"]),
			Description: common.Interface2String(item["description"]),
			Parameters:  item["parameters"],
			Strict:      strict,
		},
	}, nil
}

func responsesToolChoiceToChat(raw json.RawMessage) any {
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil
	}
	if choice, ok := value.(string); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(choice)), "web_search") {
		return nil
	}
	if choice, ok := value.(map[string]any); ok && common.Interface2String(choice["type"]) == "function" {
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": common.Interface2String(choice["name"]),
			},
		}
	}
	if choice, ok := value.(map[string]any); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(common.Interface2String(choice["type"]))), "web_search") {
		return nil
	}
	return value
}

func responsesTextToChatResponseFormat(raw json.RawMessage) *dto.ResponseFormat {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var text map[string]any
	if err := common.Unmarshal(raw, &text); err != nil {
		return nil
	}
	format, _ := text["format"].(map[string]any)
	if format == nil {
		return nil
	}
	responseFormat := &dto.ResponseFormat{Type: common.Interface2String(format["type"])}
	if responseFormat.Type == "json_schema" {
		schema := make(map[string]any)
		for _, key := range []string{"name", "description", "schema", "strict"} {
			if value, ok := format[key]; ok {
				schema[key] = value
			}
		}
		responseFormat.JsonSchema, _ = common.Marshal(schema)
	}
	return responseFormat
}

func responsesTextValue(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return responsesOutputContentText(value)
}

func responsesStringValue(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func responsesOutputContentText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if items, ok := value.([]any); ok {
		var builder strings.Builder
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				text := common.Interface2String(m["text"])
				if text != "" {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	}
	return ""
}

func responsesValueString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	data, err := common.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func responsesArgumentsString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return responsesValueString(value)
}

func responsesURLString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if m, ok := value.(map[string]any); ok {
		return common.Interface2String(m["url"])
	}
	return ""
}

func setChatMessageContent(message *dto.Message, content any) {
	if message == nil {
		return
	}
	if text, ok := content.(string); ok {
		message.SetStringContent(text)
		return
	}
	if media, ok := content.([]dto.MediaContent); ok {
		message.SetMediaContent(media)
		return
	}
	message.SetStringContent(responsesValueString(content))
}
