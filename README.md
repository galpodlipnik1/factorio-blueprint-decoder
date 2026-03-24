# factorio-blueprint-decoder

Standalone Go library for decoding Factorio `blueprint_storage2.dat` files into a flat `[]Entry`, plus Lua and JSON render helpers.

## How To Use

```go
package main

import (
	"os"

	bpdecode "github.com/galpodlipnik1/factorio-blueprint-decoder"
)

func main() {
	data, err := os.ReadFile("blueprint-storage-2.dat")
	if err != nil {
		panic(err)
	}

	entries, err := bpdecode.ParseBlueprintLibrary(data)
	if err != nil {
		panic(err)
	}

	luaText := bpdecode.RenderLuaModule(entries)
	if err := os.WriteFile("output.lua", []byte(luaText), 0644); err != nil {
		panic(err)
	}

	jsonBytes, err := bpdecode.RenderJSON(entries)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("output.json", jsonBytes, 0644); err != nil {
		panic(err)
	}
}
```

Public API:

- `ParseBlueprintLibrary(data []byte) ([]Entry, error)`
- `RenderLuaModule(entries []Entry) string`
- `RenderJSON(entries []Entry) ([]byte, error)`

`Entry` includes:

- path and path keys
- record type
- name and description
- breadcrumb and normalized search fields
- child path keys
- icon sprite
- entity count
- tags

Renderer behavior:

- Lua output is shaped as `return { entries = { ... } }`
- JSON output is shaped as `{ "entries": [...] }`
- empty `icon_sprite` becomes `false` in Lua and `""` in JSON

The module includes its own sample fixture at `testdata/blueprint-storage-2.dat`, so tests work after moving this package into a separate repository.

## Technical Notes

### Reader Model

The parser uses a small internal byte reader over little-endian data:

- `u8`, `u16`, `u32`, `s32`
- `bool` encoded as `0x00` or `0x01`
- `count` encoded as either:
  - one byte if the first byte is not `0xff`
  - `0xff` followed by a `u32`

Strings are stored as:

1. `count`
2. raw bytes

Example:

```text
04 44 72 6f 70
```

means the string `Drop`.

### High-Level File Layout

`ParseBlueprintLibrary` reads the file in this order:

1. library version
2. separator `0x00`
3. migration list
4. prototype index
5. library state byte
6. separator `0x00`
7. generation counter
8. timestamp
9. reserved `u32` in Factorio 2.x
10. object marker `0x01`
11. root library objects
12. embedded-object recovery scan over the remaining bytes

The parser then flattens the tree into `[]Entry`.

### Object Prefixes

Each used library slot starts with a one-byte prefix:

- `0` = blueprint
- `1` = blueprint book
- `2` = deconstruction planner
- `3` = upgrade planner

Only blueprints and books become exported `Entry` values. Planner records are skipped structurally so stream alignment is preserved.

### Blueprint And Book Parsing

Blueprints are currently parsed as:

1. label string
2. separator `0x00`
3. `hasRemovedMods` bool
4. content size
5. content block
6. optional removed-mod local index

Books are parsed as:

1. label string
2. description string
3. icons
4. nested object table
5. active index
6. trailing `0x00`

Books recurse through the same object parser, so normal nested book depth is supported.

### Icon Extraction

The decoder resolves icons in two ways:

1. explicit icon records near the end of a blueprint content block
2. rich-text tags in the label such as `[item=iron-plate]`

The icon block is scanned from the end because it is often near the tail of the content block, not necessarily the final bytes.

### Embedded Tail Recovery

The sample file contains valid object-like records after the main root object table appears to end. To recover these, the parser performs a second scan over the remaining bytes and accepts candidates that:

- start with a used flag
- have blueprint or book prefixes
- parse successfully
- have plausible text labels
- for blueprints, start with a plausible version/separator/migration pattern

This recovery pass is what raises the bundled sample output to thousands of entries instead of only the primary root set.

### Sample Fixture Offsets

These offsets are specific to the bundled fixture `testdata/blueprint-storage-2.dat`. They are useful for debugging and reverse-engineering, but they are not fixed across all files.

Sample facts:

- file size: `52,114,925` bytes
- bytes `0..7` encode version `2.0.76.0`
- byte `8` is the first separator `0x00`
- byte `9` is the migration count `0x0d`
- root object table starts at offset `18985`
- root object count there is `31`
- the first root blueprint starts at `18989`
- for that first blueprint:
  - label starts at `18997`
  - content starts at `19047`
  - content ends at `19727`
- embedded recovery becomes relevant after roughly `155361`

Example bytes at the root table:

```text
18985: 1f 00 00 00
```

This is the root slot count `31`.

Example bytes for the first root blueprint:

```text
18989: 01 00 05 00 00 00 4d 00 ...
```

Meaning:

- slot used
- blueprint prefix
- generation counter
- item ID

### Current Limits

This library is good at producing a usable searchable index. It is not yet a complete byte-for-byte decoder of the entire Factorio file format.

Currently reliable:

- file framing
- migration skipping
- prototype index parsing for icon resolution
- root object parsing
- recursive books
- embedded tail recovery
- many icon resolutions
- flattening into index entries

Still partial or heuristic:

- full blueprint content block decoding
- description/entity-count extraction from blueprint payloads
- full entity, schedule, and tile decoding
- full semantic meaning of every byte in the embedded tail region
