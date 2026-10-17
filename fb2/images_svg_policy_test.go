package fb2

import (
	"bytes"
	"image"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
)

func TestExplicitSVGRasterizationLogging(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(strconv.FormatBool(force), func(t *testing.T) {
			cfg := svgPolicyConfig()
			cfg.RasterizeSVG = force
			book := &FictionBook{Binaries: []BinaryObject{{ID: "art", ContentType: "image/svg+xml", Data: []byte(
				`<svg viewBox="0 0 40 20"><rect width="20" height="20"/></svg>`)}}}
			core, logs := observer.New(zapcore.DebugLevel)
			book.PrepareImages(common.OutputFmtPdf, cfg, zap.New(core))
			entries := logs.FilterMessage("Rasterizing SVG as requested by configuration").All()
			if !force {
				if len(entries) != 0 {
					t.Fatalf("unexpected explicit rasterization log: %+v", entries)
				}
				return
			}
			if len(entries) != 1 || entries[0].Level != zapcore.DebugLevel {
				t.Fatalf("want one debug log, got %+v", entries)
			}
			fields := entries[0].ContextMap()
			if fields["id"] != "art" || fields["rasterize_svg"] != true {
				t.Fatalf("missing explicit rasterization context: %+v", fields)
			}
		})
	}
}

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
							got := book.PrepareImages(format, cfg, zaptest.NewLogger(t))["svg"]
							jpeg := format.IsKindle() || (format == common.OutputFmtMd && mode == common.MarkdownImageModeEmbedded)
							if !force && (!jpeg || (format.IsKindle() && !remove)) {
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
			got := bo.PrepareImage(common.OutputFmtEpub2, tt.cover, cfg, zaptest.NewLogger(t))
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
			got := bo.PrepareImage(common.OutputFmtEpub2, false, cfg, zaptest.NewLogger(t))
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
			got := bo.PrepareImage(common.OutputFmtEpub2, false, cfg, zaptest.NewLogger(t))
			if bytes.Equal(got.Data, bo.Data) != useBroken {
				t.Fatalf("original bytes retained = %v, use_broken = %v", bytes.Equal(got.Data, bo.Data), useBroken)
			}
		})
	}
}

func TestKindlePlaceholderSVGPolicy(t *testing.T) {
	for _, format := range []common.OutputFmt{common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtMd} {
		t.Run(format.String(), func(t *testing.T) {
			for _, input := range []struct {
				name, mime string
				data       []byte
			}{
				{"missing", "image/svg+xml", notFoundImage},
				{"broken", "image/svg+xml", brokenImage},
				{"invalid PNG", "image/png", []byte("invalid PNG")},
				{"invalid SVG", "image/svg+xml", []byte(`<svg><path d="invalid"`)},
			} {
				for _, policy := range []struct {
					name                 string
					force, remove, cover bool
				}{
					{"native", false, false, false},
					{"forced raster", true, false, false},
					{"remove transparency", false, true, false},
					{"cover", false, false, true},
				} {
					t.Run(input.name+"/"+policy.name, func(t *testing.T) {
						cfg := svgPolicyConfig()
						cfg.Screen.Width = 8000
						cfg.Markdown = common.MarkdownImageModeEmbedded
						cfg.RasterizeSVG, cfg.RemoveTransparency = policy.force, policy.remove
						invalid := strings.HasPrefix(input.name, "invalid")
						bo := BinaryObject{ID: "art", ContentType: input.mime, Data: input.data, BuiltinVignette: invalid}
						got := bo.PrepareImage(format, policy.cover, cfg, zaptest.NewLogger(t))
						wantMIME := "image/jpeg"
						if format.IsKindle() && !policy.force && !policy.remove && !policy.cover {
							wantMIME = "image/svg+xml"
							if !bytes.Equal(got.Data, notFoundImage) && !bytes.Equal(got.Data, brokenImage) {
								t.Fatal("native placeholder must retain its original embedded SVG")
							}
						}
						if got.MimeType != wantMIME || got.Dim.Width != 200 || got.Dim.Height != 200 {
							t.Fatalf("placeholder = %s %v, want %s 200x200", got.MimeType, got.Dim, wantMIME)
						}
						if invalid && got.BuiltinVignette {
							t.Fatal("a substituted placeholder must not inherit vignette stroke thickening")
						}
						if invalid {
							cfg.UseBroken = true
							kept := bo.PrepareImage(format, policy.cover, cfg, zaptest.NewLogger(t))
							if !bytes.Equal(kept.Data, input.data) {
								t.Fatal("use_broken must retain the original invalid bytes")
							}
						}
					})
				}
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
	for _, rasterFormat := range []common.RasterFormat{common.RasterFormatAuto, common.RasterFormatJpeg} {
		t.Run(rasterFormat.String(), func(t *testing.T) {
			for _, format := range []common.OutputFmt{common.OutputFmtEpub3, common.OutputFmtKfx, common.OutputFmtMd} {
				t.Run(format.String(), func(t *testing.T) {
					cfg := svgPolicyConfig()
					cfg.RasterFormat = rasterFormat
					cfg.Markdown = common.MarkdownImageModeEmbedded
					book := &FictionBook{Binaries: []BinaryObject{{ID: "svg", ContentType: "image/svg+xml", Data: []byte(
						`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20">
<path d="M5 10 L35 10" stroke="black" stroke-width="1" fill="none"/></svg>`)}}}
					book.Binaries[0].BuiltinVignette = true
					legacy := book.Binaries[0].RasterizeSVG(common.OutputFmtKfx, false, cfg, zaptest.NewLogger(t))
					cfg.RasterizeSVG = true
					builtin := book.PrepareImages(format, cfg, zaptest.NewLogger(t))["svg"]
					book.Binaries[0].BuiltinVignette = false
					external := book.PrepareImages(format, cfg, zaptest.NewLogger(t))["svg"]
					if format == common.OutputFmtEpub3 {
						if !bytes.Equal(builtin.Data, external.Data) {
							t.Fatal("optional rasterization must not thicken built-in strokes")
						}
					} else if !bytes.Equal(builtin.Data, legacy.Data) || bytes.Equal(builtin.Data, external.Data) {
						t.Fatal("legacy JPEG vignette stroke adjustment changed")
					}
				})
			}
		})
	}
}
