package kfx

import (
	"fbc/css"
)

// selectorStyleName returns the KFX style name for a CSS selector.
// Rules:
//   - .foo -> "foo"
//   - tag -> "tag"
//   - tag.foo -> "foo" (class takes precedence)
//   - ancestor descendant -> "ancestor--descendant" (e.g., "p code" -> "p--code")
//   - selector::before -> "selector--before"
//   - selector::after -> "selector--after"
func selectorStyleName(s css.Selector) string {
	var base string

	// For descendant selectors, combine ancestor and descendant names
	if s.Ancestor != nil {
		ancestorName := selectorStyleName(*s.Ancestor)
		descendantName := selectorDescendantName(s)
		base = ancestorName + "--" + descendantName
	} else {
		base = s.DescendantBaseName()
	}

	switch s.Pseudo {
	case css.PseudoBefore:
		return base + "--before"
	case css.PseudoAfter:
		return base + "--after"
	default:
		return base
	}
}

// selectorDescendantName returns the base name for the rightmost part
// when this selector participates in a descendant selector.
//
// For descendant selectors we sometimes need to preserve element+class specificity
// (e.g. ".section-title h2.section-title-header") so element-qualified rules
// don't apply to other tags that share the same class.
func selectorDescendantName(s css.Selector) string {
	if s.Element != "" && s.Class != "" {
		return s.Element + "." + s.Class
	}
	return s.DescendantBaseName()
}

// flattenStylesheetForKFX returns all CSS rules from a stylesheet, flattening
// @media blocks that match the KFX context (kf8=true, et=true).
// Rules from non-matching @media blocks are excluded.
// The result preserves source order.
func flattenStylesheetForKFX(sheet *css.Stylesheet) []css.CSSRule {
	if sheet == nil {
		return nil
	}
	var rules []css.CSSRule
	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			rules = append(rules, *item.Rule)
		case item.MediaBlock != nil:
			if item.MediaBlock.Query.Evaluate(true, true) {
				rules = append(rules, item.MediaBlock.Rules...)
			}
		}
	}
	return rules
}
