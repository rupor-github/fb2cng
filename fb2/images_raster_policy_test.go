package fb2

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"fbc/common"
	"fbc/config"
)

func TestRasterFormatJPEG(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		for _, input := range rasterPolicyInputs(t, transparent) {
			for _, output := range []common.OutputFmt{
				common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
				common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
			} {
				for _, optimize := range []bool{false, true} {
					for _, cover := range []bool{false, true} {
						name := fmt.Sprintf("%s/%s/alpha=%t/optimize=%t/cover=%t", output, input.name, transparent, optimize, cover)
						t.Run(name, func(t *testing.T) {
							core, logs := observer.New(zap.DebugLevel)
							cfg := &config.ImagesConfig{
								RasterFormat: common.RasterFormatJpeg, Optimize: optimize, JPEGQuality: 100, ScaleFactor: 1,
								Markdown: common.MarkdownImageModeExternal, Screen: config.ScreenConfig{Width: 96, Height: 64, DPI: 300},
							}
							bo := BinaryObject{ID: "art", ContentType: "image/" + input.name, Data: input.data}
							got := bo.PrepareImage(output, cover, cfg, zap.New(core))
							img, format, err := image.Decode(bytes.NewReader(got.Data))
							if err != nil || format != "jpeg" || got.MimeType != "image/jpeg" {
								t.Fatalf("format=%s, MIME=%s, error=%v", format, got.MimeType, err)
							}
							src, _, err := image.Decode(bytes.NewReader(input.data))
							if err != nil {
								t.Fatal(err)
							}
							if img.Bounds() != src.Bounds() || got.Dim.Width != 48 || got.Dim.Height != 16 {
								t.Fatalf("encoding changed dimensions: %v / %v", img.Bounds(), got.Dim)
							}
							for _, x := range []int{8, 24, 40} {
								r, g, b, a := src.At(x, 8).RGBA()
								want := []uint32{(r + 0xffff - a) >> 8, (g + 0xffff - a) >> 8, (b + 0xffff - a) >> 8}
								actual := color.NRGBAModel.Convert(img.At(x, 8)).(color.NRGBA)
								for i, channel := range []uint8{actual.R, actual.G, actual.B} {
									if difference := int(channel) - int(want[i]); difference < -3 || difference > 3 {
										t.Fatalf("pixel %d: got %v, want RGB %v", x, actual, want)
									}
								}
							}
							if logs.FilterMessage("Converting WebP image to PNG for Kobo output").Len() != 0 {
								t.Fatal("explicit JPEG must take precedence over Kobo PNG conversion")
							}
						})
					}
				}
			}
		}
	}
}

func TestRasterFormatJPEGKeepsExistingJPEGPolicy(t *testing.T) {
	for _, quality := range []int{50, 100} {
		for _, optimize := range []bool{false, true} {
			t.Run(fmt.Sprintf("quality=%d/optimize=%t", quality, optimize), func(t *testing.T) {
				data := createTestJPEG(t, 48, 16, quality)
				bo := BinaryObject{ID: "jpeg", ContentType: "image/jpeg", Data: data}
				cfg := &config.ImagesConfig{JPEGQuality: 75, Optimize: optimize, ScaleFactor: 1}
				before := bo.PrepareImage(common.OutputFmtEpub3, false, cfg, zap.NewNop())
				cfg.RasterFormat = common.RasterFormatJpeg
				after := bo.PrepareImage(common.OutputFmtEpub3, false, cfg, zap.NewNop())
				if !bytes.Equal(before.Data, after.Data) {
					t.Fatal("selecting JPEG changed existing JPEG optimization behavior")
				}
				if (!optimize || quality == 50) && !bytes.Equal(after.Data, data) {
					t.Fatal("existing JPEG was unnecessarily re-encoded")
				}
				if optimize && quality == 100 && bytes.Equal(after.Data, data) {
					t.Fatal("explicit JPEG must not disable quality optimization")
				}
			})
		}
	}
}

func TestRasterFormatJPEGResizeAndQuality(t *testing.T) {
	for _, input := range rasterPolicyInputs(t, false) {
		for _, tt := range []struct {
			name string
			mode common.ImageResizeMode
			w, h int
		}{
			{"body", common.ImageResizeModeNone, 24, 8},
			{"cover none", common.ImageResizeModeNone, 48, 16},
			{"cover keepAR", common.ImageResizeModeKeepAR, 192, 64},
			{"cover stretch", common.ImageResizeModeStretch, 96, 64},
			{"cover fit", common.ImageResizeModeFit, 96, 32},
		} {
			t.Run(input.name+"/"+tt.name, func(t *testing.T) {
				cfg := &config.ImagesConfig{
					RasterFormat: common.RasterFormatJpeg, JPEGQuality: 75, ScaleFactor: 0.5,
					Screen: config.ScreenConfig{Width: 96, Height: 64, DPI: 300}, Cover: config.CoverConfig{Resize: tt.mode},
				}
				bo := BinaryObject{ID: "art", ContentType: "image/" + input.name, Data: input.data}
				got := bo.PrepareImage(common.OutputFmtEpub3, tt.name != "body", cfg, zap.NewNop())
				if got.MimeType != "image/jpeg" || got.Dim.Width != tt.w || got.Dim.Height != tt.h {
					t.Fatalf("unexpected resized image: %s, %v", got.MimeType, got.Dim)
				}
			})
		}
	}
	bo := BinaryObject{ID: "png", ContentType: "image/png", Data: createTestPNG(t, 48, 16, false)}
	cfg := &config.ImagesConfig{RasterFormat: common.RasterFormatJpeg, JPEGQuality: 40}
	low := bo.PrepareImage(common.OutputFmtEpub3, false, cfg, zap.NewNop())
	cfg.JPEGQuality = 100
	high := bo.PrepareImage(common.OutputFmtEpub3, false, cfg, zap.NewNop())
	if bytes.Equal(low.Data, high.Data) {
		t.Fatal("new JPEG encoding must honor configured quality even without optimization")
	}
}

func TestRasterFormatJPEGDoesNotForceSVG(t *testing.T) {
	svg := []byte(`<svg viewBox="0 0 40 20"><rect width="20" height="20" fill="red"/></svg>`)
	for _, output := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
		common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
	} {
		for _, force := range []bool{false, true} {
			for _, cover := range []bool{false, true} {
				name := fmt.Sprintf("%s/force=%t/cover=%t", output, force, cover)
				t.Run(name, func(t *testing.T) {
					cfg := &config.ImagesConfig{
						RasterFormat: common.RasterFormatJpeg, RasterizeSVG: force, JPEGQuality: 100,
						Markdown: common.MarkdownImageModeExternal, Screen: config.ScreenConfig{Width: 80, Height: 120},
					}
					bo := BinaryObject{ID: "svg", ContentType: "image/svg+xml", Data: svg}
					got := bo.PrepareImage(output, cover, cfg, zap.NewNop())
					if !force && !(output.IsKindle() && cover) {
						if got.MimeType != "image/svg+xml" || !bytes.Equal(got.Data, svg) {
							t.Fatal("JPEG raster encoding must not force SVG rasterization")
						}
						return
					}
					img, format, err := image.Decode(bytes.NewReader(got.Data))
					if err != nil || format != "jpeg" || got.MimeType != "image/jpeg" {
						t.Fatalf("SVG raster result: %s, %s, %v", format, got.MimeType, err)
					}
					r, g, b, _ := img.At(img.Bounds().Dx()*7/8, img.Bounds().Dy()/2).RGBA()
					if min(r, g, b) < 0xf000 {
						t.Fatal("JPEG SVG rasterization must use white instead of transparency")
					}
				})
			}
		}
	}
}

func TestRasterFormatJPEGBrokenImages(t *testing.T) {
	for _, force := range []bool{false, true} {
		for _, useBroken := range []bool{false, true} {
			t.Run(fmt.Sprintf("force=%t/use_broken=%t", force, useBroken), func(t *testing.T) {
				data := []byte("broken raster")
				bo := BinaryObject{ID: "broken", ContentType: "image/png", Data: data}
				cfg := &config.ImagesConfig{
					RasterFormat: common.RasterFormatJpeg, RasterizeSVG: force, UseBroken: useBroken, JPEGQuality: 95,
				}
				got := bo.PrepareImage(common.OutputFmtEpub3, false, cfg, zap.NewNop())
				if useBroken {
					if !bytes.Equal(got.Data, data) {
						t.Fatal("use_broken must retain invalid original bytes")
					}
				} else if !force {
					if got.MimeType != "image/svg+xml" || !bytes.Equal(got.Data, brokenImage) {
						t.Fatal("JPEG encoding alone must not rasterize a vector placeholder")
					}
				} else if got.MimeType != "image/jpeg" || got.Dim.Width != 200 || got.Dim.Height != 200 {
					t.Fatalf("unexpected raster placeholder: %s %v", got.MimeType, got.Dim)
				}
			})
		}
	}
}

func rasterPolicyInputs(t *testing.T, transparent bool) []struct {
	name string
	data []byte
} {
	t.Helper()
	src := image.NewNRGBA(image.Rect(0, 0, 48, 16))
	for y := range 16 {
		for x := range 48 {
			c := color.NRGBA{B: 255, A: 255}
			if x < 16 {
				c = color.NRGBA{}
			} else if x < 32 {
				c = color.NRGBA{R: 255, A: 128}
			}
			if !transparent {
				c.A = 255
			}
			src.SetNRGBA(x, y, c)
		}
	}
	var inputs []struct {
		name string
		data []byte
	}
	for _, encoder := range []struct {
		name   string
		encode func(io.Writer) error
	}{
		{"jpeg", func(w io.Writer) error { return jpeg.Encode(w, src, &jpeg.Options{Quality: 100}) }},
		{"png", func(w io.Writer) error { return png.Encode(w, src) }},
		{"gif", func(w io.Writer) error { return gif.Encode(w, src, nil) }},
		{"bmp", func(w io.Writer) error { return bmp.Encode(w, src) }},
		{"tiff", func(w io.Writer) error { return tiff.Encode(w, src, nil) }},
	} {
		var data bytes.Buffer
		if err := encoder.encode(&data); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, struct {
			name string
			data []byte
		}{encoder.name, data.Bytes()})
	}
	return append(inputs, struct {
		name string
		data []byte
	}{"webp", createTestWebP(t, transparent)})
}
