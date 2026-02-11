package kfx

import (
	"fmt"
	"strings"

	"fbc/css"
)

// PseudoElementContent holds the text content for CSS ::before and ::after pseudo-elements.
// Since KFX doesn't support CSS pseudo-elements natively, we inject this content directly
// into the text. The injected content inherits styling from the base element.
type PseudoElementContent struct {
	Before string // Text to prepend (from ::before { content: "..." })
	After  string // Text to append (from ::after { content: "..." })
}

// extractPseudoContent scans the stylesheet for pseudo-element rules (::before, ::after)
// and extracts their content property values. This must be called before CSS conversion
// because the content property is not converted to KFX properties.
//
// Returns warnings for any pseudo-element rules that have properties other than `content`,
// since those properties cannot be applied in KFX (the content inherits from the base element).
func (sr *StyleRegistry) extractPseudoContent(sheet *css.Stylesheet) []string {
	if sheet == nil {
		return nil
	}

	var warnings []string

	rules := flattenStylesheetForKFX(sheet)
	for _, rule := range rules {
		// Only process pseudo-element rules
		if rule.Selector.Pseudo == css.PseudoNone {
			continue
		}

		// Check for content property
		contentVal, hasContent := rule.Properties["content"]
		if !hasContent {
			continue
		}

		// Warn about any properties other than 'content' - they can't be applied in KFX
		// because pseudo-element content inherits styling from the base element
		for propName := range rule.Properties {
			if propName != "content" {
				warnings = append(warnings, fmt.Sprintf(
					"pseudo-element %q has property %q which will be ignored (only 'content' is supported)",
					rule.Selector.Raw, propName))
			}
		}

		// Parse the content value
		text := parseCSSContent(contentVal)
		if text == "" {
			continue
		}

		// Register the pseudo-element content
		styleName := selectorStyleName(rule.Selector)
		sr.registerPseudoContentByName(styleName, text)
	}

	return warnings
}

// registerPseudoContentByName stores the content property value for a pseudo-element style.
// styleName is the full style name including the --before or --after suffix.
// content is the parsed content property value (text without quotes).
func (sr *StyleRegistry) registerPseudoContentByName(styleName, content string) {
	if sr.pseudoContent == nil {
		sr.pseudoContent = make(map[string]*PseudoElementContent)
	}

	// Extract the base style name and pseudo type
	var baseName string
	var isBefore, isAfter bool

	if after, found := strings.CutSuffix(styleName, "--before"); found {
		baseName = after
		isBefore = true
	} else if after, found := strings.CutSuffix(styleName, "--after"); found {
		baseName = after
		isAfter = true
	} else {
		// Not a pseudo-element style
		return
	}

	// Get or create the content entry
	pc := sr.pseudoContent[baseName]
	if pc == nil {
		pc = &PseudoElementContent{}
		sr.pseudoContent[baseName] = pc
	}

	if isBefore {
		pc.Before = content
	} else if isAfter {
		pc.After = content
	}
}

// GetPseudoContent returns the pseudo-element content for a style.
// It checks the style name and all its classes for registered pseudo-element content.
// Returns nil if no pseudo-element content is registered.
func (sr *StyleRegistry) GetPseudoContent(classes string) *PseudoElementContent {
	if sr.pseudoContent == nil {
		return nil
	}

	// Check each class for pseudo-element content
	for _, class := range strings.Fields(classes) {
		if pc, ok := sr.pseudoContent[class]; ok {
			return pc
		}
	}

	return nil
}

// GetPseudoContentForClass returns the pseudo-element content for a specific class name.
// Returns nil if no pseudo-element content is registered for this class.
func (sr *StyleRegistry) GetPseudoContentForClass(class string) *PseudoElementContent {
	if sr.pseudoContent == nil {
		return nil
	}
	return sr.pseudoContent[class]
}

// HasPseudoContent returns true if any pseudo-element content is registered.
func (sr *StyleRegistry) HasPseudoContent() bool {
	return len(sr.pseudoContent) > 0
}

// parseCSSContent parses a CSS content property value.
// It handles quoted strings like 'content: "["' or "content: '['"
// Returns the unquoted text content, or empty string if not a string value.
func parseCSSContent(value css.Value) string {
	raw := strings.TrimSpace(value.Raw)

	// Handle none/normal keywords
	if raw == "none" || raw == "normal" || raw == "" {
		return ""
	}

	// Handle quoted strings: "..." or '...'
	if len(raw) >= 2 {
		if (raw[0] == '"' && raw[len(raw)-1] == '"') ||
			(raw[0] == '\'' && raw[len(raw)-1] == '\'') {
			return raw[1 : len(raw)-1]
		}
	}

	// Not a string value (could be attr(), counter(), etc. which we don't support)
	return ""
}
