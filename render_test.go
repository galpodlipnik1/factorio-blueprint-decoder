package bpdecode

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderLuaModule_WrapsEntries(t *testing.T) {
	lua := RenderLuaModule([]Entry{
		{
			Path:              []int{1, 2},
			PathKey:           "1.2",
			ParentPathKey:     "1",
			RecordType:        "blueprint",
			Name:              "Speed Module Line",
			Description:       "Main line",
			Breadcrumb:        "Modules / Speed Module Line",
			SearchName:        "speed module line",
			SearchDescription: "main line",
			SearchBreadcrumb:  "modules / speed module line",
			SearchText:        "speed module line main line modules / speed module line",
			LabelResolved:     true,
			ChildPathKeys:     []string{"1.2.1"},
			IconSprite:        "item/speed-module",
			EntityCount:       12,
			Tags: map[string]any{
				"alpha": true,
				"beta":  "value",
			},
		},
		{
			Path:          []int{2},
			PathKey:       "2",
			RecordType:    "blueprint-book",
			Name:          "Book",
			Breadcrumb:    "Book",
			SearchName:    "book",
			SearchText:    "book",
			ChildPathKeys: []string{},
			Tags:          map[string]any{},
		},
	})

	require.Contains(t, lua, "return {")
	require.Contains(t, lua, "entries = {")
	require.Contains(t, lua, `record_type = "blueprint"`)
	require.Contains(t, lua, `icon_sprite = "item/speed-module"`)
	require.Contains(t, lua, `icon_sprite = false`)
	require.Contains(t, lua, `parent_path_key = nil`)
	require.Contains(t, lua, `["alpha"] = true, ["beta"] = "value"`)
}

func TestRenderJSON_WrapsEntries(t *testing.T) {
	entries := []Entry{
		{
			Path:          []int{1},
			PathKey:       "1",
			RecordType:    "blueprint-book",
			Name:          "Modules",
			Breadcrumb:    "Modules",
			SearchName:    "modules",
			SearchText:    "modules",
			ChildPathKeys: []string{"1.1"},
			Tags: map[string]any{
				"category": "mall",
			},
		},
	}

	payload, err := RenderJSON(entries)
	require.NoError(t, err)

	var module struct {
		Entries []Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(payload, &module))
	require.Equal(t, entries, module.Entries)
}
