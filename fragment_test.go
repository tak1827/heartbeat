package heartbeat

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMoveFrontReassemblyFragments(t *testing.T) {
	var f *fragmentReassemblyData

	e := setDefaultOptions()
	e.rFragments[0] = &fragmentReassemblyData{seq: 10}
	e.rFragments[1] = &fragmentReassemblyData{seq: 13}
	e.rFragments[2] = &fragmentReassemblyData{seq: 12}
	e.rFragments[3] = &fragmentReassemblyData{seq: 11}
	e.rFragments[4] = &fragmentReassemblyData{seq: 14}

	moveFrontReassemblyFragments(e, 10)
	require.EqualValues(t, uint16(10), e.rFragments[0].seq)

	moveFrontReassemblyFragments(e, 11)
	require.EqualValues(t, uint16(11), e.rFragments[0].seq)

	moveFrontReassemblyFragments(e, 14)
	require.EqualValues(t, uint16(14), e.rFragments[0].seq)

	moveFrontReassemblyFragments(e, 15)
	require.EqualValues(t, f, e.rFragments[0])
}
