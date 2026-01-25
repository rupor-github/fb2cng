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

// NewStyleMapper creates a mapper with an attached tracer-aware converter.
func NewStyleMapper(log *zap.Logger, tracer *StyleTracer) *StyleMapper {
	c := NewConverter(log)
	c.SetTracer(tracer)
	return &StyleMapper{
		converter: c,
		styleMap:  NewDefaultStyleMap(),
	}
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
		props, warnings := m.MapRule(rule.Selector, rule.Properties)
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
			// When CSS rules produce the same style name (e.g., ".cite" and "blockquote.cite"),
			// use simple override semantics. In CSS, later rules with equal specificity override
			// earlier ones, they don't accumulate. The stylelist rules are for runtime style
			// merging, not CSS cascade behavior.
			mergeAllOverride(existing.Properties, style.Properties)
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

// MapRule converts a single CSS rule (selector + properties) into KFX properties.
// Used internally by MapStylesheet, but exported for testing
// and for callers that need to convert individual rules programmatically.
// It applies stylemap lookups and transformers on top of the base CSS conversion.
func (m *StyleMapper) MapRule(selector Selector, props map[string]CSSValue) (map[KFXSymbol]any, []string) {
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
				via := "stylemap:" + entry.Property
				if entry.Transformer != "" {
					via += " (" + entry.Transformer + ")"
				}
				m.converter.tracer.TraceMap(match.key.Attr, via, result.Style.Properties)
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
