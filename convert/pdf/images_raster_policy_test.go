package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func TestPDFRasterFormatSVGPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, body    string
		force, remove bool
		vector        bool
	}{
		{"native", `<rect width="20" height="20" fill="red"/>`, false, false, true},
		{"forced", `<rect width="20" height="20" fill="red"/>`, true, false, false},
		{"remove alpha", `<rect width="20" height="20" fill="red"/>`, false, true, false},
		{"fallback", `<g opacity="0.5"><rect width="20" height="20" fill="red"/></g>`, false, false, false},
	} {
		for _, prepared := range []bool{false, true} {
			for _, cover := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/prepared=%t/cover=%t", tt.name, prepared, cover), func(t *testing.T) {
					bo := fb2.BinaryObject{ID: "art", ContentType: "image/svg+xml", Data: []byte(
						`<svg viewBox="0 0 40 20">` + tt.body + `</svg>`)}
					cfg := config.ImagesConfig{
						RasterizeSVG: tt.force, RemoveTransparency: tt.remove, JPEGQuality: 95, ScaleFactor: 0.5,
						Cover:  config.CoverConfig{Resize: common.ImageResizeModeStretch},
						Screen: config.ScreenConfig{Width: 1200, Height: 1600, DPI: 300},
					}
					var beforeWidth, beforeHeight float64
					for _, rasterFormat := range []common.RasterFormat{common.RasterFormatAuto, common.RasterFormatJpeg} {
						cfg.RasterFormat = rasterFormat
						img := &fb2.BookImage{MimeType: bo.ContentType, Data: bo.Data}
						if prepared {
							img = bo.PrepareImage(common.OutputFmtPdf, cover, &cfg, zap.NewNop())
						}
						doc := pdfDocumentSpec{ScreenDPI: 300, ImageConfig: cfg, Images: fb2.BookImages{"art": img}}
						width, height, ok := fitPDFBlockImageSize(doc, img, 200, 300, 200, false)
						if !ok {
							t.Fatal("invalid layout size")
						}
						if rasterFormat == common.RasterFormatAuto {
							beforeWidth, beforeHeight = width, height
						} else if width != beforeWidth || height != beforeHeight {
							t.Fatal("JPEG encoding changed SVG layout dimensions")
						}
						nextID := 1
						resources, err := preparePDFImageResources(doc, []pdfPage{{Images: []pdfPageImage{{ImageID: "art"}}}}, &nextID)
						if err != nil {
							t.Fatal(err)
						}
						resource := resources["art"]
						if tt.vector {
							if resource.Dict["Subtype"] != docwriter.Name("Form") {
								t.Fatal("raster format must not disable native SVG")
							}
							continue
						}
						want := docwriter.Name("FlateDecode")
						if rasterFormat == common.RasterFormatJpeg {
							want = "DCTDecode"
						}
						if resource.Dict["Filter"] != want || resource.Dict["Width"] != docwriter.Integer(40) ||
							resource.Dict["Height"] != docwriter.Integer(20) {
							t.Fatalf("incorrect raster resource: %+v", resource.Dict)
						}
						if rasterFormat == common.RasterFormatJpeg {
							decoded, err := jpeg.Decode(bytes.NewReader(resource.Data))
							if err != nil {
								t.Fatal(err)
							}
							r, g, b, _ := decoded.At(35, 10).RGBA()
							if min(r, g, b) < 0xf000 {
								t.Fatal("SVG JPEG fallback must have a white background")
							}
						}
					}
				})
			}
		}
	}
}

func TestPDFRasterFormatPreservesGIFLayout(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		for _, remove := range []bool{false, true} {
			t.Run(fmt.Sprintf("alpha=%t/remove=%t", transparent, remove), func(t *testing.T) {
				palette := color.Palette{color.White, color.Black}
				if transparent {
					palette[0] = color.Transparent
				}
				var data bytes.Buffer
				if err := gif.Encode(&data, image.NewPaletted(image.Rect(0, 0, 600, 100), palette), nil); err != nil {
					t.Fatal(err)
				}
				bo := fb2.BinaryObject{ID: "gif", ContentType: "image/gif", Data: data.Bytes()}
				cfg := config.ImagesConfig{JPEGQuality: 95, RemoveTransparency: remove}
				doc := pdfDocumentSpec{PageWidth: 300, PageHeight: 400, ScreenWidthPx: 1200, ScreenHeightPx: 1600}
				before := bo.PrepareImage(common.OutputFmtPdf, false, &cfg, zap.NewNop())
				cfg.RasterFormat = common.RasterFormatJpeg
				after := bo.PrepareImage(common.OutputFmtPdf, false, &cfg, zap.NewNop())
				w1, h1, ok1 := fitPDFBlockImageSize(doc, before, 200, 300, 200, false)
				w2, h2, ok2 := fitPDFBlockImageSize(doc, after, 200, 300, 200, false)
				if w1 != w2 || h1 != h2 || ok1 != ok2 {
					t.Fatalf("GIF-to-JPEG changed layout from %g/%g/%t to %g/%g/%t", w1, h1, ok1, w2, h2, ok2)
				}
				if after.MimeType != "image/jpeg" {
					t.Fatalf("explicit JPEG not applied: %s", after.MimeType)
				}
			})
		}
	}
}
