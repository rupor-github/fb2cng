package fb2

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"go.uber.org/zap"

	"fbc/config"
)

func TestEncodeImage_GrayscaleJPEG(t *testing.T) {
	bo := &BinaryObject{ID: "test"}
	cfg := &config.ImagesConfig{JPEGQuality: 80, Optimize: true}
	log := zap.NewNop()

	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			src.Set(x, y, color.RGBA{R: 10, G: 10, B: 10, A: 255})
		}
	}

	data, err := bo.encodeImage(src, "jpeg", cfg, log)
	if err != nil {
		t.Fatalf("encodeImage: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	if _, ok := decoded.(*image.Gray); !ok {
		t.Fatalf("expected *image.Gray, got %T", decoded)
	}
}

func TestEncodeImage_ColorJPEG_NotGrayscale(t *testing.T) {
	bo := &BinaryObject{ID: "test"}
	cfg := &config.ImagesConfig{JPEGQuality: 80, Optimize: true}
	log := zap.NewNop()

	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.RGBA{R: 10, G: 20, B: 10, A: 255})
	src.Set(1, 0, color.RGBA{R: 10, G: 10, B: 10, A: 255})
	src.Set(0, 1, color.RGBA{R: 10, G: 10, B: 10, A: 255})
	src.Set(1, 1, color.RGBA{R: 10, G: 10, B: 10, A: 255})

	data, err := bo.encodeImage(src, "jpeg", cfg, log)
	if err != nil {
		t.Fatalf("encodeImage: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	if _, ok := decoded.(*image.Gray); ok {
		t.Fatalf("expected non-grayscale decode, got %T", decoded)
	}
}
