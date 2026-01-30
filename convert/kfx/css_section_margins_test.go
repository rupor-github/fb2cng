package kfx

import (
	"math"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestSectionContainerMarginsFromDefaultCSS(t *testing.T) {
	log := zap.NewNop()

	css, err := os.ReadFile("../default.css")
	if err != nil {
		t.Fatalf("Failed to read default.css: %v", err)
	}

	registry, _ := NewStyleRegistryFromCSS(css, nil, log)

	def, ok := registry.Get("section")
	if !ok {
		t.Fatal("section style not found in registry")
	}

	resolved := registry.resolveInheritance(def)

	// default.css: .section { margin: 1em 0 }
	// CSS em -> KFX lh conversion uses the default line-height ratio (1.2), so 1em ~= 0.833333lh.
	want := 1.0 / 1.2
	tol := 1e-4

	mt, ok := resolved.Properties[SymMarginTop]
	if !ok {
		t.Fatal("section margin-top missing")
	}
	mtVal, mtUnit, ok := measureParts(mt)
	if !ok {
		t.Fatalf("section margin-top parse failed: %+v", mt)
	}
	if mtUnit != SymUnitLh {
		t.Fatalf("section margin-top unit = %v, want %v", mtUnit, SymUnitLh)
	}
	if math.Abs(mtVal-want) > tol {
		t.Fatalf("section margin-top = %v, want ~%v", mtVal, want)
	}

	mb, ok := resolved.Properties[SymMarginBottom]
	if !ok {
		t.Fatal("section margin-bottom missing")
	}
	mbVal, mbUnit, ok := measureParts(mb)
	if !ok {
		t.Fatalf("section margin-bottom parse failed: %+v", mb)
	}
	if mbUnit != SymUnitLh {
		t.Fatalf("section margin-bottom unit = %v, want %v", mbUnit, SymUnitLh)
	}
	if math.Abs(mbVal-want) > tol {
		t.Fatalf("section margin-bottom = %v, want ~%v", mbVal, want)
	}
}
