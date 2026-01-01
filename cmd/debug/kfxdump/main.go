package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/filetype"

	"fbc/convert/kfx"
)

func main() {
	bcRawMedia := flag.Bool("bcRawMedia", false, "dump $417 (bcRawMedia) raw bytes into <file>-bcRawMedia directory")
	styles := flag.Bool("styles", false, "dump $157 (style) fragments into <file>-styles.txt")
	overwrite := flag.Bool("overwrite", false, "overwrite existing <file>-dump.txt/<file>-styles.txt and replace <file>-bcRawMedia directory")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: kfxdump [-bcRawMedia] [--styles] [--overwrite] <file.kfx>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	defer func(startedAt time.Time) {
		duration := time.Since(startedAt)
		fmt.Fprintf(os.Stderr, "\nExecution time: %s\n", duration)
	}(time.Now())

	path := flag.Arg(0)
	b, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	container, err := kfx.ReadContainer(b)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}

	if err := dumpDumpTxt(container, path, *overwrite); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "dump: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(container.StatsString())

	if *bcRawMedia {
		if err := dumpBcRawMedia(container, path, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump bcRawMedia: %v\n", err)
			os.Exit(1)
		}
	}

	if *styles {
		if err := dumpStylesTxt(container, path, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump styles: %v\n", err)
			os.Exit(1)
		}
	}
}

func dumpDumpTxt(container *kfx.Container, inPath string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	outPath := filepath.Join(filepath.Dir(inPath), stem+"-dump.txt")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dump := container.String() + "\n\n" + container.DumpFragments() + "\n"
	return os.WriteFile(outPath, []byte(dump), 0o644)
}

func dumpStylesTxt(container *kfx.Container, inPath string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	outPath := filepath.Join(filepath.Dir(inPath), stem+"-styles.txt")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dump, count := dumpStyleFragments(container)
	dump += "\n"
	if err := os.WriteFile(outPath, []byte(dump), 0o644); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "styles: wrote %d fragment(s) into %s\n", count, outPath)
	return nil
}

func dumpStyleFragments(c *kfx.Container) (string, int) {
	return dumpFragmentsByType(c, kfx.SymStyle)
}

func dumpFragmentsByType(c *kfx.Container, ftype kfx.KFXSymbol) (string, int) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "=== KFX Fragments: %s ===\n\n", ftype)

	if c.Fragments == nil {
		sb.WriteString("(no fragments)\n")
		return sb.String(), 0
	}

	frags := c.Fragments.GetByType(ftype)
	if len(frags) == 0 {
		sb.WriteString("(no fragments)\n")
		return sb.String(), 0
	}

	sortedFrags := make([]*kfx.Fragment, len(frags))
	copy(sortedFrags, frags)
	slices.SortFunc(sortedFrags, func(a, b *kfx.Fragment) int {
		if a.IsRoot() && !b.IsRoot() {
			return -1
		}
		if !a.IsRoot() && b.IsRoot() {
			return 1
		}

		aFID := a.FID
		if a.FIDName != "" && aFID == 0 {
			aFID = c.GetLocalSymbolID(a.FIDName)
		}
		bFID := b.FID
		if b.FIDName != "" && bFID == 0 {
			bFID = c.GetLocalSymbolID(b.FIDName)
		}

		return int(aFID - bFID)
	})

	fmt.Fprintf(&sb, "### %s (%d fragments)\n\n", ftype, len(sortedFrags))

	for _, f := range sortedFrags {
		if f.IsRoot() {
			sb.WriteString("  [root fragment]\n")
		} else {
			var fidID kfx.KFXSymbol
			var fidName string

			if f.FIDName != "" {
				fidName = f.FIDName
				if len(c.LocalSymbols) > 0 {
					fidID = c.GetLocalSymbolID(f.FIDName)
					if fidID < 0 {
						if f.FID > 0 {
							fidID = f.FID
						} else {
							fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
							fmt.Fprintf(&sb, "  value: %s\n\n", formatValueCompact(f.Value))
							continue
						}
					}
				} else if f.FID > 0 {
					fidID = f.FID
				} else {
					fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
					fmt.Fprintf(&sb, "  value: %s\n\n", formatValueCompact(f.Value))
					continue
				}
			} else {
				fidID = f.FID
				if c.DocSymbolTable != nil {
					name, ok := c.DocSymbolTable.FindByID(uint64(fidID))
					if ok && !strings.HasPrefix(name, "$") {
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
		fmt.Fprintf(&sb, "  value: %s\n", formatValueCompact(f.Value))

		// Add CSS-like decoded output for style fragments
		if css := formatStyleAsCSS(f.Value); css != "" {
			sb.WriteString(css)
		}
		sb.WriteString("\n")
	}

	return sb.String(), len(sortedFrags)
}

// formatStyleAsCSS converts a KFX style value to CSS-like syntax.
func formatStyleAsCSS(v any) string {
	// Normalize to map with KFX property names as keys
	props := normalizeStyleMap(v)
	if props == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("  /* CSS-like:\n")

	// Extract style name for selector
	styleName := extractStringValue(props["style_name"])
	if styleName != "" {
		fmt.Fprintf(&sb, "     .%s {\n", styleName)
	} else {
		sb.WriteString("     {\n")
	}

	// Show parent style inheritance if present
	if parentStyle := extractStringValue(props["parent_style"]); parentStyle != "" {
		fmt.Fprintf(&sb, "       /* inherits: .%s */\n", parentStyle)
	}

	// Extract and format CSS properties
	cssProps := extractCSSProperties(props)
	for _, prop := range cssProps {
		fmt.Fprintf(&sb, "       %s: %s;\n", prop.name, prop.value)
	}

	sb.WriteString("     }\n")
	sb.WriteString("  */\n")

	return sb.String()
}

// normalizeStyleMap converts style value to map[string]any with KFX property names as keys.
func normalizeStyleMap(v any) map[string]any {
	switch m := v.(type) {
	case kfx.StructValue:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[k.Name()] = val
		}
		return result
	case map[kfx.KFXSymbol]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[k.Name()] = val
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			// Convert "$NNN" keys to property names
			if strings.HasPrefix(k, "$") {
				if id, err := strconv.Atoi(k[1:]); err == nil {
					result[kfx.KFXSymbol(id).Name()] = val
					continue
				}
			}
			result[k] = val
		}
		return result
	default:
		return nil
	}
}

// cssNameOverrides maps KFX property names to CSS names where simple underscore-to-hyphen doesn't work.
var cssNameOverrides = map[string]string{
	"text_alignment": "text-align",
	"letterspacing":  "letter-spacing",
	"text_color":     "color",
	"fill_color":     "background-color",
	"fill_opacity":   "opacity",
	"border_weight":  "border-width",
	"baseline_shift": "vertical-align",
	"first":          "orphans",
	"last":           "widows",
}

// skipProperties are KFX properties that shouldn't be output as CSS.
var skipProperties = map[string]bool{
	"style_name":   true,
	"parent_style": true,
}

// kfxNameToCSS converts a KFX property name to CSS property name.
func kfxNameToCSS(name string) string {
	if override, ok := cssNameOverrides[name]; ok {
		return override
	}
	return strings.ReplaceAll(name, "_", "-")
}

type cssProp struct {
	name  string
	value string
}

// extractCSSProperties extracts CSS-like properties from a normalized KFX style map.
func extractCSSProperties(m map[string]any) []cssProp {
	var props []cssProp

	// Get sorted keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		if !skipProperties[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	for _, kfxName := range keys {
		v := m[kfxName]
		cssName := kfxNameToCSS(kfxName)
		cssValue := formatPropertyValue(v)
		props = append(props, cssProp{cssName, cssValue})
	}

	return props
}

// formatPropertyValue formats a KFX property value for CSS output.
func formatPropertyValue(v any) string {
	// Check if it's a dimension value (has value/unit structure)
	if isDimensionValue(v) {
		return formatDimension(v)
	}
	// Otherwise format as symbol or plain value
	return formatCSSSymbol(v)
}

// isDimensionValue checks if a value is a KFX dimension (has value and unit fields).
func isDimensionValue(v any) bool {
	switch m := v.(type) {
	case kfx.StructValue:
		_, hasValue := m[kfx.SymValue]
		_, hasUnit := m[kfx.SymUnit]
		return hasValue && hasUnit
	case map[kfx.KFXSymbol]any:
		_, hasValue := m[kfx.SymValue]
		_, hasUnit := m[kfx.SymUnit]
		return hasValue && hasUnit
	case map[string]any:
		_, hasValue := m["value"]
		_, hasValue2 := m["$307"]
		_, hasUnit := m["unit"]
		_, hasUnit2 := m["$306"]
		return (hasValue || hasValue2) && (hasUnit || hasUnit2)
	default:
		return false
	}
}

// formatDimension formats a KFX dimension value {value: X, unit: Y} to CSS.
func formatDimension(v any) string {
	var value, unit any

	// Handle different map types
	switch m := v.(type) {
	case kfx.StructValue:
		value = m[kfx.SymValue] // $307
		unit = m[kfx.SymUnit]   // $306
	case map[kfx.KFXSymbol]any:
		value = m[kfx.SymValue]
		unit = m[kfx.SymUnit]
	case map[string]any:
		// Handle both "value"/"unit" and "$307"/"$306" keys
		if v, ok := m["value"]; ok {
			value = v
		} else if v, ok := m["$307"]; ok {
			value = v
		}
		if u, ok := m["unit"]; ok {
			unit = u
		} else if u, ok := m["$306"]; ok {
			unit = u
		}
	default:
		return formatCSSValue(v)
	}

	if value == nil {
		return formatCSSValue(v)
	}

	var valStr string
	switch vv := value.(type) {
	case float64:
		valStr = formatCSSFloat(vv)
	case int, int64, int32:
		valStr = fmt.Sprintf("%d", vv)
	case string:
		// Handle Ion decimal notation like "2.08333d-1"
		valStr = formatIonDecimal(vv)
	default:
		valStr = fmt.Sprintf("%v", vv)
	}

	unitStr := ""
	switch u := unit.(type) {
	case kfx.SymbolValue:
		unitStr = kfxSymbolToCSS(kfx.KFXSymbol(u))
	case kfx.KFXSymbol:
		unitStr = kfxSymbolToCSS(u)
	case string:
		unitStr = unitNameToCSS(u)
	default:
		unitStr = fmt.Sprintf("%v", u)
	}

	return valStr + unitStr
}

// formatCSSFloat formats a float for CSS output (no scientific notation).
func formatCSSFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%d", int(f))
	}
	// Use %f and trim trailing zeros
	s := fmt.Sprintf("%.6f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// formatIonDecimal converts Ion decimal notation (e.g., "2.08333d-1") to CSS number.
func formatIonDecimal(s string) string {
	// Check for Ion decimal notation with 'd' or 'D' exponent
	if idx := strings.IndexAny(s, "dD"); idx >= 0 {
		// Replace 'd' with 'e' for Go's float parser
		normalized := strings.Replace(s, "d", "e", 1)
		normalized = strings.Replace(normalized, "D", "e", 1)
		if f, err := strconv.ParseFloat(normalized, 64); err == nil {
			return formatCSSFloat(f)
		}
	}
	// Try parsing as regular float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return formatCSSFloat(f)
	}
	return s
}

// unitNameToCSS converts a unit name (like "em", "ratio", "$308") to CSS unit.
func unitNameToCSS(u string) string {
	// Check overrides first
	if override, ok := cssValueOverrides[u]; ok {
		return override
	}
	// Handle "$308" style string representation
	if len(u) > 1 && u[0] == '$' {
		if id, err := strconv.Atoi(u[1:]); err == nil {
			return kfxSymbolToCSS(kfx.KFXSymbol(id))
		}
	}
	return u
}

// formatCSSSymbol formats a KFX symbol value to CSS keyword.
func formatCSSSymbol(v any) string {
	switch s := v.(type) {
	case kfx.SymbolValue:
		return kfxSymbolToCSS(kfx.KFXSymbol(s))
	case kfx.KFXSymbol:
		return kfxSymbolToCSS(s)
	case string:
		// Handle "$320" style string representation
		if len(s) > 1 && s[0] == '$' {
			if id, err := strconv.Atoi(s[1:]); err == nil {
				return kfxSymbolToCSS(kfx.KFXSymbol(id))
			}
		}
		return s
	case []any:
		return formatCSSList(s)
	case kfx.ListValue:
		return formatCSSList([]any(s))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatCSSList formats a list of values for CSS output.
func formatCSSList(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = formatCSSSymbol(item)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cssValueOverrides maps KFX symbol names to CSS values where they differ.
var cssValueOverrides = map[string]string{
	"semibold": "600",
	"light":    "300",
	"medium":   "500",
	"percent":  "%",
	"ratio":    "", // unitless
}

// kfxSymbolToCSS converts a KFX symbol to CSS keyword.
func kfxSymbolToCSS(sym kfx.KFXSymbol) string {
	name := sym.Name()
	if override, ok := cssValueOverrides[name]; ok {
		return override
	}
	return name
}

// formatCSSValue formats a generic value for CSS output.
func formatCSSValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "none"
	case bool:
		return fmt.Sprintf("%v", val)
	case int, int64, int32:
		return fmt.Sprintf("%d", val)
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case string:
		return fmt.Sprintf("%q", val)
	case kfx.SymbolValue:
		return kfxSymbolToCSS(kfx.KFXSymbol(val))
	case kfx.SymbolByNameValue:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// extractStringValue extracts a string from various value types.
func extractStringValue(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case kfx.SymbolByNameValue:
		return string(n)
	default:
		return ""
	}
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
	case []byte, kfx.RawValue:
		var b []byte
		switch vv := val.(type) {
		case []byte:
			b = vv
		case kfx.RawValue:
			b = vv
		}
		return fmt.Sprintf("<blob %d bytes>", len(b))
	case kfx.SymbolValue:
		return kfx.KFXSymbol(val).String()
	case kfx.SymbolByNameValue:
		return fmt.Sprintf("sym:%q", string(val))
	case kfx.StructValue:
		return formatStructCompactKFX(val)
	case map[kfx.KFXSymbol]any:
		return formatStructCompactKFX(val)
	case map[string]any:
		return formatStructCompactString(val)
	case kfx.ListValue:
		return formatListCompact([]any(val))
	case []any:
		return formatListCompact(val)
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

func formatStructCompactKFX(m map[kfx.KFXSymbol]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]kfx.KFXSymbol, 0, len(m))
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

func dumpBcRawMedia(container *kfx.Container, inPath string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	outDir := filepath.Join(filepath.Dir(inPath), stem+"-bcRawMedia")
	if st, err := os.Stat(outDir); err == nil {
		if !overwrite {
			return fmt.Errorf("output directory already exists: %s", outDir)
		}
		if !st.IsDir() {
			return fmt.Errorf("output path exists and is not a directory: %s", outDir)
		}
		if err := os.RemoveAll(outDir); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(outDir, 0o755); err != nil {
		return err
	}

	written := 0
	for _, f := range container.Fragments.GetByType(kfx.SymRawMedia) {
		blob, ok := asBlob(f.Value)
		if !ok || len(blob) == 0 {
			continue
		}

		idName := ""
		if f.FIDName != "" {
			idName = f.FIDName
		} else if container.DocSymbolTable != nil {
			if n, ok := container.DocSymbolTable.FindByID(uint64(f.FID)); ok {
				idName = n
			}
		}

		idPrefix := fmt.Sprintf("%d", f.FID)
		if idName != "" {
			idPrefix += "_" + sanitizeFileComponent(idName)
		}

		ext := extFromFiletype(blob)
		outPath := filepath.Join(outDir, idPrefix+ext)
		for i := 2; ; i++ {
			if _, err := os.Stat(outPath); err != nil {
				break
			}
			outPath = filepath.Join(outDir, idPrefix+fmt.Sprintf("_%d", i)+ext)
		}

		if err := os.WriteFile(outPath, blob, 0o644); err != nil {
			return err
		}
		written++
	}

	_, _ = fmt.Fprintf(os.Stderr, "bcRawMedia: wrote %d file(s) into %s/\n", written, outDir)
	return nil
}

func asBlob(v any) ([]byte, bool) {
	switch vv := v.(type) {
	case []byte:
		return vv, true
	case kfx.RawValue:
		return []byte(vv), true
	default:
		return nil, false
	}
}

func extFromFiletype(b []byte) string {
	kind, err := filetype.Match(b)
	if err == nil && kind != filetype.Unknown && kind.Extension != "" {
		return "." + kind.Extension
	}
	return ".bin"
}

func sanitizeFileComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		default:
			return '_'
		}
	}, s)
}
