package kfx

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestParser_ParseDefaultCSS(t *testing.T) {
	defaultCSS, err := os.ReadFile("../default.css")
	if err != nil {
		t.Fatalf("failed to read default.css: %v", err)
	}

	log := zap.NewNop()
	p := NewParser(log)

	sheet := p.Parse(defaultCSS)

	if len(sheet.Rules) == 0 {
		t.Fatal("expected rules to be parsed from default.css")
	}

	t.Logf("Parsed %d rules from default.css", len(sheet.Rules))
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
	p := NewParser(log)

	css := []byte(`p { text-indent: 1em; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
	if rule.Selector.Element != "p" {
		t.Errorf("expected element 'p', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "" {
		t.Errorf("expected no class, got '%s'", rule.Selector.Class)
	}
	if rule.Selector.StyleName() != "p" {
		t.Errorf("expected style name 'p', got '%s'", rule.Selector.StyleName())
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
	p := NewParser(log)

	css := []byte(`.epigraph { font-style: italic; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
	if rule.Selector.Element != "" {
		t.Errorf("expected no element, got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "epigraph" {
		t.Errorf("expected class 'epigraph', got '%s'", rule.Selector.Class)
	}
	if rule.Selector.StyleName() != "epigraph" {
		t.Errorf("expected style name 'epigraph', got '%s'", rule.Selector.StyleName())
	}

	val, _ := rule.GetProperty("font-style")
	if val.Keyword != "italic" {
		t.Errorf("expected keyword 'italic', got '%s'", val.Keyword)
	}
}

func TestParser_CombinedSelector(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`p.has-dropcap { text-indent: 0; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
	if rule.Selector.Element != "p" {
		t.Errorf("expected element 'p', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "has-dropcap" {
		t.Errorf("expected class 'has-dropcap', got '%s'", rule.Selector.Class)
	}
	// Class takes precedence in style name
	if rule.Selector.StyleName() != "has-dropcap" {
		t.Errorf("expected style name 'has-dropcap', got '%s'", rule.Selector.StyleName())
	}
}

func TestParser_GroupedSelectors(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`h2, h3, h4 { font-size: 120%; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 3 {
		t.Fatalf("expected 3 rules for grouped selector, got %d", len(sheet.Rules))
	}

	expected := []string{"h2", "h3", "h4"}
	for i, rule := range sheet.Rules {
		if rule.Selector.Element != expected[i] {
			t.Errorf("rule %d: expected element '%s', got '%s'", i, expected[i], rule.Selector.Element)
		}
	}
}

func TestParser_PseudoElementBefore(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`.quote::before { content: ">>"; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
	if rule.Selector.Class != "quote" {
		t.Errorf("expected class 'quote', got '%s'", rule.Selector.Class)
	}
	if rule.Selector.Pseudo != PseudoBefore {
		t.Errorf("expected PseudoBefore, got %v", rule.Selector.Pseudo)
	}
	if rule.Selector.StyleName() != "quote--before" {
		t.Errorf("expected style name 'quote--before', got '%s'", rule.Selector.StyleName())
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
	p := NewParser(log)

	css := []byte(`p.note::after { content: " *"; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
	if rule.Selector.Element != "p" {
		t.Errorf("expected element 'p', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Class != "note" {
		t.Errorf("expected class 'note', got '%s'", rule.Selector.Class)
	}
	if rule.Selector.Pseudo != PseudoAfter {
		t.Errorf("expected PseudoAfter, got %v", rule.Selector.Pseudo)
	}
	if rule.Selector.StyleName() != "note--after" {
		t.Errorf("expected style name 'note--after', got '%s'", rule.Selector.StyleName())
	}
}

func TestParser_MediaBlockKF8Included(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		p { margin: 0; }
		@media amzn-kf8 {
			p { margin: 1em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(css)

	// Should have 3 rules: p (margin: 0), p (margin: 1em from @media), .test
	// @media amzn-kf8 is now processed for KFX output
	if len(sheet.Rules) != 3 {
		t.Fatalf("expected 3 rules (@media amzn-kf8 included), got %d", len(sheet.Rules))
	}

	// First rule should be p with margin: 0
	if sheet.Rules[0].Selector.Element != "p" {
		t.Errorf("expected first rule to be 'p', got '%s'", sheet.Rules[0].Selector.Element)
	}
	val0, _ := sheet.Rules[0].GetProperty("margin")
	if val0.Raw != "0" {
		t.Errorf("expected first p margin: 0, got '%s'", val0.Raw)
	}

	// Second rule should be p with margin: 1em (from @media amzn-kf8)
	if sheet.Rules[1].Selector.Element != "p" {
		t.Errorf("expected second rule to be 'p', got '%s'", sheet.Rules[1].Selector.Element)
	}
	val1, _ := sheet.Rules[1].GetProperty("margin")
	if val1.Raw != "1em" {
		t.Errorf("expected second p margin: 1em, got '%s'", val1.Raw)
	}

	// Third rule should be .test
	if sheet.Rules[2].Selector.Class != "test" {
		t.Errorf("expected third rule to be '.test', got '%s'", sheet.Rules[2].Selector.Class)
	}
}

func TestParser_MediaBlockMobiSkipped(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		p { margin: 0; }
		@media amzn-mobi {
			p { margin: 2em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(css)

	// Should have 2 rules: p and .test. @media amzn-mobi should be skipped
	if len(sheet.Rules) != 2 {
		t.Fatalf("expected 2 rules (@media amzn-mobi skipped), got %d", len(sheet.Rules))
	}

	// First rule should be p with margin: 0
	if sheet.Rules[0].Selector.Element != "p" {
		t.Errorf("expected first rule to be 'p', got '%s'", sheet.Rules[0].Selector.Element)
	}

	// Second rule should be .test
	if sheet.Rules[1].Selector.Class != "test" {
		t.Errorf("expected second rule to be '.test', got '%s'", sheet.Rules[1].Selector.Class)
	}
}

func TestParser_MediaBlockKF8AndNotET(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		p { margin: 0; }
		@media amzn-kf8 and not amzn-et {
			p { margin: 3em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(css)

	// Should have 2 rules: p and .test
	// @media amzn-kf8 and not amzn-et should be skipped because:
	// amzn-kf8=true AND NOT amzn-et=true → true AND false → false
	if len(sheet.Rules) != 2 {
		t.Fatalf("expected 2 rules (@media amzn-kf8 and not amzn-et skipped), got %d", len(sheet.Rules))
	}

	// First rule should be p with margin: 0
	if sheet.Rules[0].Selector.Element != "p" {
		t.Errorf("expected first rule to be 'p', got '%s'", sheet.Rules[0].Selector.Element)
	}

	// Second rule should be .test
	if sheet.Rules[1].Selector.Class != "test" {
		t.Errorf("expected second rule to be '.test', got '%s'", sheet.Rules[1].Selector.Class)
	}
}

func TestParser_MediaBlockKF8AndET(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		p { margin: 0; }
		@media amzn-kf8 and amzn-et {
			p { margin: 4em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(css)

	// Should have 3 rules: p (margin: 0), p (margin: 4em from @media), .test
	// @media amzn-kf8 and amzn-et should be included because:
	// amzn-kf8=true AND amzn-et=true → true AND true → true
	if len(sheet.Rules) != 3 {
		t.Fatalf("expected 3 rules (@media amzn-kf8 and amzn-et included), got %d", len(sheet.Rules))
	}

	// First rule should be p with margin: 0
	if sheet.Rules[0].Selector.Element != "p" {
		t.Errorf("expected first rule to be 'p', got '%s'", sheet.Rules[0].Selector.Element)
	}
	val0, _ := sheet.Rules[0].GetProperty("margin")
	if val0.Raw != "0" {
		t.Errorf("expected first p margin: 0, got '%s'", val0.Raw)
	}

	// Second rule should be p with margin: 4em (from @media amzn-kf8 and amzn-et)
	if sheet.Rules[1].Selector.Element != "p" {
		t.Errorf("expected second rule to be 'p', got '%s'", sheet.Rules[1].Selector.Element)
	}
	val1, _ := sheet.Rules[1].GetProperty("margin")
	if val1.Raw != "4em" {
		t.Errorf("expected second p margin: 4em, got '%s'", val1.Raw)
	}

	// Third rule should be .test
	if sheet.Rules[2].Selector.Class != "test" {
		t.Errorf("expected third rule to be '.test', got '%s'", sheet.Rules[2].Selector.Class)
	}
}

func TestParser_MediaBlockNotMobi(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		p { margin: 0; }
		@media not amzn-mobi {
			p { margin: 5em; }
		}
		.test { color: red; }
	`)
	sheet := p.Parse(css)

	// Should have 3 rules: p (margin: 0), p (margin: 5em from @media), .test
	// @media not amzn-mobi should be included because:
	// NOT amzn-mobi=false → NOT false → true
	if len(sheet.Rules) != 3 {
		t.Fatalf("expected 3 rules (@media not amzn-mobi included), got %d", len(sheet.Rules))
	}

	// Second rule should be p with margin: 5em (from @media not amzn-mobi)
	if sheet.Rules[1].Selector.Element != "p" {
		t.Errorf("expected second rule to be 'p', got '%s'", sheet.Rules[1].Selector.Element)
	}
	val1, _ := sheet.Rules[1].GetProperty("margin")
	if val1.Raw != "5em" {
		t.Errorf("expected second p margin: 5em, got '%s'", val1.Raw)
	}
}

func TestParser_DescendantSelector(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`p code { font-family: monospace; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule for descendant selector, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
	if rule.Selector.Element != "code" {
		t.Errorf("expected element 'code', got '%s'", rule.Selector.Element)
	}
	if rule.Selector.Ancestor == nil {
		t.Fatal("expected ancestor selector")
	}
	if rule.Selector.Ancestor.Element != "p" {
		t.Errorf("expected ancestor element 'p', got '%s'", rule.Selector.Ancestor.Element)
	}
	if rule.Selector.StyleName() != "p--code" {
		t.Errorf("expected style name 'p--code', got '%s'", rule.Selector.StyleName())
	}
}

func TestParser_DescendantSelectorWithClass(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`.section-title h2.section-title-header { page-break-before: always; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	rule := sheet.Rules[0]
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
	// Class takes precedence in style name
	if rule.Selector.StyleName() != "section-title--section-title-header" {
		t.Errorf("expected style name 'section-title--section-title-header', got '%s'", rule.Selector.StyleName())
	}
}

func TestParser_FontFace(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		@font-face {
			font-family: "MyFont";
			src: url("fonts/myfont.woff2");
			font-weight: bold;
			font-style: italic;
		}
	`)
	sheet := p.Parse(css)

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
}

func TestParser_NumericValues(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

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
			if len(sheet.Rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
			}

			val, ok := sheet.Rules[0].GetProperty(tt.prop)
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

func TestParser_ShorthandMargin(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`p { margin: 1em 2em 3em 4em; }`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	val, ok := sheet.Rules[0].GetProperty("margin")
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
	p := NewParser(log)

	css := []byte(`
		/* This is a comment */
		p { 
			/* inline comment */
			text-indent: 1em; /* trailing comment */
		}
	`)
	sheet := p.Parse(css)

	if len(sheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sheet.Rules))
	}

	val, ok := sheet.Rules[0].GetProperty("text-indent")
	if !ok {
		t.Fatal("expected text-indent property")
	}
	if val.Value != 1 || val.Unit != "em" {
		t.Errorf("expected 1em, got %v%s", val.Value, val.Unit)
	}
}

func TestStylesheet_StyleNames(t *testing.T) {
	log := zap.NewNop()
	p := NewParser(log)

	css := []byte(`
		p { margin: 0; }
		.epigraph { font-style: italic; }
		h1 { font-size: 2em; }
		p.note { color: gray; }
		.quote::before { content: ">>"; }
	`)
	sheet := p.Parse(css)

	names := sheet.StyleNames()

	expected := map[string]bool{
		"p":             true,
		"epigraph":      true,
		"h1":            true,
		"note":          true,
		"quote--before": true,
	}

	if len(names) != len(expected) {
		t.Errorf("expected %d unique names, got %d: %v", len(expected), len(names), names)
	}

	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected style name: %s", name)
		}
	}
}

func TestMediaQuery_Evaluate(t *testing.T) {
	tests := []struct {
		name     string
		mq       MediaQuery
		kf8      bool
		et       bool
		expected bool
	}{
		{
			name:     "amzn-kf8 matches KFX",
			mq:       MediaQuery{Type: "amzn-kf8"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "amzn-mobi never matches KFX",
			mq:       MediaQuery{Type: "amzn-mobi"},
			kf8:      true,
			et:       true,
			expected: false,
		},
		{
			name:     "amzn-et matches KFX",
			mq:       MediaQuery{Type: "amzn-et"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name: "amzn-kf8 and not amzn-et does not match KFX",
			mq: MediaQuery{
				Type:     "amzn-kf8",
				Features: []MediaFeature{{Name: "amzn-et", Negated: true}},
			},
			kf8:      true,
			et:       true,
			expected: false,
		},
		{
			name: "amzn-kf8 and amzn-et matches KFX",
			mq: MediaQuery{
				Type:     "amzn-kf8",
				Features: []MediaFeature{{Name: "amzn-et", Negated: false}},
			},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "not amzn-mobi matches KFX",
			mq:       MediaQuery{Type: "amzn-mobi", Negated: true},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "screen matches (generic type)",
			mq:       MediaQuery{Type: "screen"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "all matches (generic type)",
			mq:       MediaQuery{Type: "all"},
			kf8:      true,
			et:       true,
			expected: true,
		},
		{
			name:     "unknown type does not match",
			mq:       MediaQuery{Type: "print"},
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

func TestMediaQuery_EvaluateForKFX(t *testing.T) {
	// Test the convenience method that uses kf8=true, et=true
	tests := []struct {
		name     string
		mq       MediaQuery
		expected bool
	}{
		{"amzn-kf8", MediaQuery{Type: "amzn-kf8"}, true},
		{"amzn-mobi", MediaQuery{Type: "amzn-mobi"}, false},
		{"amzn-et", MediaQuery{Type: "amzn-et"}, true},
		{
			"amzn-kf8 and not amzn-et",
			MediaQuery{Type: "amzn-kf8", Features: []MediaFeature{{Name: "amzn-et", Negated: true}}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mq.EvaluateForKFX(); got != tt.expected {
				t.Errorf("EvaluateForKFX() = %v, want %v", got, tt.expected)
			}
		})
	}
}
