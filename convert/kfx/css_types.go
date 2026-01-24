package kfx

import (
	"strings"
	"unicode"
)

// MediaQuery represents a parsed @media query condition.
// Supports Amazon-specific media types: amzn-mobi, amzn-kf8, amzn-et.
type MediaQuery struct {
	Raw      string         // Original media query string
	Type     string         // Media type (e.g., "amzn-mobi", "amzn-kf8")
	Negated  bool           // true if "not" modifier was used on main type
	Features []MediaFeature // Additional conditions (e.g., "and not amzn-et")
}

// MediaFeature represents a single media feature condition in a media query.
type MediaFeature struct {
	Name    string // Feature name (e.g., "amzn-et")
	Negated bool   // true if "not" modifier was used
}

// Evaluate returns true if this media query matches the given context.
// For KFX generation, amzn-kf8=true and amzn-et=true (Enhanced Typesetting).
// amzn-mobi is always false for KFX.
func (mq MediaQuery) Evaluate(kf8, et bool) bool {
	// Evaluate main type
	var typeMatches bool
	switch strings.ToLower(mq.Type) {
	case "amzn-kf8":
		typeMatches = kf8
	case "amzn-et":
		typeMatches = et
	case "amzn-mobi":
		typeMatches = false // Always false for KFX
	case "all", "screen":
		typeMatches = true // Generic media types match
	default:
		typeMatches = false // Unknown media type
	}

	if mq.Negated {
		typeMatches = !typeMatches
	}

	if !typeMatches {
		return false
	}

	// Evaluate all features (AND logic)
	for _, f := range mq.Features {
		var featureMatches bool
		switch strings.ToLower(f.Name) {
		case "amzn-kf8":
			featureMatches = kf8
		case "amzn-et":
			featureMatches = et
		case "amzn-mobi":
			featureMatches = false
		default:
			featureMatches = false
		}

		if f.Negated {
			featureMatches = !featureMatches
		}

		if !featureMatches {
			return false
		}
	}

	return true
}

// EvaluateForKFX returns true if this media query matches KFX generation context.
// In KFX context: amzn-kf8=true, amzn-et=true.
func (mq MediaQuery) EvaluateForKFX() bool {
	return mq.Evaluate(true, true)
}

// CSSValue represents a parsed CSS property value.
type CSSValue struct {
	Raw     string  // Original CSS value string (e.g., "1.2em", "bold", "#ff0000")
	Value   float64 // Numeric value if applicable
	Unit    string  // Unit if applicable: "em", "px", "%", "pt", etc.
	Keyword string  // Keyword if applicable: "bold", "italic", "center", etc.
}

// IsNumeric returns true if the value has a numeric component.
// This includes explicit zero values like "0" or "0px".
func (v CSSValue) IsNumeric() bool {
	// If there's a unit, it's definitely numeric
	if v.Unit != "" {
		return true
	}
	// Non-zero value with no keyword is numeric
	if v.Value != 0 && v.Keyword == "" {
		return true
	}
	// Check if Raw looks like a numeric value (handles "0" case)
	// We check if Raw is not empty and starts with a digit, dot, or minus
	if v.Raw != "" && v.Keyword == "" {
		firstChar := rune(v.Raw[0])
		if unicode.IsDigit(firstChar) || firstChar == '.' || firstChar == '-' || firstChar == '+' {
			return true
		}
	}
	return false
}

// IsKeyword returns true if the value is a keyword (no numeric component).
func (v CSSValue) IsKeyword() bool {
	return v.Keyword != "" && v.Unit == ""
}

// PseudoElement represents which pseudo-element a rule applies to.
type PseudoElement int

const (
	PseudoNone   PseudoElement = iota // No pseudo-element
	PseudoBefore                      // ::before
	PseudoAfter                       // ::after
)

// String returns the CSS representation of the pseudo-element.
func (p PseudoElement) String() string {
	switch p {
	case PseudoBefore:
		return "::before"
	case PseudoAfter:
		return "::after"
	default:
		return ""
	}
}

// Selector represents a parsed CSS selector with its components.
type Selector struct {
	Raw      string        // Original selector string
	Element  string        // Element name (e.g., "p", "h1") or empty for class-only
	Class    string        // Class name without dot (e.g., "paragraph") or empty
	Pseudo   PseudoElement // Pseudo-element if present
	Ancestor *Selector     // Ancestor selector for descendant selectors (e.g., "p code" -> Ancestor is "p")
}

// IsSimple returns true if this is a simple selector (element, class, or element.class).
func (s Selector) IsSimple() bool {
	return s.Element != "" || s.Class != ""
}

// IsDescendant returns true if this is a descendant selector.
func (s Selector) IsDescendant() bool {
	return s.Ancestor != nil
}

// StyleName returns the KFX style name for this selector.
// Rules:
//   - .foo -> "foo"
//   - tag -> "tag"
//   - tag.foo -> "foo" (class takes precedence)
//   - ancestor descendant -> "ancestor--descendant" (e.g., "p code" -> "p--code")
//   - selector::before -> "selector--before"
//   - selector::after -> "selector--after"
func (s Selector) StyleName() string {
	var base string

	// For descendant selectors, combine ancestor and descendant names
	if s.Ancestor != nil {
		ancestorName := s.Ancestor.StyleName()
		descendantName := s.descendantBaseName()
		base = ancestorName + "--" + descendantName
	} else {
		base = s.descendantBaseName()
	}

	switch s.Pseudo {
	case PseudoBefore:
		return base + "--before"
	case PseudoAfter:
		return base + "--after"
	default:
		return base
	}
}

// descendantBaseName returns the base name for the rightmost part of the selector.
func (s Selector) descendantBaseName() string {
	switch {
	case s.Class != "":
		return s.Class
	case s.Element != "":
		return s.Element
	default:
		return s.Raw
	}
}

// CSSRule represents a single CSS rule (selector + properties).
type CSSRule struct {
	Selector   Selector            // Parsed selector
	Properties map[string]CSSValue // Property name -> value
	SourceLine int                 // Line number in source for error reporting
}

// GetProperty returns the value for a property, or empty CSSValue if not found.
func (r CSSRule) GetProperty(name string) (CSSValue, bool) {
	v, ok := r.Properties[name]
	return v, ok
}

// CSSFontFace represents an @font-face declaration.
type CSSFontFace struct {
	Family string // font-family value
	Src    string // src value (URL or local reference)
	Style  string // font-style: normal, italic
	Weight string // font-weight: normal, bold, 400, 700
}

// Stylesheet represents a parsed CSS stylesheet.
type Stylesheet struct {
	Rules     []CSSRule     // All parsed rules
	FontFaces []CSSFontFace // @font-face declarations
	Warnings  []string      // Warnings for unsupported features
}

// RulesBySelector returns all rules matching the given selector string.
func (s *Stylesheet) RulesBySelector(selector string) []CSSRule {
	var matches []CSSRule
	for _, r := range s.Rules {
		if r.Selector.Raw == selector {
			matches = append(matches, r)
		}
	}
	return matches
}

// StyleNames returns all unique style names that would be generated.
func (s *Stylesheet) StyleNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, r := range s.Rules {
		name := r.Selector.StyleName()
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}
