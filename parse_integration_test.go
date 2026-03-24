//go:build integration

package bpdecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBlueprintLibrary_ParsesFixture(t *testing.T) {
	data := readIntegrationFixture(t)

	entries, err := ParseBlueprintLibrary(data)
	require.NoError(t, err)
	require.Len(t, entries, 3411)

	names := entryNames(entries)
	require.Contains(t, names, "Modules")
	require.Contains(t, names, "Speed Module Line")
}

func TestParseBlueprintLibrary_RendersFixtureOutputs(t *testing.T) {
	data := readIntegrationFixture(t)

	entries, err := ParseBlueprintLibrary(data)
	require.NoError(t, err)

	lua := RenderLuaModule(entries)
	require.Contains(t, lua, "return {")
	require.Contains(t, lua, "entries = {")

	payload, err := RenderJSON(entries)
	require.NoError(t, err)

	var module struct {
		Entries []Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(payload, &module))
	require.Len(t, module.Entries, len(entries))
}

func readIntegrationFixture(t *testing.T) []byte {
	t.Helper()

	path := filepath.Join("testdata", "blueprint-storage-2.dat")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("integration fixture not present: %s", path)
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}

func entryNames(entries []Entry) map[string]struct{} {
	names := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		names[entry.Name] = struct{}{}
	}
	return names
}
