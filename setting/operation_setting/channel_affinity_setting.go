package operation_setting

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

type ChannelAffinityKeySource struct {
	Type string `json:"type"` // context_int, context_string, gjson
	Key  string `json:"key,omitempty"`
	Path string `json:"path,omitempty"`
}

type ChannelAffinityRule struct {
	Name             string                     `json:"name"`
	ModelRegex       []string                   `json:"model_regex"`
	PathRegex        []string                   `json:"path_regex"`
	UserAgentInclude []string                   `json:"user_agent_include,omitempty"`
	KeySources       []ChannelAffinityKeySource `json:"key_sources"`

	ValueRegex string `json:"value_regex"`
	TTLSeconds int    `json:"ttl_seconds"`

	ParamOverrideTemplate map[string]interface{} `json:"param_override_template,omitempty"`

	SkipRetryOnFailure bool `json:"skip_retry_on_failure"`

	IncludeUsingGroup bool `json:"include_using_group"`
	IncludeModelName  bool `json:"include_model_name"`
	IncludeRuleName   bool `json:"include_rule_name"`
}

type ChannelAffinitySetting struct {
	Enabled           bool                  `json:"enabled"`
	SwitchOnSuccess   bool                  `json:"switch_on_success"`
	MaxEntries        int                   `json:"max_entries"`
	DefaultTTLSeconds int                   `json:"default_ttl_seconds"`
	AllowedChannelIDs []int                 `json:"allowed_channel_ids"`
	Rules             []ChannelAffinityRule `json:"rules"`
}

var codexCliPassThroughHeaders = []string{
	"Originator",
	"Session_id",
	"User-Agent",
	"X-Codex-Beta-Features",
	"X-Codex-Turn-Metadata",
}

var claudeCliPassThroughHeaders = []string{
	"X-Stainless-Arch",
	"X-Stainless-Lang",
	"X-Stainless-Os",
	"X-Stainless-Package-Version",
	"X-Stainless-Retry-Count",
	"X-Stainless-Runtime",
	"X-Stainless-Runtime-Version",
	"X-Stainless-Timeout",
	"User-Agent",
	"X-App",
	"Anthropic-Beta",
	"Anthropic-Dangerous-Direct-Browser-Access",
	"Anthropic-Version",
}

func buildPassHeaderTemplate(headers []string) map[string]interface{} {
	clonedHeaders := make([]string, 0, len(headers))
	clonedHeaders = append(clonedHeaders, headers...)
	return map[string]interface{}{
		"operations": []map[string]interface{}{
			{
				"mode":        "pass_headers",
				"value":       clonedHeaders,
				"keep_origin": true,
			},
		},
	}
}

// defaultAPITokenStableRouteKeySources keeps authenticated identity behind
// explicit conversation/session identifiers, but ahead of request-body user
// fields. Some clients reuse metadata.user_id as a per-request identifier;
// letting it win would continuously split one user's upstream cache route.
func defaultAPITokenStableRouteKeySources() []ChannelAffinityKeySource {
	return []ChannelAffinityKeySource{
		{Type: "gjson", Path: "prompt_cache_key"},
		{Type: "header", Key: "Session_id"},
		{Type: "header", Key: "X-Session-Id"},
		{Type: "header", Key: "X-Conversation-Id"},
		{Type: "header", Key: "Conversation-Id"},
		{Type: "header", Key: "X-Thread-Id"},
		{Type: "header", Key: "Thread-Id"},
		{Type: "gjson", Path: "metadata.conversation_id"},
		{Type: "gjson", Path: "metadata.session_id"},
		{Type: "gjson", Path: "conversation_id"},
		{Type: "gjson", Path: "conversation.id"},
		{Type: "gjson", Path: "thread_id"},
		{Type: "gjson", Path: "session_id"},
		{Type: "context_int", Key: "id"},
		{Type: "context_int", Key: "token_id"},
		{Type: "gjson", Path: "metadata.user_id"},
		{Type: "gjson", Path: "metadata.userId"},
		{Type: "gjson", Path: "user_id"},
	}
}

func normalizeDefaultAPITokenStableRouteKeySources(sources []ChannelAffinityKeySource) []ChannelAffinityKeySource {
	canonical := defaultAPITokenStableRouteKeySources()
	known := make(map[string]struct{}, len(canonical))
	for _, source := range canonical {
		known[source.Type+"\x00"+source.Key+"\x00"+source.Path] = struct{}{}
	}

	ordered := make([]ChannelAffinityKeySource, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range canonical {
		key := source.Type + "\x00" + source.Key + "\x00" + source.Path
		for _, configured := range sources {
			configuredKey := configured.Type + "\x00" + configured.Key + "\x00" + configured.Path
			if configuredKey == key {
				if _, exists := seen[configuredKey]; !exists {
					ordered = append(ordered, configured)
					seen[configuredKey] = struct{}{}
				}
			}
		}
	}
	for _, source := range sources {
		key := source.Type + "\x00" + source.Key + "\x00" + source.Path
		if _, exists := known[key]; exists {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		ordered = append(ordered, source)
		seen[key] = struct{}{}
	}
	return ordered
}

var channelAffinitySetting = ChannelAffinitySetting{
	Enabled:           true,
	SwitchOnSuccess:   true,
	MaxEntries:        100_000,
	DefaultTTLSeconds: 3600,
	AllowedChannelIDs: []int{},
	Rules: []ChannelAffinityRule{
		{
			Name:       "codex cli trace",
			ModelRegex: []string{"^gpt-.*$"},
			PathRegex:  []string{"/v1/responses", "/v1/chat/completions"},
			KeySources: []ChannelAffinityKeySource{
				{Type: "gjson", Path: "prompt_cache_key"},
				{Type: "header", Key: "Session_id"},
			},
			ValueRegex:            "",
			TTLSeconds:            0,
			ParamOverrideTemplate: buildPassHeaderTemplate(codexCliPassThroughHeaders),
			SkipRetryOnFailure:    true,
			IncludeUsingGroup:     true,
			IncludeRuleName:       true,
			UserAgentInclude:      nil,
		},
		{
			Name:       "claude cli trace",
			ModelRegex: []string{"^claude-.*$"},
			PathRegex:  []string{"/v1/messages"},
			KeySources: []ChannelAffinityKeySource{
				{Type: "gjson", Path: "metadata.user_id"},
			},
			ValueRegex:            "",
			TTLSeconds:            0,
			ParamOverrideTemplate: buildPassHeaderTemplate(claudeCliPassThroughHeaders),
			SkipRetryOnFailure:    true,
			IncludeUsingGroup:     true,
			IncludeRuleName:       true,
			UserAgentInclude:      nil,
		},
		{
			// Most OpenAI-compatible clients do not send an explicit
			// prompt_cache_key. Keep the same user's requests on one
			// upstream channel so the provider's automatic prefix cache
			// does not get split by channel rotation.
			Name:               "api token stable route",
			ModelRegex:         []string{"^.+$"},
			PathRegex:          []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"},
			KeySources:         defaultAPITokenStableRouteKeySources(),
			TTLSeconds:         0,
			SkipRetryOnFailure: false,
			IncludeUsingGroup:  true,
			IncludeModelName:   true,
			IncludeRuleName:    true,
			UserAgentInclude:   nil,
		},
	},
}

var channelAffinityDefaultsMu sync.Mutex

func defaultAPITokenStableRouteRule() ChannelAffinityRule {
	return ChannelAffinityRule{
		Name:               "api token stable route",
		ModelRegex:         []string{"^.+$"},
		PathRegex:          []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"},
		KeySources:         defaultAPITokenStableRouteKeySources(),
		TTLSeconds:         0,
		SkipRetryOnFailure: false,
		IncludeUsingGroup:  true,
		IncludeModelName:   true,
		IncludeRuleName:    true,
		UserAgentInclude:   nil,
	}
}

func ensureDefaultChannelAffinityRules() {
	channelAffinityDefaultsMu.Lock()
	defer channelAffinityDefaultsMu.Unlock()

	for i := range channelAffinitySetting.Rules {
		rule := &channelAffinitySetting.Rules[i]
		if !strings.EqualFold(strings.TrimSpace(rule.Name), "api token stable route") {
			continue
		}
		rule.KeySources = normalizeDefaultAPITokenStableRouteKeySources(rule.KeySources)

		hasPromptCacheKey := false
		hasUserID := false
		for _, source := range rule.KeySources {
			if source.Type == "gjson" && source.Path == "prompt_cache_key" {
				hasPromptCacheKey = true
			}
			if source.Type == "context_int" && source.Key == "id" {
				hasUserID = true
			}
		}
		if !hasPromptCacheKey {
			rule.KeySources = append([]ChannelAffinityKeySource{
				{Type: "gjson", Path: "prompt_cache_key"},
			}, rule.KeySources...)
		}
		if !hasUserID {
			userSource := ChannelAffinityKeySource{Type: "context_int", Key: "id"}
			insertAt := len(rule.KeySources)
			for i, source := range rule.KeySources {
				if source.Type == "context_int" && source.Key == "token_id" {
					insertAt = i
					break
				}
			}
			rule.KeySources = append(rule.KeySources, ChannelAffinityKeySource{})
			copy(rule.KeySources[insertAt+1:], rule.KeySources[insertAt:])
			rule.KeySources[insertAt] = userSource
		}

		requiredSources := []ChannelAffinityKeySource{
			{Type: "header", Key: "Session_id"},
			{Type: "header", Key: "X-Session-Id"},
			{Type: "header", Key: "X-Conversation-Id"},
			{Type: "header", Key: "Conversation-Id"},
			{Type: "header", Key: "X-Thread-Id"},
			{Type: "header", Key: "Thread-Id"},
			{Type: "gjson", Path: "metadata.conversation_id"},
			{Type: "gjson", Path: "metadata.session_id"},
			{Type: "gjson", Path: "metadata.user_id"},
			{Type: "gjson", Path: "metadata.userId"},
			{Type: "gjson", Path: "conversation_id"},
			{Type: "gjson", Path: "conversation.id"},
			{Type: "gjson", Path: "thread_id"},
			{Type: "gjson", Path: "session_id"},
			{Type: "gjson", Path: "user_id"},
		}
		present := make(map[string]struct{}, len(rule.KeySources))
		for _, source := range rule.KeySources {
			present[source.Type+"\x00"+source.Key+"\x00"+source.Path] = struct{}{}
		}
		missing := make([]ChannelAffinityKeySource, 0, len(requiredSources))
		for _, source := range requiredSources {
			if _, exists := present[source.Type+"\x00"+source.Key+"\x00"+source.Path]; !exists {
				missing = append(missing, source)
			}
		}
		if len(missing) > 0 {
			insertAt := len(rule.KeySources)
			for i, source := range rule.KeySources {
				if source.Type == "context_int" && (source.Key == "id" || source.Key == "token_id") {
					insertAt = i
					break
				}
			}
			updated := make([]ChannelAffinityKeySource, 0, len(rule.KeySources)+len(missing))
			updated = append(updated, rule.KeySources[:insertAt]...)
			updated = append(updated, missing...)
			updated = append(updated, rule.KeySources[insertAt:]...)
			rule.KeySources = updated
			rule.KeySources = normalizeDefaultAPITokenStableRouteKeySources(rule.KeySources)
		}
		return
	}

	channelAffinitySetting.Rules = append(
		channelAffinitySetting.Rules,
		defaultAPITokenStableRouteRule(),
	)
}

func init() {
	config.GlobalConfig.Register("channel_affinity_setting", &channelAffinitySetting)
}

func GetChannelAffinitySetting() *ChannelAffinitySetting {
	ensureDefaultChannelAffinityRules()
	return &channelAffinitySetting
}
