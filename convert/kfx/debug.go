package kfx

import (
	"fmt"
	"slices"
	"strings"

	"fbc/utils/debug"
)

// String returns a tree-like debug representation of the container.
func (c *Container) String() string {
	tw := debug.NewTreeWriter()

	tw.Line(0, "KFX Container")
	tw.Line(1, "Version: %d", c.Version)
	tw.Line(1, "ContainerID: %q", c.ContainerID)
	tw.Line(1, "Format: %q", c.ContainerFormat)
	tw.Line(1, "Generator: %s / %s", c.GeneratorApp, c.GeneratorPkg)
	tw.Line(1, "ChunkSize: %d", c.ChunkSize)
	tw.Line(1, "Compression: %d", c.CompressionType)
	tw.Line(1, "DRM: %d", c.DRMScheme)

	if c.Fragments != nil && c.Fragments.Len() > 0 {
		tw.Line(1, "Fragments: %d", c.Fragments.Len())

		// Group by type
		types := c.Fragments.Types()
		for _, ftype := range types {
			frags := c.Fragments.GetByType(ftype)
			tw.Line(2, "%s (%d)", FormatSymbol(ftype), len(frags))
			for _, f := range frags {
				if f.IsRoot() {
					tw.Line(3, "[root]")
				} else {
					tw.Line(3, "id=%s", FormatSymbol(f.FID))
				}
				formatValue(tw, 4, f.Value)
			}
		}
	}

	if c.DocSymbolTable != nil {
		// Show local symbols from the document symbol table
		tw.Line(1, "DocSymbolTable: present")
	}

	return tw.String()
}

// formatValue formats a value for debug output.
func formatValue(tw *debug.TreeWriter, depth int, value any) {
	switch v := value.(type) {
	case nil:
		tw.Line(depth, "null")
	case bool:
		tw.Line(depth, "%v", v)
	case int, int64, int32:
		tw.Line(depth, "%d", v)
	case float64:
		tw.Line(depth, "%f", v)
	case string:
		if len(v) > 100 {
			tw.Line(depth, "%q... (%d chars)", v[:100], len(v))
		} else {
			tw.Line(depth, "%q", v)
		}
	case []byte:
		tw.Line(depth, "blob(%d bytes)", len(v))
	case SymbolValue:
		tw.Line(depth, "sym:%s", FormatSymbol(int(v)))
	case RawValue:
		tw.Line(depth, "raw(%d bytes)", len(v))
	case StructValue:
		formatStructValueInt(tw, depth, map[int]any(v))
	case map[int]any:
		formatStructValueInt(tw, depth, v)
	case map[string]any:
		formatStructValueString(tw, depth, v)
	case ListValue:
		formatListValue(tw, depth, []any(v))
	case []any:
		formatListValue(tw, depth, v)
	default:
		tw.Line(depth, "<%T>", v)
	}
}

func formatStructValueInt(tw *debug.TreeWriter, depth int, m map[int]any) {
	if len(m) == 0 {
		tw.Line(depth, "{}")
		return
	}

	// Sort keys
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	tw.Line(depth, "{")
	for _, k := range keys {
		tw.Line(depth+1, "%s:", FormatSymbol(k))
		formatValue(tw, depth+2, m[k])
	}
	tw.Line(depth, "}")
}

func formatStructValueString(tw *debug.TreeWriter, depth int, m map[string]any) {
	if len(m) == 0 {
		tw.Line(depth, "{}")
		return
	}

	// Sort keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	tw.Line(depth, "{")
	for _, k := range keys {
		tw.Line(depth+1, "%s:", FormatSymbol(k))
		formatValue(tw, depth+2, m[k])
	}
	tw.Line(depth, "}")
}

func formatListValue(tw *debug.TreeWriter, depth int, items []any) {
	if len(items) == 0 {
		tw.Line(depth, "[]")
		return
	}

	// For short lists of simple values, show inline
	if len(items) <= 5 && allSimple(items) {
		parts := make([]string, len(items))
		for i, item := range items {
			parts[i] = formatSimple(item)
		}
		tw.Line(depth, "[%s]", strings.Join(parts, ", "))
		return
	}

	tw.Line(depth, "[%d items]", len(items))
	for i, item := range items {
		if i >= 10 {
			tw.Line(depth+1, "... and %d more", len(items)-10)
			break
		}
		formatValue(tw, depth+1, item)
	}
}

func allSimple(items []any) bool {
	for _, item := range items {
		switch item.(type) {
		case nil, bool, int, int64, int32, float64, string, SymbolValue:
			continue
		default:
			return false
		}
	}
	return true
}

func formatSimple(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		return fmt.Sprintf("%v", val)
	case int, int64, int32:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%f", val)
	case string:
		if len(val) > 20 {
			return fmt.Sprintf("%q...", val[:20])
		}
		return fmt.Sprintf("%q", val)
	case SymbolValue:
		return FormatSymbol(int(val))
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

// DumpFragments returns a detailed dump of all fragments.
func (c *Container) DumpFragments() string {
	var sb strings.Builder

	sb.WriteString("=== KFX Fragments ===\n\n")

	if c.Fragments == nil || c.Fragments.Len() == 0 {
		sb.WriteString("(no fragments)\n")
		return sb.String()
	}

	types := c.Fragments.Types()
	for _, ftype := range types {
		frags := c.Fragments.GetByType(ftype)
		fmt.Fprintf(&sb, "### %s (%d fragments)\n\n", FormatSymbol(ftype), len(frags))

		for _, f := range frags {
			if f.IsRoot() {
				sb.WriteString("  [root fragment]\n")
			} else {
				fmt.Fprintf(&sb, "  id: %s\n", FormatSymbol(f.FID))
			}
			fmt.Fprintf(&sb, "  value: %s\n\n", formatValueCompact(f.Value))
		}
	}

	return sb.String()
}

func formatValueCompact(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		return fmt.Sprintf("%v", val)
	case int, int64, int32:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case string:
		if len(val) > 50 {
			return fmt.Sprintf("%q... (%d chars)", val[:50], len(val))
		}
		return fmt.Sprintf("%q", val)
	case []byte, RawValue:
		var b []byte
		switch vv := val.(type) {
		case []byte:
			b = vv
		case RawValue:
			b = vv
		}
		return fmt.Sprintf("<blob %d bytes>", len(b))
	case SymbolValue:
		return FormatSymbol(int(val))
	case StructValue:
		return formatStructCompactInt(map[int]any(val))
	case map[int]any:
		return formatStructCompactInt(val)
	case map[string]any:
		return formatStructCompactString(val)
	case ListValue:
		return formatListCompact([]any(val))
	case []any:
		return formatListCompact(val)
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

func formatStructCompactInt(m map[int]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", FormatSymbol(k), formatValueCompact(m[k])))
	}
	result := "{" + strings.Join(parts, ", ") + "}"
	if len(result) > 200 {
		return fmt.Sprintf("{...%d fields}", len(m))
	}
	return result
}

func formatStructCompactString(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, formatValueCompact(m[k])))
	}
	result := "{" + strings.Join(parts, ", ") + "}"
	if len(result) > 200 {
		return fmt.Sprintf("{...%d fields}", len(m))
	}
	return result
}

func formatListCompact(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	if len(items) > 5 {
		return fmt.Sprintf("[...%d items]", len(items))
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = formatValueCompact(item)
	}
	result := "[" + strings.Join(parts, ", ") + "]"
	if len(result) > 100 {
		return fmt.Sprintf("[...%d items]", len(items))
	}
	return result
}
