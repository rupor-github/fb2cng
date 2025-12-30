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

	// Show format with classification reason
	formatReason := c.getFormatReason()
	if formatReason != "" {
		tw.Line(1, "Format: %q (%s)", c.ContainerFormat, formatReason)
	} else {
		tw.Line(1, "Format: %q", c.ContainerFormat)
	}

	tw.Line(1, "Generator: %s / %s", c.GeneratorApp, c.GeneratorPkg)
	tw.Line(1, "ChunkSize: %d", c.ChunkSize)
	tw.Line(1, "Compression: %d", c.CompressionType)
	tw.Line(1, "DRM: %d", c.DRMScheme)

	// Show format capabilities if present
	if c.FormatCapabilities != nil {
		tw.Line(1, "FormatCapabilities:")
		formatValue(tw, 2, c.FormatCapabilities)
	}

	if c.Fragments != nil && c.Fragments.Len() > 0 {
		// Calculate fragment statistics
		rootCount, rawCount, singletonCount := c.getFragmentStats()
		regularCount := c.Fragments.Len() - rootCount

		// Build stats parts
		stats := []string{fmt.Sprintf("%d total", c.Fragments.Len())}
		if rootCount > 0 {
			stats = append(stats, fmt.Sprintf("%d root", rootCount))
		}
		if rawCount > 0 {
			stats = append(stats, fmt.Sprintf("%d raw", rawCount))
		}
		if singletonCount > 0 && singletonCount != rootCount {
			stats = append(stats, fmt.Sprintf("%d singleton", singletonCount))
		}
		if regularCount > 0 {
			stats = append(stats, fmt.Sprintf("%d regular", regularCount))
		}

		tw.Line(1, "Fragments: %s", strings.Join(stats, ", "))

		// Group by type
		types := c.Fragments.Types()
		for _, ftype := range types {
			frags := c.Fragments.GetByType(ftype)

			// Sort fragments for deterministic output
			sortedFrags := make([]*Fragment, len(frags))
			copy(sortedFrags, frags)
			slices.SortFunc(sortedFrags, func(a, b *Fragment) int {
				// Root fragments first
				if a.IsRoot() && !b.IsRoot() {
					return -1
				}
				if !a.IsRoot() && b.IsRoot() {
					return 1
				}

				// Resolve FIDs for comparison
				aFID := a.FID
				if a.FIDName != "" && aFID == 0 {
					aFID = c.GetLocalSymbolID(a.FIDName)
				}
				bFID := b.FID
				if b.FIDName != "" && bFID == 0 {
					bFID = c.GetLocalSymbolID(b.FIDName)
				}

				// Sort by resolved FID
				return int(aFID - bFID)
			})

			// Add type markers
			markers := c.getTypeMarkers(ftype)
			if markers != "" {
				tw.Line(2, "%s %s (%d)", ftype, markers, len(sortedFrags))
			} else {
				tw.Line(2, "%s (%d)", ftype, len(sortedFrags))
			}

			for _, f := range sortedFrags {
				if f.IsRoot() {
					tw.Line(3, "[root]")
				} else {
					// Determine the symbol ID and name for display
					var fidID KFXSymbol
					var fidName string

					if f.FIDName != "" {
						// Fragment was created with FIDName - resolve it to symbol ID
						fidName = f.FIDName
						if len(c.LocalSymbols) > 0 {
							fidID = c.GetLocalSymbolID(f.FIDName)
							if fidID < 0 {
								// Not found in local symbols, use FID if it's valid
								if f.FID > 0 {
									fidID = f.FID
								} else {
									// FID not resolved yet, show as unknown
									tw.Line(3, "id=(unresolved) (%s)", f.FIDName)
									formatValue(tw, 4, f.Value)
									continue
								}
							}
						} else if f.FID > 0 {
							// No local symbols, use FID directly
							fidID = f.FID
						} else {
							// Can't resolve
							tw.Line(3, "id=(unresolved) (%s)", f.FIDName)
							formatValue(tw, 4, f.Value)
							continue
						}
					} else {
						// Fragment uses numeric FID directly
						fidID = f.FID
						// Try to look up the name from DocSymbolTable
						if c.DocSymbolTable != nil {
							name, ok := c.DocSymbolTable.FindByID(uint64(fidID))
							if ok && !strings.HasPrefix(name, "$") {
								// Only use if it's a real name, not a "$NNN" placeholder
								fidName = name
							}
						}
					}

					if fidName != "" {
						tw.Line(3, "id=$%d (%s)", fidID, fidName)
					} else {
						tw.Line(3, "id=%s", fidID)
					}
				}
				formatValue(tw, 4, f.Value)
			}
		}
	}

	if c.DocSymbolTable != nil {
		tw.Line(0, "")
		tw.Line(1, "DocSymbolTable: maxID=%d", c.DocSymbolTable.MaxID())

		// Show imports with their ranges
		imports := c.DocSymbolTable.Imports()
		if len(imports) > 0 {
			tw.Line(2, "Imports:")
			currentID := 1
			for _, imp := range imports {
				if imp.Name() == "$ion" {
					// System symbols 1-9
					tw.Line(3, "$ion v%d: symbols $%d-$%d (Ion system symbols)",
						imp.Version(), currentID, imp.MaxID())
				} else if imp.Name() == "YJ_symbols" {
					// YJ_symbols
					endID := currentID + len(imp.Symbols()) - 1
					tw.Line(3, "YJ_symbols v%d: symbols $%d-$%d (%d known KFX symbols)",
						imp.Version(), currentID, endID, len(imp.Symbols()))
				} else {
					tw.Line(3, "%s v%d: symbols $%d-$%d",
						imp.Name(), imp.Version(), currentID, currentID+len(imp.Symbols())-1)
				}
				currentID += len(imp.Symbols())
			}
		}

		// Show local symbols range if present
		if len(c.LocalSymbols) > 0 {
			startID := LargestKnownSymbol + 1
			endID := startID + KFXSymbol(len(c.LocalSymbols)) - 1
			tw.Line(2, "Local: symbols $%d-$%d (%d document-specific symbols)",
				startID, endID, len(c.LocalSymbols))
		}
	}

	if len(c.LocalSymbols) > 0 {
		tw.Line(0, "")
		tw.Line(1, "LocalSymbols: %d", len(c.LocalSymbols))
		for i, sym := range c.LocalSymbols {
			tw.Line(2, "$%d: %s", LargestKnownSymbol+1+KFXSymbol(i), sym)
		}
	}

	// Show kfxgen metadata if present
	if len(c.KfxgenExtra) > 0 {
		tw.Line(0, "")
		tw.Line(1, "KfxgenExtra: %d", len(c.KfxgenExtra))
		// Show all keys sorted
		allKeys := make([]string, 0, len(c.KfxgenExtra))
		for k := range c.KfxgenExtra {
			allKeys = append(allKeys, k)
		}
		slices.Sort(allKeys)
		for _, k := range allKeys {
			tw.Line(2, "%s: %q", k, c.KfxgenExtra[k])
		}
	}

	return tw.String()
}

// getFormatReason returns the reason for container format classification.
func (c *Container) getFormatReason() string {
	if c.Fragments == nil {
		return ""
	}

	// Check for main container types
	mainTypes := []struct {
		sym  KFXSymbol
		name string
	}{
		{SymStoryline, "has $storyline"},
		{SymSection, "has $section"},
		{SymDocumentData, "has $document_data"},
	}
	for _, t := range mainTypes {
		if len(c.Fragments.GetByType(t.sym)) > 0 {
			return t.name
		}
	}

	// Check for metadata container types
	metaTypes := []struct {
		sym  KFXSymbol
		name string
	}{
		{SymMetadata, "has $metadata"},
		{SymContEntityMap, "has $cont_entity_map"},
		{SymBookMetadata, "has $book_metadata"},
	}
	for _, t := range metaTypes {
		if len(c.Fragments.GetByType(t.sym)) > 0 {
			return t.name
		}
	}

	// Check for attachable
	if len(c.Fragments.GetByType(SymRawMedia)) > 0 {
		return "has $raw_media"
	}

	return ""
}

// getFragmentStats returns counts of root, raw, and singleton fragments.
func (c *Container) getFragmentStats() (root, raw, singleton int) {
	if c.Fragments == nil {
		return 0, 0, 0
	}

	for _, f := range c.Fragments.All() {
		if f.IsRoot() {
			root++
		}
		if f.IsRaw() {
			raw++
		}
		if f.IsSingleton() {
			singleton++
		}
	}
	return
}

// getTypeMarkers returns marker string for a fragment type.
func (c *Container) getTypeMarkers(ftype KFXSymbol) string {
	markers := []string{}

	if ROOT_FRAGMENT_TYPES[ftype] {
		markers = append(markers, "[root]")
	}
	if RAW_FRAGMENT_TYPES[ftype] {
		markers = append(markers, "[raw]")
	}
	if CONTAINER_FRAGMENT_TYPES[ftype] {
		markers = append(markers, "[container]")
	}

	if len(markers) == 0 {
		return ""
	}
	return strings.Join(markers, " ")
}

// formatValue formats a value for debug output in a compact way.
func formatValue(tw *debug.TreeWriter, depth int, value any) {
	switch v := value.(type) {
	case nil, bool, int, int64, int32, float64, string, SymbolValue, SymbolByNameValue:
		tw.Line(depth, "%s", formatSimpleValue(v))
	case []byte:
		tw.Line(depth, "blob(%d bytes)", len(v))
	case RawValue:
		tw.Line(depth, "raw(%d bytes)", len(v))
	case StructValue:
		formatStructValueKFX(tw, depth, v)
	case map[KFXSymbol]any:
		formatStructValueKFX(tw, depth, v)
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

func formatSimpleValue(v any) string {
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
		return fmt.Sprintf("%q", val)
	case SymbolValue:
		return KFXSymbol(val).String()
	case SymbolByNameValue:
		return fmt.Sprintf("sym:%q", string(val))
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

func formatStructValueKFX(tw *debug.TreeWriter, depth int, m map[KFXSymbol]any) {
	if len(m) == 0 {
		tw.Line(depth, "{}")
		return
	}

	// Sort keys
	keys := make([]KFXSymbol, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		keyName := k.String()
		val := m[k]

		// Try to format simple values inline
		if isSimpleValue(val) {
			tw.Line(depth, "%s: %s", keyName, formatSimpleValue(val))
		} else if blob, ok := val.([]byte); ok {
			tw.Line(depth, "%s: blob(%d bytes)", keyName, len(blob))
		} else if raw, ok := val.(RawValue); ok {
			tw.Line(depth, "%s: raw(%d bytes)", keyName, len(raw))
		} else if list, ok := val.(ListValue); ok {
			formatListValueWithKey(tw, depth, keyName, []any(list))
		} else if list, ok := val.([]any); ok {
			formatListValueWithKey(tw, depth, keyName, list)
		} else {
			tw.Line(depth, "%s:", keyName)
			formatValue(tw, depth+1, val)
		}
	}
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

	for _, k := range keys {
		val := m[k]

		// Try to format simple values inline
		if isSimpleValue(val) {
			tw.Line(depth, "%s: %s", k, formatSimpleValue(val))
		} else if blob, ok := val.([]byte); ok {
			tw.Line(depth, "%s: blob(%d bytes)", k, len(blob))
		} else if raw, ok := val.(RawValue); ok {
			tw.Line(depth, "%s: raw(%d bytes)", k, len(raw))
		} else if list, ok := val.(ListValue); ok {
			formatListValueWithKey(tw, depth, k, []any(list))
		} else if list, ok := val.([]any); ok {
			formatListValueWithKey(tw, depth, k, list)
		} else {
			tw.Line(depth, "%s:", k)
			formatValue(tw, depth+1, val)
		}
	}
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
			parts[i] = formatSimpleValue(item)
		}
		tw.Line(depth, "[%s]", strings.Join(parts, ", "))
		return
	}

	// For longer lists, show items with index
	for i, item := range items {
		if isSimpleValue(item) {
			tw.Line(depth, "[%d]: %s", i, formatSimpleValue(item))
		} else {
			tw.Line(depth, "[%d]:", i)
			formatValue(tw, depth+1, item)
		}
	}
}

func formatListValueWithKey(tw *debug.TreeWriter, depth int, key string, items []any) {
	if len(items) == 0 {
		tw.Line(depth, "%s: []", key)
		return
	}

	// For short lists of simple values, show inline
	if len(items) <= 5 && allSimple(items) {
		parts := make([]string, len(items))
		for i, item := range items {
			parts[i] = formatSimpleValue(item)
		}
		tw.Line(depth, "%s: [%s]", key, strings.Join(parts, ", "))
		return
	}

	// For longer lists, show count on parent line
	tw.Line(depth, "%s: (%d)", key, len(items))
	for i, item := range items {
		if isSimpleValue(item) {
			tw.Line(depth+1, "[%d]: %s", i, formatSimpleValue(item))
		} else {
			tw.Line(depth+1, "[%d]:", i)
			formatValue(tw, depth+2, item)
		}
	}
}

func isSimpleValue(v any) bool {
	switch v.(type) {
	case nil, bool, int, int64, int32, float64, string, SymbolValue, SymbolByNameValue:
		return true
	default:
		return false
	}
}

func allSimple(items []any) bool {
	for _, item := range items {
		if !isSimpleValue(item) {
			return false
		}
	}
	return true
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

		// Sort fragments for deterministic output
		sortedFrags := make([]*Fragment, len(frags))
		copy(sortedFrags, frags)
		slices.SortFunc(sortedFrags, func(a, b *Fragment) int {
			// Root fragments first
			if a.IsRoot() && !b.IsRoot() {
				return -1
			}
			if !a.IsRoot() && b.IsRoot() {
				return 1
			}

			// Resolve FIDs for comparison
			aFID := a.FID
			if a.FIDName != "" && aFID == 0 {
				aFID = c.GetLocalSymbolID(a.FIDName)
			}
			bFID := b.FID
			if b.FIDName != "" && bFID == 0 {
				bFID = c.GetLocalSymbolID(b.FIDName)
			}

			// Sort by resolved FID
			return int(aFID - bFID)
		})

		fmt.Fprintf(&sb, "### %s (%d fragments)\n\n", ftype, len(sortedFrags))

		for _, f := range sortedFrags {
			if f.IsRoot() {
				sb.WriteString("  [root fragment]\n")
			} else {
				// Determine the symbol ID and name for display
				var fidID KFXSymbol
				var fidName string

				if f.FIDName != "" {
					// Fragment was created with FIDName - resolve it to symbol ID
					fidName = f.FIDName
					if len(c.LocalSymbols) > 0 {
						fidID = c.GetLocalSymbolID(f.FIDName)
						if fidID < 0 {
							// Not found in local symbols, use FID if it's valid
							if f.FID > 0 {
								fidID = f.FID
							} else {
								// FID not resolved yet
								fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
								fmt.Fprintf(&sb, "  value: %s\n\n", formatValueCompact(f.Value))
								continue
							}
						}
					} else if f.FID > 0 {
						// No local symbols, use FID directly
						fidID = f.FID
					} else {
						// Can't resolve
						fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
						fmt.Fprintf(&sb, "  value: %s\n\n", formatValueCompact(f.Value))
						continue
					}
				} else {
					// Fragment uses numeric FID directly
					fidID = f.FID
					// Try to look up the name from DocSymbolTable
					if c.DocSymbolTable != nil {
						name, ok := c.DocSymbolTable.FindByID(uint64(fidID))
						if ok && !strings.HasPrefix(name, "$") {
							// Only use if it's a real name, not a "$NNN" placeholder
							fidName = name
						}
					}
				}

				if fidName != "" {
					fmt.Fprintf(&sb, "  id: $%d (%s)\n", fidID, fidName)
				} else {
					fmt.Fprintf(&sb, "  id: %s\n", fidID)
				}
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
		return KFXSymbol(val).String()
	case SymbolByNameValue:
		return fmt.Sprintf("sym:%q", string(val))
	case StructValue:
		return formatStructCompactKFX(val)
	case map[KFXSymbol]any:
		return formatStructCompactKFX(val)
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

func formatStructCompactKFX(m map[KFXSymbol]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]KFXSymbol, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, formatValueCompact(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
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
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatListCompact(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = formatValueCompact(item)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
