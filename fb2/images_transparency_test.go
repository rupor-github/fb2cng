package fb2

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"testing"

	"go.uber.org/zap/zaptest"
	"golang.org/x/image/tiff"

	"fbc/common"
	"fbc/config"
)

func TestBinaryObject_PrepareImage_TIFFWebPTransparency(t *testing.T) {
	for _, transparent := range []bool{true, false} {
		name := "opaque"
		if transparent {
			name = "transparent"
		}
		t.Run(name, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, 48, 16))
			for y := range 16 {
				for x := range 48 {
					var c color.NRGBA
					switch {
					case x < 16:
						c = color.NRGBA{}
					case x < 32:
						c = color.NRGBA{R: 255, A: 128}
					default:
						c = color.NRGBA{B: 255, A: 255}
					}
					if !transparent {
						c.A = 255
					}
					src.SetNRGBA(x, y, c)
				}
			}
			var tiffData bytes.Buffer
			if err := tiff.Encode(&tiffData, src, nil); err != nil {
				t.Fatal(err)
			}

			// Lossless WebP fixtures with the same three bands as src, generated
			// with Pillow. No external encoder is needed to run these tests.
			webpBase64 := "UklGRiwAAABXRUJQVlA4TCAAAAAvL8ADABcQ8y8gKPJ/tPkP2kBIQDjppLFyRP8DoEr1GQ=="
			if transparent {
				webpBase64 = "UklGRjAAAABXRUJQVlA4TCQAAAAvL8ADEBcQ8y8gKPJ/NAFB0XXLBfyCkIBw0klj5Yj+B0CV6jM="
			}
			webpData, err := base64.StdEncoding.DecodeString(webpBase64)
			if err != nil {
				t.Fatal(err)
			}

			for _, input := range []struct {
				format string
				data   []byte
			}{
				{format: "tiff", data: tiffData.Bytes()},
				{format: "webp", data: webpData},
			} {
				for _, mode := range []struct {
					name   string
					kindle bool
					remove bool
				}{
					{name: "preserve"},
					{name: "remove", remove: true},
					{name: "kindle", kindle: true},
				} {
					t.Run(input.format+"/"+mode.name, func(t *testing.T) {
						cfg := &config.ImagesConfig{
							JPEGQuality:        100,
							RemoveTransparency: mode.remove,
							ScaleFactor:        1,
							Screen:             config.ScreenConfig{Width: 1600, Height: 2560},
							Cover:              config.CoverConfig{Resize: common.ImageResizeModeNone},
						}
						bo := &BinaryObject{ID: "test-" + input.format, ContentType: "image/" + input.format, Data: input.data}
						bi := bo.PrepareImage(mode.kindle, false, cfg, zaptest.NewLogger(t))
						wantFormat := input.format
						flatten := transparent && (mode.kindle || mode.remove)
						if mode.kindle {
							wantFormat = "jpeg"
						} else if flatten {
							wantFormat = "png"
						}
						if bi.MimeType != "image/"+wantFormat {
							t.Fatalf("expected image/%s, got %s", wantFormat, bi.MimeType)
						}
						if !mode.kindle && !flatten && !bytes.Equal(bi.Data, input.data) {
							t.Error("unmodified image should retain its original bytes")
						}
						got, format, err := image.Decode(bytes.NewReader(bi.Data))
						if err != nil {
							t.Fatal(err)
						}
						if format != wantFormat || got.Bounds() != src.Bounds() {
							t.Fatalf("unexpected output: format %s, bounds %v", format, got.Bounds())
						}
						for _, x := range []int{8, 24, 40} {
							want := src.NRGBAAt(x, 8)
							if flatten {
								switch x {
								case 8:
									want = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
								case 24:
									want = color.NRGBA{R: 255, G: 127, B: 127, A: 255}
								}
							}
							pixel := color.NRGBAModel.Convert(got.At(x, 8)).(color.NRGBA)
							for i, pair := range [][2]uint8{{pixel.R, want.R}, {pixel.G, want.G}, {pixel.B, want.B}, {pixel.A, want.A}} {
								diff := int(pair[0]) - int(pair[1])
								// Allow minor JPEG rounding differences.
								if diff < -2 || diff > 2 {
									t.Errorf("pixel (%d, 8), channel %d: got %d, want %d", x, i, pair[0], pair[1])
								}
							}
						}
					})
				}
			}
		})
	}
}
