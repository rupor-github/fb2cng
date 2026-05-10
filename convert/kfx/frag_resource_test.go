package kfx

import (
	"testing"

	"fbc/fb2"
)

func TestNextResourceLocationIndexUsesHighestRawMediaLocation(t *testing.T) {
	rawMedia := []*Fragment{
		{FType: SymRawMedia, FIDName: "resource/rsrc1"},
		{FType: SymRawMedia, FIDName: "resource/rsrc5"},
		{FType: SymRawMedia, FIDName: "resource/rsrcA"},
	}

	if got := nextResourceLocationIndex(rawMedia); got != 11 {
		t.Fatalf("nextResourceLocationIndex() = %d, want 11", got)
	}
}

func TestNextResourceLocationIndexEmpty(t *testing.T) {
	if got := nextResourceLocationIndex(nil); got != 1 {
		t.Fatalf("nextResourceLocationIndex(nil) = %d, want 1", got)
	}
}

func TestBuildImageResourceFragmentsSortsAndFiltersImages(t *testing.T) {
	images := fb2.BookImages{
		"z":     imageForResourceTest("image/png", []byte{1}, 20, 30),
		"a":     imageForResourceTest(" image/jpeg ", []byte{2}, 10, 15),
		"empty": imageForResourceTest("image/png", nil, 1, 1),
		"bad":   imageForResourceTest("image/webp", []byte{3}, 1, 1),
		"nil":   nil,
	}

	external, raw, info := buildImageResourceFragments(images)
	if len(external) != 2 || len(raw) != 2 || len(info) != 2 {
		t.Fatalf("buildImageResourceFragments() counts = ext:%d raw:%d info:%d, want 2 each", len(external), len(raw), len(info))
	}

	if external[0].FIDName != "e1" || raw[0].FIDName != "resource/rsrc1" {
		t.Fatalf("first resource names = %q/%q, want e1/resource/rsrc1", external[0].FIDName, raw[0].FIDName)
	}
	if external[1].FIDName != "e2" || raw[1].FIDName != "resource/rsrc2" {
		t.Fatalf("second resource names = %q/%q, want e2/resource/rsrc2", external[1].FIDName, raw[1].FIDName)
	}

	firstValue := external[0].Value.(StructValue)
	if got, _ := firstValue.GetString(SymLocation); got != "resource/rsrc1" {
		t.Fatalf("first external location = %q, want resource/rsrc1", got)
	}
	if got, _ := firstValue.GetString(SymMIME); got != "image/jpg" {
		t.Fatalf("first external MIME = %q, want image/jpg", got)
	}
	if got := firstValue[SymResourceName]; got != SymbolByNameValue("e1") {
		t.Fatalf("first external resource_name = %#v, want symbol e1", got)
	}

	if got := info["a"]; got != (imageResourceInfo{ResourceName: "e1", Width: 10, Height: 15}) {
		t.Fatalf("info[a] = %#v, want e1 dimensions", got)
	}
	if got := info["z"]; got != (imageResourceInfo{ResourceName: "e2", Width: 20, Height: 30}) {
		t.Fatalf("info[z] = %#v, want e2 dimensions", got)
	}
}

func TestBuildImageResourceFragmentsEmpty(t *testing.T) {
	external, raw, info := buildImageResourceFragments(nil)
	if external != nil || raw != nil || info != nil {
		t.Fatalf("buildImageResourceFragments(nil) = %#v, %#v, %#v; want nils", external, raw, info)
	}
}

func TestResourceNameAndFormatHelpers(t *testing.T) {
	if got := makeResourceName(36); got != "e10" {
		t.Fatalf("makeResourceName(36) = %q, want e10", got)
	}
	if got := makeResourceLocation(36); got != "resource/rsrc10" {
		t.Fatalf("makeResourceLocation(36) = %q, want resource/rsrc10", got)
	}
	if got := resourceLocationIndex("resource/rsrc10"); got != 36 {
		t.Fatalf("resourceLocationIndex(resource/rsrc10) = %d, want 36", got)
	}
	if got := resourceLocationIndex("resource/not-base36"); got != 0 {
		t.Fatalf("resourceLocationIndex(invalid) = %d, want 0", got)
	}

	formatTests := []struct {
		mime       string
		wantSymbol KFXSymbol
		wantMIME   string
	}{
		{mime: "image/jpeg", wantSymbol: SymFormatJPG, wantMIME: "image/jpg"},
		{mime: "image/jpg", wantSymbol: SymFormatJPG, wantMIME: "image/jpg"},
		{mime: "image/png", wantSymbol: SymFormatPNG, wantMIME: "image/png"},
		{mime: "image/gif", wantSymbol: SymFormatGIF, wantMIME: "image/gif"},
		{mime: "image/webp", wantSymbol: -1, wantMIME: ""},
	}
	for _, tt := range formatTests {
		t.Run(tt.mime, func(t *testing.T) {
			gotSymbol := imageFormatSymbol(tt.mime)
			if gotSymbol != tt.wantSymbol {
				t.Fatalf("imageFormatSymbol(%q) = %v, want %v", tt.mime, gotSymbol, tt.wantSymbol)
			}
			if got := formatSymbolToMIME(gotSymbol); got != tt.wantMIME {
				t.Fatalf("formatSymbolToMIME(%v) = %q, want %q", gotSymbol, got, tt.wantMIME)
			}
		})
	}
}

func imageForResourceTest(mimeType string, data []byte, width, height int) *fb2.BookImage {
	img := &fb2.BookImage{MimeType: mimeType, Data: data}
	img.Dim.Width = width
	img.Dim.Height = height
	return img
}
