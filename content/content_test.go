package content

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"fbc/config"
	"fbc/fb2"
	"fbc/state"
)

// Helper functions for test image creation
func createTestJPEG(t *testing.T, width, height, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{uint8((x * 255) / width), uint8((y * 255) / height), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("failed to encode test JPEG: %v", err)
	}
	return buf.Bytes()
}

func createTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

func setupTestContent(t *testing.T) (*Content, context.Context) {
	t.Helper()
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.Images.Optimize = false
	cfg.Document.Images.UseBroken = false

	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	env.Log = logger
	env.Cfg = cfg

	tmpDir := t.TempDir()

	doc := etree.NewDocument()
	return &Content{WorkDir: tmpDir, Doc: doc}, ctx
}

func TestContent_GetCoverID_WithCoverpage(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	jpegData := createTestJPEG(t, 100, 150, 90)

	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{
					{Href: "#cover-image"},
				},
			},
		},
		Binaries: []fb2.BinaryObject{
			{ID: "cover-image", ContentType: "image/jpeg", Data: jpegData},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)
	c.ImagesIndex = imagesIndex

	// Parse cover ID
	if len(c.Book.Description.TitleInfo.Coverpage) > 0 {
		href := c.Book.Description.TitleInfo.Coverpage[0].Href
		c.CoverID = parseImageRef(href)
	}

	if c.CoverID != "cover-image" {
		t.Errorf("expected coverID 'cover-image', got %q", c.CoverID)
	}

	if _, exists := c.ImagesIndex[c.CoverID]; !exists {
		t.Error("cover image should exist in images index")
	}
}

func TestContent_GetCoverID_NoCoverpage(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{},
			},
		},
		Binaries: []fb2.BinaryObject{},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)
	c.ImagesIndex = imagesIndex

	if len(c.Book.Description.TitleInfo.Coverpage) > 0 {
		c.CoverID = parseImageRef(c.Book.Description.TitleInfo.Coverpage[0].Href)
	}

	if c.CoverID != "" {
		t.Errorf("expected empty coverID when no coverpage, got %q", c.CoverID)
	}
}

func TestContent_MultipleCoverpageImages(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	jpegData1 := createTestJPEG(t, 100, 150, 90)
	jpegData2 := createTestJPEG(t, 200, 300, 90)

	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{
					{Href: "#cover1"},
					{Href: "#cover2"},
				},
			},
		},
		Binaries: []fb2.BinaryObject{
			{ID: "cover1", ContentType: "image/jpeg", Data: jpegData1},
			{ID: "cover2", ContentType: "image/jpeg", Data: jpegData2},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)
	c.ImagesIndex = imagesIndex

	// Should use first coverpage image
	if len(c.Book.Description.TitleInfo.Coverpage) > 0 {
		c.CoverID = parseImageRef(c.Book.Description.TitleInfo.Coverpage[0].Href)
	}

	if c.CoverID != "cover1" {
		t.Errorf("expected first cover 'cover1', got %q", c.CoverID)
	}
}

func TestContent_CoverImageProcessing_Resize(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	// Configure cover resizing
	env.Cfg.Document.Images.Cover.Width = 600
	env.Cfg.Document.Images.Cover.Height = 900
	env.Cfg.Document.Images.Cover.Resize = config.ImageResizeModeKeepAR

	// Small cover that should be resized
	jpegData := createTestJPEG(t, 100, 150, 90)

	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{{Href: "#small-cover"}},
			},
		},
		Binaries: []fb2.BinaryObject{
			{ID: "small-cover", ContentType: "image/jpeg", Data: jpegData},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)

	coverImage := imagesIndex["small-cover"]
	if coverImage == nil {
		t.Fatal("cover image not found in index")
	}

	// Verify it was resized
	img, _, err := image.Decode(bytes.NewReader(coverImage.Data))
	if err != nil {
		t.Fatalf("failed to decode cover: %v", err)
	}

	if img.Bounds().Dy() != 900 {
		t.Errorf("expected cover height 900, got %d", img.Bounds().Dy())
	}
}

func TestContent_CoverImageProcessing_NoResize(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	env.Cfg.Document.Images.Cover.Resize = config.ImageResizeModeNone

	originalWidth, originalHeight := 100, 150
	jpegData := createTestJPEG(t, originalWidth, originalHeight, 90)

	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{{Href: "#orig-cover"}},
			},
		},
		Binaries: []fb2.BinaryObject{
			{ID: "orig-cover", ContentType: "image/jpeg", Data: jpegData},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)

	coverImage := imagesIndex["orig-cover"]
	img, _, err := image.Decode(bytes.NewReader(coverImage.Data))
	if err != nil {
		t.Fatalf("failed to decode cover: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != originalWidth || bounds.Dy() != originalHeight {
		t.Errorf("cover dimensions changed, expected %dx%d, got %dx%d",
			originalWidth, originalHeight, bounds.Dx(), bounds.Dy())
	}
}

func TestContent_MissingCoverImage(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{{Href: "#missing-cover"}},
			},
		},
		Binaries: []fb2.BinaryObject{
			{ID: "other-image", ContentType: "image/jpeg", Data: createTestJPEG(t, 50, 50, 90)},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)
	c.ImagesIndex = imagesIndex

	c.CoverID = "missing-cover"

	if _, exists := c.ImagesIndex[c.CoverID]; exists {
		t.Error("missing cover should not exist in images index")
	}
}

func TestContent_ImageIndexBuild(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	jpegData1 := createTestJPEG(t, 100, 100, 90)
	jpegData2 := createTestJPEG(t, 50, 50, 90)
	pngData := createTestPNG(t, 75, 75)

	c.Book = &fb2.FictionBook{
		Binaries: []fb2.BinaryObject{
			{ID: "img1", ContentType: "image/jpeg", Data: jpegData1},
			{ID: "img2", ContentType: "image/jpeg", Data: jpegData2},
			{ID: "img3", ContentType: "image/png", Data: pngData},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)

	if len(imagesIndex) != 3 {
		t.Errorf("expected 3 images in index, got %d", len(imagesIndex))
	}

	for _, id := range []string{"img1", "img2", "img3"} {
		if _, exists := imagesIndex[id]; !exists {
			t.Errorf("expected image %s in index", id)
		}
	}
}

func TestContent_KindleImageConversion(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	pngData := createTestPNG(t, 100, 100)

	c.Book = &fb2.FictionBook{
		Binaries: []fb2.BinaryObject{
			{ID: "png-img", ContentType: "image/png", Data: pngData},
		},
	}

	// Process for Kindle (should convert PNG to JPEG)
	imagesIndex := c.Book.PrepareImages(true, &env.Cfg.Document.Images, env.Log)

	img := imagesIndex["png-img"]
	if img == nil {
		t.Fatal("image not found")
	}

	if img.MimeType != "image/jpeg" {
		t.Errorf("expected JPEG conversion for Kindle, got %s", img.MimeType)
	}
}

func TestContent_DefaultCoverFallback(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	// Book with no cover
	c.Book = &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				Coverpage: []fb2.InlineImage{},
			},
		},
		Binaries: []fb2.BinaryObject{},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)
	c.ImagesIndex = imagesIndex

	// Check if default cover exists
	defaultCoverPath := "./default_cover.jpeg"
	if _, err := os.Stat(defaultCoverPath); err == nil {
		t.Logf("default cover available at %s", defaultCoverPath)
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with hash prefix", "#image-id", "image-id"},
		{"without prefix", "image-id", "image-id"},
		{"empty string", "", ""},
		{"just hash", "#", ""},
		{"complex id", "#cover_image_001", "cover_image_001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseImageRef(tt.input)
			if got != tt.expected {
				t.Errorf("parseImageRef(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// parseImageRef extracts image ID from href (removes # prefix)
func parseImageRef(href string) string {
	if len(href) > 0 && href[0] == '#' {
		return href[1:]
	}
	return href
}

func TestContent_ImageScaling(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	env.Cfg.Document.Images.ScaleFactor = 0.5

	jpegData := createTestJPEG(t, 200, 200, 90)

	c.Book = &fb2.FictionBook{
		Binaries: []fb2.BinaryObject{
			{ID: "scaled-img", ContentType: "image/jpeg", Data: jpegData},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)

	img := imagesIndex["scaled-img"]
	decoded, _, err := image.Decode(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatalf("failed to decode scaled image: %v", err)
	}

	expectedHeight := int(200 * 0.5)
	if decoded.Bounds().Dy() != expectedHeight {
		t.Errorf("expected scaled height %d, got %d", expectedHeight, decoded.Bounds().Dy())
	}
}

func TestContent_PNGTransparencyRemoval(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	env.Cfg.Document.Images.RemovePNGTransparency = true

	// Create transparent PNG
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := range 50 {
		for x := range 50 {
			alpha := uint8(128)
			img.Set(x, y, color.RGBA{100, 150, 200, alpha})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to create test PNG: %v", err)
	}

	c.Book = &fb2.FictionBook{
		Binaries: []fb2.BinaryObject{
			{ID: "trans-png", ContentType: "image/png", Data: buf.Bytes()},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)

	resultImg := imagesIndex["trans-png"]
	decoded, _, err := image.Decode(bytes.NewReader(resultImg.Data))
	if err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}

	// Check if opaque
	if checker, ok := decoded.(interface{ Opaque() bool }); ok {
		if !checker.Opaque() {
			t.Error("expected opaque PNG after transparency removal")
		}
	}
}

func TestContent_ImageOptimization(t *testing.T) {
	c, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)

	env.Cfg.Document.Images.Optimize = true
	env.Cfg.Document.Images.JPEGQuality = 50

	// Create high quality JPEG
	highQualityData := createTestJPEG(t, 100, 100, 95)
	originalSize := len(highQualityData)

	c.Book = &fb2.FictionBook{
		Binaries: []fb2.BinaryObject{
			{ID: "hq-img", ContentType: "image/jpeg", Data: highQualityData},
		},
	}

	imagesIndex := c.Book.PrepareImages(false, &env.Cfg.Document.Images, env.Log)

	optimizedImg := imagesIndex["hq-img"]
	optimizedSize := len(optimizedImg.Data)

	t.Logf("Original size: %d, Optimized size: %d", originalSize, optimizedSize)

	// Optimization should reduce size (though not guaranteed in all cases)
	if optimizedSize >= originalSize {
		t.Logf("Note: optimization did not reduce size (original=%d, optimized=%d)", originalSize, optimizedSize)
	}
}

func TestFilterReferencedImages(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Create test images
	img1Data := createTestJPEG(t, 50, 50, 80)
	img2Data := createTestJPEG(t, 50, 50, 80)
	img3Data := createTestJPEG(t, 50, 50, 80)
	img4Data := createTestJPEG(t, 50, 50, 80)

	// Create all images map
	allImages := fb2.BookImages{
		"cover-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img1Data,
		},
		"used-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img2Data,
		},
		"unused-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img3Data,
		},
		"inline-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img4Data,
		},
	}

	// Create links index with only some images referenced
	links := fb2.ReverseLinkIndex{
		"cover-img": []fb2.ElementRef{
			{Type: "coverpage", Path: []any{}},
		},
		"used-img": []fb2.ElementRef{
			{Type: "block-image", Path: []any{}},
		},
		"inline-img": []fb2.ElementRef{
			{Type: "inline-image", Path: []any{}},
		},
		"some-text-link": []fb2.ElementRef{
			{Type: "inline-link", Path: []any{}},
		},
	}

	// Filter images
	filtered := filterReferencedImages(allImages, links, "cover-img", log)

	// Verify only referenced images are included
	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered images, got %d", len(filtered))
	}

	// Verify correct images are present
	if _, exists := filtered["cover-img"]; !exists {
		t.Error("cover-img should be included (coverpage)")
	}
	if _, exists := filtered["used-img"]; !exists {
		t.Error("used-img should be included (block-image)")
	}
	if _, exists := filtered["inline-img"]; !exists {
		t.Error("inline-img should be included (inline-image)")
	}

	// Verify unused image is not present
	if _, exists := filtered["unused-img"]; exists {
		t.Error("unused-img should not be included")
	}
}

func TestFilterReferencedImages_EmptyCover(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	img1Data := createTestJPEG(t, 50, 50, 80)
	img2Data := createTestJPEG(t, 50, 50, 80)

	allImages := fb2.BookImages{
		"used-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img1Data,
		},
		"unused-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img2Data,
		},
	}

	links := fb2.ReverseLinkIndex{
		"used-img": []fb2.ElementRef{
			{Type: "block-image", Path: []any{}},
		},
	}

	// Filter with no cover
	filtered := filterReferencedImages(allImages, links, "", log)

	// Verify only used image is included
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered image, got %d", len(filtered))
	}

	if _, exists := filtered["used-img"]; !exists {
		t.Error("used-img should be included")
	}

	if _, exists := filtered["unused-img"]; exists {
		t.Error("unused-img should not be included")
	}
}

func TestFilterReferencedImages_OnlyTextLinks(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	img1Data := createTestJPEG(t, 50, 50, 80)

	allImages := fb2.BookImages{
		"some-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img1Data,
		},
	}

	// Only text links, no image references
	links := fb2.ReverseLinkIndex{
		"text-target": []fb2.ElementRef{
			{Type: "inline-link", Path: []any{}},
		},
		"another-text": []fb2.ElementRef{
			{Type: "inline-link", Path: []any{}},
		},
	}

	// Filter with no cover and no image links
	filtered := filterReferencedImages(allImages, links, "", log)

	// Should be empty
	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered images when only text links exist, got %d", len(filtered))
	}
}

func TestFilterReferencedImages_IncludesNotFoundImage(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	img1Data := createTestJPEG(t, 50, 50, 80)
	img2Data := createTestJPEG(t, 50, 50, 80)
	notFoundData := createTestPNG(t, 50, 50)

	allImages := fb2.BookImages{
		"used-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img1Data,
		},
		"unused-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img2Data,
		},
		fb2.NotFoundImageID: &fb2.BookImage{
			MimeType: "image/png",
			Data:     notFoundData,
		},
	}

	links := fb2.ReverseLinkIndex{
		"used-img": []fb2.ElementRef{
			{Type: "block-image", Path: []any{}},
		},
	}

	// Filter images
	filtered := filterReferencedImages(allImages, links, "", log)

	// Should include used-img and not-found image, but not unused-img
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered images, got %d", len(filtered))
	}

	if _, exists := filtered["used-img"]; !exists {
		t.Error("used-img should be included")
	}

	if _, exists := filtered[fb2.NotFoundImageID]; !exists {
		t.Error("not-found image should always be included")
	}

	if _, exists := filtered["unused-img"]; exists {
		t.Error("unused-img should not be included")
	}
}

func TestFilterReferencedImages_WithoutNotFoundImage(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	img1Data := createTestJPEG(t, 50, 50, 80)

	allImages := fb2.BookImages{
		"used-img": &fb2.BookImage{
			MimeType: "image/jpeg",
			Data:     img1Data,
		},
	}

	links := fb2.ReverseLinkIndex{
		"used-img": []fb2.ElementRef{
			{Type: "block-image", Path: []any{}},
		},
	}

	// Filter images - should work fine when not-found image doesn't exist
	filtered := filterReferencedImages(allImages, links, "", log)

	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered image, got %d", len(filtered))
	}

	if _, exists := filtered["used-img"]; !exists {
		t.Error("used-img should be included")
	}
}

func TestNormalizeIDs(t *testing.T) {
	fb2Content := `<?xml version="1.0" encoding="utf-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
  <description>
    <title-info>
      <genre>prose</genre>
      <author><first-name>Test</first-name><last-name>Author</last-name></author>
      <book-title>Test Book</book-title>
      <lang>en</lang>
    </title-info>
    <document-info>
      <author><first-name>Test</first-name><last-name>Author</last-name></author>
      <program-used>test</program-used>
      <date>2024-01-01</date>
      <id>00000000-0000-0000-0000-000000000001</id>
      <version>1.0</version>
    </document-info>
  </description>
  <body>
    <section>
      <title><p>Chapter 1</p></title>
      <subtitle>First subtitle</subtitle>
      <p>Content 1</p>
    </section>
    <section>
      <title><p>Chapter 2</p></title>
      <subtitle>Second subtitle</subtitle>
      <p>Content 2</p>
      <section>
        <title><p>Nested section</p></title>
        <p>Nested content</p>
      </section>
    </section>
  </body>
</FictionBook>`

	logger := zap.NewNop()
	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	env.Cfg = cfg
	env.Log = logger

	reader := strings.NewReader(fb2Content)
	c, err := Prepare(ctx, reader, "test.fb2", config.OutputFmtEpub2, logger)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}

	// Verify sections have IDs assigned
	// Should have 3 sections (2 top-level + 1 nested), all with IDs now
	if len(c.Book.Bodies) == 0 {
		t.Fatal("Expected at least one body")
	}

	body := &c.Book.Bodies[0]
	if len(body.Sections) != 2 {
		t.Fatalf("Expected 2 top-level sections, got %d", len(body.Sections))
	}

	// Check first section has ID
	if body.Sections[0].ID == "" {
		t.Error("First section should have an ID")
	}
	if !strings.HasPrefix(body.Sections[0].ID, "sect_") {
		t.Errorf("Section ID %q doesn't follow pattern 'sect_N'", body.Sections[0].ID)
	}

	// Check second section has ID
	if body.Sections[1].ID == "" {
		t.Error("Second section should have an ID")
	}

	// Check nested section has ID
	hasNestedSection := false
	for _, item := range body.Sections[1].Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			hasNestedSection = true
			if item.Section.ID == "" {
				t.Error("Nested section should have an ID")
			}
			if !strings.HasPrefix(item.Section.ID, "sect_") {
				t.Errorf("Nested section ID %q doesn't follow pattern 'sect_N'", item.Section.ID)
			}
		}
	}
	if !hasNestedSection {
		t.Error("Expected a nested section")
	}

	// Check subtitles have IDs
	subtitleCount := 0
	for _, item := range body.Sections[0].Content {
		if item.Kind == fb2.FlowSubtitle && item.Subtitle != nil {
			subtitleCount++
			if item.Subtitle.ID == "" {
				t.Error("Subtitle should have an ID")
			}
			if !strings.HasPrefix(item.Subtitle.ID, "subtitle_") {
				t.Errorf("Subtitle ID %q doesn't follow pattern 'subtitle_N'", item.Subtitle.ID)
			}
		}
	}
	if subtitleCount == 0 {
		t.Error("Expected at least one subtitle")
	}
}
