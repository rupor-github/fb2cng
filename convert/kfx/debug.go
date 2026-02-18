package kfx

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/utils/debug"
)

// StatsString returns a compact, stats-only representation of the container.
func (c *Container) StatsString() string {
	tw := debug.NewTreeWriter()

	tw.Line(0, "KFX Container")
	tw.Line(1, "Version: %d", c.Version)
	tw.Line(1, "ContainerID: %q", c.ContainerID)

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

	if c.Fragments != nil && c.Fragments.Len() > 0 {
		rootCount, rawCount, singletonCount := c.getFragmentStats()
		regularCount := c.Fragments.Len() - rootCount

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

		for _, ftype := range c.Fragments.Types() {
			tw.Line(2, "%s (%d)", ftype, len(c.Fragments.GetByType(ftype)))
		}
	}

	if c.DocSymbolTable != nil {
		tw.Line(0, "")
		tw.Line(1, "DocSymbolTable: maxID=%d", c.DocSymbolTable.MaxID())
	}
	if len(c.LocalSymbols) > 0 {
		tw.Line(1, "LocalSymbols: %d", len(c.LocalSymbols))
	}

	return tw.String()
}

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
	case nil, bool, int, int64, int32, float64, string, SymbolValue, SymbolByNameValue, ReadSymbolValue, *ion.Decimal:
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
	case int:
		return fmt.Sprintf("int(%d)", val)
	case int64:
		return fmt.Sprintf("int(%d)", val)
	case int32:
		return fmt.Sprintf("int(%d)", val)
	case float64:
		return fmt.Sprintf("float(%g)", val)
	case *ion.Decimal:
		return fmt.Sprintf("decimal(%s)", val.String())
	case string:
		return fmt.Sprintf("%q", val)
	case SymbolValue:
		return fmt.Sprintf("symbol(%s)", KFXSymbol(val).String())
	case SymbolByNameValue:
		return fmt.Sprintf("symbol(%q)", string(val))
	case ReadSymbolValue:
		return fmt.Sprintf("symbol(%s)", string(val))
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
	case nil, bool, int, int64, int32, float64, string, SymbolValue, SymbolByNameValue, ReadSymbolValue, *ion.Decimal:
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
								fmt.Fprintf(&sb, "  value: %s\n\n", FormatValueCompact(f.Value))
								continue
							}
						}
					} else if f.FID > 0 {
						// No local symbols, use FID directly
						fidID = f.FID
					} else {
						// Can't resolve
						fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
						fmt.Fprintf(&sb, "  value: %s\n\n", FormatValueCompact(f.Value))
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
			fmt.Fprintf(&sb, "  value: %s\n", FormatValueCompact(f.Value))
			if f.FType == SymDocumentData {
				if pretty := formatDocumentDataPretty(f.Value); pretty != "" {
					sb.WriteString(pretty)
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatDocumentDataPretty(v any) string {
	// We intentionally keep this output stable and human-oriented: the compact
	// one-line value is still printed above, but document_data ($538) is often the
	// first thing people compare between our output and KP3.
	//
	// Note: document_data may be represented as map[string]any because KP3 uses
	// non-$ field names (e.g. "max_id").
	docData, ok := v.(map[string]any)
	if !ok {
		return ""
	}

	// Helper: translate "$NNN" keys into readable symbol names.
	fieldName := func(k string) string {
		if after, ok0 := strings.CutPrefix(k, "$"); ok0 {
			if id, err := strconv.Atoi(after); err == nil {
				return KFXSymbol(id).Name()
			}
		}
		return k
	}

	// Helper: measureParts variant for debug output.
	//
	// In dumps produced from serialized KFX, we frequently see Ion structs decoded as
	// map[string]any with keys like "$306"/"$307" rather than map[KFXSymbol]any.
	// The core merge logic uses measureParts (map[KFXSymbol]any), but for dumps we
	// accept either representation.
	measurePartsDebug := func(v any) (float64, KFXSymbol, bool) {
		// Fast path: reuse the canonical helper.
		if mv, unit, ok := measureParts(v); ok {
			return mv, unit, ok
		}

		m, ok := v.(map[string]any)
		if !ok {
			return 0, 0, false
		}

		rawVal, hasVal := m["$307"]
		if !hasVal {
			rawVal, hasVal = m["value"]
		}
		rawUnit, hasUnit := m["$306"]
		if !hasUnit {
			rawUnit, hasUnit = m["unit"]
		}
		if !hasVal || !hasUnit {
			return 0, 0, false
		}

		unit, ok := symbolIDFromAny(rawUnit)
		if !ok {
			return 0, 0, false
		}

		if dec, ok := rawVal.(*ion.Decimal); ok {
			return decimalToFloat64(dec), unit, true
		}
		if num, ok := numericFromAny(rawVal); ok {
			return num, unit, true
		}
		return 0, 0, false
	}

	// Helper: render a value in a compact but readable way.
	formatVal := func(v any) string {
		formatMeasure := func(mv float64, unit KFXSymbol) string {
			unitName := unit.Name()
			// For common units we prefer CSS-like spellings.
			switch unit {
			case SymUnitPercent:
				return fmt.Sprintf("%g%%", mv)
			case SymUnitEm:
				return fmt.Sprintf("%gem", mv)
			case SymUnitRem:
				return fmt.Sprintf("%grem", mv)
			case SymUnitLh:
				return fmt.Sprintf("%glh", mv)
			case SymUnitPx:
				return fmt.Sprintf("%gpx", mv)
			default:
				return fmt.Sprintf("%g%s", mv, unitName)
			}
		}

		// Dimension values: {$306: unit, $307: value}
		if mv, unit, ok := measurePartsDebug(v); ok {
			return formatMeasure(mv, unit)
		}

		switch vv := v.(type) {
		case SymbolValue:
			return KFXSymbol(vv).Name()
		case KFXSymbol:
			return vv.Name()
		case ReadSymbolValue:
			return string(vv)
		case SymbolByNameValue:
			return string(vv)
		case *ion.Decimal:
			return vv.String()
		case string:
			return vv
		case int:
			return strconv.Itoa(vv)
		case int64:
			return strconv.FormatInt(vv, 10)
		case bool:
			if vv {
				return "true"
			}
			return "false"
		case StructValue:
			// Try to render common "dimension" structs as human units.
			if mv, unit, ok := measurePartsDebug(vv); ok {
				return formatMeasure(mv, unit)
			}
			return FormatValueCompact(v)
		case map[KFXSymbol]any:
			if mv, unit, ok := measurePartsDebug(vv); ok {
				return formatMeasure(mv, unit)
			}
			return FormatValueCompact(v)
		default:
			// Keep it compact; the raw Ion-struct form is already printed above.
			return FormatValueCompact(v)
		}
	}

	formatValShort := func(v any) string {
		// Prefer a human unit form when possible (dimensions and symbols).
		if mv, unit, ok := measurePartsDebug(v); ok {
			unitName := unit.Name()
			// Debug output should be stable and concise, so we avoid raw float noise
			// from decimal->float conversions (e.g. 1.2 -> 1.2000000000000002).
			mv = RoundSignificant(mv, SignificantFigures)
			switch unit {
			case SymUnitPercent:
				return fmt.Sprintf("%g%%", mv)
			case SymUnitEm:
				return fmt.Sprintf("%gem", mv)
			case SymUnitRem:
				return fmt.Sprintf("%grem", mv)
			case SymUnitLh:
				return fmt.Sprintf("%glh", mv)
			case SymUnitPx:
				return fmt.Sprintf("%gpx", mv)
			default:
				return fmt.Sprintf("%g%s", mv, unitName)
			}
		}
		if sym, ok := symbolIDFromAny(v); ok {
			return sym.Name()
		}
		s := formatVal(v)
		// If this is still a nested struct/list, collapse it to a placeholder.
		// (The full representation is already printed above as the raw value.)
		if strings.HasPrefix(s, "{") {
			return "(struct)"
		}
		if strings.HasPrefix(s, "[") {
			return "(list)"
		}
		return s
	}

	formatReadingOrders := func(v any) string {
		items, ok := v.([]any)
		if !ok {
			return "(unrecognized reading_orders)"
		}
		if len(items) == 0 {
			return "[]"
		}

		// We usually have a single default reading order.
		var b strings.Builder
		for i, it := range items {
			ro, ok := it.(StructValue)
			if !ok {
				// Sometimes this can be map[KFXSymbol]any after parsing.
				if m, ok := it.(map[KFXSymbol]any); ok {
					ro = StructValue(m)
				} else {
					fmt.Fprintf(&b, "\n    - [%d]: %s", i, FormatValueCompact(it))
					continue
				}
			}

			name := ""
			if n, ok := ro[SymReadOrderName]; ok {
				name = formatValShort(n)
			}
			secs := ""
			if s, ok := ro[SymSections]; ok {
				list, ok := s.([]any)
				if ok {
					// Truncate long lists for readability.
					max := min(len(list), 20)
					parts := make([]string, 0, max)
					for _, sym := range list[:max] {
						parts = append(parts, formatValShort(sym))
					}
					secs = "[" + strings.Join(parts, ", ") + "]"
					if len(list) > max {
						secs = strings.TrimSuffix(secs, "]") + ", ...]"
					}
				} else {
					secs = formatValShort(s)
				}
			}

			fmt.Fprintf(&b, "\n    - name: %s", name)
			if secs != "" {
				fmt.Fprintf(&b, "\n      sections: %s", secs)
			}
		}
		return b.String()
	}

	// Render a stable field order for easy diffs.
	order := []string{"$169", "$16", "$42", "$112", "$192", "$436", "$477", "$560", "max_id"}
	seen := map[string]bool{}

	var out strings.Builder
	out.WriteString("  pretty:\n")
	for _, k := range order {
		val, ok := docData[k]
		if !ok {
			continue
		}
		seen[k] = true
		name := fieldName(k)
		if k == "$169" {
			out.WriteString("    ")
			out.WriteString(name)
			out.WriteString(":")
			out.WriteString(formatReadingOrders(val))
			out.WriteString("\n")
			continue
		}

		// For $538 we prefer a readable value form over the nested Ion struct.
		out.WriteString("    ")
		out.WriteString(name)
		out.WriteString(": ")
		out.WriteString(formatValShort(val))
		out.WriteString("\n")
	}

	// Include any remaining keys (sorted) so nothing is hidden.
	extra := make([]string, 0, len(docData))
	for k := range docData {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	slices.Sort(extra)
	for _, k := range extra {
		out.WriteString("    ")
		out.WriteString(fieldName(k))
		out.WriteString(": ")
		out.WriteString(formatVal(docData[k]))
		out.WriteString("\n")
	}

	// Trim the trailing newline to keep surrounding formatting consistent.
	return strings.TrimSuffix(out.String(), "\n")
}

// FormatValueCompact formats a value as a compact single-line string for debug output.
func FormatValueCompact(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("int(%d)", val)
	case int64:
		return fmt.Sprintf("int(%d)", val)
	case int32:
		return fmt.Sprintf("int(%d)", val)
	case float64:
		return fmt.Sprintf("float(%g)", val)
	case *ion.Decimal:
		return fmt.Sprintf("decimal(%s)", val.String())
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
		return fmt.Sprintf("symbol(%s)", KFXSymbol(val).String())
	case SymbolByNameValue:
		return fmt.Sprintf("symbol(%q)", string(val))
	case ReadSymbolValue:
		return fmt.Sprintf("symbol(%s)", string(val))
	case StructValue:
		return FormatStructCompactKFX(val)
	case map[KFXSymbol]any:
		return FormatStructCompactKFX(val)
	case map[string]any:
		return FormatStructCompactString(val)
	case ListValue:
		return FormatListCompact([]any(val))
	case []any:
		return FormatListCompact(val)
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

// FormatStructCompactKFX formats a KFX struct map as a compact single-line string.
func FormatStructCompactKFX(m map[KFXSymbol]any) string {
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
		parts = append(parts, fmt.Sprintf("%s: %s", k, FormatValueCompact(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// FormatStructCompactString formats a string-keyed map as a compact single-line string.
func FormatStructCompactString(m map[string]any) string {
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
		parts = append(parts, fmt.Sprintf("%s: %s", k, FormatValueCompact(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// FormatListCompact formats a list as a compact single-line string.
func FormatListCompact(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = FormatValueCompact(item)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
