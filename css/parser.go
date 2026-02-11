package css

import (
	"bytes"
	"maps"
	"strconv"
	"strings"
	"unicode"

	parse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"go.uber.org/zap"
)

// Parser parses CSS stylesheets into structured rules.
type Parser struct {
	log *zap.Logger
}

// NewParser creates a new CSS parser.
func NewParser(log *zap.Logger) *Parser {
	if log == nil {
		log = zap.NewNop()
	}
	return &Parser{log: log.Named("css-parser")}
}

// Parse parses CSS text into a Stylesheet.
// The optional source parameter identifies what's being parsed (for debug logging).
func (p *Parser) Parse(data []byte, source ...string) *Stylesheet {
	sheet := &Stylesheet{
		Items:    make([]StylesheetItem, 0),
		Warnings: make([]string, 0),
	}

	// Log parsing start with source identifier if provided
	if len(source) > 0 && source[0] != "" {
		p.log.Debug("Parsing CSS", zap.String("source", source[0]), zap.Int("bytes", len(data)))
	}

	input := parse.NewInput(bytes.NewReader(data))
	parser := css.NewParser(input, false)

	var currentSelectors []string

	for {
		gt, _, data := parser.Next()

		switch gt {
		case css.ErrorGrammar:
			// End of input or error
			if parser.Err() != nil && parser.Err().Error() != "EOF" {
				p.log.Debug("CSS parse error", zap.Error(parser.Err()))
			}
			return sheet

		case css.BeginAtRuleGrammar:
			atRule := string(data)
			switch atRule {
			case "@media":
				// Parse @media query and preserve the block in the AST
				mq := p.parseMediaQueryFromTokens(parser.Values())
				rules := p.parseMediaBlockRules(parser, sheet)
				p.log.Debug("Parsed @media block", zap.String("query", mq.Raw), zap.Int("rules", len(rules)))
				sheet.Items = append(sheet.Items, StylesheetItem{
					MediaBlock: &MediaBlock{Query: mq, Rules: rules},
				})
			case "@font-face":
				// Parse @font-face
				ff := p.parseFontFace(parser)
				sheet.Items = append(sheet.Items, StylesheetItem{FontFace: &ff})
				if ff.Family != "" {
					sheet.FontFaces = append(sheet.FontFaces, ff)
				}
			default:
				// Skip other @-rules with blocks
				p.skipAtRuleBlock(parser)
				p.log.Debug("Skipping @-rule", zap.String("rule", atRule))
			}
		case css.AtRuleGrammar:
			// Simple @-rule without block (e.g., @import)
			atRule := string(data)
			if atRule == "@import" {
				url := extractImportURL(parser.Values())
				if url != "" {
					sheet.Items = append(sheet.Items, StylesheetItem{Import: &url})
					sheet.Imports = append(sheet.Imports, url)
					p.log.Debug("Parsed @import", zap.String("url", url))
				}
			} else {
				p.log.Debug("Skipping @-rule", zap.String("rule", atRule))
			}

		case css.BeginRulesetGrammar:
			// Collect selector tokens
			currentSelectors = p.parseSelectors(data, parser.Values())

		case css.DeclarationGrammar:
			// Property declaration - already handled in EndRulesetGrammar

		case css.EndRulesetGrammar:
			// End of ruleset - we need to re-parse to get declarations
			// This is handled differently - the declarations come before EndRulesetGrammar

		case css.QualifiedRuleGrammar:
			// This shouldn't happen in our flow, but handle it
			currentSelectors = p.parseSelectors(data, parser.Values())
		}

		// Check for declarations after BeginRulesetGrammar
		if gt == css.BeginRulesetGrammar {
			props := p.parseDeclarations(parser, sheet)

			// Create rules for each selector
			for _, selStr := range currentSelectors {
				sel := p.parseSelector(selStr, sheet)
				if sel.IsSimple() {
					// Clone properties for each rule
					propsCopy := make(map[string]CSSValue, len(props))
					maps.Copy(propsCopy, props)
					rule := CSSRule{
						Selector:   sel,
						Properties: propsCopy,
					}
					sheet.Items = append(sheet.Items, StylesheetItem{Rule: &rule})
				}
			}
			currentSelectors = nil
		}
	}
}

// extractImportURL extracts the URL from @import tokens.
// Handles: @import "url"; @import url("url"); @import url(url);
func extractImportURL(tokens []css.Token) string {
	for _, t := range tokens {
		switch t.TokenType {
		case css.StringToken:
			return unquote(string(t.Data))
		case css.URLToken:
			// url(something) — the token data is the full url(...) string
			s := string(t.Data)
			// Strip url( prefix and ) suffix
			s = strings.TrimPrefix(s, "url(")
			s = strings.TrimSuffix(s, ")")
			return unquote(strings.TrimSpace(s))
		}
	}
	return ""
}

// parseSelectors extracts selector strings from token data.
func (p *Parser) parseSelectors(data []byte, values []css.Token) []string {
	// Build full selector string from data and values
	var sb strings.Builder
	sb.Write(data)
	for _, v := range values {
		sb.Write(v.Data)
	}

	selectorStr := sb.String()

	// Split by comma for grouped selectors
	var selectors []string
	for s := range strings.SplitSeq(selectorStr, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			selectors = append(selectors, s)
		}
	}
	return selectors
}

// parseDeclarations parses property declarations until EndRulesetGrammar.
func (p *Parser) parseDeclarations(parser *css.Parser, _ *Stylesheet) map[string]CSSValue {
	props := make(map[string]CSSValue)

	for {
		gt, _, data := parser.Next()

		switch gt {
		case css.ErrorGrammar, css.EndRulesetGrammar:
			return props

		case css.DeclarationGrammar:
			propName := string(data)
			values := parser.Values()
			if len(values) > 0 {
				props[propName] = p.parsePropertyValue(values)
			}

		case css.CustomPropertyGrammar:
			// CSS custom properties (--var) - skip for now
			continue
		}
	}
}

// parsePropertyValue converts CSS tokens to a CSSValue.
func (p *Parser) parsePropertyValue(tokens []css.Token) CSSValue {
	if len(tokens) == 0 {
		return CSSValue{}
	}

	// Build raw value string
	var rawParts []string
	for _, t := range tokens {
		if t.TokenType != css.WhitespaceToken {
			rawParts = append(rawParts, string(t.Data))
		} else if len(rawParts) > 0 {
			// Add space between non-whitespace tokens
			rawParts = append(rawParts, " ")
		}
	}
	raw := strings.TrimSpace(strings.Join(rawParts, ""))

	val := CSSValue{Raw: raw}

	// Handle single token cases
	if len(tokens) == 1 || (len(tokens) == 2 && tokens[1].TokenType == css.WhitespaceToken) {
		t := tokens[0]
		switch t.TokenType {
		case css.DimensionToken:
			val.Value, val.Unit = parseDimension(string(t.Data))
		case css.PercentageToken:
			val.Value, _ = strconv.ParseFloat(strings.TrimSuffix(string(t.Data), "%"), 64)
			val.Unit = "%"
		case css.NumberToken:
			val.Value, _ = strconv.ParseFloat(string(t.Data), 64)
		case css.IdentToken:
			val.Keyword = strings.ToLower(string(t.Data))
		case css.StringToken:
			// Remove quotes
			s := string(t.Data)
			val.Keyword = unquote(s)
		case css.HashToken:
			// Color value
			val.Keyword = string(t.Data)
		}
		return val
	}

	// Handle function tokens (rgb(), url(), etc.)
	if tokens[0].TokenType == css.FunctionToken {
		val.Keyword = raw
		return val
	}

	// Multi-value properties - store as keyword with raw value
	val.Keyword = raw
	return val
}

// parseDimension extracts numeric value and unit from dimension token.
func parseDimension(s string) (float64, string) {
	// Find where number ends
	numEnd := 0
	for i, r := range s {
		if unicode.IsDigit(r) || r == '.' || r == '-' || r == '+' {
			numEnd = i + 1
		} else {
			break
		}
	}

	if numEnd == 0 {
		return 0, ""
	}

	num, _ := strconv.ParseFloat(s[:numEnd], 64)
	unit := strings.ToLower(s[numEnd:])
	return num, unit
}

// parseSelector parses a single selector string into a Selector.
func (p *Parser) parseSelector(selStr string, sheet *Stylesheet) Selector {
	selStr = strings.TrimSpace(selStr)
	sel := Selector{Raw: selStr}

	// Check for unsupported selector patterns first
	if strings.ContainsAny(selStr, "+~>") {
		// Sibling/child combinators
		sheet.Warnings = append(sheet.Warnings, "unsupported combinator selector: "+selStr)
		p.log.Debug("Skipping combinator selector", zap.String("selector", selStr))
		return sel
	}
	if strings.Contains(selStr, "[") {
		// Attribute selector
		sheet.Warnings = append(sheet.Warnings, "unsupported attribute selector: "+selStr)
		p.log.Debug("Skipping attribute selector", zap.String("selector", selStr))
		return sel
	}

	// Check for descendant selector (contains whitespace)
	if strings.ContainsAny(selStr, " \t\n") {
		return p.parseDescendantSelector(selStr, sheet)
	}

	// Parse simple selector
	return p.parseSimpleSelector(selStr, sheet)
}

// parseDescendantSelector parses a descendant selector like "p code" or ".section-title h2".
func (p *Parser) parseDescendantSelector(selStr string, sheet *Stylesheet) Selector {
	sel := Selector{Raw: selStr}

	// Split by whitespace
	parts := strings.Fields(selStr)
	if len(parts) < 2 {
		return sel
	}

	// Parse the rightmost part as the main selector
	mainPart := parts[len(parts)-1]
	mainSel := p.parseSimpleSelector(mainPart, sheet)
	if !mainSel.IsSimple() {
		// Failed to parse the main part
		return sel
	}

	// Copy main selector properties
	sel.Element = mainSel.Element
	sel.Class = mainSel.Class
	sel.Pseudo = mainSel.Pseudo

	// Parse ancestor parts (all parts except the last one)
	// For simplicity, we combine all ancestor parts into a single ancestor selector
	// e.g., ".section-title h2.section-title-header" -> ancestor is ".section-title"
	ancestorParts := parts[:len(parts)-1]
	if len(ancestorParts) == 1 {
		// Single ancestor
		ancestorSel := p.parseSimpleSelector(ancestorParts[0], sheet)
		if ancestorSel.IsSimple() {
			sel.Ancestor = &ancestorSel
		}
	} else {
		// Multiple ancestors - recursively parse as descendant selector
		ancestorStr := strings.Join(ancestorParts, " ")
		ancestorSel := p.parseDescendantSelector(ancestorStr, sheet)
		if ancestorSel.IsSimple() || ancestorSel.IsDescendant() {
			sel.Ancestor = &ancestorSel
		}
	}

	return sel
}

// parseSimpleSelector parses a simple selector (element, class, or element.class with optional pseudo).
func (p *Parser) parseSimpleSelector(selStr string, sheet *Stylesheet) Selector {
	selStr = strings.TrimSpace(selStr)
	sel := Selector{Raw: selStr}

	// Parse pseudo-element (::before, ::after)
	remaining := selStr
	if before, pseudo, found := strings.Cut(selStr, "::"); found {
		remaining = before
		switch strings.ToLower(pseudo) {
		case "before":
			sel.Pseudo = PseudoBefore
		case "after":
			sel.Pseudo = PseudoAfter
		default:
			sheet.Warnings = append(sheet.Warnings, "unsupported pseudo-element: "+selStr)
			p.log.Debug("Skipping unsupported pseudo-element", zap.String("selector", selStr))
			return sel
		}
	} else if before, pseudo, found := strings.Cut(remaining, ":"); found {
		// Single colon - could be pseudo-class or old-style pseudo-element
		switch strings.ToLower(pseudo) {
		case "before":
			sel.Pseudo = PseudoBefore
			remaining = before
		case "after":
			sel.Pseudo = PseudoAfter
			remaining = before
		default:
			// Pseudo-class (e.g., :hover, :first-child) - not supported
			sheet.Warnings = append(sheet.Warnings, "unsupported pseudo-class: "+selStr)
			p.log.Debug("Skipping pseudo-class selector", zap.String("selector", selStr))
			return sel
		}
	}

	// Parse element and class from remaining
	if remaining == "" {
		// Just a pseudo-element on universal selector - not meaningful
		return sel
	}

	// Split by dot for class
	if element, class, found := strings.Cut(remaining, "."); found {
		if element != "" {
			sel.Element = element
		}
		sel.Class = class
	} else {
		sel.Element = remaining
	}

	return sel
}

// skipAtRuleBlock skips tokens until the matching end of an @-rule block.
func (p *Parser) skipAtRuleBlock(parser *css.Parser) {
	depth := 1
	for depth > 0 {
		gt, _, _ := parser.Next()
		switch gt {
		case css.ErrorGrammar:
			return
		case css.BeginAtRuleGrammar, css.BeginRulesetGrammar:
			depth++
		case css.EndAtRuleGrammar, css.EndRulesetGrammar:
			depth--
		}
	}
}

// parseFontFace parses an @font-face block.
func (p *Parser) parseFontFace(parser *css.Parser) CSSFontFace {
	ff := CSSFontFace{}

	for {
		gt, _, data := parser.Next()

		switch gt {
		case css.ErrorGrammar, css.EndAtRuleGrammar:
			return ff

		case css.DeclarationGrammar:
			propName := string(data)
			values := parser.Values()
			if len(values) == 0 {
				continue
			}

			// Build value string
			var parts []string
			for _, v := range values {
				if v.TokenType != css.WhitespaceToken {
					parts = append(parts, string(v.Data))
				}
			}
			valStr := strings.Join(parts, " ")

			switch propName {
			case "font-family":
				ff.Family = unquote(valStr)
			case "src":
				ff.Src = valStr
			case "font-style":
				ff.Style = valStr
			case "font-weight":
				ff.Weight = valStr
			}
		}
	}
}

// parseMediaQueryFromTokens parses a media query from CSS tokens.
// Handles queries like "amzn-kf8", "amzn-kf8 and not amzn-et", etc.
func (p *Parser) parseMediaQueryFromTokens(tokens []css.Token) MediaQuery {
	mq := MediaQuery{}

	// Build raw string for logging
	var rawParts []string
	for _, t := range tokens {
		if t.TokenType != css.WhitespaceToken {
			rawParts = append(rawParts, string(t.Data))
		} else if len(rawParts) > 0 {
			rawParts = append(rawParts, " ")
		}
	}
	mq.Raw = strings.TrimSpace(strings.Join(rawParts, ""))

	// Parse tokens into media query components
	// Format: [not] type [and [not] feature]...
	var idents []string
	for _, t := range tokens {
		if t.TokenType == css.IdentToken {
			idents = append(idents, strings.ToLower(string(t.Data)))
		}
	}

	if len(idents) == 0 {
		return mq
	}

	i := 0
	// Check for leading "not"
	if idents[i] == "not" {
		mq.Negated = true
		i++
	}

	// Get main media type
	if i < len(idents) {
		mq.Type = idents[i]
		i++
	}

	// Parse "and [not] feature" pairs
	for i < len(idents) {
		if idents[i] == "and" {
			i++
			if i >= len(idents) {
				break
			}

			feature := MediaFeature{}
			if idents[i] == "not" {
				feature.Negated = true
				i++
				if i >= len(idents) {
					break
				}
			}
			feature.Name = idents[i]
			mq.Features = append(mq.Features, feature)
			i++
		} else {
			i++
		}
	}

	return mq
}

// parseMediaBlockRules parses rules inside an @media block and returns them.
// Unlike the old parseMediaBlock which appended directly to sheet.Rules,
// this returns the rules for the caller to wrap in a MediaBlock.
func (p *Parser) parseMediaBlockRules(parser *css.Parser, sheet *Stylesheet) []CSSRule {
	var rules []CSSRule
	var currentSelectors []string

	for {
		gt, _, data := parser.Next()

		switch gt {
		case css.ErrorGrammar, css.EndAtRuleGrammar:
			return rules

		case css.BeginRulesetGrammar:
			currentSelectors = p.parseSelectors(data, parser.Values())
			props := p.parseDeclarations(parser, sheet)

			// Create rules for each selector
			for _, selStr := range currentSelectors {
				sel := p.parseSelector(selStr, sheet)
				if sel.IsSimple() {
					propsCopy := make(map[string]CSSValue, len(props))
					maps.Copy(propsCopy, props)
					rule := CSSRule{
						Selector:   sel,
						Properties: propsCopy,
					}
					rules = append(rules, rule)
				}
			}
			currentSelectors = nil
		}
	}
}

// unquote removes surrounding quotes from a string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	if (s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
