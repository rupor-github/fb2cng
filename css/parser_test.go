package css_test

import (
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"

	"fbc/css"
)

// allRules collects all top-level rules from a stylesheet's Items.
// It does NOT flatten @media blocks — use this only for tests that
// care about plain top-level rules.
func allRules(sheet *css.Stylesheet) []css.Rule {
	var rules []css.Rule
	for _, item := range sheet.Items {
		if item.Rule != nil {
			rules = append(rules, *item.Rule)
		}
	}
	return rules
}

func TestParser_ParseDefaultCSS(t *testing.T) {
	defaultCSS, err := os.ReadFile("../convert/default.css")
	if err != nil {
		t.Fatalf("failed to read default.css: %v", err)
	}

	log := zap.NewNop()
	p := css.NewParser(log)

	sheet := p.Parse(defaultCSS)

	rules := allRules(sheet)
	if len(rules) == 0 {
		t.Fatal("expected rules to be parsed from default.css")
	}

	t.Logf("Parsed %d top-level rules from default.css", len(rules))
	t.Logf("Warnings: %d", len(sheet.Warnings))
	for _, w := range sheet.Warnings {
		t.Logf("  - %s", w)
	}

	// Check for some expected rules
	pRules := sheet.RulesBySelector("p")
	if len(pRules) == 0 {
		t.Error("expected 'p' selector rule")
	}

	h1Rules := sheet.RulesBySelector("h1")
	if len(h1Rules) == 0 {
		t.Error("expected 'h1' selector rule")
	}

	// Check for class selectors
	epigraphRules := sheet.RulesBySelector(".epigraph")
	if len(epigraphRules) == 0 {
		t.Error("expected '.epigraph' selector rule")
	}
}

func TestParser_ElementSelector(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { text-indent: 1em; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Element != "p" {
		t.Errorf("expected element 'p', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "" {
		t.Errorf("expected no class, got '%s'", rule.Selector.Class)
	}

	val, ok := rule.GetProperty("text-indent")
	if !ok {
		t.Fatal("expected text-indent property")
	}
	if val.Value != 1 || val.Unit != "em" {
		t.Errorf("expected 1em, got %v%s", val.Value, val.Unit)
	}
}

func TestParser_ClassSelector(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`.epigraph { font-style: italic; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Element != "" {
		t.Errorf("expected no element, got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "epigraph" {
		t.Errorf("expected class 'epigraph', got '%s'", rule.Selector.Class)
	}

	val, _ := rule.GetProperty("font-style")
	if val.Keyword != "italic" {
		t.Errorf("expected keyword 'italic', got '%s'", val.Keyword)
	}
}

func TestParser_CombinedSelector(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p.has-dropcap { text-indent: 0; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Element != "p" {
		t.Errorf("expected element 'p', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "has-dropcap" {
		t.Errorf("expected class 'has-dropcap', got '%s'", rule.Selector.Class)
	}
}

func TestParser_GroupedSelectors(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`h2, h3, h4 { font-size: 120%; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules for grouped selector, got %d", len(rules))
	}

	expected := []string{"h2", "h3", "h4"}
	for i, rule := range rules {
		if rule.Selector.Element != expected[i] {
			t.Errorf("rule %d: expected element '%s', got '%s'", i, expected[i], rule.Selector.Element)
		}
	}
}

func TestParser_PseudoElementBefore(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`.quote::before { content: ">>"; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Class != "quote" {
		t.Errorf("expected class 'quote', got '%s'", rule.Selector.Class)
	}
	if rule.Selector.Pseudo != css.PseudoBefore {
		t.Errorf("expected PseudoBefore, got %v", rule.Selector.Pseudo)
	}

	val, ok := rule.GetProperty("content")
	if !ok {
		t.Fatal("expected content property")
	}
	if val.Keyword != ">>" {
		t.Errorf("expected content '>>', got '%s'", val.Keyword)
	}
}

func TestParser_PseudoElementAfter(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p.note::after { content: " *"; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Element != "p" {
		t.Errorf("expected element 'p', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "note" {
		t.Errorf("expected class 'note', got '%s'", rule.Selector.Class)
	}
	if rule.Selector.Pseudo != css.PseudoAfter {
		t.Errorf("expected PseudoAfter, got %v", rule.Selector.Pseudo)
	}
}

func TestParser_MediaBlockPreserved(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { margin: 0; }
		@media amzn-kf8 {
			p { margin: 1em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(input)

	// Should have 3 items: rule(p), media-block(@media amzn-kf8), rule(.test)
	if len(sheet.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sheet.Items))
	}

	// First item: plain rule p { margin: 0 }
	if sheet.Items[0].Rule == nil {
		t.Fatal("expected first item to be a Rule")
	}
	if sheet.Items[0].Rule.Selector.Element != "p" {
		t.Errorf("expected first rule selector 'p', got '%s'", sheet.Items[0].Rule.Selector.Element)
	}
	val0, _ := sheet.Items[0].Rule.GetProperty("margin")
	if val0.Raw != "0" {
		t.Errorf("expected first p margin: 0, got '%s'", val0.Raw)
	}

	// Second item: @media amzn-kf8 block
	if sheet.Items[1].MediaBlock == nil {
		t.Fatal("expected second item to be a MediaBlock")
	}
	mb := sheet.Items[1].MediaBlock
	if mb.Query.Type != "amzn-kf8" {
		t.Errorf("expected media type 'amzn-kf8', got '%s'", mb.Query.Type)
	}
	if len(mb.Rules) != 1 {
		t.Fatalf("expected 1 rule inside @media block, got %d", len(mb.Rules))
	}
	if mb.Rules[0].Selector.Element != "p" {
		t.Errorf("expected media block rule selector 'p', got '%s'", mb.Rules[0].Selector.Element)
	}
	val1, _ := mb.Rules[0].GetProperty("margin")
	if val1.Raw != "1em" {
		t.Errorf("expected media block p margin: 1em, got '%s'", val1.Raw)
	}

	// Third item: plain rule .test { color: red }
	if sheet.Items[2].Rule == nil {
		t.Fatal("expected third item to be a Rule")
	}
	if sheet.Items[2].Rule.Selector.Class != "test" {
		t.Errorf("expected third rule selector '.test', got class '%s'", sheet.Items[2].Rule.Selector.Class)
	}
}

func TestParser_MediaBlockMobiPreserved(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { margin: 0; }
		@media amzn-mobi {
			p { margin: 2em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(input)

	// All 3 items preserved: rule, media-block, rule
	if len(sheet.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sheet.Items))
	}

	// First item: rule
	if sheet.Items[0].Rule == nil {
		t.Fatal("expected first item to be a Rule")
	}

	// Second item: @media amzn-mobi block (preserved, not skipped)
	if sheet.Items[1].MediaBlock == nil {
		t.Fatal("expected second item to be a MediaBlock")
	}
	mb := sheet.Items[1].MediaBlock
	if mb.Query.Type != "amzn-mobi" {
		t.Errorf("expected media type 'amzn-mobi', got '%s'", mb.Query.Type)
	}
	// The block should NOT be evaluated — just preserved
	if len(mb.Rules) != 1 {
		t.Fatalf("expected 1 rule inside @media amzn-mobi block, got %d", len(mb.Rules))
	}

	// Third item: rule
	if sheet.Items[2].Rule == nil {
		t.Fatal("expected third item to be a Rule")
	}
}

func TestParser_MediaBlockKF8AndNotET(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { margin: 0; }
		@media amzn-kf8 and not amzn-et {
			p { margin: 3em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(input)

	// 3 items: rule, media-block, rule
	if len(sheet.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sheet.Items))
	}

	// Verify the media block query
	if sheet.Items[1].MediaBlock == nil {
		t.Fatal("expected second item to be a MediaBlock")
	}
	mb := sheet.Items[1].MediaBlock
	if mb.Query.Type != "amzn-kf8" {
		t.Errorf("expected media type 'amzn-kf8', got '%s'", mb.Query.Type)
	}
	if len(mb.Query.Features) != 1 || mb.Query.Features[0].Name != "amzn-et" || !mb.Query.Features[0].Negated {
		t.Errorf("expected feature 'not amzn-et', got %+v", mb.Query.Features)
	}

	// Evaluation is a consumer concern — verify the query evaluates as expected
	if mb.Query.Evaluate(true, true) {
		t.Error("expected amzn-kf8 and not amzn-et to NOT match kf8=true, et=true")
	}
	if !mb.Query.Evaluate(true, false) {
		t.Error("expected amzn-kf8 and not amzn-et to match kf8=true, et=false")
	}
}

func TestParser_MediaBlockKF8AndET(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { margin: 0; }
		@media amzn-kf8 and amzn-et {
			p { margin: 4em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(input)

	// 3 items: rule, media-block, rule
	if len(sheet.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sheet.Items))
	}

	mb := sheet.Items[1].MediaBlock
	if mb == nil {
		t.Fatal("expected second item to be a MediaBlock")
	}
	if !mb.Query.Evaluate(true, true) {
		t.Error("expected amzn-kf8 and amzn-et to match kf8=true, et=true")
	}

	// Verify the nested rule
	if len(mb.Rules) != 1 {
		t.Fatalf("expected 1 rule in media block, got %d", len(mb.Rules))
	}
	val, _ := mb.Rules[0].GetProperty("margin")
	if val.Raw != "4em" {
		t.Errorf("expected margin: 4em, got '%s'", val.Raw)
	}
}

func TestParser_MediaBlockNotMobi(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { margin: 0; }
		@media not amzn-mobi {
			p { margin: 5em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(input)

	if len(sheet.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sheet.Items))
	}

	mb := sheet.Items[1].MediaBlock
	if mb == nil {
		t.Fatal("expected second item to be a MediaBlock")
	}
	if !mb.Query.Negated {
		t.Error("expected negated media query")
	}
	if mb.Query.Type != "amzn-mobi" {
		t.Errorf("expected media type 'amzn-mobi', got '%s'", mb.Query.Type)
	}
	// not amzn-mobi should match kf8 context (mobi is always false, negated = true)
	if !mb.Query.Evaluate(true, true) {
		t.Error("expected 'not amzn-mobi' to match kf8=true, et=true")
	}

	if len(mb.Rules) != 1 {
		t.Fatalf("expected 1 rule in media block, got %d", len(mb.Rules))
	}
	val, _ := mb.Rules[0].GetProperty("margin")
	if val.Raw != "5em" {
		t.Errorf("expected margin: 5em, got '%s'", val.Raw)
	}
}

func TestParser_DescendantSelector(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p code { font-family: monospace; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule for descendant selector, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Element != "code" {
		t.Errorf("expected element 'code', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Ancestor == nil {
		t.Fatal("expected ancestor selector")
	}
	if rule.Selector.Ancestor.Element != "p" {
		t.Errorf("expected ancestor element 'p', got '%s'", rule.Selector.Ancestor.Element)
	}
	// DescendantBaseName returns the base name of the rightmost part
	if rule.Selector.DescendantBaseName() != "code" {
		t.Errorf("expected DescendantBaseName 'code', got '%s'", rule.Selector.DescendantBaseName())
	}
}

func TestParser_DescendantSelectorWithClass(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`.section-title h2.section-title-header { page-break-before: always; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.Selector.Element != "h2" {
		t.Errorf("expected element 'h2', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "section-title-header" {
		t.Errorf("expected class 'section-title-header', got '%s'", rule.Selector.Class)
	}
	if rule.Selector.Ancestor == nil {
		t.Fatal("expected ancestor selector")
	}
	if rule.Selector.Ancestor.Class != "section-title" {
		t.Errorf("expected ancestor class 'section-title', got '%s'", rule.Selector.Ancestor.Class)
	}
	// DescendantBaseName for class selector returns the class
	if rule.Selector.DescendantBaseName() != "section-title-header" {
		t.Errorf("expected DescendantBaseName 'section-title-header', got '%s'", rule.Selector.DescendantBaseName())
	}
}

func TestParser_FontFace(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		@font-face {
			font-family: "MyFont";
			src: url("fonts/myfont.woff2");
			font-weight: bold;
			font-style: italic;
		}
	`)
	sheet := p.Parse(input)

	if len(sheet.FontFaces) != 1 {
		t.Fatalf("expected 1 font-face, got %d", len(sheet.FontFaces))
	}

	ff := sheet.FontFaces[0]
	if ff.Family != "MyFont" {
		t.Errorf("expected family 'MyFont', got '%s'", ff.Family)
	}
	if ff.Weight != "bold" {
		t.Errorf("expected weight 'bold', got '%s'", ff.Weight)
	}
	if ff.Style != "italic" {
		t.Errorf("expected style 'italic', got '%s'", ff.Style)
	}

	// Also verify the font-face appears in Items
	var fontFaceItems int
	for _, item := range sheet.Items {
		if item.FontFace != nil {
			fontFaceItems++
		}
	}
	if fontFaceItems != 1 {
		t.Errorf("expected 1 FontFace item, got %d", fontFaceItems)
	}
}

func TestParser_Import(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		@import "other.css";
		@import url("another.css");
		p { margin: 0; }
	`)
	sheet := p.Parse(input)

	// Should have 3 items: 2 imports + 1 rule
	if len(sheet.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sheet.Items))
	}

	// Convenience Imports slice
	if len(sheet.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(sheet.Imports))
	}
	if sheet.Imports[0] != "other.css" {
		t.Errorf("expected first import 'other.css', got '%s'", sheet.Imports[0])
	}
	if sheet.Imports[1] != "another.css" {
		t.Errorf("expected second import 'another.css', got '%s'", sheet.Imports[1])
	}

	// Verify items
	if sheet.Items[0].Import == nil {
		t.Fatal("expected first item to be an Import")
	}
	if *sheet.Items[0].Import != "other.css" {
		t.Errorf("expected import 'other.css', got '%s'", *sheet.Items[0].Import)
	}
	if sheet.Items[1].Import == nil {
		t.Fatal("expected second item to be an Import")
	}
	if *sheet.Items[1].Import != "another.css" {
		t.Errorf("expected import 'another.css', got '%s'", *sheet.Items[1].Import)
	}
	if sheet.Items[2].Rule == nil {
		t.Fatal("expected third item to be a Rule")
	}
}

func TestParser_NumericValues(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	tests := []struct {
		css     string
		prop    string
		value   float64
		unit    string
		keyword string
	}{
		{`p { font-size: 1.2em; }`, "font-size", 1.2, "em", ""},
		{`p { font-size: 100%; }`, "font-size", 100, "%", ""},
		{`p { font-size: 12px; }`, "font-size", 12, "px", ""},
		{`p { font-size: 12pt; }`, "font-size", 12, "pt", ""},
		{`p { line-height: 1.5; }`, "line-height", 1.5, "", ""},
		{`p { margin-top: -0.5em; }`, "margin-top", -0.5, "em", ""},
		{`p { text-align: center; }`, "text-align", 0, "", "center"},
		{`p { font-weight: bold; }`, "font-weight", 0, "", "bold"},
	}

	for _, tt := range tests {
		t.Run(tt.css, func(t *testing.T) {
			sheet := p.Parse([]byte(tt.css))
			rules := allRules(sheet)
			if len(rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(rules))
			}

			val, ok := rules[0].GetProperty(tt.prop)
			if !ok {
				t.Fatalf("expected property %s", tt.prop)
			}

			if tt.unit != "" || tt.value != 0 {
				if val.Value != tt.value {
					t.Errorf("expected value %v, got %v", tt.value, val.Value)
				}
				if val.Unit != tt.unit {
					t.Errorf("expected unit '%s', got '%s'", tt.unit, val.Unit)
				}
			}
			if tt.keyword != "" {
				if val.Keyword != tt.keyword {
					t.Errorf("expected keyword '%s', got '%s'", tt.keyword, val.Keyword)
				}
			}
		})
	}
}

func TestParser_DimensionEdgeCases(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	tests := []struct {
		name      string
		css       string
		prop      string
		wantValue float64
		wantUnit  string
		// wantKeyword, if non-empty, means parseDimension should have
		// failed and the value should have fallen back to keyword=raw.
		wantKeyword string
	}{
		{
			name:      "fractional-only .5em",
			css:       `p { margin-top: .5em; }`,
			prop:      "margin-top",
			wantValue: 0.5,
			wantUnit:  "em",
		},
		{
			name:      "positive sign +1px",
			css:       `p { margin-top: +1px; }`,
			prop:      "margin-top",
			wantValue: 1,
			wantUnit:  "px",
		},
		{
			name:      "negative value -3px",
			css:       `p { margin-top: -3px; }`,
			prop:      "margin-top",
			wantValue: -3,
			wantUnit:  "px",
		},
		{
			name:      "zero with unit 0px",
			css:       `p { margin-top: 0px; }`,
			prop:      "margin-top",
			wantValue: 0,
			wantUnit:  "px",
		},
		{
			name:      "negative fractional -.25rem",
			css:       `p { margin-top: -.25rem; }`,
			prop:      "margin-top",
			wantValue: -0.25,
			wantUnit:  "rem",
		},
		{
			name:      "large value 100vw",
			css:       `p { width: 100vw; }`,
			prop:      "width",
			wantValue: 100,
			wantUnit:  "vw",
		},
		{
			name:      "unit is lowercased 12PX -> px",
			css:       `p { margin-top: 12PX; }`,
			prop:      "margin-top",
			wantValue: 12,
			wantUnit:  "px",
		},
		{
			name:      "positive fractional +.75em",
			css:       `p { margin-top: +.75em; }`,
			prop:      "margin-top",
			wantValue: 0.75,
			wantUnit:  "em",
		},
		// Note: "5.em" is not valid CSS — the tokenizer does not produce a
		// DimensionToken for it, so parseDimension is never called. That
		// edge case is handled at the tokenizer level, not by us.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheet := p.Parse([]byte(tt.css))
			rules := allRules(sheet)
			if len(rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(rules))
			}

			val, ok := rules[0].GetProperty(tt.prop)
			if !ok {
				t.Fatalf("expected property %s", tt.prop)
			}

			if tt.wantKeyword != "" {
				if val.Keyword != tt.wantKeyword {
					t.Errorf("expected keyword %q, got keyword=%q value=%v unit=%q",
						tt.wantKeyword, val.Keyword, val.Value, val.Unit)
				}
				return
			}

			if val.Value != tt.wantValue {
				t.Errorf("expected value %v, got %v", tt.wantValue, val.Value)
			}
			if val.Unit != tt.wantUnit {
				t.Errorf("expected unit %q, got %q", tt.wantUnit, val.Unit)
			}
		})
	}
}

func TestParser_ShorthandMargin(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { margin: 1em 2em 3em 4em; }`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	val, ok := rules[0].GetProperty("margin")
	if !ok {
		t.Fatal("expected margin property")
	}

	// Shorthand should be stored as raw value
	if val.Raw != "1em 2em 3em 4em" {
		t.Errorf("expected raw '1em 2em 3em 4em', got '%s'", val.Raw)
	}
}

func TestParser_Comments(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		/* This is a comment */
		p { 
			/* inline comment */
			text-indent: 1em; /* trailing comment */
		}
	`)
	sheet := p.Parse(input)

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	val, ok := rules[0].GetProperty("text-indent")
	if !ok {
		t.Fatal("expected text-indent property")
	}
	if val.Value != 1 || val.Unit != "em" {
		t.Errorf("expected 1em, got %v%s", val.Value, val.Unit)
	}
}

func TestMediaQuery_Evaluate(t *testing.T) {
	tests := []struct {
		name     string
		mq       css.MediaQuery
		kf8      bool
		et       bool
		expected bool
	}{
		{
			name:     "amzn-kf8 matches KFX",
			mq:       css.MediaQuery{Type: "amzn-kf8"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "amzn-mobi never matches KFX",
			mq:       css.MediaQuery{Type: "amzn-mobi"},
			kf8:      true,
			et:       true,
			expected: false,
		},
		{
			name:     "amzn-et matches KFX",
			mq:       css.MediaQuery{Type: "amzn-et"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name: "amzn-kf8 and not amzn-et does not match KFX",
			mq: css.MediaQuery{
				Type:     "amzn-kf8",
				Features: []css.MediaFeature{{Name: "amzn-et", Negated: true}},
			},
			kf8:      true,
			et:       true,
			expected: false,
		},
		{
			name: "amzn-kf8 and amzn-et matches KFX",
			mq: css.MediaQuery{
				Type:     "amzn-kf8",
				Features: []css.MediaFeature{{Name: "amzn-et", Negated: false}},
			},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "not amzn-mobi matches KFX",
			mq:       css.MediaQuery{Type: "amzn-mobi", Negated: true},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "screen matches (generic type)",
			mq:       css.MediaQuery{Type: "screen"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "all matches (generic type)",
			mq:       css.MediaQuery{Type: "all"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "unknown type does not match",
			mq:       css.MediaQuery{Type: "print"},
			kf8:      true,
			et:       true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mq.Evaluate(tt.kf8, tt.et)
			if got != tt.expected {
				t.Errorf("MediaQuery.Evaluate(%v, %v) = %v, want %v", tt.kf8, tt.et, got, tt.expected)
			}
		})
	}
}

func TestMediaQuery_EvaluateKFXContext(t *testing.T) {
	// Test using Evaluate(true, true) directly — the KFX context
	tests := []struct {
		name     string
		mq       css.MediaQuery
		expected bool
	}{
		{"amzn-kf8", css.MediaQuery{Type: "amzn-kf8"}, true},
		{"amzn-mobi", css.MediaQuery{Type: "amzn-mobi"}, false},
		{"amzn-et", css.MediaQuery{Type: "amzn-et"}, true},
		{
			"amzn-kf8 and not amzn-et",
			css.MediaQuery{Type: "amzn-kf8", Features: []css.MediaFeature{{Name: "amzn-et", Negated: true}}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mq.Evaluate(true, true); got != tt.expected {
				t.Errorf("Evaluate(true, true) = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParser_SourceOrderPreserved(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		@import "reset.css";
		p { margin: 0; }
		@font-face { font-family: "MyFont"; src: url("f.woff"); }
		@media amzn-kf8 { h1 { color: red; } }
		.footer { font-size: small; }
	`)
	sheet := p.Parse(input)

	// 5 items in source order: import, rule, font-face, media-block, rule
	if len(sheet.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(sheet.Items))
	}

	if sheet.Items[0].Import == nil {
		t.Error("expected item 0 to be Import")
	}
	if sheet.Items[1].Rule == nil {
		t.Error("expected item 1 to be Rule")
	}
	if sheet.Items[2].FontFace == nil {
		t.Error("expected item 2 to be FontFace")
	}
	if sheet.Items[3].MediaBlock == nil {
		t.Error("expected item 3 to be MediaBlock")
	}
	if sheet.Items[4].Rule == nil {
		t.Error("expected item 4 to be Rule")
	}
}

func TestValue_IsNumeric(t *testing.T) {
	tests := []struct {
		val  css.Value
		want bool
	}{
		{css.Value{Raw: "1em", Value: 1, Unit: "em"}, true},
		{css.Value{Raw: "0", Value: 0}, true},
		{css.Value{Raw: "100%", Value: 100, Unit: "%"}, true},
		{css.Value{Raw: "-0.5em", Value: -0.5, Unit: "em"}, true},
		{css.Value{Raw: "bold", Keyword: "bold"}, false},
		{css.Value{Raw: "italic", Keyword: "italic"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.val.Raw, func(t *testing.T) {
			if got := tt.val.IsNumeric(); got != tt.want {
				t.Errorf("Value{Raw: %q}.IsNumeric() = %v, want %v", tt.val.Raw, got, tt.want)
			}
		})
	}
}

func TestValue_IsKeyword(t *testing.T) {
	tests := []struct {
		val  css.Value
		want bool
	}{
		{css.Value{Keyword: "bold"}, true},
		{css.Value{Keyword: "italic"}, true},
		{css.Value{Value: 1, Unit: "em"}, false},
		{css.Value{Raw: "0"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.val.Keyword+tt.val.Raw, func(t *testing.T) {
			if got := tt.val.IsKeyword(); got != tt.want {
				t.Errorf("Value.IsKeyword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelector_DescendantBaseName(t *testing.T) {
	tests := []struct {
		name string
		sel  css.Selector
		want string
	}{
		{"class only", css.Selector{Class: "foo"}, "foo"},
		{"element only", css.Selector{Element: "p"}, "p"},
		{"both - class wins", css.Selector{Element: "p", Class: "foo"}, "foo"},
		{"raw fallback", css.Selector{Raw: "something"}, "something"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sel.DescendantBaseName(); got != tt.want {
				t.Errorf("DescendantBaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRulesBySelector(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { margin: 0; }
		p { text-indent: 1em; }
		h1 { font-size: 2em; }
	`)
	sheet := p.Parse(input)

	pRules := sheet.RulesBySelector("p")
	if len(pRules) != 2 {
		t.Fatalf("expected 2 rules for 'p', got %d", len(pRules))
	}

	h1Rules := sheet.RulesBySelector("h1")
	if len(h1Rules) != 1 {
		t.Fatalf("expected 1 rule for 'h1', got %d", len(h1Rules))
	}

	noRules := sheet.RulesBySelector("h2")
	if len(noRules) != 0 {
		t.Fatalf("expected 0 rules for 'h2', got %d", len(noRules))
	}
}

func TestStylesheet_String_SimpleRule(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { text-indent: 1em; margin: 0; }`)
	sheet := p.Parse(input)

	output := sheet.String()

	// Properties are sorted alphabetically
	if !strings.Contains(output, "p {") {
		t.Errorf("expected selector 'p' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "margin: 0;") {
		t.Errorf("expected 'margin: 0;' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "text-indent: 1em;") {
		t.Errorf("expected 'text-indent: 1em;' in output, got:\n%s", output)
	}

	// margin should come before text-indent (alphabetical)
	marginIdx := strings.Index(output, "margin")
	textIndentIdx := strings.Index(output, "text-indent")
	if marginIdx > textIndentIdx {
		t.Errorf("expected properties in alphabetical order, margin before text-indent:\n%s", output)
	}
}

func TestStylesheet_String_FontFace(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`@font-face {
		font-family: "MyFont";
		src: url("fonts/myfont.woff2");
		font-weight: bold;
		font-style: italic;
	}`)
	sheet := p.Parse(input)

	output := sheet.String()

	if !strings.Contains(output, "@font-face {") {
		t.Errorf("expected '@font-face {' in output, got:\n%s", output)
	}
	if !strings.Contains(output, `font-family: "MyFont";`) {
		t.Errorf("expected font-family in output, got:\n%s", output)
	}
	if !strings.Contains(output, "font-weight: bold;") {
		t.Errorf("expected font-weight in output, got:\n%s", output)
	}
	if !strings.Contains(output, "font-style: italic;") {
		t.Errorf("expected font-style in output, got:\n%s", output)
	}
}

func TestStylesheet_String_Import(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`@import "other.css";
p { margin: 0; }`)
	sheet := p.Parse(input)

	output := sheet.String()

	if !strings.Contains(output, `@import url("other.css");`) {
		t.Errorf("expected '@import url(\"other.css\");' in output, got:\n%s", output)
	}
}

func TestStylesheet_String_MediaBlock(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		@media amzn-kf8 {
			p { margin: 1em; }
		}
	`)
	sheet := p.Parse(input)

	output := sheet.String()

	if !strings.Contains(output, "@media amzn-kf8 {") {
		t.Errorf("expected '@media amzn-kf8 {' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "margin: 1em;") {
		t.Errorf("expected 'margin: 1em;' in output, got:\n%s", output)
	}
}

func TestStylesheet_String_SourceOrder(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		@import "reset.css";
		p { margin: 0; }
		@font-face { font-family: "MyFont"; src: url("f.woff"); }
		@media amzn-kf8 { h1 { color: red; } }
		.footer { font-size: small; }
	`)
	sheet := p.Parse(input)

	output := sheet.String()

	// Verify source order is preserved
	importIdx := strings.Index(output, "@import")
	pIdx := strings.Index(output, "p {")
	fontFaceIdx := strings.Index(output, "@font-face")
	mediaIdx := strings.Index(output, "@media")
	footerIdx := strings.Index(output, ".footer")

	if importIdx >= pIdx || pIdx >= fontFaceIdx || fontFaceIdx >= mediaIdx || mediaIdx >= footerIdx {
		t.Errorf("expected items in source order, got:\n%s", output)
	}
}

func TestStylesheet_String_RoundTrip(t *testing.T) {
	// Parse → String → Parse again → compare rule count
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`
		p { text-indent: 1em; margin: 0; }
		.bold { font-weight: bold; }
		@media amzn-kf8 { h1 { color: red; } }
	`)
	sheet1 := p.Parse(input)
	output1 := sheet1.String()

	sheet2 := p.Parse([]byte(output1))

	rules1 := allRules(sheet1)
	rules2 := allRules(sheet2)
	if len(rules1) != len(rules2) {
		t.Errorf("round-trip: got %d rules, want %d", len(rules2), len(rules1))
	}

	if len(sheet1.Items) != len(sheet2.Items) {
		t.Errorf("round-trip: got %d items, want %d", len(sheet2.Items), len(sheet1.Items))
	}
}

func TestStylesheet_WriteTo(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { margin: 0; }`)
	sheet := p.Parse(input)

	var buf strings.Builder
	n, err := sheet.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	if n == 0 {
		t.Error("WriteTo returned 0 bytes")
	}
	if int64(buf.Len()) != n {
		t.Errorf("WriteTo returned %d but wrote %d bytes", n, buf.Len())
	}
	if !strings.Contains(buf.String(), "margin: 0;") {
		t.Errorf("expected 'margin: 0;' in output, got: %s", buf.String())
	}
}

func TestStylesheet_RewriteURLs_Import(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`@import "old.css";`)
	sheet := p.Parse(input)

	sheet.RewriteURLs(func(url string) string {
		if url == "old.css" {
			return "new.css"
		}
		return url
	})

	if len(sheet.Imports) != 1 || sheet.Imports[0] != "new.css" {
		t.Errorf("expected import rewritten to 'new.css', got %v", sheet.Imports)
	}

	output := sheet.String()
	if !strings.Contains(output, `@import url("new.css");`) {
		t.Errorf("expected rewritten import in output, got:\n%s", output)
	}
}

func TestStylesheet_RewriteURLs_FontFace(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`@font-face {
		font-family: "MyFont";
		src: url("fonts/old.woff2");
	}`)
	sheet := p.Parse(input)

	sheet.RewriteURLs(func(url string) string {
		if url == "fonts/old.woff2" {
			return "fonts/new.woff2"
		}
		return url
	})

	if len(sheet.FontFaces) != 1 {
		t.Fatalf("expected 1 font-face, got %d", len(sheet.FontFaces))
	}

	if !strings.Contains(sheet.FontFaces[0].Src, "fonts/new.woff2") {
		t.Errorf("expected rewritten font src, got: %s", sheet.FontFaces[0].Src)
	}

	output := sheet.String()
	if !strings.Contains(output, "fonts/new.woff2") {
		t.Errorf("expected rewritten URL in output, got:\n%s", output)
	}
}

func TestStylesheet_RewriteURLs_PropertyValue(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { background: url("images/bg.png"); }`)
	sheet := p.Parse(input)

	sheet.RewriteURLs(func(url string) string {
		if url == "images/bg.png" {
			return "img/background.png"
		}
		return url
	})

	rules := allRules(sheet)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	val, ok := rules[0].GetProperty("background")
	if !ok {
		t.Fatal("expected background property")
	}
	if !strings.Contains(val.Raw, "img/background.png") {
		t.Errorf("expected rewritten URL in property, got: %s", val.Raw)
	}
}

func TestStylesheet_RewriteURLs_MediaBlockProperty(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`@media amzn-kf8 { p { background: url("old.png"); } }`)
	sheet := p.Parse(input)

	sheet.RewriteURLs(func(url string) string {
		if url == "old.png" {
			return "new.png"
		}
		return url
	})

	if len(sheet.Items) != 1 || sheet.Items[0].MediaBlock == nil {
		t.Fatal("expected 1 media block item")
	}

	mb := sheet.Items[0].MediaBlock
	if len(mb.Rules) != 1 {
		t.Fatalf("expected 1 rule in media block, got %d", len(mb.Rules))
	}

	val, ok := mb.Rules[0].GetProperty("background")
	if !ok {
		t.Fatal("expected background property")
	}
	if !strings.Contains(val.Raw, "new.png") {
		t.Errorf("expected rewritten URL in media block property, got: %s", val.Raw)
	}
}

func TestStylesheet_RewriteURLs_FragmentRef(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`@font-face {
		font-family: "Test";
		src: url('#font1');
	}`)
	sheet := p.Parse(input)

	sheet.RewriteURLs(func(url string) string {
		if url == "#font1" {
			return "fonts/font1.woff2"
		}
		return url
	})

	output := sheet.String()
	if !strings.Contains(output, "fonts/font1.woff2") {
		t.Errorf("expected rewritten fragment URL in output, got:\n%s", output)
	}
}

func TestStylesheet_RewriteURLs_NoChange(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { color: red; font-size: 12pt; }`)
	sheet := p.Parse(input)

	before := sheet.String()

	sheet.RewriteURLs(func(url string) string {
		return "should-not-appear"
	})

	after := sheet.String()
	if before != after {
		t.Errorf("expected no change for CSS without URLs, before:\n%s\nafter:\n%s", before, after)
	}
}

// Tests for CSS double-quote escaping in WriteTo output.

func TestStylesheet_String_ImportEscapesQuotes(t *testing.T) {
	// Construct a stylesheet with an import URL containing double quotes.
	importURL := `foo"};body{background:red}/*`
	sheet := &css.Stylesheet{
		Items: []css.StylesheetItem{
			{Import: &importURL},
		},
		Imports: []string{importURL},
	}

	out := sheet.String()
	// The output must not contain an unescaped double quote inside url("...").
	// The escaped version should use \" inside the quotes.
	if strings.Contains(out, `url("foo"`) {
		t.Errorf("import URL with embedded quote was not escaped:\n%s", out)
	}
	if !strings.Contains(out, `\"`) {
		t.Errorf("expected escaped quote in output:\n%s", out)
	}
}

func TestStylesheet_String_FontFaceEscapesQuotes(t *testing.T) {
	// Construct a stylesheet with a font-family containing double quotes.
	sheet := &css.Stylesheet{
		Items: []css.StylesheetItem{
			{FontFace: &css.FontFace{
				Family: `My"Font`,
				Src:    `url("myfont.woff2")`,
			}},
		},
	}

	out := sheet.String()
	// The output must escape the embedded double quote in font-family.
	if strings.Contains(out, `"My"Font"`) {
		t.Errorf("font-family with embedded quote was not escaped:\n%s", out)
	}
	if !strings.Contains(out, `My\"Font`) {
		t.Errorf("expected escaped quote in font-family output:\n%s", out)
	}
}

func TestStylesheet_String_FontFaceEscapesBackslash(t *testing.T) {
	sheet := &css.Stylesheet{
		Items: []css.StylesheetItem{
			{FontFace: &css.FontFace{
				Family: `My\Font`,
				Src:    `url("myfont.woff2")`,
			}},
		},
	}

	out := sheet.String()
	if !strings.Contains(out, `My\\Font`) {
		t.Errorf("expected escaped backslash in font-family output:\n%s", out)
	}
}

func TestStylesheet_RewriteURLs_EscapesQuotesInRewrittenURL(t *testing.T) {
	log := zap.NewNop()
	p := css.NewParser(log)

	input := []byte(`p { background: url("original.png"); }`)
	sheet := p.Parse(input)

	// Rewrite to a URL that contains a double quote.
	sheet.RewriteURLs(func(url string) string {
		return `injected"}.evil{color:red}/*`
	})

	out := sheet.String()
	if strings.Contains(out, `url("injected"`) {
		t.Errorf("rewritten URL with embedded quote was not escaped:\n%s", out)
	}
	if !strings.Contains(out, `\"`) {
		t.Errorf("expected escaped quote in rewritten URL output:\n%s", out)
	}
}
