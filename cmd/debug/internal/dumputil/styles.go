package dumputil

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"fbc/convert/kfx"
)

// DumpStylesTxt writes style fragments with CSS formatting and usage info to <stem>-styles.txt.
func DumpStylesTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	dump, count := dumpStyleFragments(container)
	dump += "\n"
	if err := WriteOutput(inPath, outDir, "-styles.txt", []byte(dump), overwrite); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "styles: wrote %d fragment(s) into output\n", count)
	return nil
}

func dumpStyleFragments(c *kfx.Container) (string, int) {
	styleUsage := collectStyleUsage(c)
	return dumpFragmentsByTypeWithUsage(c, kfx.SymStyle, styleUsage)
}

// StyleUsageInfo tracks which fragments use a particular style.
type StyleUsageInfo struct {
	// Fragments that reference this style via $157 (style field)
	DirectUsers []string
	// Fragments that reference this style via $142 (style_events)
	StyleEventUsers []string
}

// collectStyleUsage scans all fragments to find style references.
func collectStyleUsage(c *kfx.Container) map[string]*StyleUsageInfo {
	usage := make(map[string]*StyleUsageInfo)

	if c.Fragments == nil {
		return usage
	}

	for _, frag := range c.Fragments.All() {
		fragID := formatFragmentID(c, frag)

		if frag.FType == kfx.SymStyle {
			continue
		}

		directStyles, eventStyles := findAllStyleReferences(frag.Value)

		for _, styleName := range directStyles {
			if usage[styleName] == nil {
				usage[styleName] = &StyleUsageInfo{}
			}
			usage[styleName].DirectUsers = append(usage[styleName].DirectUsers, fragID)
		}

		for _, styleName := range eventStyles {
			if usage[styleName] == nil {
				usage[styleName] = &StyleUsageInfo{}
			}
			usage[styleName].StyleEventUsers = append(usage[styleName].StyleEventUsers, fragID)
		}
	}

	return usage
}

// findAllStyleReferences recursively finds all style references in a value.
func findAllStyleReferences(v any) (directStyles, eventStyles []string) {
	seenDirect := make(map[string]bool)
	seenEvent := make(map[string]bool)

	var walk func(val any)
	walk = func(val any) {
		switch m := val.(type) {
		case kfx.StructValue:
			if styleVal, ok := m[kfx.SymStyle]; ok {
				if name := extractSymbolName(styleVal); name != "" && !seenDirect[name] {
					seenDirect[name] = true
					directStyles = append(directStyles, name)
				}
			}
			if eventsVal, ok := m[kfx.SymStyleEvents]; ok {
				for _, styleName := range extractStylesFromEventList(eventsVal) {
					if !seenEvent[styleName] {
						seenEvent[styleName] = true
						eventStyles = append(eventStyles, styleName)
					}
				}
			}
			for _, subVal := range m {
				walk(subVal)
			}

		case map[kfx.KFXSymbol]any:
			if styleVal, ok := m[kfx.SymStyle]; ok {
				if name := extractSymbolName(styleVal); name != "" && !seenDirect[name] {
					seenDirect[name] = true
					directStyles = append(directStyles, name)
				}
			}
			if eventsVal, ok := m[kfx.SymStyleEvents]; ok {
				for _, styleName := range extractStylesFromEventList(eventsVal) {
					if !seenEvent[styleName] {
						seenEvent[styleName] = true
						eventStyles = append(eventStyles, styleName)
					}
				}
			}
			for _, subVal := range m {
				walk(subVal)
			}

		case map[string]any:
			if styleVal, ok := m["$157"]; ok {
				if name := extractSymbolName(styleVal); name != "" && !seenDirect[name] {
					seenDirect[name] = true
					directStyles = append(directStyles, name)
				}
			}
			if eventsVal, ok := m["$142"]; ok {
				for _, styleName := range extractStylesFromEventList(eventsVal) {
					if !seenEvent[styleName] {
						seenEvent[styleName] = true
						eventStyles = append(eventStyles, styleName)
					}
				}
			}
			for _, subVal := range m {
				walk(subVal)
			}

		case []any:
			for _, item := range m {
				walk(item)
			}

		case kfx.ListValue:
			for _, item := range m {
				walk(item)
			}
		}
	}

	walk(v)
	return directStyles, eventStyles
}

// extractStylesFromEventList extracts style names from a list of style events.
func extractStylesFromEventList(v any) []string {
	list := toListAny(v)
	if list == nil {
		return nil
	}

	var styles []string
	seen := make(map[string]bool)

	for _, item := range list {
		if m := toMapAny(item); m != nil {
			for key, val := range m {
				keyID := keyToSymbolID(key)
				if keyID == int(kfx.SymStyle) {
					if styleName := extractSymbolName(val); styleName != "" && !seen[styleName] {
						seen[styleName] = true
						styles = append(styles, styleName)
					}
				}
			}
		}
	}
	return styles
}

func dumpFragmentsByTypeWithUsage(c *kfx.Container, ftype kfx.KFXSymbol, styleUsage map[string]*StyleUsageInfo) (string, int) {
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
		styleName := ""

		if f.IsRoot() {
			sb.WriteString("  [root fragment]\n")
		} else {
			var fidID kfx.KFXSymbol
			var fidName string

			if f.FIDName != "" {
				fidName = f.FIDName
				styleName = fidName
				if len(c.LocalSymbols) > 0 {
					fidID = c.GetLocalSymbolID(f.FIDName)
					if fidID < 0 {
						if f.FID > 0 {
							fidID = f.FID
						}
					}
				} else if f.FID > 0 {
					fidID = f.FID
				}
			} else {
				fidID = f.FID
				if c.DocSymbolTable != nil {
					name, ok := c.DocSymbolTable.FindByID(uint64(fidID))
					if ok && !strings.HasPrefix(name, "$") {
						fidName = name
						styleName = fidName
					}
				}
			}

			if fidName != "" && fidID > 0 {
				fmt.Fprintf(&sb, "  id: $%d (%s)\n", fidID, fidName)
			} else if fidName != "" {
				fmt.Fprintf(&sb, "  id: %s\n", fidName)
			} else {
				fmt.Fprintf(&sb, "  id: %s\n", fidID)
			}
		}
		fmt.Fprintf(&sb, "  value: %s\n", kfx.FormatValueCompact(f.Value))

		if css := kfx.FormatStylePropsAsCSSMultiLine(f.Value); css != "" {
			sb.WriteString(css)
		}

		if styleName != "" && styleUsage != nil {
			if usage := styleUsage[styleName]; usage != nil {
				sb.WriteString("  Usage:\n")
				if len(usage.DirectUsers) > 0 {
					fmt.Fprintf(&sb, "    Direct ($157): %d fragment(s)\n", len(usage.DirectUsers))
					for _, user := range usage.DirectUsers {
						fmt.Fprintf(&sb, "      %s\n", user)
					}
				}
				if len(usage.StyleEventUsers) > 0 {
					fmt.Fprintf(&sb, "    Style events ($142): %d fragment(s)\n", len(usage.StyleEventUsers))
					for _, user := range usage.StyleEventUsers {
						fmt.Fprintf(&sb, "      %s\n", user)
					}
				}
			} else {
				sb.WriteString("  Usage: UNUSED (no references found)\n")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String(), len(sortedFrags)
}

// formatFragmentID returns a readable identifier for a fragment.
func formatFragmentID(c *kfx.Container, frag *kfx.Fragment) string {
	if frag.IsRoot() {
		return fmt.Sprintf("%s (root)", frag.FType.Name())
	}
	if frag.FIDName != "" {
		return fmt.Sprintf("%s:%s", frag.FType.Name(), frag.FIDName)
	}
	if c.DocSymbolTable != nil {
		if name, ok := c.DocSymbolTable.FindByID(uint64(frag.FID)); ok && !strings.HasPrefix(name, "$") {
			return fmt.Sprintf("%s:%s", frag.FType.Name(), name)
		}
	}
	return fmt.Sprintf("%s:$%d", frag.FType.Name(), frag.FID)
}

// extractSymbolName extracts a style name from a symbol value.
func extractSymbolName(v any) string {
	switch s := v.(type) {
	case kfx.SymbolValue:
		return kfx.KFXSymbol(s).Name()
	case kfx.SymbolByNameValue:
		return string(s)
	case kfx.ReadSymbolValue:
		str := string(s)
		if strings.HasPrefix(str, "$") {
			if id, err := strconv.Atoi(str[1:]); err == nil {
				return kfx.KFXSymbol(id).Name()
			}
		}
		return str
	case string:
		if strings.HasPrefix(s, "$") {
			if id, err := strconv.Atoi(s[1:]); err == nil {
				return kfx.KFXSymbol(id).Name()
			}
		}
		return s
	default:
		return ""
	}
}

// toMapAny converts various map types to map[string]any for uniform access.
func toMapAny(v any) map[string]any {
	switch m := v.(type) {
	case kfx.StructValue:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("$%d", k)] = val
		}
		return result
	case map[kfx.KFXSymbol]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("$%d", k)] = val
		}
		return result
	case map[string]any:
		return m
	default:
		return nil
	}
}

// toListAny converts various list types to []any.
func toListAny(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case kfx.ListValue:
		return []any(l)
	default:
		return nil
	}
}

// keyToSymbolID extracts a symbol ID from a map key.
func keyToSymbolID(key string) int {
	if strings.HasPrefix(key, "$") {
		if id, err := strconv.Atoi(key[1:]); err == nil {
			return id
		}
	}
	return -1
}

// truncateText truncates a string to maxLen runes, adding "..." if truncated.
func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if r := []rune(s); len(r) > maxLen {
		return string(r[:maxLen]) + "..."
	}
	return s
}

// toInt64 converts various numeric types to int64.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}
