package kfx

import "testing"

func TestNormalizeCSSPropertiesDropsZeroValues(t *testing.T) {
	// Test properties that should be dropped when zero (per Amazon's YJHtmlMapper)
	props := map[string]CSSValue{
		"margin-left":  {Raw: "0px", Value: 0, Unit: "px"},
		"padding-top":  {Raw: "0", Value: 0},
		"font-size":    {Raw: "0em", Value: 0, Unit: "em"},
		"margin-right": {Raw: "1em", Value: 1, Unit: "em"}, // non-zero, should be kept
		"width":        {Raw: "0px", Value: 0, Unit: "px"}, // not in zeroValueProps, should be kept
	}

	norm := normalizeCSSProperties(props, "", nil, "test")

	if _, ok := norm["margin-left"]; ok {
		t.Fatalf("expected margin-left to be dropped")
	}
	if _, ok := norm["padding-top"]; ok {
		t.Fatalf("expected padding-top to be dropped")
	}
	if _, ok := norm["font-size"]; ok {
		t.Fatalf("expected font-size to be dropped")
	}
	if v, ok := norm["margin-right"]; !ok || v.Raw != "1em" {
		t.Fatalf("expected margin-right preserved, got %v", norm["margin-right"])
	}
	if _, ok := norm["width"]; !ok {
		t.Fatalf("expected width to be kept (not in zeroValueProps)")
	}
}

func TestNormalizeCSSPropertiesKeepsNonZeroValues(t *testing.T) {
	props := map[string]CSSValue{
		"margin-left":  {Raw: "10%", Value: 10, Unit: "%"},
		"padding-top":  {Raw: "0.5em", Value: 0.5, Unit: "em"},
		"margin-right": {Raw: "1rem", Value: 1, Unit: "rem"},
	}

	norm := normalizeCSSProperties(props, "", nil, "test")

	if len(norm) != len(props) {
		t.Fatalf("expected no properties dropped, got %d", len(norm))
	}
	if norm["margin-left"].Raw != "10%" || norm["padding-top"].Raw != "0.5em" {
		t.Fatalf("unexpected normalization result: %v", norm)
	}
}

func TestNormalizeCSSPropertiesAllZeroValueProps(t *testing.T) {
	// Test all properties in zeroValueProps
	zeroProps := []string{
		"font-size",
		"padding-right", "padding-left", "padding-top", "padding-bottom",
		"margin-right", "margin-left", "margin-top", "margin-bottom",
	}

	for _, prop := range zeroProps {
		props := map[string]CSSValue{
			prop: {Raw: "0", Value: 0},
		}
		norm := normalizeCSSProperties(props, "", nil, "test")
		if _, ok := norm[prop]; ok {
			t.Errorf("expected %s with zero value to be dropped", prop)
		}
	}
}

func TestNormalizeTextDecorationNone(t *testing.T) {
	// KP3 (com/amazon/yjhtmlmapper/h/b.java:373-395) removes text-decoration: none
	// for elements NOT in the decoration-control set {u, a, ins, del, s, strike, br}.
	// For elements IN the set, text-decoration: none is preserved because it has
	// semantic meaning (e.g., removing the inherent underline from <u>).

	tests := []struct {
		name       string
		element    string
		keyword    string
		raw        string
		wantKept   bool
		wantReason string
	}{
		// Normal elements: text-decoration: none should be DROPPED (it's a no-op)
		{
			name:       "p element, keyword none",
			element:    "p",
			keyword:    "none",
			wantKept:   false,
			wantReason: "no-op for <p>",
		},
		{
			name:       "div element, raw none",
			element:    "div",
			raw:        "none",
			wantKept:   false,
			wantReason: "no-op for <div>",
		},
		{
			name:       "h1 element, keyword none",
			element:    "h1",
			keyword:    "none",
			wantKept:   false,
			wantReason: "no-op for <h1>",
		},
		{
			name:       "span element, keyword none",
			element:    "span",
			keyword:    "none",
			wantKept:   false,
			wantReason: "no-op for <span>",
		},

		// Decoration-control elements: text-decoration: none should be KEPT
		{
			name:       "u element preserves none",
			element:    "u",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<u> has inherent underline, none removes it",
		},
		{
			name:       "a element preserves none (reflowable)",
			element:    "a",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<a> has inherent underline in reflowable books",
		},
		{
			name:       "ins element preserves none",
			element:    "ins",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<ins> has inherent underline",
		},
		{
			name:       "del element preserves none",
			element:    "del",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<del> has inherent strikethrough",
		},
		{
			name:       "s element preserves none",
			element:    "s",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<s> has inherent strikethrough",
		},
		{
			name:       "strike element preserves none",
			element:    "strike",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<strike> has inherent strikethrough",
		},
		{
			name:       "br element preserves none",
			element:    "br",
			keyword:    "none",
			wantKept:   true,
			wantReason: "<br> is in the control set per KP3",
		},

		// Case insensitivity
		{
			name:       "U element (uppercase) preserves none",
			element:    "U",
			keyword:    "none",
			wantKept:   true,
			wantReason: "element name matching is case-insensitive",
		},
		{
			name:       "P element (uppercase) drops none",
			element:    "P",
			keyword:    "none",
			wantKept:   false,
			wantReason: "element name matching is case-insensitive",
		},
		{
			name:       "keyword None (mixed case) on p",
			element:    "p",
			keyword:    "None",
			wantKept:   false,
			wantReason: "keyword matching is case-insensitive",
		},

		// Empty element (class-only selector): conservative — keep it
		{
			name:       "empty element preserves none (conservative)",
			element:    "",
			keyword:    "none",
			wantKept:   true,
			wantReason: "class-only selector, can't determine element",
		},

		// Non-none values should always be preserved regardless of element
		{
			name:       "underline on p is kept",
			element:    "p",
			keyword:    "underline",
			wantKept:   true,
			wantReason: "non-none values are never stripped",
		},
		{
			name:       "line-through on div is kept",
			element:    "div",
			raw:        "line-through",
			wantKept:   true,
			wantReason: "non-none values are never stripped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := CSSValue{Keyword: tt.keyword, Raw: tt.raw}
			props := map[string]CSSValue{
				"text-decoration": val,
			}

			norm := normalizeCSSProperties(props, tt.element, nil, "test")

			_, kept := norm["text-decoration"]
			if kept != tt.wantKept {
				t.Errorf("text-decoration kept=%v, want %v (%s)", kept, tt.wantKept, tt.wantReason)
			}
		})
	}
}

func TestNormalizeTextDecorationNoneWithOtherProps(t *testing.T) {
	// Verify that text-decoration stripping doesn't affect other properties
	props := map[string]CSSValue{
		"text-decoration": {Keyword: "none"},
		"font-weight":     {Keyword: "bold"},
		"margin-left":     {Raw: "1em", Value: 1, Unit: "em"},
	}

	norm := normalizeCSSProperties(props, "p", nil, "test")

	if _, ok := norm["text-decoration"]; ok {
		t.Error("expected text-decoration: none to be dropped for <p>")
	}
	if v, ok := norm["font-weight"]; !ok || v.Keyword != "bold" {
		t.Error("expected font-weight to be preserved")
	}
	if v, ok := norm["margin-left"]; !ok || v.Raw != "1em" {
		t.Error("expected margin-left to be preserved")
	}
}
