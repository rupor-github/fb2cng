package kfx

import (
	"testing"

	"go.uber.org/zap"
)

func TestEmptyLineMarginValue(t *testing.T) {
	log := zap.NewNop()

	cssContent := []byte(`
.emptyline {
    display: block;
    margin: 1em;
}
`)
	registry, warnings := NewStyleRegistryFromCSS(cssContent, nil, log)
	if len(warnings) > 0 {
		t.Logf("CSS Warnings: %v", warnings)
	}

	// Check the emptyline style
	def, ok := registry.Get("emptyline")
	if !ok {
		t.Fatal("emptyline style NOT found")
	}

	t.Logf("emptyline style found")

	// Resolve inheritance to get full properties
	resolved := registry.resolveInheritance(def)
	t.Logf("Resolved properties: %+v", resolved.Properties)

	// Check margin-top
	if mt, ok := resolved.Properties[SymMarginTop]; ok {
		val, unit, ok := measureParts(mt)
		if ok {
			t.Logf("margin-top: %f %v", val, unit)
		} else {
			t.Logf("margin-top: failed to parse: %+v", mt)
		}
	} else {
		t.Error("margin-top not found")
	}

	// Check margin-bottom
	if mb, ok := resolved.Properties[SymMarginBottom]; ok {
		val, unit, ok := measureParts(mb)
		if ok {
			t.Logf("margin-bottom: %f %v", val, unit)
		} else {
			t.Logf("margin-bottom: failed to parse: %+v", mb)
		}
	} else {
		t.Error("margin-bottom not found")
	}
}

func TestEmptyLineWithInheritance(t *testing.T) {
	log := zap.NewNop()

	// Use actual default.css content for emptyline
	cssContent := []byte(`
/* Base paragraph style */
p {
    text-indent: 1em;
    text-align: justify;
    margin: 0 0 0.3em 0;
}

/* Empty lines for spacing */
.emptyline {
    display: block;
    margin: 1em;
}
`)
	registry, warnings := NewStyleRegistryFromCSS(cssContent, nil, log)
	if len(warnings) > 0 {
		t.Logf("CSS Warnings: %v", warnings)
	}

	// Check the emptyline style
	def, ok := registry.Get("emptyline")
	if !ok {
		t.Fatal("emptyline style NOT found")
	}

	// Resolve inheritance to get full properties
	resolved := registry.resolveInheritance(def)

	// Check margin-top
	if mt, ok := resolved.Properties[SymMarginTop]; ok {
		val, unit, ok := measureParts(mt)
		if ok {
			t.Logf("emptyline margin-top: %f %v (expected ~0.5lh based on KP3 reference)", val, unit)
		}
	}

	// Check line-height
	if lh, ok := resolved.Properties[SymLineHeight]; ok {
		val, unit, ok := measureParts(lh)
		if ok {
			t.Logf("emptyline line-height: %f %v", val, unit)
		}
	} else {
		t.Log("emptyline has no explicit line-height")
	}
}
