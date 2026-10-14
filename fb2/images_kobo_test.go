package fb2

import (
	"bytes"
	"image"
	"testing"

	"go.uber.org/zap/zaptest"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"fbc/common"
	"fbc/config"
)

func TestFictionBook_PrepareImagesForKobo_PreservesSupportedFormats(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 48, 16))
	var bmpData, tiffData bytes.Buffer
	if err := bmp.Encode(&bmpData, src); err != nil {
		t.Fatal(err)
	}
	if err := tiff.Encode(&tiffData, src, nil); err != nil {
		t.Fatal(err)
	}
	for _, output := range []common.OutputFmt{common.OutputFmtKepub, common.OutputFmtKepub3} {
		for _, input := range []struct {
			format string
			data   []byte
		}{
			{format: "jpeg", data: createTestJPEG(t, 48, 16, 85)},
			{format: "png", data: createTestPNG(t, 48, 16, true)},
			{format: "gif", data: createTestGIF(t, 48, 16, true)},
			{format: "bmp", data: bmpData.Bytes()},
			{format: "tiff", data: tiffData.Bytes()},
		} {
			t.Run(output.String()+"/"+input.format, func(t *testing.T) {
				book := &FictionBook{Binaries: []BinaryObject{{ID: "image", ContentType: "image/" + input.format, Data: input.data}}}
				bi := book.PrepareImagesForOutput(output, &config.ImagesConfig{}, zaptest.NewLogger(t))["image"]
				if bi == nil || !bytes.Equal(bi.Data, input.data) {
					t.Fatal("supported image should retain its original bytes")
				}
			})
		}
	}
}

func TestFictionBook_PrepareImagesForKobo_WebPProcessing(t *testing.T) {
	for _, output := range []common.OutputFmt{common.OutputFmtKepub, common.OutputFmtKepub3} {
		for _, mode := range []struct {
			name     string
			cover    bool
			scale    float64
			optimize bool
			wantW    int
			wantH    int
		}{
			{name: "scale", scale: 0.5, wantW: 24, wantH: 8},
			{name: "cover", cover: true, wantW: 96, wantH: 32},
			{name: "optimize", optimize: true, wantW: 48, wantH: 16},
		} {
			t.Run(output.String()+"/"+mode.name, func(t *testing.T) {
				// A misleading MIME type must not prevent conversion of decoded WebP.
				book := &FictionBook{Binaries: []BinaryObject{{ID: "image", ContentType: "image/png", Data: createTestWebP(t, true)}}}
				if mode.cover {
					book.Description.TitleInfo.Coverpage = []InlineImage{{Href: "#image"}}
				}
				cfg := &config.ImagesConfig{
					ScaleFactor: mode.scale,
					Optimize:    mode.optimize,
					Screen:      config.ScreenConfig{Width: 96, Height: 32},
					Cover:       config.CoverConfig{Resize: common.ImageResizeModeStretch},
				}
				bi := book.PrepareImagesForOutput(output, cfg, zaptest.NewLogger(t))["image"]
				if bi == nil || bi.MimeType != "image/png" || bi.Filename != "images/img00001.png" {
					t.Fatalf("expected prepared PNG, got %+v", bi)
				}
				img, format, err := image.Decode(bytes.NewReader(bi.Data))
				if err != nil {
					t.Fatal(err)
				}
				if format != "png" || img.Bounds().Dx() != mode.wantW || img.Bounds().Dy() != mode.wantH {
					t.Fatalf("unexpected image: format %s, bounds %v", format, img.Bounds())
				}
				if bi.Dim.Width != mode.wantW || bi.Dim.Height != mode.wantH {
					t.Errorf("unexpected stored dimensions: %+v", bi.Dim)
				}
				if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
					t.Error("WebP processing should preserve transparency")
				}
			})
		}
	}
}
