package bpdecode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Entry struct {
	Path              []int          `json:"path"`
	PathKey           string         `json:"path_key"`
	ParentPathKey     string         `json:"parent_path_key"`
	RecordType        string         `json:"record_type"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Breadcrumb        string         `json:"breadcrumb"`
	SearchName        string         `json:"search_name"`
	SearchDescription string         `json:"search_description"`
	SearchBreadcrumb  string         `json:"search_breadcrumb"`
	SearchText        string         `json:"search_text"`
	LabelResolved     bool           `json:"label_resolved"`
	ChildPathKeys     []string       `json:"child_path_keys"`
	IconSprite        string         `json:"icon_sprite"`
	EntityCount       int            `json:"entity_count"`
	Tags              map[string]any `json:"tags"`
}

func buildSearchText(name string, description string, breadcrumb string, tags map[string]any) string {
	parts := []string{name, description, breadcrumb}

	if len(tags) > 0 {
		keys := make([]string, 0, len(tags))
		for key := range tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			parts = append(parts, key)
			parts = append(parts, fmt.Sprint(tags[key]))
		}
	}

	return normalize(strings.Join(parts, " "))
}

func normalize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func joinPath(path []int) string {
	parts := make([]string, len(path))
	for i, part := range path {
		parts[i] = strconv.Itoa(part)
	}
	return strings.Join(parts, ".")
}

func copyInts(values []int) []int {
	out := make([]int, len(values))
	copy(out, values)
	return out
}

func copyStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func fallbackName(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "(unnamed)"
	}
	return label
}

func stringsJoin(parts []string, separator string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += separator + parts[i]
	}
	return result
}
