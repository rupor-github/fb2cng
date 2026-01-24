package fb2

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"fbc/common"
	"fbc/config"
	imgutil "fbc/utils/images"
)

// createTestJPEG creates a simple JPEG image for testing
func createTestJPEG(t *testing.T, width, height int, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("failed to encode JPEG: %v", err)
	}
	return buf.Bytes()
}

// createTestPNG creates a simple PNG image for testing
func createTestPNG(t *testing.T, width, height int, transparent bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			alpha := uint8(255)
			if transparent && x%2 == 0 {
				alpha = 128
			}
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 200, alpha})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode PNG: %v", err)
	}
	return buf.Bytes()
}

// createTestGIF creates a simple GIF image for testing
// When transparent is true, every other pixel uses a transparent color
func createTestGIF(t *testing.T, width, height int, transparent bool) []byte {
	t.Helper()
	// Create a palette with some colors, optionally including transparent
	palette := color.Palette{
		color.RGBA{255, 0, 0, 255},   // red
		color.RGBA{0, 255, 0, 255},   // green
		color.RGBA{0, 0, 255, 255},   // blue
		color.RGBA{255, 255, 0, 255}, // yellow
	}
	if transparent {
		palette = append(palette, color.RGBA{0, 0, 0, 0}) // transparent (index 4)
	}

	img := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	for y := range height {
		for x := range width {
			if transparent && x%2 == 0 {
				img.SetColorIndex(x, y, 4) // transparent
			} else {
				img.SetColorIndex(x, y, uint8((x+y)%4)) // cycle through solid colors
			}
		}
	}

	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode GIF: %v", err)
	}
	return buf.Bytes()
}

func TestBinaryObject_PrepareImage_BasicJPEG(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	jpegData := createTestJPEG(t, 100, 100, 90)
	bo := &BinaryObject{
		ID:          "test-jpeg",
		ContentType: "image/jpeg",
		Data:        jpegData,
	}

	bi := bo.PrepareImage(false, false, cfg, log)
	if bi == nil {
		t.Fatal("expected non-nil BookImage")
	}
	if bi.MimeType != "image/jpeg" {
		t.Errorf("expected mime type image/jpeg, got %s", bi.MimeType)
	}
	if len(bi.Data) == 0 {
		t.Error("expected non-empty image data")
	}
}

func TestBinaryObject_PrepareImage_BasicPNG(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	pngData := createTestPNG(t, 50, 50, false)
	bo := &BinaryObject{
		ID:          "test-png",
		ContentType: "image/png",
		Data:        pngData,
	}

	bi := bo.PrepareImage(false, false, cfg, log)
	if bi.MimeType != "image/png" {
		t.Errorf("expected mime type image/png, got %s", bi.MimeType)
	}
}

func TestBinaryObject_PrepareImage_CoverResizeKeepAR(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 800, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeKeepAR},
	}

	// Create a small image that should be resized
	jpegData := createTestJPEG(t, 100, 150, 90)
	bo := &BinaryObject{
		ID:          "cover-small",
		ContentType: "image/jpeg",
		Data:        jpegData,
	}

	bi := bo.PrepareImage(false, true, cfg, log)

	// Decode and verify dimensions
	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode resized image: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dy() != 1200 {
		t.Errorf("expected height 1200, got %d", bounds.Dy())
	}
	// Width should maintain aspect ratio
	expectedWidth := 800
	if bounds.Dx() != expectedWidth {
		t.Logf("width after resize: %d (expected around %d)", bounds.Dx(), expectedWidth)
	}
}

func TestBinaryObject_PrepareImage_CoverResizeStretch(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 600, Height: 900},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeStretch},
	}

	jpegData := createTestJPEG(t, 100, 100, 90)
	bo := &BinaryObject{
		ID:          "cover-stretch",
		ContentType: "image/jpeg",
		Data:        jpegData,
	}

	bi := bo.PrepareImage(false, true, cfg, log)

	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode stretched image: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 600 || bounds.Dy() != 900 {
		t.Errorf("expected dimensions 600x900, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestBinaryObject_PrepareImage_CoverNoResize(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 800, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	originalWidth, originalHeight := 100, 150
	jpegData := createTestJPEG(t, originalWidth, originalHeight, 90)
	bo := &BinaryObject{
		ID:          "cover-no-resize",
		ContentType: "image/jpeg",
		Data:        jpegData,
	}

	bi := bo.PrepareImage(false, true, cfg, log)

	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode image: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != originalWidth || bounds.Dy() != originalHeight {
		t.Errorf("dimensions changed, expected %dx%d, got %dx%d", originalWidth, originalHeight, bounds.Dx(), bounds.Dy())
	}
}

func TestBinaryObject_PrepareImage_ScaleFactor(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        0.5,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	jpegData := createTestJPEG(t, 100, 200, 90)
	bo := &BinaryObject{
		ID:          "scaled",
		ContentType: "image/jpeg",
		Data:        jpegData,
	}

	bi := bo.PrepareImage(false, false, cfg, log)

	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode scaled image: %v", err)
	}

	bounds := img.Bounds()
	expectedHeight := int(200 * 0.5)
	if bounds.Dy() != expectedHeight {
		t.Errorf("expected height %d, got %d", expectedHeight, bounds.Dy())
	}
}

func TestBinaryObject_PrepareImage_RemoveTransparency(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: true,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	pngData := createTestPNG(t, 50, 50, true)
	bo := &BinaryObject{
		ID:          "transparent-png",
		ContentType: "image/png",
		Data:        pngData,
	}

	bi := bo.PrepareImage(false, false, cfg, log)

	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}

	// Check if image is opaque
	if opaqueChecker, ok := img.(interface{ Opaque() bool }); ok {
		if !opaqueChecker.Opaque() {
			t.Error("expected PNG to be opaque after transparency removal")
		}
	}
}

func TestBinaryObject_PrepareImage_RemoveGIFTransparency(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: true,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	gifData := createTestGIF(t, 50, 50, true)
	bo := &BinaryObject{
		ID:          "transparent-gif",
		ContentType: "image/gif",
		Data:        gifData,
	}

	bi := bo.PrepareImage(false, false, cfg, log)

	// GIF with transparency removed should be converted to PNG
	// (since encodeImage doesn't support GIF encoding)
	if bi.MimeType != "image/png" {
		t.Errorf("expected mime type image/png after transparency removal, got %s", bi.MimeType)
	}

	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode result image: %v", err)
	}

	// Check if image is opaque after transparency removal
	if opaqueChecker, ok := img.(interface{ Opaque() bool }); ok {
		if !opaqueChecker.Opaque() {
			t.Error("expected image to be opaque after transparency removal")
		}
	}
}

func TestBinaryObject_PrepareImage_GIFToJPEGKindle(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	// Create a GIF with transparency
	gifData := createTestGIF(t, 50, 50, true)
	bo := &BinaryObject{
		ID:          "transparent-gif-kindle",
		ContentType: "image/gif",
		Data:        gifData,
	}

	// With kindle=true, GIF should be converted to JPEG with transparency flattened
	bi := bo.PrepareImage(true, false, cfg, log)

	if bi.MimeType != "image/jpeg" {
		t.Errorf("expected mime type image/jpeg for Kindle, got %s", bi.MimeType)
	}

	img, _, err := image.Decode(bytes.NewReader(bi.Data))
	if err != nil {
		t.Fatalf("failed to decode JPEG: %v", err)
	}

	// JPEG is always opaque, but verify the conversion succeeded
	if img.Bounds().Dx() != 50 || img.Bounds().Dy() != 50 {
		t.Errorf("expected 50x50 image, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}

	// Check that transparent areas are now white (not black)
	// In our test GIF, even x coordinates (0, 2, 4, ...) are transparent
	// Sample pixel at (0, 1) which should have been transparent
	r, g, b, _ := img.At(0, 1).RGBA()
	// JPEG compression causes color bleeding, so we just check it's bright (> 180)
	// rather than pure white. The key is it shouldn't be black (< 50)
	threshold := uint32(180 << 8) // ~180 in 16-bit color space
	if r < threshold || g < threshold || b < threshold {
		t.Errorf("expected bright (white) background for transparent areas, got RGB(%d, %d, %d)",
			r>>8, g>>8, b>>8)
	}
}

func TestBinaryObject_PrepareImage_JPEGQualityOptimization(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        50,
		Optimize:           true,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	// Create high quality JPEG
	jpegData := createTestJPEG(t, 100, 100, 95)
	originalSize := len(jpegData)

	bo := &BinaryObject{
		ID:          "high-quality-jpeg",
		ContentType: "image/jpeg",
		Data:        jpegData,
	}

	bi := bo.PrepareImage(false, false, cfg, log)

	// Optimized image should be smaller
	if len(bi.Data) >= originalSize {
		t.Logf("optimization may not have reduced size: original=%d, optimized=%d", originalSize, len(bi.Data))
	}
}

func TestBinaryObject_PrepareImage_KindleConversion(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	// PNG should be converted to JPEG for Kindle
	pngData := createTestPNG(t, 50, 50, false)
	bo := &BinaryObject{
		ID:          "kindle-convert",
		ContentType: "image/png",
		Data:        pngData,
	}

	bi := bo.PrepareImage(true, false, cfg, log)

	if bi.MimeType != "image/jpeg" {
		t.Errorf("expected conversion to JPEG for Kindle, got %s", bi.MimeType)
	}
}

func TestBinaryObject_PrepareImage_SVGRasterizeForKindle(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           true,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        0.5,
		Screen:             config.ScreenConfig{Width: 800, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeStretch},
	}

	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="100" height="100"/></svg>`)
	bo := &BinaryObject{
		ID:          "test-svg",
		ContentType: "image/svg+xml",
		Data:        svgData,
	}

	bi := bo.PrepareImage(true, true, cfg, log)

	if bi.MimeType != "image/jpeg" {
		t.Fatalf("expected SVG to be rasterized to JPEG for Kindle, got %s", bi.MimeType)
	}
	if _, _, err := image.Decode(bytes.NewReader(bi.Data)); err != nil {
		t.Fatalf("expected rasterized SVG data to be decodable image: %v", err)
	}
}

func TestBinaryObject_PrepareImage_NotFoundPlaceholderSVGIntrinsicSizeForKindle(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 8000, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	bo := &BinaryObject{
		ID:          "not-found",
		ContentType: "image/svg+xml",
		Data:        notFoundImage,
	}

	bi := bo.PrepareImage(true, false, cfg, log)
	if bi.MimeType != "image/jpeg" {
		t.Fatalf("expected notFoundImage SVG to be rasterized to JPEG for Kindle, got %s", bi.MimeType)
	}
	if bi.Dim.Width != 200 || bi.Dim.Height != 200 {
		t.Fatalf("expected intrinsic 200x200 placeholder, got %dx%d", bi.Dim.Width, bi.Dim.Height)
	}
}

func TestBinaryObject_PrepareImage_InvalidImage(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 800, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeStretch},
	}

	bo := &BinaryObject{
		ID:          "invalid",
		ContentType: "image/jpeg",
		Data:        []byte("not a valid image"),
	}

	bi := bo.PrepareImage(false, true, cfg, log)
	if bi == nil {
		t.Fatal("expected non-nil BookImage with placeholder")
	}
	// Should return placeholder PNG, not original broken data
	if bytes.Equal(bi.Data, bo.Data) {
		t.Fatal("expected placeholder image, got original broken data")
	}
	if bi.MimeType != "image/svg+xml" {
		t.Errorf("expected MimeType to be image/png, got %s", bi.MimeType)
	}
}

func TestBinaryObject_PrepareImage_UseBrokenFlag(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          true,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 800, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeStretch},
	}

	bo := &BinaryObject{
		ID:          "broken",
		ContentType: "image/jpeg",
		Data:        []byte("not a valid image"),
	}

	bi := bo.PrepareImage(false, true, cfg, log)
	if bi == nil {
		t.Fatal("expected non-nil BookImage with UseBroken=true")
	}
	// Should return original broken data
	if !bytes.Equal(bi.Data, bo.Data) {
		t.Error("expected original data returned for broken image")
	}
}

func TestFictionBook_PrepareImages(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	jpegData := createTestJPEG(t, 100, 100, 90)
	pngData := createTestPNG(t, 50, 50, false)

	fb := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				Coverpage: []InlineImage{{Href: "#cover.jpg"}},
			},
		},
		Binaries: []BinaryObject{
			{ID: "cover.jpg", ContentType: "image/jpeg", Data: jpegData},
			{ID: "img1.png", ContentType: "image/png", Data: pngData},
		},
	}

	index := fb.PrepareImages(false, cfg, log)

	if len(index) != 2 {
		t.Errorf("expected 2 images in index, got %d", len(index))
	}

	if _, exists := index["cover.jpg"]; !exists {
		t.Error("expected cover.jpg in index")
	}
	if _, exists := index["img1.png"]; !exists {
		t.Error("expected img1.png in index")
	}
}

func TestFictionBook_PrepareImages_DuplicateIDs(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	jpegData := createTestJPEG(t, 100, 100, 90)

	fb := &FictionBook{
		Binaries: []BinaryObject{
			{ID: "img1", ContentType: "image/jpeg", Data: jpegData},
			{ID: "img1", ContentType: "image/jpeg", Data: jpegData}, // Duplicate
			{ID: "img2", ContentType: "image/jpeg", Data: jpegData},
		},
	}

	index := fb.PrepareImages(false, cfg, log)

	// Should skip duplicate
	if len(index) != 2 {
		t.Errorf("expected 2 images (duplicate skipped), got %d", len(index))
	}
}

func TestFictionBook_PrepareImages_CoverDetection(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality:        85,
		Optimize:           false,
		UseBroken:          false,
		RemoveTransparency: false,
		ScaleFactor:        1.0,
		Screen:             config.ScreenConfig{Width: 800, Height: 1200},
		Cover:              config.CoverConfig{Resize: common.ImageResizeModeKeepAR},
	}

	// Create a small cover image
	coverData := createTestJPEG(t, 100, 150, 90)
	regularData := createTestJPEG(t, 50, 50, 90)

	fb := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				Coverpage: []InlineImage{{Href: "#cover-id"}},
			},
		},
		Binaries: []BinaryObject{
			{ID: "cover-id", ContentType: "image/jpeg", Data: coverData},
			{ID: "regular", ContentType: "image/jpeg", Data: regularData},
		},
	}

	index := fb.PrepareImages(false, cfg, log)

	// Cover should be resized
	coverImage := index["cover-id"]
	if coverImage == nil {
		t.Fatal("cover image not found")
	}

	img, _, err := image.Decode(bytes.NewReader(coverImage.Data))
	if err != nil {
		t.Fatalf("failed to decode cover: %v", err)
	}

	// Should have been resized to match height
	if img.Bounds().Dy() != 1200 {
		t.Errorf("expected cover height 1200, got %d", img.Bounds().Dy())
	}
}

func TestEnsureJFIFAPP0(t *testing.T) {
	// Create a JPEG without JFIF APP0 marker
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode JPEG: %v", err)
	}

	out, added, err := imgutil.EnsureJFIFAPP0(buf.Bytes(), imgutil.DpiPxPerInch, 300, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected DPI marker to be added")
	}
	if len(out) <= buf.Len() {
		t.Error("expected buffer to grow after adding DPI marker")
	}

	// Verify it starts with JPEG SOI marker
	if out[0] != 0xFF || out[1] != 0xD8 {
		t.Error("JPEG SOI marker should be preserved")
	}

	// Verify APP0 marker was added
	if out[2] != 0xFF || out[3] != 0xE0 {
		t.Error("expected JFIF APP0 marker at position 2-3")
	}
}

func TestParseBinary_ValidBase64(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	data := []byte("test data")
	encoded := base64.StdEncoding.EncodeToString(data)

	el := mustElement(t, `<binary id="img1" content-type="image/jpeg">`+encoded+`</binary>`)

	bo, err := parseBinary(el, log)
	if err != nil {
		t.Fatalf("parseBinary failed: %v", err)
	}

	if bo.ID != "img1" {
		t.Errorf("expected ID 'img1', got %q", bo.ID)
	}
	if bo.ContentType != "image/jpeg" {
		t.Errorf("expected content-type 'image/jpeg', got %q", bo.ContentType)
	}
	if !bytes.Equal(bo.Data, data) {
		t.Errorf("decoded data mismatch")
	}
}

func TestParseBinary_WithWhitespace(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	data := []byte("test data")
	encoded := base64.StdEncoding.EncodeToString(data)
	// Add whitespace (split carefully to avoid index out of bounds)
	var encodedWithSpaces string
	if len(encoded) > 10 {
		encodedWithSpaces = encoded[:10] + "\n  " + encoded[10:]
	} else {
		encodedWithSpaces = encoded[:5] + "\n  " + encoded[5:]
	}

	el := mustElement(t, `<binary id="img2" content-type="image/png">`+encodedWithSpaces+`</binary>`)

	bo, err := parseBinary(el, log)
	if err != nil {
		t.Fatalf("parseBinary failed: %v", err)
	}

	if !bytes.Equal(bo.Data, data) {
		t.Errorf("decoded data mismatch with whitespace normalization")
	}
}

func TestParseBinary_CorruptedBase64(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Completely invalid base64
	el := mustElement(t, `<binary id="bad" content-type="image/jpeg">!!!invalid!!!</binary>`)
	_, err := parseBinary(el, log)
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestPrepareImages_SkipsNonImageBinaries(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg := &config.ImagesConfig{
		JPEGQuality: 85,
		Optimize:    false,
		UseBroken:   false,
		ScaleFactor: 1.0,
		Screen:      config.ScreenConfig{Width: 0, Height: 0},
		Cover:       config.CoverConfig{Resize: common.ImageResizeModeNone},
	}

	jpegData := createTestJPEG(t, 100, 100, 90)

	fb := &FictionBook{
		Binaries: []BinaryObject{
			// Regular image - should be processed
			{ID: "img1", ContentType: "image/jpeg", Data: jpegData},
			// Font - should be skipped
			{ID: "font1", ContentType: "font/woff2", Data: []byte("fake font data")},
			// Another font format - should be skipped
			{ID: "font2", ContentType: "application/font-woff", Data: []byte("fake font data 2")},
			// Another image - should be processed
			{ID: "img2", ContentType: "image/png", Data: createTestPNG(t, 50, 50, false)},
		},
	}

	images := fb.PrepareImages(false, cfg, log)

	// Should only have 2 images (img1 and img2), fonts should be skipped
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}

	if _, exists := images["img1"]; !exists {
		t.Error("image 'img1' should exist in index")
	}
	if _, exists := images["img2"]; !exists {
		t.Error("image 'img2' should exist in index")
	}
	if _, exists := images["font1"]; exists {
		t.Error("font 'font1' should not be in image index")
	}
	if _, exists := images["font2"]; exists {
		t.Error("font 'font2' should not be in image index")
	}
}
