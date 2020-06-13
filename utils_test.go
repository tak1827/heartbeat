package heartbeat

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSearchUint8(t *testing.T) {
	var (
		i  int
		ok bool
	)

	var data []uint8 = []uint8{1, 2, 3, 5}

	i, ok = searchUint8(1, data)
	require.True(t, ok)
	require.EqualValues(t, 0, i)

	_, ok = searchUint8(4, data)
	require.False(t, ok)
}
