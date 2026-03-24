package bpdecode

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gluaparse "github.com/yuin/gopher-lua/parse"
)

func TestRenderLuaModule_ProducesParseableLuaWithControlCharacters(t *testing.T) {
	lua := RenderLuaModule([]Entry{
		{
			Path:              []int{1, 2},
			PathKey:           "1.2",
			ParentPathKey:     "1",
			RecordType:        "blueprint",
			Name:              "belt\tbus\bcore",
			Description:       "line\x00with\ncontrols",
			Breadcrumb:        "root / belt\tbus\bcore",
			SearchName:        "belt\tbus\bcore",
			SearchDescription: "line\x00with\ncontrols",
			SearchBreadcrumb:  "root / belt\tbus\bcore",
			SearchText:        "quote\" slash\\ bell\a form\f vert\v unit\x01 del\x7f",
			LabelResolved:     true,
			ChildPathKeys:     []string{"1.2.1"},
			IconSprite:        "item/transport-belt",
			EntityCount:       12,
			Tags: map[string]any{
				"count":   5,
				"enabled": true,
				"note":    "tab\tzero\x00",
			},
		},
	})

	_, err := gluaparse.Parse(strings.NewReader(lua), "index.lua")
	require.NoError(t, err)

	require.Contains(t, lua, "return {")
	require.Contains(t, lua, "entries = {")
	require.Contains(t, lua, `name = "belt\tbus\bcore"`)
	require.Contains(t, lua, `description = "line\000with\ncontrols"`)
	require.Contains(t, lua, `search_text = "quote\" slash\\ bell\a form\f vert\v unit\001 del\127"`)
	require.Contains(t, lua, `["note"] = "tab\tzero\000"`)
	require.Contains(t, lua, `["count"] = 5`)
}

func TestRenderLuaModule_PreservesOptionalFieldBehavior(t *testing.T) {
	lua := RenderLuaModule([]Entry{
		{
			Path:          []int{2},
			PathKey:       "2",
			RecordType:    "blueprint-book",
			Name:          "Book",
			Breadcrumb:    "Book",
			SearchName:    "book",
			SearchText:    "book",
			ChildPathKeys: []string{},
			Tags: map[string]any{
				"alpha": true,
				"beta":  "value",
			},
		},
	})

	_, err := gluaparse.Parse(strings.NewReader(lua), "index.lua")
	require.NoError(t, err)

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
