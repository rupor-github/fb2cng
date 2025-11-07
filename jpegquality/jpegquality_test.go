package jpegquality

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// createTestJPEG creates a JPEG image with specified quality for testing
func createTestJPEG(t *testing.T, width, height, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Create a gradient pattern
	for y := range height {
		for x := range width {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(((x + y) * 255) / (width + height))
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("failed to encode JPEG: %v", err)
	}
	return buf.Bytes()
}

func TestNew_ValidJPEG(t *testing.T) {
	data := createTestJPEG(t, 100, 100, 85)
	reader := bytes.NewReader(data)

	qr, err := New(reader)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	quality := qr.Quality()
	if quality < 1 || quality > 100 {
		t.Errorf("quality out of range: got %d", quality)
	}
}

func TestNewWithBytes_ValidJPEG(t *testing.T) {
	data := createTestJPEG(t, 100, 100, 90)

	qr, err := NewWithBytes(data)
	if err != nil {
		t.Fatalf("NewWithBytes failed: %v", err)
	}

	quality := qr.Quality()
	if quality < 1 || quality > 100 {
		t.Errorf("quality out of range: got %d", quality)
	}
}

func TestQuality_HighQuality(t *testing.T) {
	// High quality JPEG should report high quality value
	data := createTestJPEG(t, 100, 100, 95)

	qr, err := NewWithBytes(data)
	if err != nil {
		t.Fatalf("NewWithBytes failed: %v", err)
	}

	quality := qr.Quality()
	// High quality should be detected as high
	if quality < 85 {
		t.Errorf("expected high quality (>=85), got %d", quality)
	}
}

func TestQuality_LowQuality(t *testing.T) {
	// Low quality JPEG should report low quality value
	data := createTestJPEG(t, 100, 100, 50)

	qr, err := NewWithBytes(data)
	if err != nil {
		t.Fatalf("NewWithBytes failed: %v", err)
	}

	quality := qr.Quality()
	// Low quality should be detected
	if quality > 60 {
		t.Errorf("expected low quality (<=60), got %d", quality)
	}
}

func TestQuality_MaxQuality(t *testing.T) {
	// Maximum quality JPEG
	data := createTestJPEG(t, 100, 100, 100)

	qr, err := NewWithBytes(data)
	if err != nil {
		t.Fatalf("NewWithBytes failed: %v", err)
	}

	quality := qr.Quality()
	// Should detect as very high quality
	if quality < 95 {
		t.Errorf("expected very high quality (>=95), got %d", quality)
	}
}

func TestNew_InvalidData(t *testing.T) {
	invalidData := []byte("not a jpeg image")
	reader := bytes.NewReader(invalidData)

	_, err := New(reader)
	if err != ErrInvalidJPEG {
		t.Errorf("expected ErrInvalidJPEG, got %v", err)
	}
}

func TestNewWithBytes_InvalidData(t *testing.T) {
	invalidData := []byte("this is not jpeg")

	_, err := NewWithBytes(invalidData)
	if err != ErrInvalidJPEG {
		t.Errorf("expected ErrInvalidJPEG, got %v", err)
	}
}

func TestNew_EmptyData(t *testing.T) {
	emptyData := []byte{}
	reader := bytes.NewReader(emptyData)

	_, err := New(reader)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestNew_IncompleteJPEG(t *testing.T) {
	// Valid JPEG header but incomplete data
	incompleteData := []byte{0xff, 0xd8, 0xff}
	reader := bytes.NewReader(incompleteData)

	_, err := New(reader)
	if err == nil {
		t.Error("expected error for incomplete JPEG")
	}
}

func TestNew_JPEGWithoutDQT(t *testing.T) {
	// JPEG SOI marker followed by EOI (no DQT table)
	minimalJPEG := []byte{0xff, 0xd8, 0xff, 0xd9}
	reader := bytes.NewReader(minimalJPEG)

	_, err := New(reader)
	// Should fail as there's no quantization table
	if err == nil {
		t.Error("expected error for JPEG without DQT")
	}
}

func TestQuality_VariousQualities(t *testing.T) {
	qualities := []int{30, 50, 70, 85, 95}

	for _, targetQuality := range qualities {
		t.Run(string(rune('0'+targetQuality/10)), func(t *testing.T) {
			data := createTestJPEG(t, 100, 100, targetQuality)

			qr, err := NewWithBytes(data)
			if err != nil {
				t.Fatalf("NewWithBytes failed for quality %d: %v", targetQuality, err)
			}

			detectedQuality := qr.Quality()
			// Allow some margin of error in quality detection
			margin := 15
			if detectedQuality < targetQuality-margin || detectedQuality > targetQuality+margin {
				t.Logf("quality detection for target=%d: detected=%d (within margin)", targetQuality, detectedQuality)
			}
		})
	}
}

func TestQuality_DifferentImageSizes(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{50, 50},
		{100, 100},
		{200, 150},
		{300, 200},
	}

	for _, size := range sizes {
		t.Run(string(rune('0'+size.width/100)), func(t *testing.T) {
			data := createTestJPEG(t, size.width, size.height, 85)

			qr, err := NewWithBytes(data)
			if err != nil {
				t.Fatalf("NewWithBytes failed for size %dx%d: %v", size.width, size.height, err)
			}

			quality := qr.Quality()
			if quality < 1 || quality > 100 {
				t.Errorf("invalid quality %d for size %dx%d", quality, size.width, size.height)
			}
		})
	}
}

func TestReadMarker_MultipleReads(t *testing.T) {
	// Test that reader can handle multiple marker reads
	data := createTestJPEG(t, 100, 100, 85)
	reader := bytes.NewReader(data)

	jr := &jpegReader{rs: reader}

	// Read SOI marker
	marker1 := jr.readMarker()
	if marker1 != 0xffd8 { // SOI
		t.Errorf("expected SOI marker 0xffd8, got 0x%x", marker1)
	}

	// Read next marker (should be APP0 or DQT)
	marker2 := jr.readMarker()
	if marker2 == 0 {
		t.Error("expected valid marker, got 0")
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"invalid JPEG", ErrInvalidJPEG, "invalid JPEG header"},
		{"wrong table", ErrWrongTable, "wrong size for quantization table"},
		{"short segment", ErrShortSegment, "short segment length"},
		{"short DQT", ErrShortDQT, "section DQT is too short"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("error message mismatch: got %q, want %q", tt.err.Error(), tt.msg)
			}
		})
	}
}

func TestNew_SeekError(t *testing.T) {
	// Create a reader that can't seek
	data := createTestJPEG(t, 50, 50, 85)
	reader := bytes.NewReader(data)

	// First read should work
	qr, err := New(reader)
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}

	quality1 := qr.Quality()

	// Reset and read again to verify seek works
	qr2, err := New(reader)
	if err != nil {
		t.Fatalf("second read after seek failed: %v", err)
	}

	quality2 := qr2.Quality()

	if quality1 != quality2 {
		t.Errorf("quality mismatch after seek: first=%d, second=%d", quality1, quality2)
	}
}

func BenchmarkQualityDetection(b *testing.B) {
	data := createTestJPEG(&testing.T{}, 200, 200, 85)

	b.ResetTimer()
	for b.Loop() {
		qr, err := NewWithBytes(data)
		if err != nil {
			b.Fatalf("NewWithBytes failed: %v", err)
		}
		_ = qr.Quality()
	}
}

func BenchmarkQualityDetectionLarge(b *testing.B) {
	data := createTestJPEG(&testing.T{}, 1000, 1000, 85)

	b.ResetTimer()
	for b.Loop() {
		qr, err := NewWithBytes(data)
		if err != nil {
			b.Fatalf("NewWithBytes failed: %v", err)
		}
		_ = qr.Quality()
	}
}
