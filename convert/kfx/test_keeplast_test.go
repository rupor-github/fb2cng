package kfx

import (
	"testing"

	"go.uber.org/zap"
)

func TestKeepLastType(t *testing.T) {
	log := zap.NewNop()

	css := []byte(`
.section-subtitle {
    page-break-after: avoid;
}
`)

	registry, _ := NewStyleRegistryFromCSS(css, nil, log)
	def, ok := registry.Get("section-subtitle")
	if !ok {
		t.Fatal("section-subtitle not found!")
	}

	t.Logf("section-subtitle properties before PostProcessForKFX:")
	for sym, val := range def.Properties {
		t.Logf("  %d: %v (type: %T)", sym, val, val)
	}

	// Check for SymKeepLast
	if val, ok := def.Properties[SymKeepLast]; ok {
		t.Logf("SymKeepLast value: %v (type: %T)", val, val)
		switch v := val.(type) {
		case SymbolValue:
			t.Logf("  SymbolValue: %d", v)
		case KFXSymbol:
			t.Logf("  KFXSymbol: %d", v)
		default:
			t.Logf("  Unknown type")
		}
	} else {
		t.Log("SymKeepLast NOT found in properties")
	}
}
