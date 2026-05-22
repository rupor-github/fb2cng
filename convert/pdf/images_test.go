package pdf

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func TestLayoutPDFPagesAddsCoverImagePage(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      100,
		PageHeight:     160,
		ScreenWidthPx:  100,
		ScreenHeightPx: 160,
		Title:          "Title",
		Author:         "Author",
		CoverID:        "cover",
		Images: fb2.BookImages{"cover": &fb2.BookImage{
			MimeType: "image/png",
			Dim: struct {
				Width  int
				Height int
			}{Width: 50, Height: 80},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want cover only", len(pages))
	}
	if len(pages[0].Images) != 1 || pages[0].Images[0].ImageID != "cover" {
		t.Fatalf("cover page images = %#v", pages[0].Images)
	}
	if len(pages[0].Anchors) != 1 || pages[0].Anchors[0] != "cover" {
		t.Fatalf("cover page anchors = %#v", pages[0].Anchors)
	}
}

func TestMakePDFImageResourceEmbedsJPEGDirectly(t *testing.T) {
	resource, err := makePDFImageResource(&fb2.BookImage{
		MimeType: "image/jpeg",
		Data:     testJPEG(t, 2, 3),
	})
	if err != nil {
		t.Fatalf("makePDFImageResource() error = %v", err)
	}
	got := docwriter.Format(resource.Dict)
	for _, want := range []string{"/Filter /DCTDecode", "/Subtype /Image", "/Width 2", "/Height 3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("image dict = %q, missing %q", got, want)
		}
	}
}

func TestNaturalPDFImageSizeUsesConfiguredDPIWithoutScreenPixels(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 300
	img.Dim.Height = 600

	width, height := naturalPDFImageSize(pdfDocumentSpec{ScreenDPI: 150}, img)
	if width != 144 || height != 288 {
		t.Fatalf("naturalPDFImageSize() = %v/%v, want dimensions from configured dpi", width, height)
	}

	width, height = naturalPDFImageSize(pdfDocumentSpec{}, img)
	if width != 0 || height != 0 {
		t.Fatalf("naturalPDFImageSize() without screen geometry/dpi = %v/%v, want zero", width, height)
	}
}

func TestGenerateEmbedsPDFImageXObject(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 100, Height: 160, DPI: 100}}}
	imageData := testPNG(t, 2, 3)
	c := &content.Content{
		SrcName: "book.fb2",
		CoverID: "cover",
		ImagesIndex: fb2.BookImages{"cover": &fb2.BookImage{
			MimeType: "image/png",
			Data:     imageData,
			Dim: struct {
				Width  int
				Height int
			}{Width: 2, Height: 3},
		}},
		Book: &fb2.FictionBook{
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Image Book"},
				Coverpage: []fb2.InlineImage{{Href: "#cover"}},
			}},
		},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	pdfText := string(data)
	for _, want := range []string{
		"/XObject << /Im1",
		"/Subtype /Image",
		"/ColorSpace /DeviceRGB",
		"/Width 2",
		"/Height 3",
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain %q", want)
		}
	}
}

func testPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := testImage(width, height)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func testJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := testImage(width, height)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func testImage(width int, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.NRGBA{R: uint8(40 + x), G: uint8(80 + y), B: 120, A: 255})
		}
	}
	return img
}
