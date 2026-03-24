package bpdecode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func RenderLuaModule(entries []Entry) string {
	var builder strings.Builder
	builder.WriteString("return {\n")
	builder.WriteString("  entries = {\n")

	for _, entry := range entries {
		builder.WriteString("    {\n")
		builder.WriteString("      path = " + renderIntArray(entry.Path) + ",\n")
		builder.WriteString("      path_key = " + renderLuaString(entry.PathKey) + ",\n")
		builder.WriteString("      parent_path_key = " + renderOptionalString(entry.ParentPathKey) + ",\n")
		builder.WriteString("      record_type = " + renderLuaString(entry.RecordType) + ",\n")
		builder.WriteString("      name = " + renderLuaString(entry.Name) + ",\n")
		builder.WriteString("      description = " + renderLuaString(entry.Description) + ",\n")
		builder.WriteString("      breadcrumb = " + renderLuaString(entry.Breadcrumb) + ",\n")
		builder.WriteString("      search_name = " + renderLuaString(entry.SearchName) + ",\n")
		builder.WriteString("      search_description = " + renderLuaString(entry.SearchDescription) + ",\n")
		builder.WriteString("      search_breadcrumb = " + renderLuaString(entry.SearchBreadcrumb) + ",\n")
		builder.WriteString("      search_text = " + renderLuaString(entry.SearchText) + ",\n")
		builder.WriteString("      label_resolved = true,\n")
		builder.WriteString("      child_path_keys = " + renderStringArray(entry.ChildPathKeys) + ",\n")
		builder.WriteString("      icon_sprite = " + renderOptionalBoolString(entry.IconSprite) + ",\n")
		builder.WriteString("      entity_count = " + strconv.Itoa(entry.EntityCount) + ",\n")
		builder.WriteString("      tags = " + renderStringKeyedTable(entry.Tags) + ",\n")
		builder.WriteString("    },\n")
	}

	builder.WriteString("  }\n")
	builder.WriteString("}\n")
	return builder.String()
}

func renderIntArray(values []int) string {
	if len(values) == 0 {
		return "{}"
	}

	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func renderStringArray(values []string) string {
	if len(values) == 0 {
		return "{}"
	}

	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = renderLuaString(value)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func renderStringKeyedTable(values map[string]any) string {
	if len(values) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, "["+renderLuaString(key)+"] = "+renderLuaScalar(values[key]))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func renderOptionalString(value string) string {
	if value == "" {
		return "nil"
	}
	return renderLuaString(value)
}

func renderOptionalBoolString(value string) string {
	if value == "" {
		return "false"
	}
	return renderLuaString(value)
}

func renderLuaScalar(value any) string {
	switch typed := value.(type) {
	case nil:
		return "nil"
	case string:
		return renderLuaString(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return renderLuaString(fmt.Sprint(typed))
	}
}

func renderLuaString(value string) string {
	var builder strings.Builder
	builder.Grow(len(value) + 2)
	builder.WriteByte('"')

	for _, r := range value {
		switch r {
		case '\\':
			builder.WriteString(`\\`)
		case '"':
			builder.WriteString(`\"`)
		case '\a':
			builder.WriteString(`\a`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '\v':
			builder.WriteString(`\v`)
		default:
			if r < 0x20 || r == 0x7f {
				builder.WriteString(fmt.Sprintf(`\%03d`, r))
				continue
			}

			builder.WriteRune(r)
		}
	}

	builder.WriteByte('"')
	return builder.String()
}
