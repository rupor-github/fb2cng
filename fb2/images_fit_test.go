package fb2

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"github.com/beevik/etree"
	"github.com/disintegration/imaging"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
)

func TestBodyImageScreenLimit(t *testing.T) {
	for _, tt := range []struct {
		name         string
		w, h         int
		scale        float64
		fit          bool
		wantW, wantH int
	}{
		{"unchanged", 120, 150, 1, false, 120, 150},
		{"half", 120, 150, 0.5, false, 60, 75},
		{"half limited", 120, 150, 0.5, true, 53, 66},
		{"double limited", 120, 150, 2, true, 53, 66},
		{"small", 10, 20, 1, true, 10, 20},
		{"explicit enlargement", 10, 20, 2, true, 20, 40},
		{"small half", 10, 20, 0.5, true, 5, 10},
		{"tall", 60, 180, 1, true, 27, 80},
		{"wide", 120, 60, 1, true, 53, 27},
		{"skip scaling", 120, 150, 0, true, 53, 66},
		{"exact", 53, 80, 1, true, 53, 80},
	} {
		for _, output := range []common.OutputFmt{
			common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
			common.OutputFmtAzw3, common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
		} {
			t.Run(tt.name+"/"+output.String(), func(t *testing.T) {
				data := createTestJPEG(t, tt.w, tt.h, 90)
				cfg := &config.ImagesConfig{
					ScaleFactor: tt.scale, FitToScreen: tt.fit, JPEGQuality: 95,
					Screen: config.ScreenConfig{Width: 53, Height: 80, DPI: 300},
				}
				bo := BinaryObject{ID: "body", ContentType: "image/jpeg", Data: data}
				got := bo.PrepareImage(ImagePolicyFor(output, common.MarkdownImageModeExternal), false, cfg, zap.NewNop())
				decoded, _, err := image.DecodeConfig(bytes.NewReader(got.Data))
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Width != tt.wantW || decoded.Height != tt.wantH {
					t.Fatalf("got %dx%d, want %dx%d", decoded.Width, decoded.Height, tt.wantW, tt.wantH)
				}
				if tt.wantW == tt.w && tt.wantH == tt.h {
					if !bytes.Equal(got.Data, data) {
						t.Fatal("unchanged image was re-encoded")
					}
					return
				}
				src, _, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				want, err := bo.encodeImage(imaging.Resize(src, tt.wantW, tt.wantH, imaging.Linear), "jpeg", cfg, zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got.Data, want) {
					t.Fatal("scaling and fitting must use one resize and encoding pass")
				}
			})
		}
	}
}

func TestBodySVGScreenLimit(t *testing.T) {
	for _, size := range []struct{ w, h, wantW, wantH int }{{10, 20, 20, 40}, {120, 150, 53, 66}} {
		for _, force := range []bool{false, true} {
			for _, output := range []common.OutputFmt{
				common.OutputFmtEpub3, common.OutputFmtKepub3, common.OutputFmtAzw3, common.OutputFmtKfx,
				common.OutputFmtPdf, common.OutputFmtMd,
			} {
				t.Run(fmt.Sprintf("%dx%d/%s/raster=%t", size.w, size.h, output, force), func(t *testing.T) {
					data := []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">
<rect width="100%%" height="100%%" fill="red"/></svg>`, size.w, size.h))
					cfg := &config.ImagesConfig{
						ScaleFactor: 2, FitToScreen: true, RasterizeSVG: force, JPEGQuality: 95,
						Screen: config.ScreenConfig{Width: 53, Height: 80},
					}
					bo := BinaryObject{ID: "svg", ContentType: "image/svg+xml", Data: data}
					got := bo.PrepareImage(ImagePolicyFor(output, common.MarkdownImageModeExternal), false, cfg, zap.NewNop())
					if output == common.OutputFmtPdf {
						if !force && !bytes.Equal(data, got.Data) {
							t.Fatal("PDF SVG must retain intrinsic sizing")
						}
						if !force {
							return
						}
						if got.Dim.Width != size.w || got.Dim.Height != size.h {
							t.Fatal("PDF SVG rasterization changed intrinsic dimensions")
						}
						return
					}
					if got.Dim.Width != size.wantW || got.Dim.Height != size.wantH {
						t.Fatalf("got %+v, want %dx%d", got.Dim, size.wantW, size.wantH)
					}
					if force {
						img, _, err := image.DecodeConfig(bytes.NewReader(got.Data))
						if err != nil || img.Width != size.wantW || img.Height != size.wantH {
							t.Fatalf("raster dimensions = %+v, error = %v", img, err)
						}
					} else if output.IsKindle() {
						if !bytes.Equal(data, got.Data) {
							t.Fatal("native Kindle SVG must retain vector data")
						}
					} else {
						doc := etree.NewDocument()
						if err := doc.ReadFromBytes(got.Data); err != nil {
							t.Fatal(err)
						}
						if doc.Root().SelectAttrValue("width", "") != fmt.Sprint(size.wantW) ||
							doc.Root().SelectAttrValue("height", "") != fmt.Sprint(size.wantH) {
							t.Fatal("retained SVG viewport was not updated")
						}
					}
				})
			}
		}
	}
}

func TestScreenLimitDoesNotAffectCovers(t *testing.T) {
	cfg := &config.ImagesConfig{
		FitToScreen: true, ScaleFactor: 2, JPEGQuality: 95,
		Screen: config.ScreenConfig{Width: 53, Height: 80},
	}
	data := createTestJPEG(t, 120, 150, 90)
	bo := BinaryObject{ID: "cover", ContentType: "image/jpeg", Data: data}
	got := bo.PrepareImage(ImagePolicyFor(common.OutputFmtEpub3, common.MarkdownImageModePlaceholder), true, cfg, zap.NewNop())
	if !bytes.Equal(data, got.Data) {
		t.Fatal("body-image screen limit must not affect covers")
	}
}
