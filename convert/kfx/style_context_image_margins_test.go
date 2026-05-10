package kfx

import (
	"reflect"
	"testing"
)

func TestEnsureFixedBlockImageMargins(t *testing.T) {
	styles := NewStyleRegistry()
	baseProps := map[KFXSymbol]any{
		SymWidth:     DimensionValue(80, SymUnitPercent),
		SymMarginTop: DimensionValue(1, SymUnitLh),
	}
	styles.Register(StyleDef{Name: "image", Properties: baseProps})
	styles.usage["image"] = styleUsageImage

	name := ensureFixedBlockImageMargins(styles, "image", 0.25, 0.5)
	if name == "" || name == "image" {
		t.Fatalf("ensureFixedBlockImageMargins() = %q, want generated style name", name)
	}
	if got := styles.GetUsage(name); got != styleUsageImage {
		t.Fatalf("generated style usage = %v, want image usage", got)
	}

	def, ok := styles.Get(name)
	if !ok {
		t.Fatalf("generated style %q not registered", name)
	}
	if !reflect.DeepEqual(def.Properties[SymWidth], baseProps[SymWidth]) {
		t.Fatalf("generated width = %#v, want base width %#v", def.Properties[SymWidth], baseProps[SymWidth])
	}
	if !reflect.DeepEqual(def.Properties[SymMarginTop], DimensionValue(0.25, SymUnitLh)) {
		t.Fatalf("generated margin top = %#v, want 0.25lh", def.Properties[SymMarginTop])
	}
	if !reflect.DeepEqual(def.Properties[SymMarginBottom], DimensionValue(0.5, SymUnitLh)) {
		t.Fatalf("generated margin bottom = %#v, want 0.5lh", def.Properties[SymMarginBottom])
	}

	// The helper must not mutate the base style while building the variant.
	baseDef, _ := styles.Get("image")
	if !reflect.DeepEqual(baseDef.Properties[SymMarginTop], DimensionValue(1, SymUnitLh)) {
		t.Fatalf("base margin top mutated to %#v", baseDef.Properties[SymMarginTop])
	}
	if _, ok := baseDef.Properties[SymMarginBottom]; ok {
		t.Fatalf("base margin bottom unexpectedly added: %#v", baseDef.Properties[SymMarginBottom])
	}
}

func TestEnsureFixedBlockImageMarginsReturnsBaseForMissingInputs(t *testing.T) {
	if got := ensureFixedBlockImageMargins(nil, "image", 1, 2); got != "image" {
		t.Fatalf("nil registry result = %q, want base style", got)
	}

	styles := NewStyleRegistry()
	if got := ensureFixedBlockImageMargins(styles, "", 1, 2); got != "" {
		t.Fatalf("empty base style result = %q, want empty", got)
	}
	if got := ensureFixedBlockImageMargins(styles, "missing", 1, 2); got != "missing" {
		t.Fatalf("missing style result = %q, want missing", got)
	}
}
