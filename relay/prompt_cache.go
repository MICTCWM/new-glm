package relay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func responsesPromptCacheKeyString(raw json.RawMessage) string {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return ""
	}
	var key string
	if err := json.Unmarshal(raw, &key); err != nil {
		return ""
	}
	return strings.TrimSpace(key)
}

func addPromptCacheKeyToRawBody(body []byte, key string) ([]byte, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return body, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return body, nil
	}
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	payload["prompt_cache_key"] = keyJSON
	return json.Marshal(payload)
}

func supportsAutomaticOpenAIPromptCacheKey(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.ChannelMeta != nil &&
		info.ChannelType == constant.ChannelTypeOpenAI &&
		info.ApiType == constant.APITypeOpenAI
}

func automaticOpenAIPromptCacheKey(c *gin.Context, info *relaycommon.RelayInfo) string {
	if !supportsAutomaticOpenAIPromptCacheKey(info) {
		return ""
	}
	routeKey, ok := service.GetChannelAffinityRouteKey(c)
	if !ok || strings.TrimSpace(routeKey) == "" {
		return ""
	}
	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	// Do not include the local channel ID. A retry may move to another
	// channel that reaches the same upstream project; keeping the key stable
	// preserves the provider-side cache partition across that transition.
	identity := fmt.Sprintf("%s:%s", modelName, routeKey)
	digest := common.Sha1([]byte(identity))
	if len(digest) > 40 {
		digest = digest[:40]
	}
	return "new-api-" + digest
}

// ensureOpenAIPromptCacheKey adds a stable key only for the native OpenAI
// upstream. Other OpenAI-compatible providers may reject this field, so they
// must continue to receive the original request shape.
func ensureOpenAIPromptCacheKey(c *gin.Context, info *relaycommon.RelayInfo, body []byte, explicitKey string) ([]byte, error) {
	if !supportsAutomaticOpenAIPromptCacheKey(info) || len(body) == 0 {
		return body, nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return body, nil
	}
	if raw, exists := payload["prompt_cache_key"]; exists {
		var existing string
		if json.Unmarshal(raw, &existing) == nil && strings.TrimSpace(existing) != "" {
			return body, nil
		}
	}

	key := strings.TrimSpace(explicitKey)
	if key == "" {
		key = automaticOpenAIPromptCacheKey(c, info)
	}
	if key == "" {
		return body, nil
	}
	return addPromptCacheKeyToRawBody(body, key)
}

// addClaudePromptCacheBreakpointToRawBody adds the same history-aware cache
// boundary used by the normalized Claude path while preserving unknown
// top-level request fields from pass-through clients.
func addClaudePromptCacheBreakpointToRawBody(body []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	var request dto.ClaudeRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	if !ensureClaudePromptCacheBreakpoint(&request) {
		return body, nil
	}

	if request.System != nil {
		system, err := json.Marshal(request.System)
		if err != nil {
			return nil, err
		}
		payload["system"] = system
	}
	if len(request.Messages) > 0 {
		messages, err := json.Marshal(request.Messages)
		if err != nil {
			return nil, err
		}
		payload["messages"] = messages
	}
	return json.Marshal(payload)
}

var claudeEphemeralCacheControl = json.RawMessage(`{"type":"ephemeral"}`)

func claudeCacheControlCount(request *dto.ClaudeRequest) int {
	if request == nil {
		return 0
	}

	count := 0
	if request.System != nil && !request.IsStringSystem() {
		for _, block := range request.ParseSystem() {
			if len(block.CacheControl) > 0 {
				count++
			}
		}
	}
	for _, message := range request.Messages {
		if message.Content == nil || message.IsStringContent() {
			continue
		}
		blocks, _ := message.ParseContent()
		for _, block := range blocks {
			if len(block.CacheControl) > 0 {
				count++
			}
		}
	}
	count += claudeToolCacheControlCount(request.Tools)
	return count
}

func claudeToolCacheControlCount(tools any) int {
	if tools == nil {
		return 0
	}

	rawTools, err := json.Marshal(tools)
	if err != nil {
		return 0
	}

	var toolList []map[string]json.RawMessage
	if err := json.Unmarshal(rawTools, &toolList); err != nil {
		return 0
	}

	count := 0
	for _, tool := range toolList {
		if raw, ok := tool["cache_control"]; ok && len(raw) > 0 && string(raw) != "null" {
			count++
		}
	}
	return count
}

func addClaudeCacheControlToLastUncachedBlock(blocks []dto.ClaudeMediaMessage) bool {
	for i := len(blocks) - 1; i >= 0; i-- {
		if len(blocks[i].CacheControl) > 0 {
			continue
		}
		blocks[i].CacheControl = append(json.RawMessage(nil), claudeEphemeralCacheControl...)
		return true
	}
	return false
}

// ensureClaudePromptCacheBreakpoint adds one stable Anthropic cache boundary
// when the client did not provide enough boundaries itself. The current user
// message is skipped when there is previous history, so changing the current
// prompt does not invalidate the cached conversation prefix.
func ensureClaudePromptCacheBreakpoint(request *dto.ClaudeRequest) bool {
	if request == nil || claudeCacheControlCount(request) >= 4 {
		return false
	}

	added := false
	if request.System != nil {
		if request.IsStringSystem() {
			block := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
			block.SetText(request.GetStringSystem())
			block.CacheControl = append(json.RawMessage(nil), claudeEphemeralCacheControl...)
			request.System = []dto.ClaudeMediaMessage{block}
			added = true
		} else {
			systemBlocks := request.ParseSystem()
			if addClaudeCacheControlToLastUncachedBlock(systemBlocks) {
				request.System = systemBlocks
				added = true
			}
		}
	}
	if claudeCacheControlCount(request) >= 4 {
		return added
	}

	start := len(request.Messages) - 1
	if start > 0 && request.Messages[start].Role == "user" {
		start--
	}
	for i := start; i >= 0; i-- {
		message := &request.Messages[i]
		if message.Content == nil {
			continue
		}
		if message.IsStringContent() {
			block := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
			block.SetText(message.GetStringContent())
			block.CacheControl = append(json.RawMessage(nil), claudeEphemeralCacheControl...)
			message.Content = []dto.ClaudeMediaMessage{block}
			return true
		}
		blocks, _ := message.ParseContent()
		if addClaudeCacheControlToLastUncachedBlock(blocks) {
			message.Content = blocks
			return true
		}
	}
	return added
}
