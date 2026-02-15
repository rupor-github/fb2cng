package css

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// cssEscapeDoubleQuoted escapes a string for use inside CSS double quotes.
// Backslashes and double quotes are escaped per CSS syntax: \" and \\.
func cssEscapeDoubleQuoted(s string) string {
	// Fast path: nothing to escape.
	if !strings.ContainsAny(s, `"\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

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

// Value represents a parsed CSS property value.
type Value struct {
	Raw     string  // Original CSS value string (e.g., "1.2em", "bold", "#ff0000")
	Value   float64 // Numeric value if applicable
	Unit    string  // Unit if applicable: "em", "px", "%", "pt", etc.
	Keyword string  // Keyword if applicable: "bold", "italic", "center", etc.
}

// IsNumeric returns true if the value has a numeric component.
// This includes explicit zero values like "0" or "0px".
func (v Value) IsNumeric() bool {
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
func (v Value) IsKeyword() bool {
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

// DescendantBaseName returns the base name for the rightmost part of the selector.
// Class takes precedence over element.
func (s Selector) DescendantBaseName() string {
	switch {
	case s.Class != "":
		return s.Class
	case s.Element != "":
		return s.Element
	default:
		return s.Raw
	}
}

// Rule represents a single CSS rule (selector + properties).
type Rule struct {
	Selector   Selector         // Parsed selector
	Properties map[string]Value // Property name -> value
	SourceLine int              // Line number in source for error reporting
}

// GetProperty returns the value for a property, or empty Value if not found.
func (r Rule) GetProperty(name string) (Value, bool) {
	v, ok := r.Properties[name]
	return v, ok
}

// FontFace represents an @font-face declaration.
type FontFace struct {
	Family string // font-family value
	Src    string // src value (URL or local reference)
	Style  string // font-style: normal, italic
	Weight string // font-weight: normal, bold, 400, 700
}

// StylesheetItem is a single top-level item in a stylesheet.
// Exactly one of Rule, MediaBlock, or Import is non-nil.
type StylesheetItem struct {
	Rule       *Rule       // A plain rule (selector + properties)
	MediaBlock *MediaBlock // A @media block containing nested rules
	FontFace   *FontFace   // A @font-face declaration
	Import     *string     // An @import URL
}

// MediaBlock represents a @media block with its query and nested rules.
type MediaBlock struct {
	Query MediaQuery
	Rules []Rule
}

// Stylesheet represents a parsed CSS stylesheet.
type Stylesheet struct {
	Items    []StylesheetItem // All top-level items in source order
	Warnings []string         // Warnings for unsupported features
}

// Imports returns all @import URLs from the stylesheet in source order.
func (s *Stylesheet) Imports() []string {
	var urls []string
	for _, item := range s.Items {
		if item.Import != nil {
			urls = append(urls, *item.Import)
		}
	}
	return urls
}

// FontFaces returns all @font-face declarations from the stylesheet in source order.
// Only font-faces with a non-empty Family are included (matching parser behavior).
func (s *Stylesheet) FontFaces() []FontFace {
	var faces []FontFace
	for _, item := range s.Items {
		if item.FontFace != nil && item.FontFace.Family != "" {
			faces = append(faces, *item.FontFace)
		}
	}
	return faces
}

// RulesBySelector returns all top-level rules matching the given selector string.
func (s *Stylesheet) RulesBySelector(selector string) []Rule {
	var matches []Rule
	for _, item := range s.Items {
		if item.Rule != nil && item.Rule.Selector.Raw == selector {
			matches = append(matches, *item.Rule)
		}
	}
	return matches
}

// urlRewritePattern matches url() references in CSS values for RewriteURLs.
// Handles: url("path"), url('path'), url(path)
var urlRewritePattern = regexp.MustCompile(`url\s*\(\s*(?:["']([^"']*)["']|([^)"]*))\s*\)`)

// WriteTo writes the stylesheet to w in source order, implementing io.WriterTo.
// Property order within a rule is sorted alphabetically for deterministic output.
func (s *Stylesheet) WriteTo(w io.Writer) (int64, error) {
	var total int64
	for i, item := range s.Items {
		var n int
		var err error

		switch {
		case item.Import != nil:
			n, err = fmt.Fprintf(w, "@import url(\"%s\");\n", cssEscapeDoubleQuoted(*item.Import))
		case item.FontFace != nil:
			n, err = writeFontFace(w, item.FontFace)
		case item.MediaBlock != nil:
			n, err = writeMediaBlock(w, item.MediaBlock)
		case item.Rule != nil:
			n, err = writeRule(w, item.Rule)
		}

		total += int64(n)
		if err != nil {
			return total, err
		}

		// Add blank line between items (except after last)
		if i < len(s.Items)-1 {
			n, err = fmt.Fprint(w, "\n")
			total += int64(n)
			if err != nil {
				return total, err
			}
		}
	}
	return total, nil
}

// String returns the CSS text of the stylesheet.
func (s *Stylesheet) String() string {
	var sb strings.Builder
	s.WriteTo(&sb) //nolint:errcheck
	return sb.String()
}

// writeRule writes a single CSS rule to w.
func writeRule(w io.Writer, rule *Rule) (int, error) {
	var total int
	n, err := fmt.Fprintf(w, "%s {\n", rule.Selector.Raw)
	total += n
	if err != nil {
		return total, err
	}
	n, err = writeProperties(w, rule.Properties)
	total += n
	if err != nil {
		return total, err
	}
	n, err = fmt.Fprint(w, "}\n")
	total += n
	return total, err
}

// writeProperties writes property declarations sorted alphabetically.
func writeProperties(w io.Writer, props map[string]Value) (int, error) {
	// Sort property names for deterministic output
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	var total int
	for _, name := range names {
		val := props[name]
		n, err := fmt.Fprintf(w, "  %s: %s;\n", name, val.Raw)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// writeFontFace writes an @font-face block to w.
func writeFontFace(w io.Writer, ff *FontFace) (int, error) {
	var total int
	n, err := fmt.Fprint(w, "@font-face {\n")
	total += n
	if err != nil {
		return total, err
	}

	// Write properties in a stable order
	if ff.Family != "" {
		n, err = fmt.Fprintf(w, "  font-family: \"%s\";\n", cssEscapeDoubleQuoted(ff.Family))
		total += n
		if err != nil {
			return total, err
		}
	}
	if ff.Src != "" {
		n, err = fmt.Fprintf(w, "  src: %s;\n", ff.Src)
		total += n
		if err != nil {
			return total, err
		}
	}
	if ff.Style != "" {
		n, err = fmt.Fprintf(w, "  font-style: %s;\n", ff.Style)
		total += n
		if err != nil {
			return total, err
		}
	}
	if ff.Weight != "" {
		n, err = fmt.Fprintf(w, "  font-weight: %s;\n", ff.Weight)
		total += n
		if err != nil {
			return total, err
		}
	}

	n, err = fmt.Fprint(w, "}\n")
	total += n
	return total, err
}

// writeMediaBlock writes an @media block to w.
func writeMediaBlock(w io.Writer, mb *MediaBlock) (int, error) {
	var total int
	n, err := fmt.Fprintf(w, "@media %s {\n", mb.Query.Raw)
	total += n
	if err != nil {
		return total, err
	}

	for i, rule := range mb.Rules {
		// Indent each rule line within the media block
		n, err = fmt.Fprintf(w, "  %s {\n", rule.Selector.Raw)
		total += n
		if err != nil {
			return total, err
		}

		// Write properties with double indent
		names := make([]string, 0, len(rule.Properties))
		for name := range rule.Properties {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			val := rule.Properties[name]
			n, err = fmt.Fprintf(w, "    %s: %s;\n", name, val.Raw)
			total += n
			if err != nil {
				return total, err
			}
		}

		n, err = fmt.Fprint(w, "  }\n")
		total += n
		if err != nil {
			return total, err
		}

		// Blank line between rules in a media block (except after last)
		if i < len(mb.Rules)-1 {
			n, err = fmt.Fprint(w, "\n")
			total += n
			if err != nil {
				return total, err
			}
		}
	}

	n, err = fmt.Fprint(w, "}\n")
	total += n
	return total, err
}

// RewriteURLs walks all URL references in the stylesheet and applies fn to each.
// This covers @import URLs, @font-face src, and url() references in rule properties.
func (s *Stylesheet) RewriteURLs(fn func(originalURL string) string) {
	for i := range s.Items {
		item := &s.Items[i]

		switch {
		case item.Import != nil:
			newURL := fn(*item.Import)
			item.Import = &newURL

		case item.FontFace != nil:
			item.FontFace.Src = rewriteURLsInValue(item.FontFace.Src, fn)

		case item.Rule != nil:
			rewriteURLsInProperties(item.Rule.Properties, fn)

		case item.MediaBlock != nil:
			for j := range item.MediaBlock.Rules {
				rewriteURLsInProperties(item.MediaBlock.Rules[j].Properties, fn)
			}
		}
	}
}

// rewriteURLsInProperties rewrites url() references in property values.
func rewriteURLsInProperties(props map[string]Value, fn func(string) string) {
	for name, val := range props {
		if strings.Contains(val.Raw, "url(") {
			val.Raw = rewriteURLsInValue(val.Raw, fn)
			if val.Keyword != "" && strings.Contains(val.Keyword, "url(") {
				val.Keyword = rewriteURLsInValue(val.Keyword, fn)
			}
			props[name] = val
		}
	}
}

// rewriteURLsInValue replaces url() references in a CSS value string.
func rewriteURLsInValue(value string, fn func(string) string) string {
	return urlRewritePattern.ReplaceAllStringFunc(value, func(match string) string {
		sub := urlRewritePattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		// Group 1 is quoted URL, group 2 is unquoted URL
		originalURL := sub[1]
		if originalURL == "" {
			originalURL = sub[2]
		}
		originalURL = strings.TrimSpace(originalURL)
		newURL := fn(originalURL)
		return fmt.Sprintf("url(\"%s\")", cssEscapeDoubleQuoted(newURL))
	})
}
