package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"io"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func TestPDFGrayscalePreservesGIFLayout(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		t.Run(fmt.Sprint(transparent), func(t *testing.T) {
			palette := color.Palette{color.RGBA{R: 255, A: 255}, color.Black}
			if transparent {
				palette[1] = color.Transparent
			}
			var data bytes.Buffer
			if err := gif.Encode(&data, image.NewPaletted(image.Rect(0, 0, 600, 100), palette), nil); err != nil {
				t.Fatal(err)
			}
			bo := fb2.BinaryObject{ID: "gif", ContentType: "image/gif", Data: data.Bytes()}
			cfg := config.ImagesConfig{JPEGQuality: 95}
			policy := fb2.ImagePolicyFor(common.OutputFmtPdf, common.MarkdownImageModePlaceholder)
			before := bo.PrepareImage(policy, false, &cfg, zap.NewNop())
			cfg.Grayscale = common.GrayscaleModeLuma
			after := bo.PrepareImage(policy, false, &cfg, zap.NewNop())
			doc := pdfDocumentSpec{PageWidth: 300, PageHeight: 400, ScreenWidthPx: 1200, ScreenHeightPx: 1600}
			w1, h1, ok1 := fitPDFBlockImageSize(doc, before, 200, 300, 200, false)
			w2, h2, ok2 := fitPDFBlockImageSize(doc, after, 200, 300, 200, false)
			if w1 != w2 || h1 != h2 || ok1 != ok2 {
				t.Fatalf("GIF-to-gray-PNG changed layout from %g/%g/%t to %g/%g/%t", w1, h1, ok1, w2, h2, ok2)
			}
		})
	}
}

func TestPDFGrayscaleSVG(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		force      bool
		vector     bool
	}{
		{"native", `<rect width="40" height="20" fill="red"/>`, false, true},
		{"forced", `<rect width="40" height="20" fill="red"/>`, true, false},
		{"fallback", `<g opacity="0.5"><rect width="40" height="20" fill="red"/></g>`, false, false},
	} {
		for _, format := range []common.RasterFormat{common.RasterFormatAuto, common.RasterFormatJpeg} {
			for _, prepared := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/prepared=%t", tt.name, format, prepared), func(t *testing.T) {
					cfg := config.ImagesConfig{
						Grayscale: common.GrayscaleModeLuma, RasterFormat: format, RasterizeSVG: tt.force, JPEGQuality: 95,
					}
					bo := fb2.BinaryObject{ID: "svg", ContentType: "image/svg+xml",
						Data: []byte(`<svg viewBox="0 0 40 20">` + tt.body + `</svg>`)}
					img := &fb2.BookImage{MimeType: bo.ContentType, Data: bo.Data}
					if prepared {
						img = bo.PrepareImage(fb2.ImagePolicyFor(common.OutputFmtPdf, common.MarkdownImageModePlaceholder), false, &cfg, zap.NewNop())
					}
					got, err := makePDFImageResource(img, cfg, zap.NewNop())
					if err != nil {
						t.Fatal(err)
					}
					if tt.vector {
						if got.Dict["Subtype"] != docwriter.Name("Form") {
							t.Fatal("grayscale must not force rasterization of native PDF vectors")
						}
						return
					}
					if format == common.RasterFormatJpeg {
						decoded, err := jpeg.Decode(bytes.NewReader(got.Data))
						if err != nil {
							t.Fatal(err)
						}
						if _, ok := decoded.(*image.Gray); !ok || got.Dict["ColorSpace"] != docwriter.Name("DeviceGray") {
							t.Fatal("PDF grayscale JPEG must be single-channel")
						}
					} else {
						reader, err := zlib.NewReader(bytes.NewReader(got.Data))
						if err != nil {
							t.Fatal(err)
						}
						data, err := io.ReadAll(reader)
						if err != nil {
							t.Fatal(err)
						}
						if err := reader.Close(); err != nil {
							t.Fatal(err)
						}
						for i := 0; i < len(data); i += 3 {
							if data[i] != data[i+1] || data[i+1] != data[i+2] {
								t.Fatal("PDF SVG raster fallback must be grayscale")
							}
						}
					}
				})
			}
		}
	}
}
