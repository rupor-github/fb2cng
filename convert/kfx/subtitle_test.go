package kfx

import (
	"testing"

	"go.uber.org/zap"
)

func TestSectionSubtitleMarginBottom(t *testing.T) {
	log := zap.NewNop()

	css := []byte(`
.section-subtitle {
    page-break-before: auto;
    text-align: center;
    font-weight: bold;
    text-indent: 0;
    margin: 1em 0;
    page-break-after: avoid;
}
`)

	registry, warnings := parseAndCreateRegistry(css, nil, log)
	t.Logf("Warnings: %v", warnings)

	// Get the style definition
	def, ok := registry.Get("section-subtitle")
	if !ok {
		t.Fatal("section-subtitle not found!")
	}

	t.Logf("section-subtitle properties:")
	for sym, val := range def.Properties {
		t.Logf("  %d: %v (type: %T)", sym, val, val)
	}

	// Check margin-top
	if _, ok := def.Properties[SymMarginTop]; !ok {
		t.Error("section-subtitle should have margin-top")
	}

	// Check margin-bottom - should be preserved for subtitle with page-break-after: avoid
	if _, ok := def.Properties[SymMarginBottom]; !ok {
		t.Error("section-subtitle should have margin-bottom (page-break-after: avoid should preserve it)")
	}

	// Check yj-break-after
	if _, ok := def.Properties[SymYjBreakAfter]; !ok {
		t.Error("section-subtitle should have yj-break-after")
	}
}
