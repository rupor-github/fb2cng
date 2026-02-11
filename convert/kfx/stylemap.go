package kfx

import (
	"strconv"
	"strings"

	"fbc/css"
)

// HTMLKey identifies a stylemap entry trigger: tag + attribute + optional value/unit.
type HTMLKey struct {
	Tag   string
	Attr  string
	Value string
	Unit  string
}

// StyleMapEntry represents a single stylemap rule.
type StyleMapEntry struct {
	Key          HTMLKey
	Property     string
	Value        string
	ValueList    []string
	Unit         string
	Display      string
	Transformer  string
	Conversion   string
	CSSStyles    map[string]string // optional css_styles emitted by entry
	IgnoreHTML   bool
	ValueType    string
	Transformers []string
}

// StyleMap stores entries by HTMLKey.
type StyleMap struct {
	entries map[HTMLKey][]StyleMapEntry
}

// NewStyleMap creates an empty style map.
func NewStyleMap() *StyleMap {
	return &StyleMap{
		entries: make(map[HTMLKey][]StyleMapEntry),
	}
}

// NewStyleMapFromEntries builds a StyleMap from the provided entries.
func NewStyleMapFromEntries(entries []StyleMapEntry) *StyleMap {
	sm := NewStyleMap()
	for _, e := range entries {
		sm.Add(e)
	}
	return sm
}

// NewDefaultStyleMap constructs a StyleMap containing the bundled default entries.
func NewDefaultStyleMap() *StyleMap {
	return NewStyleMapFromEntries(defaultStyleMapEntries)
}

// Add inserts an entry keyed by its HTMLKey.
func (sm *StyleMap) Add(entry StyleMapEntry) {
	sm.entries[entry.Key] = append(sm.entries[entry.Key], entry)
}

// EntriesFor returns all entries matching the given key.
func (sm *StyleMap) EntriesFor(key HTMLKey) []StyleMapEntry {
	return sm.entries[key]
}

func propertyToCSSName(prop string) (string, bool) {
	if prop == "" {
		return "", false
	}
	// Skip conversion-style properties for now.
	if strings.Contains(prop, ".") {
		return "", false
	}
	// Skip compound properties (comma-separated); they are handled via
	// styleMapKFXOverride which splits them and processes each separately.
	if strings.Contains(prop, ",") {
		return "", false
	}
	if prop == "text_combine" {
		return "text-combine-upright", true
	}
	if prop == "text_alignment" {
		return "text-align", true
	}
	return strings.ReplaceAll(prop, "_", "-"), true
}

func parseStyleMapCSSValue(val, unit string) css.CSSValue {
	val = strings.TrimSpace(val)
	if val == "" {
		return css.CSSValue{}
	}

	if n, err := strconv.ParseFloat(val, 64); err == nil {
		return css.CSSValue{Value: n, Unit: unit, Raw: val}
	}

	// Try to split number+unit (e.g., 1.5em)
	numPart := ""
	unitPart := ""
	for i, r := range val {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '+' {
			numPart += string(r)
		} else {
			unitPart = val[i:]
			break
		}
	}
	if numPart != "" {
		if n, err := strconv.ParseFloat(numPart, 64); err == nil {
			if unitPart == "" {
				unitPart = unit
			}
			return css.CSSValue{Value: n, Unit: unitPart, Raw: val}
		}
	}

	return css.CSSValue{Raw: val, Keyword: val}
}
