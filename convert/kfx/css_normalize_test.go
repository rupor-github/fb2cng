package kfx

import "testing"

func TestNormalizeCSSPropertiesDropsZeroSizes(t *testing.T) {
	props := map[string]CSSValue{
		"width":       {Raw: "0px", Value: 0, Unit: "px"},
		"height":      {Raw: "0", Value: 0},
		"margin-left": {Raw: "1em", Value: 1, Unit: "em"},
	}

	norm := normalizeCSSProperties(props, nil, "test")

	if _, ok := norm["width"]; ok {
		t.Fatalf("expected width to be dropped")
	}
	if _, ok := norm["height"]; ok {
		t.Fatalf("expected height to be dropped")
	}
	if v, ok := norm["margin-left"]; !ok || v.Raw != "1em" {
		t.Fatalf("expected margin-left preserved, got %v", norm["margin-left"])
	}
}

func TestNormalizeCSSPropertiesKeepsNonZeroSize(t *testing.T) {
	props := map[string]CSSValue{
		"width":  {Raw: "10%", Value: 10, Unit: "%"},
		"height": {Raw: "0.5em", Value: 0.5, Unit: "em"},
	}

	norm := normalizeCSSProperties(props, nil, "test")

	if len(norm) != len(props) {
		t.Fatalf("expected no properties dropped, got %d", len(norm))
	}
	if norm["width"].Raw != "10%" || norm["height"].Raw != "0.5em" {
		t.Fatalf("unexpected normalization result: %v", norm)
	}
}
