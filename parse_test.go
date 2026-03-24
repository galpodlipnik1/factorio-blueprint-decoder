package bpdecode

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBlueprintLibrary_EmptyData(t *testing.T) {
	entries, err := ParseBlueprintLibrary(nil)

	require.Nil(t, entries)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read library version")
}

func TestParseBlueprintLibrary_TruncatedData(t *testing.T) {
	entries, err := ParseBlueprintLibrary([]byte{0x02, 0x00, 0x00})

	require.Nil(t, entries)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read library version")
}
