package kfx

import (
	"strings"

	"go.uber.org/zap"

	"fbc/css"
)

func (m *StyleMapper) applyStyleMapCSS(sel css.Selector, props map[string]css.Value) map[string]css.Value {
	if m.styleMap == nil {
		return props
	}

	filtered := m.filterIgnorable(sel, props)

	merged := make(map[string]css.Value, len(filtered))
	for k, v := range filtered {
		merged[k] = v
	}

	for _, match := range styleMapMatches(sel, merged) {
		for _, entry := range m.styleMap.EntriesFor(match.key) {
			if sel.Ancestor != nil && strings.Contains(entry.Transformer, "UserAgentStyleAddingTransformer") {
				continue
			}
			if cssProp, overrideVal, ok := styleMapCSSOverride(entry); ok {
				if !cssPropertyCoveredByMerged(merged, cssProp) {
					val := overrideVal
					if val == "" {
						val = firstNonEmpty(entry.Value, entry.ValueList...)
					}
					merged[cssProp] = parseStyleMapCSSValue(val, entry.Unit)
				}
			} else if cssProp, ok := propertyToCSSName(entry.Property); ok {
				if !cssPropertyCoveredByMerged(merged, cssProp) {
					val := firstNonEmpty(entry.Value, entry.ValueList...)
					if val != "" {
						merged[cssProp] = parseStyleMapCSSValue(val, entry.Unit)
					}
				}
			}
			if entry.Display != "" {
				if _, exists := merged["display"]; !exists {
					merged["display"] = parseStyleMapCSSValue(entry.Display, "")
				}
			}
			for cssName, cssVal := range entry.CSSStyles {
				if cssPropertyCoveredByMerged(merged, cssName) {
					continue
				}
				// Parse the CSS value string to properly extract value and unit
				merged[cssName] = parseStyleMapCSSValue(cssVal, "")
			}
		}
	}

	return merged
}

// cssPropertyCoveredByMerged returns true if the CSS property is already covered
// by existing properties in merged, either directly or via a shorthand property.
// For example, if merged has "margin", then "margin-left" is covered.
func cssPropertyCoveredByMerged(merged map[string]css.Value, prop string) bool {
	if _, exists := merged[prop]; exists {
		return true
	}
	// Check if a shorthand property covers this property
	shorthand := shorthandForProperty(prop)
	if shorthand != "" {
		if _, exists := merged[shorthand]; exists {
			return true
		}
	}
	return false
}

// shorthandForProperty returns the shorthand property name that covers the given
// property, or empty string if none.
func shorthandForProperty(prop string) string {
	switch {
	case strings.HasPrefix(prop, "margin-"):
		return "margin"
	case strings.HasPrefix(prop, "padding-"):
		return "padding"
	case strings.HasPrefix(prop, "border-"):
		return "border"
	case strings.HasPrefix(prop, "background-"):
		return "background"
	}
	return ""
}

func styleMapKeysFromSelector(sel css.Selector) []HTMLKey {
	keys := make([]HTMLKey, 0, 3)

	// Tag match
	if sel.Element != "" {
		keys = append(keys, HTMLKey{Tag: sel.Element})
	}

	// Class matches: generic and tag-scoped
	if sel.Class != "" {
		keys = append(keys,
			HTMLKey{Tag: "*", Attr: "class", Value: sel.Class},
			HTMLKey{Tag: sel.Element, Attr: "class", Value: sel.Class},
		)
	}

	return keys
}

type styleMapMatch struct {
	key     HTMLKey
	val     css.Value
	element string
}

func styleMapMatches(sel css.Selector, props map[string]css.Value) []styleMapMatch {
	matches := make([]styleMapMatch, 0, len(props)+3)

	for _, key := range styleMapKeysFromSelector(sel) {
		matches = append(matches, styleMapMatch{key: key, element: sel.Element})
	}

	for name, val := range props {
		formatted := formatCSSValue(val)
		keys := []HTMLKey{
			{Attr: name, Value: formatted, Unit: val.Unit},
			{Attr: name, Value: "", Unit: val.Unit},
			{Attr: name, Value: formatted, Unit: "special_unit"},
			{Attr: name, Value: "", Unit: "special_unit"},
		}
		if _, _, _, ok := ParseColor(val); ok {
			keys = append(keys,
				HTMLKey{Attr: name, Value: formatted, Unit: "color_unit"},
				HTMLKey{Attr: name, Value: "", Unit: "color_unit"},
			)
		}
		if sel.Element != "" {
			keys = append(keys,
				HTMLKey{Tag: sel.Element, Attr: name, Value: formatted, Unit: val.Unit},
				HTMLKey{Tag: sel.Element, Attr: name, Value: "", Unit: val.Unit},
				HTMLKey{Tag: sel.Element, Attr: name, Value: formatted, Unit: "special_unit"},
				HTMLKey{Tag: sel.Element, Attr: name, Value: "", Unit: "special_unit"},
			)
			if _, _, _, ok := ParseColor(val); ok {
				keys = append(keys,
					HTMLKey{Tag: sel.Element, Attr: name, Value: formatted, Unit: "color_unit"},
					HTMLKey{Tag: sel.Element, Attr: name, Value: "", Unit: "color_unit"},
				)
			}
		}
		for _, k := range keys {
			matches = append(matches, styleMapMatch{key: k, val: val, element: sel.Element})
		}
	}

	return matches
}

func (m *StyleMapper) filterIgnorable(sel css.Selector, props map[string]css.Value) map[string]css.Value {
	if len(defaultIgnorablePatterns) == 0 {
		return props
	}

	out := make(map[string]css.Value, len(props))
	for name, val := range props {
		if isIgnorable(sel, name, val) {
			if m.converter != nil && m.converter.log != nil {
				m.converter.log.Debug("Style ignored by mapping_ignorable_patterns",
					zap.String("selector", sel.Raw),
					zap.String("property", name),
					zap.String("value", formatCSSValue(val)))
			}
			continue
		}
		out[name] = val
	}
	return out
}

func isIgnorable(sel css.Selector, propName string, val css.Value) bool {
	// Known KFX properties are never ignorable
	if KFXPropertySymbol(propName) != SymbolUnknown {
		return false
	}

	// Shorthand properties are never ignorable - they expand to known properties
	if IsShorthandProperty(propName) {
		return false
	}

	for _, p := range defaultIgnorablePatterns {
		if p.Tag != "*" && p.Tag != sel.Element {
			continue
		}
		if p.Style != "*" && p.Style != propName {
			continue
		}
		if p.Value != "*" && p.Value != formatCSSValue(val) {
			continue
		}
		if p.Unit != "*" && p.Unit != val.Unit {
			continue
		}
		return true
	}
	return false
}

func styleMapCSSOverride(entry StyleMapEntry) (cssProp string, overrideVal string, ok bool) {
	switch entry.Property {
	case "underline":
		return "text-decoration", "underline", true
	case "strikethrough":
		return "text-decoration", "line-through", true
	case "text_color":
		return "color", "", true
	}
	return "", "", false
}

func styleMapKFXOverride(entry StyleMapEntry, cssVal css.Value, sourceAttr string, element string, log *zap.Logger) (map[KFXSymbol]any, bool) {
	if strings.Contains(entry.Transformer, "NonBlockingBlockImageTransformer") && element != "img" {
		return nil, false
	}

	props := strings.Split(entry.Property, ",")
	values := splitCSV(entry.Value)
	units := splitCSV(entry.Unit)
	valueTypes := splitCSV(entry.ValueType)

	result := make(map[KFXSymbol]any)

	for i, prop := range props {
		prop = strings.TrimSpace(prop)
		if prop == "" {
			continue
		}
		val := pickCSSValue(cssVal, entry)
		if i < len(values) && values[i] != "" {
			val = parseStyleMapCSSValue(values[i], firstOrEmpty(units, i))
		}
		valType := firstOrEmpty(valueTypes, i)
		if kvs, ok := convertStyleMapProp(prop, val, firstOrEmpty(values, i), firstOrEmpty(units, i), valType, sourceAttr, log); ok {
			for sym, v := range kvs {
				result[sym] = v
			}
		}
	}

	if len(result) == 0 {
		return nil, false
	}
	return result, true
}

func pickCSSValue(val css.Value, entry StyleMapEntry) css.Value {
	if !isEmptyCSSValue(val) {
		return val
	}
	v := firstNonEmpty(entry.Value, entry.ValueList...)
	return parseStyleMapCSSValue(v, entry.Unit)
}
