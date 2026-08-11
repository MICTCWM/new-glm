package common

import (
	"testing"
)

func TestLimitedInputTokenChannelIDsLifecycle(t *testing.T) {
	// Start from a clean state.
	if err := UpdateLimitedInputTokenChannelIDs("[]"); err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	if IsLimitedInputTokenChannel(1) {
		t.Fatal("channel 1 should not be limited initially")
	}

	// Invalid payloads are rejected.
	if err := UpdateLimitedInputTokenChannelIDs("not-json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if err := UpdateLimitedInputTokenChannelIDs("[0, -3]"); err == nil {
		t.Fatal("expected error for non-positive IDs")
	}

	// Duplicates are normalized and persisted.
	if err := UpdateLimitedInputTokenChannelIDs("[7, 7, 9]"); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !IsLimitedInputTokenChannel(7) || !IsLimitedInputTokenChannel(9) {
		t.Fatal("expected channels 7 and 9 to be limited")
	}
	if IsLimitedInputTokenChannel(1) {
		t.Fatal("channel 1 must remain unlimited")
	}

	// JSON round-trips and is sorted.
	got := LimitedInputTokenChannelIDsJSONString()
	if got != "[7,9]" {
		t.Fatalf("unexpected JSON %q, want [7,9]", got)
	}

	// Clearing works.
	if err := UpdateLimitedInputTokenChannelIDs("[]"); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if IsLimitedInputTokenChannel(7) {
		t.Fatal("channel 7 should be unlimited after clear")
	}
}

func TestLimitedInputTokenMaxTokensConstant(t *testing.T) {
	if LimitedInputTokenMaxTokens != 360000 {
		t.Fatalf("expected cap 360000, got %d", LimitedInputTokenMaxTokens)
	}
}
