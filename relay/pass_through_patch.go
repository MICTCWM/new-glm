package relay

import "encoding/json"

// patchChangedJSONFields keeps pass-through-only fields while applying changes
// made to the typed request by model/prompt adapters.
func patchChangedJSONFields(body []byte, before any, after any) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return body, nil
	}

	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return nil, err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return nil, err
	}
	var beforeFields, afterFields map[string]json.RawMessage
	if err = json.Unmarshal(beforeJSON, &beforeFields); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(afterJSON, &afterFields); err != nil {
		return nil, err
	}

	for key := range beforeFields {
		if _, exists := afterFields[key]; exists {
			continue
		}
		delete(payload, passThroughFieldKey(payload, key))
	}
	for key, value := range afterFields {
		if rawBefore, exists := beforeFields[key]; exists && jsonEqual(rawBefore, value) {
			continue
		}
		fieldKey := passThroughFieldKey(payload, key)
		if merged, ok := mergePassThroughArrays(payload[fieldKey], value); ok {
			payload[fieldKey] = merged
			continue
		}
		if merged, ok := mergePassThroughObjects(payload[fieldKey], value); ok {
			payload[fieldKey] = merged
		} else {
			payload[fieldKey] = value
		}
	}
	return json.Marshal(payload)
}

func passThroughFieldKey(payload map[string]json.RawMessage, key string) string {
	aliases := map[string]string{
		"systemInstruction": "system_instruction",
		"toolConfig":        "tool_config",
		"generationConfig":  "generation_config",
		"cachedContent":     "cached_content",
	}
	if _, exists := payload[key]; !exists {
		if alias, ok := aliases[key]; ok {
			if _, exists := payload[alias]; exists {
				return alias
			}
		}
	}
	return key
}

func mergePassThroughObjects(existing, replacement json.RawMessage) (json.RawMessage, bool) {
	var oldObject, newObject map[string]json.RawMessage
	if json.Unmarshal(existing, &oldObject) != nil || json.Unmarshal(replacement, &newObject) != nil || oldObject == nil || newObject == nil {
		return nil, false
	}
	for key, value := range newObject {
		if merged, ok := mergePassThroughObjects(oldObject[key], value); ok {
			oldObject[key] = merged
		} else if merged, ok := mergePassThroughArrays(oldObject[key], value); ok {
			oldObject[key] = merged
			continue
		} else {
			oldObject[key] = value
		}
	}
	merged, err := json.Marshal(oldObject)
	if err != nil {
		return nil, false
	}
	return merged, true
}

func mergePassThroughArrays(existing, replacement json.RawMessage) (json.RawMessage, bool) {
	var oldItems, newItems []json.RawMessage
	if json.Unmarshal(existing, &oldItems) != nil || json.Unmarshal(replacement, &newItems) != nil {
		return nil, false
	}
	if len(oldItems) == len(newItems) {
		for i := range newItems {
			if merged, ok := mergePassThroughObjects(oldItems[i], newItems[i]); ok {
				newItems[i] = merged
			}
		}
		merged, err := json.Marshal(newItems)
		return merged, err == nil
	}

	// System prompt injection commonly prepends one message. Align the old
	// sequence with the unchanged suffix/prefix so unknown message fields stay.
	if len(newItems) == len(oldItems)+1 {
		bestOffset, bestMatches := -1, -1
		for offset := 0; offset <= 1; offset++ {
			matches := 0
			for oldIndex := range oldItems {
				if jsonSubsetEqual(newItems[oldIndex+offset], oldItems[oldIndex]) {
					matches++
				}
			}
			if matches > bestMatches {
				bestOffset, bestMatches = offset, matches
			}
		}
		if bestOffset >= 0 && bestMatches >= len(oldItems)-1 {
			mergedItems := make([]json.RawMessage, 0, len(newItems))
			for newIndex, item := range newItems {
				oldIndex := newIndex - bestOffset
				if oldIndex >= 0 && oldIndex < len(oldItems) {
					if merged, ok := mergePassThroughObjects(oldItems[oldIndex], item); ok {
						item = merged
					}
				}
				mergedItems = append(mergedItems, item)
			}
			merged, err := json.Marshal(mergedItems)
			return merged, err == nil
		}
	}
	return nil, false
}

func jsonSubsetEqual(candidate, source json.RawMessage) bool {
	var candidateObject, sourceObject map[string]json.RawMessage
	if json.Unmarshal(candidate, &candidateObject) == nil && json.Unmarshal(source, &sourceObject) == nil && candidateObject != nil && sourceObject != nil {
		for key, value := range candidateObject {
			sourceValue, exists := sourceObject[key]
			if !exists || !jsonEqual(value, sourceValue) {
				return false
			}
		}
		return true
	}
	return jsonEqual(candidate, source)
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return string(left) == string(right)
	}
	return deepEqualJSON(leftValue, rightValue)
}

func deepEqualJSON(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}
