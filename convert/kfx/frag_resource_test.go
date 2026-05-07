package kfx

import "testing"

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
