package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestGetEnabledKeyByPreferredIndexUsesStableSlot(t *testing.T) {
	channel := &Channel{
		Key: "key-a\nkey-b\nkey-c",
		ChannelInfo: ChannelInfo{
			IsMultiKey: true,
		},
	}

	key, index, err := channel.GetEnabledKeyByPreferredIndex(1)
	if err != nil {
		t.Fatalf("GetEnabledKeyByPreferredIndex returned error: %v", err)
	}
	if key != "key-b" || index != 1 {
		t.Fatalf("unexpected preferred key: key=%q index=%d", key, index)
	}
}

func TestGetEnabledKeyByPreferredIndexSkipsDisabledSlots(t *testing.T) {
	channel := &Channel{
		Key: "key-a\nkey-b\nkey-c",
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled, 2: common.ChannelStatusEnabled},
		},
	}

	key, index, err := channel.GetEnabledKeyByPreferredIndex(1)
	if err != nil {
		t.Fatalf("GetEnabledKeyByPreferredIndex returned error: %v", err)
	}
	if key != "key-c" || index != 2 {
		t.Fatalf("expected disabled preferred slot to advance, got key=%q index=%d", key, index)
	}

	if status := channel.ChannelInfo.MultiKeyStatusList[1]; status != common.ChannelStatusManuallyDisabled {
		t.Fatalf("status %d was unexpectedly changed", status)
	}
}
