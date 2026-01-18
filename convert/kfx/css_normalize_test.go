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

	norm := normalizeCSSProperties(props, nil, "test")

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

	norm := normalizeCSSProperties(props, nil, "test")

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
		norm := normalizeCSSProperties(props, nil, "test")
		if _, ok := norm[prop]; ok {
			t.Errorf("expected %s with zero value to be dropped", prop)
		}
	}
}
