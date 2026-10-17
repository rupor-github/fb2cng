package fb2

import (
	"bytes"
	"image"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/common"
	"fbc/config"
)

func TestSVGOutputPolicy(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20">
<rect width="20" height="20" fill="red" fill-opacity="0.5"/></svg>`)
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
		common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
	} {
		for _, mode := range []common.MarkdownImageMode{common.MarkdownImageModeExternal, common.MarkdownImageModeEmbedded} {
			for _, force := range []bool{false, true} {
				for _, remove := range []bool{false, true} {
					for _, optimize := range []bool{false, true} {
						name := strings.Join([]string{
							format.String(), mode.String(), strconv.FormatBool(force), strconv.FormatBool(remove), strconv.FormatBool(optimize),
						}, "/")
						t.Run(name, func(t *testing.T) {
							cfg := svgPolicyConfig()
							cfg.RasterizeSVG, cfg.RemoveTransparency, cfg.Optimize, cfg.Markdown = force, remove, optimize, mode
							book := &FictionBook{Binaries: []BinaryObject{{ID: "svg", ContentType: "image/svg+xml", Data: svg}}}
							got := book.PrepareImagesForOutput(format, cfg, zaptest.NewLogger(t))["svg"]
							jpeg := format.IsKindle() || (format == common.OutputFmtMd && mode == common.MarkdownImageModeEmbedded)
							if !force && !jpeg {
								if got.MimeType != "image/svg+xml" || !bytes.Equal(got.Data, svg) {
									t.Fatal("default SVG representation changed")
								}
								return
							}
							wantFormat := "png"
							if jpeg {
								wantFormat = "jpeg"
							}
							img, encoding, err := image.Decode(bytes.NewReader(got.Data))
							if err != nil || encoding != wantFormat || got.MimeType != "image/"+wantFormat {
								t.Fatalf("encoding = %q, MIME = %q, error = %v", encoding, got.MimeType, err)
							}
							wantW, wantH := 80, 40
							if format == common.OutputFmtPdf {
								wantW, wantH = 40, 20
							}
							if got.Dim.Width != wantW || got.Dim.Height != wantH || img.Bounds().Dx() != wantW || img.Bounds().Dy() != wantH {
								t.Fatalf("unexpected dimensions: metadata %v, pixels %v", got.Dim, img.Bounds())
							}
							r, g, b, a := img.At(wantW*7/8, wantH/2).RGBA()
							if jpeg || remove {
								if a != 0xffff || r < 0xf000 || g < 0xf000 || b < 0xf000 {
									t.Fatalf("expected white background, got %d/%d/%d/%d", r, g, b, a)
								}
							} else {
								if a != 0 {
									t.Fatalf("background alpha = %d, want 0", a)
								}
								_, _, _, a = img.At(wantW/4, wantH/2).RGBA()
								if a < 0x7f00 || a > 0x8100 {
									t.Fatalf("fill alpha = %d, want half opacity", a)
								}
							}
							if !strings.HasSuffix(got.Filename, "."+mimeToExt(got.MimeType)) {
								t.Fatalf("filename does not match MIME: %q", got.Filename)
							}
						})
					}
				}
			}
		}
	}
}

func TestSVGRasterizationSizing(t *testing.T) {
	for _, tt := range []struct {
		name  string
		cover bool
		mode  common.ImageResizeMode
		scale float64
		w, h  int
	}{
		{"body", false, common.ImageResizeModeNone, 1, 80, 40},
		{"scaled body", false, common.ImageResizeModeNone, 0.5, 40, 20},
		{"cover none", true, common.ImageResizeModeNone, 0.5, 80, 40},
		{"cover keepAR", true, common.ImageResizeModeKeepAR, 1, 240, 120},
		{"cover stretch", true, common.ImageResizeModeStretch, 1, 80, 120},
		{"cover fit", true, common.ImageResizeModeFit, 1, 80, 40},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := svgPolicyConfig()
			cfg.RasterizeSVG, cfg.ScaleFactor, cfg.Cover.Resize = true, tt.scale, tt.mode
			bo := &BinaryObject{ID: "svg", ContentType: "image/svg+xml", Data: []byte(
				`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20"><rect width="20" height="20"/></svg>`)}
			got := bo.PrepareImage(false, tt.cover, cfg, zaptest.NewLogger(t))
			if got.Dim.Width != tt.w || got.Dim.Height != tt.h {
				t.Fatalf("dimensions = %v, want %dx%d", got.Dim, tt.w, tt.h)
			}
		})
	}
}

func TestSVGRasterizationPlaceholders(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
		w, h int
	}{
		{"missing", notFoundImage, 200, 200},
		{"broken", brokenImage, 200, 200},
		{"invalid SVG", []byte(`<svg><path d="invalid"`), 200, 200},
		{"invalid raster", []byte("invalid"), 200, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := svgPolicyConfig()
			cfg.RasterizeSVG = true
			bo := &BinaryObject{ID: "image", ContentType: "image/svg+xml", Data: tt.data}
			if tt.name == "invalid raster" {
				bo.ContentType = "image/png"
			}
			got := bo.PrepareImage(false, false, cfg, zaptest.NewLogger(t))
			if got.MimeType != "image/png" || got.Dim.Width != tt.w || got.Dim.Height != tt.h {
				t.Fatalf("placeholder MIME = %q, dimensions = %v", got.MimeType, got.Dim)
			}
		})
	}
	for _, useBroken := range []bool{false, true} {
		t.Run(strconv.FormatBool(useBroken), func(t *testing.T) {
			cfg := svgPolicyConfig()
			cfg.RasterizeSVG, cfg.UseBroken = true, useBroken
			bo := &BinaryObject{ID: "image", ContentType: "image/svg+xml", Data: []byte(`<svg><path d="invalid"`)}
			got := bo.PrepareImage(false, false, cfg, zaptest.NewLogger(t))
			if bytes.Equal(got.Data, bo.Data) != useBroken {
				t.Fatalf("original bytes retained = %v, use_broken = %v", bytes.Equal(got.Data, bo.Data), useBroken)
			}
		})
	}
}

func svgPolicyConfig() *config.ImagesConfig {
	return &config.ImagesConfig{
		JPEGQuality: 90,
		ScaleFactor: 1,
		Screen:      config.ScreenConfig{Width: 80, Height: 120},
		Cover:       config.CoverConfig{Resize: common.ImageResizeModeNone},
	}
}

func TestSVGRasterizationVignettePolicy(t *testing.T) {
	for _, format := range []common.OutputFmt{common.OutputFmtEpub3, common.OutputFmtKfx, common.OutputFmtMd} {
		t.Run(format.String(), func(t *testing.T) {
			cfg := svgPolicyConfig()
			cfg.Markdown = common.MarkdownImageModeEmbedded
			book := &FictionBook{Binaries: []BinaryObject{{ID: "svg", ContentType: "image/svg+xml", Data: []byte(
				`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20">
<path d="M5 10 L35 10" stroke="black" stroke-width="1" fill="none"/></svg>`)}}}
			book.Binaries[0].BuiltinVignette = true
			legacy := book.PrepareImages(true, cfg, zaptest.NewLogger(t))["svg"]
			cfg.RasterizeSVG = true
			builtin := book.PrepareImagesForOutput(format, cfg, zaptest.NewLogger(t))["svg"]
			book.Binaries[0].BuiltinVignette = false
			external := book.PrepareImagesForOutput(format, cfg, zaptest.NewLogger(t))["svg"]
			if format == common.OutputFmtEpub3 {
				if !bytes.Equal(builtin.Data, external.Data) {
					t.Fatal("optional PNG rasterization must not thicken built-in strokes")
				}
			} else if !bytes.Equal(builtin.Data, legacy.Data) || bytes.Equal(builtin.Data, external.Data) {
				t.Fatal("legacy JPEG vignette stroke adjustment changed")
			}
		})
	}
}
