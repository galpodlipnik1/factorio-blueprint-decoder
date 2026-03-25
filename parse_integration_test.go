//go:build integration

package bpdecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestParseBlueprintLibrary_UsesBookLocalSlotIndexesForRealNestedEntries(t *testing.T) {
	data := readIntegrationFixture(t)

	entries, err := ParseBlueprintLibrary(data)
	require.NoError(t, err)

	book := findEntryByNameAndType(t, entries, "[item=splitter] Belt balancers - TAKE OUT FOR BETTER SCROLLING", "blueprint-book")
	require.Equal(t, []int{5}, book.Path)
	require.Contains(t, book.ChildPathKeys, "5.0")

	child := findEntryByPathKey(t, entries, "5.0")
	require.Equal(t, "blueprint", child.RecordType)
	require.Equal(t, "1 to 1", child.Name)
	require.Equal(t, []int{5, 0}, child.Path)
	require.Equal(t, "5", child.ParentPathKey)
	require.Equal(t, "[item=splitter] Belt balancers - TAKE OUT FOR BETTER SCROLLING / 1 to 1", child.Breadcrumb)
}

func TestParseBlueprintLibrary_DoesNotClaimLivePathsForRecoveredEntries(t *testing.T) {
	data := readIntegrationFixture(t)

	entries, err := ParseBlueprintLibrary(data)
	require.NoError(t, err)

	recoveredBook := findEntryByNameAndType(t, entries, "◾ Grids", "blueprint-book")
	require.Empty(t, recoveredBook.Path)
	require.True(t, strings.HasPrefix(recoveredBook.PathKey, "recovered:"))

	recoveredChild := findEntryByNameAndType(t, entries, "Roboport Scout", "blueprint")
	require.Equal(t, recoveredBook.PathKey, recoveredChild.ParentPathKey)
	require.Empty(t, recoveredChild.Path)
	require.True(t, strings.HasPrefix(recoveredChild.PathKey, recoveredBook.PathKey+"/"))

	for _, entry := range entries {
		if len(entry.Path) == 0 {
			continue
		}
		require.LessOrEqual(t, entry.Path[0], 31, "entry with live path escaped real top-level slot space: %s", entry.PathKey)
	}
}

func TestParseBlueprintLibrary_ProducesInternallyConsistentHierarchy(t *testing.T) {
	data := readIntegrationFixture(t)

	entries, err := ParseBlueprintLibrary(data)
	require.NoError(t, err)

	byPathKey := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		_, exists := byPathKey[entry.PathKey]
		require.False(t, exists, "duplicate path_key: %s", entry.PathKey)
		byPathKey[entry.PathKey] = entry

		if isRecoveredPathKey(entry.PathKey) {
			require.Empty(t, entry.Path, "recovered entries must not claim live runtime paths: %s", entry.PathKey)
			continue
		}

		require.NotEmpty(t, entry.Path, "live entries must expose a runtime path: %s", entry.PathKey)
		require.LessOrEqual(t, entry.Path[0], 31, "live entry escaped real top-level slot space: %s", entry.PathKey)
	}

	for _, entry := range entries {
		if entry.ParentPathKey != "" {
			parent, ok := byPathKey[entry.ParentPathKey]
			require.True(t, ok, "missing parent for %s", entry.PathKey)
			require.Contains(t, parent.ChildPathKeys, entry.PathKey, "parent does not reference child %s", entry.PathKey)
		}

		for _, childPathKey := range entry.ChildPathKeys {
			child, ok := byPathKey[childPathKey]
			require.True(t, ok, "missing child for %s", entry.PathKey)
			require.Equal(t, entry.PathKey, child.ParentPathKey, "child %s points at wrong parent", childPathKey)
		}
	}
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

func findEntryByNameAndType(t *testing.T, entries []Entry, name string, recordType string) Entry {
	t.Helper()

	for _, entry := range entries {
		if entry.Name == name && entry.RecordType == recordType {
			return entry
		}
	}

	t.Fatalf("entry not found: type=%s name=%q", recordType, name)
	return Entry{}
}

func findEntryByPathKey(t *testing.T, entries []Entry, pathKey string) Entry {
	t.Helper()

	for _, entry := range entries {
		if entry.PathKey == pathKey {
			return entry
		}
	}

	t.Fatalf("entry not found: path_key=%q", pathKey)
	return Entry{}
}

func isRecoveredPathKey(pathKey string) bool {
	return strings.HasPrefix(pathKey, "recovered:")
}
