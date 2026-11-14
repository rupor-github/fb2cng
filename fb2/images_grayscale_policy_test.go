package fb2

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	imgutil "fbc/utils/images"
)

func TestGrayscaleRasterPolicy(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		for _, input := range rasterPolicyInputs(t, transparent) {
			for _, output := range []common.OutputFmt{
				common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
				common.OutputFmtAzw3, common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
			} {
				for _, cover := range []bool{false, true} {
					for _, optimize := range []bool{false, true} {
						name := fmt.Sprintf("%s/%s/alpha=%t/cover=%t/optimize=%t", output, input.name, transparent, cover, optimize)
						t.Run(name, func(t *testing.T) {
							cfg := &config.ImagesConfig{
								Grayscale: common.GrayscaleModeLuma, Optimize: optimize, JPEGQuality: 100, ScaleFactor: 1,
								Screen: config.ScreenConfig{Width: 96, Height: 64, DPI: 300},
							}
							policy := ImagePolicyFor(output, common.MarkdownImageModeExternal)
							bo := BinaryObject{ID: "art", ContentType: "image/" + input.name, Data: input.data}
							got := bo.PrepareImage(policy, cover, cfg, zap.NewNop())
							img, encoding, err := image.Decode(bytes.NewReader(got.Data))
							if err != nil {
								t.Fatal(err)
							}
							wantEncoding := "png"
							if policy.RequireJPEG || input.name == "jpeg" {
								wantEncoding = "jpeg"
							}
							if encoding != wantEncoding || got.MimeType != "image/"+wantEncoding {
								t.Fatalf("got %s/%s, want %s", encoding, got.MimeType, wantEncoding)
							}
							src, _, err := image.Decode(bytes.NewReader(input.data))
							if err != nil {
								t.Fatal(err)
							}
							if img.Bounds() != src.Bounds() {
								t.Fatal("grayscale conversion changed dimensions")
							}
							if encoding == "jpeg" {
								if _, ok := img.(*image.Gray); !ok {
									t.Fatalf("expected single-channel grayscale JPEG, got %T", img)
								}
							}
							for _, x := range []int{8, 24, 40} {
								c := color.NRGBAModel.Convert(src.At(x, 8)).(color.NRGBA)
								var want uint8
								if encoding == "jpeg" {
									r, g, b, a := src.At(x, 8).RGBA()
									c.A = 255
									want = color.GrayModel.Convert(color.RGBA64{
										R: uint16(r + 0xffff - a), G: uint16(g + 0xffff - a),
										B: uint16(b + 0xffff - a), A: 0xffff,
									}).(color.Gray).Y
								} else {
									want = color.GrayModel.Convert(color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}).(color.Gray).Y
								}
								actual := color.NRGBAModel.Convert(img.At(x, 8)).(color.NRGBA)
								if actual.A != c.A || actual.R != actual.G || actual.G != actual.B {
									t.Fatalf("got %+v, want gray with alpha %d", actual, c.A)
								}
								delta := int(actual.R) - int(want)
								if c.A != 0 && (delta < -2 || delta > 2) {
									t.Fatalf("pixel %d: gray = %d, want %d", x, actual.R, want)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestGrayscalePNG16Transparency(t *testing.T) {
	src := image.NewNRGBA64(image.Rect(0, 0, 3, 1))
	for x, a := range []uint16{1, 0x1234, 0x8000} {
		src.SetNRGBA64(x, 0, color.NRGBA64{R: 0xf123, G: 0x5123, B: 0x2123, A: a})
	}
	var data bytes.Buffer
	if err := png.Encode(&data, src); err != nil {
		t.Fatal(err)
	}
	bo := BinaryObject{ID: "png16", ContentType: "image/png", Data: data.Bytes()}
	cfg := &config.ImagesConfig{Grayscale: common.GrayscaleModeLuma}
	got := bo.PrepareImage(ImagePolicyFor(common.OutputFmtEpub3, common.MarkdownImageModePlaceholder), false, cfg, zap.NewNop())
	decoded, err := png.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatal(err)
	}
	want := imgutil.GrayscaleLuma(src)
	for x := range 3 {
		actual := color.NRGBA64Model.Convert(decoded.At(x, 0)).(color.NRGBA64)
		expected := color.NRGBA64Model.Convert(want.At(x, 0)).(color.NRGBA64)
		if actual != expected {
			t.Fatalf("pixel %d: got %+v, want %+v", x, actual, expected)
		}
	}
}

func TestGrayscaleCombinedProcessing(t *testing.T) {
	data := createTestPNG(t, 120, 150, true)
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.ImagesConfig{
		Grayscale: common.GrayscaleModeLuma, BaselineJPEG: true, FitToScreen: true, ScaleFactor: 0.5,
		RasterFormat: common.RasterFormatJpeg, JPEGQuality: 85,
		Screen: config.ScreenConfig{Width: 53, Height: 80, DPI: 300},
	}
	bo := BinaryObject{ID: "combined", ContentType: "image/png", Data: data}
	got := bo.PrepareImage(ImagePolicyFor(common.OutputFmtEpub3, common.MarkdownImageModePlaceholder), false, cfg, zap.NewNop())
	resized := imaging.Resize(src, 53, 66, imaging.Linear)
	opaque := image.NewRGBA(resized.Bounds())
	draw.Draw(opaque, opaque.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), resized, resized.Bounds().Min, draw.Over)
	want, err := bo.encodeImage(opaque, "jpeg", cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, want) {
		t.Fatal("combined policies must use one resize and encoding pass")
	}
}

func TestGrayscaleSVGPolicy(t *testing.T) {
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20"><rect width="40" height="20" fill="red"/></svg>`)
	for _, output := range []common.OutputFmt{
		common.OutputFmtEpub3, common.OutputFmtKepub3, common.OutputFmtAzw3, common.OutputFmtKfx,
		common.OutputFmtPdf, common.OutputFmtMd,
	} {
		for _, force := range []bool{false, true} {
			for _, cover := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/raster=%t/cover=%t", output, force, cover), func(t *testing.T) {
					cfg := &config.ImagesConfig{
						Grayscale: common.GrayscaleModeLuma, RasterizeSVG: force, JPEGQuality: 95,
						Screen: config.ScreenConfig{Width: 80, Height: 120},
					}
					bo := BinaryObject{ID: "svg", ContentType: "image/svg+xml", Data: data}
					got := bo.PrepareImage(ImagePolicyFor(output, common.MarkdownImageModeExternal), cover, cfg, zap.NewNop())
					if !force && !(cover && output.IsKindle()) {
						if !bytes.Equal(data, got.Data) {
							t.Fatal("grayscale must not change native vector artwork")
						}
						return
					}
					img, _, err := image.Decode(bytes.NewReader(got.Data))
					if err != nil {
						t.Fatal(err)
					}
					r, g, b, _ := img.At(10, 10).RGBA()
					if r != g || g != b {
						t.Fatal("SVG raster output must be grayscale")
					}
				})
			}
		}
	}
}
