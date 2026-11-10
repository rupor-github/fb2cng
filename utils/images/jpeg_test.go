package images

import (
	"bytes"
	"image"
	"image/jpeg"
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

func TestJFIFDensityUnits(t *testing.T) {
	for _, tt := range []struct {
		name string
		unit DpiType
		wire byte
	}{
		{name: "unspecified", unit: DpiNoUnits, wire: 0},
		{name: "pixels per inch", unit: DpiPxPerInch, wire: 1},
		{name: "pixels per centimeter", unit: DpiPxPerSm, wire: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x04}
			out, added, err := EnsureJFIFAPP0(data, tt.unit, 300, 144)
			if err != nil || !added {
				t.Fatalf("EnsureJFIFAPP0() added = %t, error = %v", added, err)
			}
			// Use the protocol's literal values, independently of the Go enum.
			want := []byte{
				0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x02,
				tt.wire, 0x01, 0x2c, 0x00, 0x90, 0x00, 0x00,
			}
			if !bytes.Equal(out[2:20], want) {
				t.Errorf("JFIF segment = %x, want %x", out[2:20], want)
			}
			if !bytes.Equal(out[20:], data[2:]) {
				t.Error("inserting JFIF metadata changed the original JPEG payload")
			}
		})
	}
}

func TestEncodeJPEGWithDPI(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 20))
	out, err := EncodeJPEGWithDPI(img, 85, DpiPxPerInch, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out[13:18], []byte{1, 0x01, 0x2c, 0x01, 0x2c}) {
		t.Errorf("JFIF density = %x, want 300 pixels/inch on both axes", out[13:18])
	}
	decoded, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != img.Bounds() {
		t.Errorf("image bounds = %v, want %v", decoded.Bounds(), img.Bounds())
	}
}

func TestJPEGDensityFromDPI(t *testing.T) {
	tests := []struct {
		name string
		dpi  int
		want int16
	}{
		{name: "default", dpi: 0, want: 300},
		{name: "configured", dpi: 144, want: 144},
		{name: "negative default", dpi: -1, want: 300},
		{name: "clamped", dpi: 40000, want: 32767},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JPEGDensityFromDPI(tt.dpi); got != tt.want {
				t.Fatalf("JPEGDensityFromDPI(%d) = %d, want %d", tt.dpi, got, tt.want)
			}
		})
	}
}
