package kfx

import (
	"strings"

	"go.uber.org/zap"
)

// StyleMapper is a thin façade for upcoming stylemap-driven mapping.
// Currently it reuses the CSS converter to translate normalized CSS maps
// into KFX properties while emitting tracer MAP entries per property.
type StyleMapper struct {
	converter *Converter
	styleMap  *StyleMap
}

// WrapperCSS represents a normalized wrapper with tag, classes, and CSS map.
type WrapperCSS struct {
	Tag        string
	Classes    []string
	Properties map[string]CSSValue
}

// NewStyleMapper creates a mapper with an attached tracer-aware converter.
func NewStyleMapper(log *zap.Logger, tracer *StyleTracer) *StyleMapper {
	c := NewConverter(log)
	c.SetTracer(tracer)
	return &StyleMapper{
		converter: c,
		styleMap:  NewDefaultStyleMap(),
	}
}

// SetStyleMap attaches a stylemap for lookup-driven mapping.
func (m *StyleMapper) SetStyleMap(sm *StyleMap) {
	m.styleMap = sm
}

// MapStylesheet converts an entire stylesheet using stylemap-aware mapping.
func (m *StyleMapper) MapStylesheet(sheet *Stylesheet) ([]StyleDef, []string) {
	if sheet == nil {
		return nil, nil
	}

	styles := make([]StyleDef, 0, len(sheet.Rules))
	allWarnings := make([]string, 0)

	styleMap := make(map[string]*StyleDef)
	var styleOrder []string

	dropcapInfo := map[string]dropcapConfig{}
	if m.converter != nil {
		dropcapInfo = m.converter.detectDropcapPatterns(sheet)
	}

	for _, rule := range sheet.Rules {
		props, warnings := m.MapCSS(rule.Selector, rule.Properties)
		allWarnings = append(allWarnings, warnings...)

		if len(props) == 0 {
			continue
		}

		style := StyleDef{
			Name:       rule.Selector.StyleName(),
			Properties: props,
		}

		if rule.Selector.Ancestor != nil && style.Parent == "" {
			descendantName := rule.Selector.descendantBaseName()
			if descendantName != "" && descendantName != style.Name {
				style.Parent = descendantName
			}
		}

		if info, ok := dropcapInfo[style.Name]; ok {
			style.Properties[SymDropcapChars] = info.chars
			style.Properties[SymDropcapLines] = info.lines
		}

		if m.converter != nil && m.converter.tracer != nil && m.converter.tracer.IsEnabled() {
			m.converter.tracer.TraceCSSConvert(rule.Selector.Raw, style.Properties)
		}

		if existing, ok := styleMap[style.Name]; ok {
			mergeAllWithRules(existing.Properties, style.Properties, mergeContextInline, m.converter.tracer)
		} else {
			styleCopy := style
			styleMap[style.Name] = &styleCopy
			styleOrder = append(styleOrder, style.Name)
		}
	}

	for _, name := range styleOrder {
		styles = append(styles, *styleMap[name])
	}

	allWarnings = append(allWarnings, sheet.Warnings...)
	return styles, allWarnings
}

// MapWrapper converts a normalized wrapper CSS map (tag + classes) to a StyleDef.
// Multiple classes are flattened using the first class as the primary selector component.
func (m *StyleMapper) MapWrapper(tag string, classes []string, props map[string]CSSValue) (StyleDef, []string) {
	selector := selectorFromTagClasses(tag, classes)
	styleProps, warnings := m.MapCSS(selector, props)
	return StyleDef{
		Name:       selector.StyleName(),
		Properties: styleProps,
	}, warnings
}

// MapWrappers converts multiple wrappers and merges styles with the same name using stylelist rules.
func (m *StyleMapper) MapWrappers(wrappers []WrapperCSS) ([]StyleDef, []string) {
	warnings := make([]string, 0)
	merged := make(map[string]map[KFXSymbol]any)
	order := make([]string, 0)

	for _, w := range wrappers {
		def, ws := m.MapWrapper(w.Tag, w.Classes, w.Properties)
		warnings = append(warnings, ws...)
		if props, ok := merged[def.Name]; ok {
			if m.converter != nil && m.converter.log != nil {
				m.converter.log.Debug("merging wrapper style with stylelist rules",
					zap.String("style", def.Name),
					zap.Int("existingProperties", len(props)),
					zap.Int("incomingProperties", len(def.Properties)))
			}
			mergeAllWithRules(props, def.Properties, mergeContextWrapper, m.converter.tracer)
		} else {
			merged[def.Name] = def.Properties
			order = append(order, def.Name)
		}
	}

	out := make([]StyleDef, 0, len(order))
	for _, name := range order {
		out = append(out, StyleDef{Name: name, Properties: merged[name]})
	}
	return out, warnings
}

// MapCSS converts a normalized CSS map for a tag/class selector into KFX properties.
// Further stylemap/transformer logic will plug into this path; for now we delegate
// to the CSS converter to keep behavior unchanged while providing a staging hook.
func (m *StyleMapper) MapCSS(selector Selector, props map[string]CSSValue) (map[KFXSymbol]any, []string) {
	props = m.applyStyleMapCSS(selector, props)

	result := m.converter.ConvertRule(CSSRule{
		Selector:   selector,
		Properties: props,
	})
	warnings := result.Warnings

	if m.styleMap != nil {
		for _, match := range styleMapMatches(selector, props) {
			entries := m.styleMap.EntriesFor(match.key)
			if len(entries) == 0 || m.converter.tracer == nil || !m.converter.tracer.IsEnabled() {
				continue
			}
			for _, entry := range entries {
				m.converter.tracer.TraceMap(match.key.Attr, "stylemap:"+entry.Property, result.Style.Properties)
			}
		}
	}

	// Apply direct stylemap→KFX property overrides.
	if m.styleMap != nil {
		for _, match := range styleMapMatches(selector, props) {
			for _, entry := range m.styleMap.EntriesFor(match.key) {
				if selector.Ancestor != nil && strings.Contains(entry.Transformer, "UserAgentStyleAddingTransformer") {
					continue
				}
				if strings.Contains(entry.Transformer, "TransformerForWebkitTransform") && m.converter != nil && m.converter.log != nil {
					m.converter.log.Debug("ignoring -webkit-transform per KP3 behavior",
						zap.String("selector", selector.Raw),
						zap.String("value", formatCSSValue(match.val)))
				}
				if kvs, ok := styleMapKFXOverride(entry, match.val, match.key.Attr, match.element, m.converter.log); ok {
					for sym, val := range kvs {
						if _, exists := result.Style.Properties[sym]; exists {
							if entry.Transformer == "" || strings.Contains(entry.Transformer, "UserAgentStyleAddingTransformer") {
								continue
							}
						}
						mergePropertyWithRules(result.Style.Properties, sym, val, mergeContextInline, m.converter.tracer)
					}
				}
			}
		}
	}

	if snapSym, ok := symbolIDFromString("snap_block"); ok {
		if sym, ok := symbolIDFromAny(result.Style.Properties[SymFloat]); ok && sym == snapSym && selector.Element != "img" {
			delete(result.Style.Properties, SymFloat)
		}
	}

	return result.Style.Properties, warnings
}
