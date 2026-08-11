package common

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

var (
	overloadProtectionChannelIDs   = make(map[int]struct{})
	overloadProtectionChannelIDsMu sync.RWMutex
)

// ParseOverloadProtectionChannelIDs validates the persisted channel ID list.
// The option intentionally uses JSON so it can be shared by both management
// frontends without relying on an ambiguous comma-separated representation.
func ParseOverloadProtectionChannelIDs(value string) ([]int, error) {
	var ids []int
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("invalid overload protection channel IDs: %w", err)
	}

	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("overload protection channel ID must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

// UpdateOverloadProtectionChannelIDs replaces the set of channels whose RPM
// contributes to overload protection.
func UpdateOverloadProtectionChannelIDs(value string) error {
	ids, err := ParseOverloadProtectionChannelIDs(value)
	if err != nil {
		return err
	}

	next := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		next[id] = struct{}{}
	}

	overloadProtectionChannelIDsMu.Lock()
	overloadProtectionChannelIDs = next
	overloadProtectionChannelIDsMu.Unlock()
	return nil
}

// IsOverloadProtectionChannel reports whether a channel participates in the
// shared overload-protection RPM budget.
func IsOverloadProtectionChannel(channelID int) bool {
	overloadProtectionChannelIDsMu.RLock()
	_, exists := overloadProtectionChannelIDs[channelID]
	overloadProtectionChannelIDsMu.RUnlock()
	return exists
}

func OverloadProtectionChannelIDsJSONString() string {
	overloadProtectionChannelIDsMu.RLock()
	ids := make([]int, 0, len(overloadProtectionChannelIDs))
	for id := range overloadProtectionChannelIDs {
		ids = append(ids, id)
	}
	overloadProtectionChannelIDsMu.RUnlock()

	sort.Ints(ids)
	bytes, _ := json.Marshal(ids)
	return string(bytes)
}

var (
	reassuranceChannelIDs   = make(map[int]struct{})
	reassuranceChannelIDsMu sync.RWMutex
)

// ParseReassuranceChannelIDs validates the persisted channel ID list that is
// allowed to show reassurance messages (排队安抚性语言 / 硬推理提示) while waiting.
// The option intentionally uses JSON so it can be shared by both management
// frontends without relying on an ambiguous comma-separated representation.
func ParseReassuranceChannelIDs(value string) ([]int, error) {
	var ids []int
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("invalid reassurance channel IDs: %w", err)
	}

	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("reassurance channel ID must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

// UpdateReassuranceChannelIDs replaces the set of channels that may show
// reassurance messages while queued.
func UpdateReassuranceChannelIDs(value string) error {
	ids, err := ParseReassuranceChannelIDs(value)
	if err != nil {
		return err
	}

	next := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		next[id] = struct{}{}
	}

	reassuranceChannelIDsMu.Lock()
	reassuranceChannelIDs = next
	reassuranceChannelIDsMu.Unlock()
	return nil
}

// IsReassuranceChannel reports whether a channel is allowed to show reassurance
// messages (排队安抚性语言 / 硬推理提示) while waiting.
func IsReassuranceChannel(channelID int) bool {
	reassuranceChannelIDsMu.RLock()
	_, exists := reassuranceChannelIDs[channelID]
	reassuranceChannelIDsMu.RUnlock()
	return exists
}

func ReassuranceChannelIDsJSONString() string {
	reassuranceChannelIDsMu.RLock()
	ids := make([]int, 0, len(reassuranceChannelIDs))
	for id := range reassuranceChannelIDs {
		ids = append(ids, id)
	}
	reassuranceChannelIDsMu.RUnlock()

	sort.Ints(ids)
	bytes, _ := json.Marshal(ids)
	return string(bytes)
}

// LimitedInputTokenMaxTokens is the maximum estimated prompt token count a
// limited-input-token channel accepts. Channels not selected in
// LimitedInputTokenChannelIds are unaffected and may receive larger inputs.
const LimitedInputTokenMaxTokens = 360000

var (
	limitedInputTokenChannelIDs   = make(map[int]struct{})
	limitedInputTokenChannelIDsMu sync.RWMutex
)

// ParseLimitedInputTokenChannelIDs validates the persisted channel ID list
// whose input is capped at LimitedInputTokenMaxTokens. The option uses JSON so
// both management frontends can share it unambiguously.
func ParseLimitedInputTokenChannelIDs(value string) ([]int, error) {
	var ids []int
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("invalid limited input-token channel IDs: %w", err)
	}

	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("limited input-token channel ID must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

// UpdateLimitedInputTokenChannelIDs replaces the set of channels whose input is
// capped at LimitedInputTokenMaxTokens.
func UpdateLimitedInputTokenChannelIDs(value string) error {
	ids, err := ParseLimitedInputTokenChannelIDs(value)
	if err != nil {
		return err
	}

	next := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		next[id] = struct{}{}
	}

	limitedInputTokenChannelIDsMu.Lock()
	limitedInputTokenChannelIDs = next
	limitedInputTokenChannelIDsMu.Unlock()
	return nil
}

// IsLimitedInputTokenChannel reports whether a channel's input is capped at
// LimitedInputTokenMaxTokens.
func IsLimitedInputTokenChannel(channelID int) bool {
	limitedInputTokenChannelIDsMu.RLock()
	_, exists := limitedInputTokenChannelIDs[channelID]
	limitedInputTokenChannelIDsMu.RUnlock()
	return exists
}

func LimitedInputTokenChannelIDsJSONString() string {
	limitedInputTokenChannelIDsMu.RLock()
	ids := make([]int, 0, len(limitedInputTokenChannelIDs))
	for id := range limitedInputTokenChannelIDs {
		ids = append(ids, id)
	}
	limitedInputTokenChannelIDsMu.RUnlock()

	sort.Ints(ids)
	bytes, _ := json.Marshal(ids)
	return string(bytes)
}
