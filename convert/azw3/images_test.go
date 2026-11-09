package azw3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
	"fbc/convert/publication"
	"fbc/fb2"
)

const fallbackSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20">` +
	`<path d="M5 10 H35" stroke="black" stroke-width="1"/></svg>`

func imageTestConfig() *config.ImagesConfig {
	return &config.ImagesConfig{
		JPEGQuality: 95, ScaleFactor: 0.5,
		Screen: config.ScreenConfig{Width: 80, Height: 100, DPI: 300},
		Cover:  config.CoverConfig{Resize: common.ImageResizeModeStretch},
	}
}

func TestPreparePublicationImages(t *testing.T) {
	for _, optimize := range []bool{false, true} {
		for _, rasterize := range []bool{false, true} {
			t.Run(fmt.Sprintf("optimize=%t/force=%t", optimize, rasterize), func(t *testing.T) {
				cfg := imageTestConfig()
				cfg.Optimize, cfg.RasterizeSVG = optimize, rasterize
				cfg.RemoveTransparency = !rasterize
				svg := &fb2.BookImage{Filename: "images/art.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG)}
				pub := &publication.Publication{
					Images: fb2.BookImages{"art": svg},
					Stylesheet: publication.Stylesheet{Resources: []fb2.StylesheetResource{
						{Filename: "other/alias.svg", ResolvedID: "art", MimeType: "image/svg+xml", Data: []byte(fallbackSVG)},
						{Filename: "other/css.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG)},
					}},
				}
				before := bytes.Clone(svg.Data)
				prepared, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				body := prepared.Images["art"]
				if body.MimeType != "image/jpeg" || body.Filename != svg.Filename || body.Dim.Width != 40 || body.Dim.Height != 20 {
					t.Fatalf("incorrect SVG fallback: %+v", body)
				}
				for _, resource := range prepared.Stylesheet.Resources {
					if resource.MimeType != "image/jpeg" || !bytes.Equal(resource.Data, body.Data) {
						t.Error("CSS aliases/fallbacks must reuse the same raster policy")
					}
				}
				decoded, _, err := image.Decode(bytes.NewReader(body.Data))
				if err != nil || decoded.Bounds().Dx() != 40 || decoded.Bounds().Dy() != 20 {
					t.Fatalf("invalid raster: %v", err)
				}
				r, g, b, a := decoded.At(0, 0).RGBA()
				if min(r, g, b, a) < 0xf000 {
					t.Error("Kindle fallback must flatten SVG transparency onto white")
				}
				if pub.Images["art"] != svg || !bytes.Equal(svg.Data, before) || pub.Stylesheet.Resources[0].MimeType != "image/svg+xml" {
					t.Error("native image preparation mutated the shared publication")
				}
				if _, err := compileResources(context.Background(), prepared); err != nil {
					t.Fatalf("prepared SVG aliases are not valid native resources: %v", err)
				}
			})
		}
	}
}

func TestPreparePublicationImagesCoverAndVignette(t *testing.T) {
	for _, tt := range []struct {
		name    string
		cover   bool
		builtin bool
		width   int
		height  int
	}{
		{"body", false, false, 40, 20},
		{"cover", true, false, 80, 100},
		{"builtin vignette", false, true, 40, 20},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := imageTestConfig()
			cfg.RasterizeSVG = true
			pub := &publication.Publication{Images: fb2.BookImages{
				"art": {Filename: "images/art.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG), BuiltinVignette: tt.builtin},
			}}
			if tt.cover {
				pub.CoverID = "art"
			}
			got, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			binary := fb2.BinaryObject{ContentType: "image/svg+xml", Data: []byte(fallbackSVG), BuiltinVignette: tt.builtin}
			want := binary.RasterizeSVG(fb2.ImagePolicy{RequireJPEG: true, KindleSVGStrokes: true}, tt.cover, cfg, zap.NewNop())
			img := got.Images["art"]
			if img.Dim.Width != tt.width || img.Dim.Height != tt.height || !bytes.Equal(img.Data, want.Data) {
				t.Fatal("cover sizing or builtin Kindle stroke policy was lost")
			}
		})
	}
}

func TestPreparePublicationCSSRasterImages(t *testing.T) {
	cfg := imageTestConfig()
	pixels := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	pixels.Set(20, 10, color.NRGBA{R: 255, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, pixels); err != nil {
		t.Fatal(err)
	}
	binary := fb2.BinaryObject{ID: "body", ContentType: "image/png", Data: data.Bytes()}
	body := binary.PrepareImage(fb2.ImagePolicy{RequireJPEG: true}, false, cfg, zap.NewNop())
	body.Filename = "images/body.jpg"
	pub := &publication.Publication{
		Images: fb2.BookImages{"body": body},
		Stylesheet: publication.Stylesheet{Resources: []fb2.StylesheetResource{
			{Filename: "other/body.png", ResolvedID: "body", MimeType: "image/png", Data: data.Bytes()},
			{Filename: "other/css.png", MimeType: "image/png", Data: data.Bytes()},
			{Filename: "fonts/unchanged.ttf", MimeType: "font/ttf", Data: []byte{0, 1, 0, 0}},
		}},
	}
	got, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if got.Images["body"] != body || body.Dim.Width != 20 || body.Dim.Height != 10 {
		t.Error("already prepared body raster was processed twice")
	}
	for _, resource := range got.Stylesheet.Resources[:2] {
		if resource.MimeType != "image/jpeg" || !bytes.Equal(resource.Data, body.Data) {
			t.Error("CSS-only and body-aliased rasters must have identical single-pass preparation")
		}
	}
	if !reflect.DeepEqual(got.Stylesheet.Resources[2], pub.Stylesheet.Resources[2]) {
		t.Error("font resource changed during image preparation")
	}
}

func TestPreparePublicationImagesBrokenAndCancellation(t *testing.T) {
	cfg := imageTestConfig()
	cfg.RasterizeSVG = true
	pub := &publication.Publication{Images: fb2.BookImages{
		"broken": {Filename: "images/broken.svg", MimeType: "image/svg+xml", Data: []byte("<svg")},
	}}
	core, logs := observer.New(zap.DebugLevel)
	got, err := preparePublicationImages(context.Background(), pub, cfg, zap.New(core))
	if err != nil {
		t.Fatal(err)
	}
	if got.Images["broken"].MimeType != "image/jpeg" || logs.FilterMessageSnippet("broken placeholder").Len() == 0 {
		t.Error("broken SVG must use the shared warning/placeholder behavior")
	}
	cfg.UseBroken = true
	kept, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
	if err != nil || !bytes.Equal(kept.Images["broken"].Data, pub.Images["broken"].Data) {
		t.Fatalf("use_broken did not retain the original failed source: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := preparePublicationImages(ctx, pub, cfg, zap.NewNop()); !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation was not preserved: %v", err)
	}
	for _, tt := range []struct {
		name string
		pub  *publication.Publication
		cfg  *config.ImagesConfig
	}{
		{"nil publication", nil, cfg},
		{"nil configuration", pub, nil},
		{"nil image", &publication.Publication{Images: fb2.BookImages{"nil": nil}}, cfg},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := preparePublicationImages(context.Background(), tt.pub, tt.cfg, zap.NewNop()); err == nil {
				t.Error("invalid input accepted")
			}
		})
	}
	if !strings.HasPrefix(string(pub.Images["broken"].Data), "<svg") {
		t.Error("broken source data changed")
	}
}
