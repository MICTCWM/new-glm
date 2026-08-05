package model

import (
	"encoding/json"
	"strings"
)

type channelSettingFlags struct {
	EmergencyPlanEnabled bool `json:"emergency_plan_enabled,omitempty"`
	FallbackModelEnabled bool `json:"fallback_model_enabled,omitempty"`
	FallbackPriority     int  `json:"fallback_priority,omitempty"`
	SupportFallback      bool `json:"support_fallback,omitempty"`
}

func isEmergencyPlanEnabledSetting(setting *string) bool {
	if setting == nil {
		return false
	}
	trimmed := strings.TrimSpace(*setting)
	if trimmed == "" {
		return false
	}

	var flags channelSettingFlags
	if err := json.Unmarshal([]byte(trimmed), &flags); err != nil {
		return false
	}
	return flags.EmergencyPlanEnabled
}

func isFallbackModelEnabledSetting(setting *string) bool {
	if setting == nil {
		return false
	}
	trimmed := strings.TrimSpace(*setting)
	if trimmed == "" {
		return false
	}
	var flags channelSettingFlags
	if err := json.Unmarshal([]byte(trimmed), &flags); err != nil {
		return false
	}
	return flags.FallbackModelEnabled
}

func getFallbackPrioritySetting(setting *string) int {
	if setting == nil {
		return 0
	}
	trimmed := strings.TrimSpace(*setting)
	if trimmed == "" {
		return 0
	}
	var flags channelSettingFlags
	if err := json.Unmarshal([]byte(trimmed), &flags); err != nil {
		return 0
	}
	return flags.FallbackPriority
}

func isSupportFallbackSetting(setting *string) bool {
	if setting == nil {
		return false
	}
	trimmed := strings.TrimSpace(*setting)
	if trimmed == "" {
		return false
	}
	var flags channelSettingFlags
	if err := json.Unmarshal([]byte(trimmed), &flags); err != nil {
		return false
	}
	return flags.SupportFallback
}
