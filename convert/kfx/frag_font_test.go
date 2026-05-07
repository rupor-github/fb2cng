package kfx

import "testing"

func TestBuildFontFragmentsDeterministicFamilyOrder(t *testing.T) {
	info := &FontInfo{
		FontOrder: []string{"bookerly", "dropcaps", "sangha"},
		AllFonts: map[string][]fontResource{
			"sangha": {
				{Data: []byte("sangha"), MimeType: "font/ttf", Style: SymNormal, Weight: SymNormal, OrigURL: "fonts/sangha.ttf"},
			},
			"bookerly": {
				{Data: []byte("bookerly"), MimeType: "font/ttf", Style: SymNormal, Weight: SymNormal, OrigURL: "fonts/bookerly.ttf"},
			},
			"dropcaps": {
				{Data: []byte("dropcaps"), MimeType: "font/ttf", Style: SymNormal, Weight: SymNormal, OrigURL: "fonts/dropcaps.ttf"},
			},
		},
	}

	fontFrags, rawFrags := BuildFontFragments(info, 12)
	if len(fontFrags) != 3 {
		t.Fatalf("font fragments: got %d, want 3", len(fontFrags))
	}
	if len(rawFrags) != 3 {
		t.Fatalf("raw font fragments: got %d, want 3", len(rawFrags))
	}

	wantFamilies := []string{"nav-bookerly", "nav-dropcaps", "nav-sangha"}
	wantLocations := []string{"resource/rsrcC", "resource/rsrcD", "resource/rsrcE"}
	for i := range fontFrags {
		fontData, ok := fontFrags[i].Value.(StructValue)
		if !ok {
			t.Fatalf("font fragment %d value has type %T, want StructValue", i, fontFrags[i].Value)
		}

		if got := fontData[SymFontFamily]; got != wantFamilies[i] {
			t.Fatalf("font fragment %d family: got %v, want %q", i, got, wantFamilies[i])
		}
		if got := fontData[SymLocation]; got != wantLocations[i] {
			t.Fatalf("font fragment %d location: got %v, want %q", i, got, wantLocations[i])
		}
		if got := rawFrags[i].FIDName; got != wantLocations[i] {
			t.Fatalf("raw font fragment %d location: got %q, want %q", i, got, wantLocations[i])
		}
	}
}
