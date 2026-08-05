package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortFallbackChannelsByFallbackPriority(t *testing.T) {
	channels := []*Channel{
		{Id: 1, Setting: stringPointer(`{"fallback_priority":1}`)},
		{Id: 2, Setting: stringPointer(`{"fallback_priority":10}`)},
		{Id: 3, Setting: stringPointer(`{"fallback_priority":5}`)},
	}

	sortFallbackChannelsByPriority(channels)

	require.Equal(t, []int{2, 3, 1}, []int{channels[0].Id, channels[1].Id, channels[2].Id})
}

func TestSortFallbackChannelsUsesChannelPriorityAsTieBreaker(t *testing.T) {
	priorityOne := int64(1)
	priorityTwo := int64(2)
	channels := []*Channel{
		{Id: 1, Priority: &priorityOne, Setting: stringPointer(`{"fallback_priority":5}`)},
		{Id: 2, Priority: &priorityTwo, Setting: stringPointer(`{"fallback_priority":5}`)},
	}

	sortFallbackChannelsByPriority(channels)

	require.Equal(t, 2, channels[0].Id)
}

func stringPointer(value string) *string {
	return &value
}
