package fb2

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
)

func TestCoverFill(t *testing.T) {
	for _, tt := range []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"portrait", 120, 150, 64, 80},
		{"narrow", 60, 100, 53, 89},
		{"landscape", 80, 60, 107, 80},
		{"exact", 53, 80, 53, 80},
		{"small", 20, 30, 54, 80},
	} {
		for _, source := range []string{"jpeg", "png", "svg"} {
			for _, output := range []common.OutputFmt{
				common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
				common.OutputFmtAzw3, common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
			} {
				t.Run(fmt.Sprintf("%s/%s/%s", tt.name, source, output), func(t *testing.T) {
					cfg := &config.ImagesConfig{
						JPEGQuality: 95, ScaleFactor: 0.5, RasterizeSVG: source == "svg",
						Screen: config.ScreenConfig{Width: 53, Height: 80, DPI: 300},
						Cover:  config.CoverConfig{Resize: common.ImageResizeModeFill},
					}
					var data []byte
					switch source {
					case "jpeg":
						data = createTestJPEG(t, tt.w, tt.h, 90)
					case "png":
						data = createTestPNG(t, tt.w, tt.h, true)
					case "svg":
						data = []byte(fmt.Sprintf(`<svg viewBox="0 0 %d %d"><rect width="%d" height="%d" fill="red"/></svg>`,
							tt.w, tt.h, tt.w, tt.h))
					}
					bo := BinaryObject{ID: "cover", ContentType: "image/" + source, Data: data}
					if source == "svg" {
						bo.ContentType = "image/svg+xml"
					}
					got := bo.PrepareImage(ImagePolicyFor(output, common.MarkdownImageModeExternal), true, cfg, zap.NewNop())
					decoded, _, err := image.DecodeConfig(bytes.NewReader(got.Data))
					if err != nil {
						t.Fatal(err)
					}
					wantW, wantH := tt.wantW, tt.wantH
					if source == "svg" && output == common.OutputFmtPdf {
						wantW, wantH = tt.w, tt.h
					}
					if decoded.Width != wantW || decoded.Height != wantH || got.Dim.Width != wantW || got.Dim.Height != wantH {
						t.Fatalf("got %dx%d / %+v, want %dx%d", decoded.Width, decoded.Height, got.Dim, wantW, wantH)
					}
				})
			}
		}
	}
}

func TestCoverFillRetainsSVG(t *testing.T) {
	data := []byte(`<svg viewBox="0 0 120 150"><rect width="120" height="150"/></svg>`)
	for _, output := range []common.OutputFmt{common.OutputFmtEpub3, common.OutputFmtKepub3, common.OutputFmtPdf, common.OutputFmtMd} {
		t.Run(output.String(), func(t *testing.T) {
			cfg := &config.ImagesConfig{
				Screen: config.ScreenConfig{Width: 53, Height: 80},
				Cover:  config.CoverConfig{Resize: common.ImageResizeModeFill},
			}
			bo := BinaryObject{ID: "svg", ContentType: "image/svg+xml", Data: data}
			got := bo.PrepareImage(ImagePolicyFor(output, common.MarkdownImageModeExternal), true, cfg, zap.NewNop())
			if !bytes.Equal(data, got.Data) {
				t.Fatal("cover sizing must not force SVG rasterization or change retained SVG")
			}
		})
	}
}
