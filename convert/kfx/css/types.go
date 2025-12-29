package css

// CSSValue represents a parsed CSS property value.
type CSSValue struct {
	Raw     string  // Original CSS value string (e.g., "1.2em", "bold", "#ff0000")
	Value   float64 // Numeric value if applicable
	Unit    string  // Unit if applicable: "em", "px", "%", "pt", etc.
	Keyword string  // Keyword if applicable: "bold", "italic", "center", etc.
}

// IsNumeric returns true if the value has a numeric component.
func (v CSSValue) IsNumeric() bool {
	return v.Unit != "" || (v.Value != 0 && v.Keyword == "")
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

// HasProperty returns true if the rule has the specified property.
func (r CSSRule) HasProperty(name string) bool {
	_, ok := r.Properties[name]
	return ok
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

// RulesByStyleName returns all rules that would produce the given style name.
func (s *Stylesheet) RulesByStyleName(styleName string) []CSSRule {
	var matches []CSSRule
	for _, r := range s.Rules {
		if r.Selector.StyleName() == styleName {
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
