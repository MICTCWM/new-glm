package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateOverloadProtectionChannelIDs(t *testing.T) {
	original := OverloadProtectionChannelIDsJSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateOverloadProtectionChannelIDs(original))
	})

	require.NoError(t, UpdateOverloadProtectionChannelIDs("[7,3,7]"))
	require.True(t, IsOverloadProtectionChannel(3))
	require.True(t, IsOverloadProtectionChannel(7))
	require.False(t, IsOverloadProtectionChannel(8))

	_, err := ParseOverloadProtectionChannelIDs("[1,0]")
	require.Error(t, err)
	_, err = ParseOverloadProtectionChannelIDs("not-json")
	require.Error(t, err)
}
