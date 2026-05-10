package kfx

import "testing"

func TestCollectReferencedResourceNamesScansNestedValues(t *testing.T) {
	fragments := NewFragmentList()
	mustAddFragment(t, fragments, &Fragment{
		FType: SymContent,
		FID:   1,
		Value: StructValue{
			SymContentList: []any{
				StructValue{SymResourceName: SymbolByName("e1")},
				map[KFXSymbol]any{SymResourceName: "e2"},
				map[string]any{"nested": []any{StructValue{SymResourceName: ReadSymbolValue("e3")}}},
				StructValue{SymResourceName: SymbolValue(SymFormatPNG)},
			},
		},
	})

	got := collectReferencedResourceNames(fragments)
	for _, name := range []string{"e1", "e2", "e3", "png"} {
		if !got[name] {
			t.Fatalf("collectReferencedResourceNames() missing %q in %#v", name, got)
		}
	}
	if len(got) != 4 {
		t.Fatalf("collectReferencedResourceNames() = %#v, want exactly four names", got)
	}
}

func TestCollectReferencedResourceNamesNilList(t *testing.T) {
	got := collectReferencedResourceNames(nil)
	if len(got) != 0 {
		t.Fatalf("collectReferencedResourceNames(nil) = %#v, want empty map", got)
	}
}

func TestFilterImageResourcesKeepsOnlyReferencedResources(t *testing.T) {
	external := []*Fragment{
		{FType: SymExtResource, FIDName: "e1", Value: NewStruct().SetString(SymLocation, "resource/rsrc1")},
		{FType: SymExtResource, FIDName: "e2", Value: NewStruct().SetString(SymLocation, "resource/rsrc2")},
		{FType: SymExtResource, FIDName: "e3", Value: "not a struct"},
	}
	raw := []*Fragment{
		{FType: SymRawMedia, FIDName: "resource/rsrc1", Value: RawValue("one")},
		{FType: SymRawMedia, FIDName: "resource/rsrc2", Value: RawValue("two")},
	}
	info := imageResourceInfoByID{
		"img1": {ResourceName: "e1", Width: 10, Height: 20},
		"img2": {ResourceName: "e2", Width: 30, Height: 40},
		"img3": {ResourceName: "e3", Width: 50, Height: 60},
	}

	filteredExternal, filteredRaw, filteredInfo := filterImageResources(external, raw, info, map[string]bool{"e2": true, "missing": true})
	if len(filteredExternal) != 1 || filteredExternal[0].FIDName != "e2" {
		t.Fatalf("filtered external = %#v, want only e2", fragmentNames(filteredExternal))
	}
	if len(filteredRaw) != 1 || filteredRaw[0].FIDName != "resource/rsrc2" {
		t.Fatalf("filtered raw = %#v, want only resource/rsrc2", fragmentNames(filteredRaw))
	}
	if len(filteredInfo) != 1 {
		t.Fatalf("filtered info = %#v, want one image", filteredInfo)
	}
	if got := filteredInfo["img2"]; got.ResourceName != "e2" || got.Width != 30 || got.Height != 40 {
		t.Fatalf("filtered info img2 = %#v, want original e2 info", got)
	}
}

func TestFilterImageResourcesEmptyReferencesDropsAll(t *testing.T) {
	external, raw, info := filterImageResources(
		[]*Fragment{{FType: SymExtResource, FIDName: "e1"}},
		[]*Fragment{{FType: SymRawMedia, FIDName: "resource/rsrc1"}},
		imageResourceInfoByID{"img1": {ResourceName: "e1"}},
		nil,
	)
	if external != nil || raw != nil || info != nil {
		t.Fatalf("filterImageResources(..., nil) = %#v, %#v, %#v; want nils", external, raw, info)
	}
}

func fragmentNames(fragments []*Fragment) []string {
	names := make([]string, len(fragments))
	for i, frag := range fragments {
		names[i] = frag.FIDName
	}
	return names
}
