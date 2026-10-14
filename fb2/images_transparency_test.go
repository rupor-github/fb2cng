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

func TestFictionBook_PrepareImagesForOutput_TIFFWebPTransparency(t *testing.T) {
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

			for _, input := range []struct {
				format string
				data   []byte
			}{
				{format: "tiff", data: tiffData.Bytes()},
				{format: "webp", data: createTestWebP(t, transparent)},
			} {
				for _, mode := range []struct {
					name   string
					output common.OutputFmt
					remove bool
				}{
					{name: "epub2", output: common.OutputFmtEpub2},
					{name: "epub3", output: common.OutputFmtEpub3},
					{name: "pdf", output: common.OutputFmtPdf},
					{name: "remove", output: common.OutputFmtEpub2, remove: true},
					{name: "kfx", output: common.OutputFmtKfx},
					{name: "azw8", output: common.OutputFmtAzw8},
					{name: "kepub", output: common.OutputFmtKepub},
					{name: "kepub3", output: common.OutputFmtKepub3},
					{name: "kepub-remove", output: common.OutputFmtKepub, remove: true},
					{name: "kepub3-remove", output: common.OutputFmtKepub3, remove: true},
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
						book := &FictionBook{Binaries: []BinaryObject{*bo}}
						bi := book.PrepareImagesForOutput(mode.output, cfg, zaptest.NewLogger(t))[bo.ID]
						if bi == nil {
							t.Fatal("missing prepared image")
						}
						wantFormat := input.format
						flatten := transparent && (mode.output.IsKindle() || mode.remove)
						convertWebP := mode.output.IsKobo() && input.format == "webp"
						if mode.output.IsKindle() {
							wantFormat = "jpeg"
						} else if flatten || convertWebP {
							wantFormat = "png"
						}
						if bi.MimeType != "image/"+wantFormat {
							t.Fatalf("expected image/%s, got %s", wantFormat, bi.MimeType)
						}
						if !mode.output.IsKindle() && !flatten && !convertWebP && !bytes.Equal(bi.Data, input.data) {
							t.Error("unmodified image should retain its original bytes")
						}
						if bi.Filename != "images/img00001."+mimeToExt(bi.MimeType) {
							t.Errorf("unexpected image filename: %s", bi.Filename)
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

// createTestWebP returns a lossless 48x16 WebP with three vertical bands:
// transparent black, half-transparent red, and opaque blue (or all opaque).
// Fixtures were generated with Pillow; tests do not need an external encoder.
func createTestWebP(t *testing.T, transparent bool) []byte {
	t.Helper()
	encoded := "UklGRiwAAABXRUJQVlA4TCAAAAAvL8ADABcQ8y8gKPJ/tPkP2kBIQDjppLFyRP8DoEr1GQ=="
	if transparent {
		encoded = "UklGRjAAAABXRUJQVlA4TCQAAAAvL8ADEBcQ8y8gKPJ/NAFB0XXLBfyCkIBw0klj5Yj+B0CV6jM="
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
