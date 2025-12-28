package images

import (
	"bytes"
	"testing"
)

func TestEnsureJFIFAPP0_AddsMarker(t *testing.T) {
	// Minimal JPEG without APP0
	data := []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x04}

	out, added, err := EnsureJFIFAPP0(data, DpiPxPerInch, 300, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Fatal("expected marker to be added")
	}
	if len(out) <= len(data) {
		t.Fatal("expected output to grow")
	}
	if out[0] != 0xFF || out[1] != 0xD8 {
		t.Fatal("expected SOI marker preserved")
	}
	if !bytes.Equal(out[2:4], []byte{0xFF, 0xE0}) {
		t.Fatal("expected JFIF APP0 marker at position 2-3")
	}
}

func TestEnsureJFIFAPP0_AlreadyPresent(t *testing.T) {
	// Minimal JPEG with APP0 already present
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}

	out, added, err := EnsureJFIFAPP0(data, DpiPxPerInch, 300, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added {
		t.Fatal("expected no marker addition")
	}
	if !bytes.Equal(out, data) {
		t.Fatal("expected same bytes")
	}
}
