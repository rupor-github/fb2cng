package fb2

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
)

func TestBaselineJPEG(t *testing.T) {
	progressive, err := os.ReadFile("testdata/images-progressive.jpg")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name string
		data []byte
	}{{"progressive", progressive}, {"baseline", createTestJPEG(t, 16, 8, 80)}} {
		for _, enabled := range []bool{false, true} {
			for _, cover := range []bool{false, true} {
				for _, scale := range []float64{1, 0.5} {
					for _, output := range []common.OutputFmt{
						common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
						common.OutputFmtAzw3, common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtMd,
					} {
						name := fmt.Sprintf("%s/%s/enabled=%t/cover=%t/scale=%g", input.name, output, enabled, cover, scale)
						t.Run(name, func(t *testing.T) {
							cfg := &config.ImagesConfig{
								BaselineJPEG: enabled, JPEGQuality: 95, ScaleFactor: scale,
								Screen: config.ScreenConfig{Width: 53, Height: 80},
							}
							bo := BinaryObject{ID: "jpeg", ContentType: "image/jpeg", Data: input.data}
							core, logs := observer.New(zap.DebugLevel)
							got := bo.PrepareImage(ImagePolicyFor(output, common.MarkdownImageModeExternal), cover, cfg, zap.New(core))
							resized := !cover && scale != 1
							converted := enabled && input.name == "progressive"
							if !resized && !converted {
								if !bytes.Equal(got.Data, input.data) {
									t.Fatal("unnecessary JPEG re-encoding")
								}
							} else {
								if !isBaselineJPEG(got.Data) {
									t.Fatal("result must use baseline encoding")
								}
								src, _, err := image.Decode(bytes.NewReader(input.data))
								if err != nil {
									t.Fatal(err)
								}
								if resized {
									src = imaging.Resize(src, 8, 4, imaging.Linear)
								}
								want, err := bo.encodeImage(src, "jpeg", cfg, zap.NewNop())
								if err != nil {
									t.Fatal(err)
								}
								if !bytes.Equal(got.Data, want) {
									t.Fatal("baseline conversion and resizing must share one encoding pass")
								}
							}
							wantLogs := 0
							if converted {
								wantLogs = 1
							}
							if logs.FilterMessage("Converting non-baseline JPEG to baseline").Len() != wantLogs {
								t.Fatal("forced baseline conversion log mismatch")
							}
						})
					}
				}
			}
		}
	}
}

func TestBaselineJPEGBrokenImages(t *testing.T) {
	for _, useBroken := range []bool{false, true} {
		t.Run(fmt.Sprint(useBroken), func(t *testing.T) {
			data := []byte("broken jpeg")
			bo := BinaryObject{ID: "broken", ContentType: "image/jpeg", Data: data}
			cfg := &config.ImagesConfig{BaselineJPEG: true, UseBroken: useBroken}
			got := bo.PrepareImage(ImagePolicyFor(common.OutputFmtEpub3, common.MarkdownImageModePlaceholder), false, cfg, zap.NewNop())
			if useBroken && !bytes.Equal(got.Data, data) {
				t.Fatal("broken original must be retained when requested")
			}
			if !useBroken && got.MimeType != "image/svg+xml" {
				t.Fatal("broken JPEG must use the normal placeholder policy")
			}
		})
	}
}

func TestBaselineJPEGMarkers(t *testing.T) {
	baseline := createTestJPEG(t, 16, 8, 80)
	// An embedded thumbnail with SOF0 must not hide a progressive frame.
	withThumbnail := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, 4, 0xff, 0xc0}, baseline[2:]...)
	for _, tt := range []struct {
		name string
		data []byte
		want bool
	}{
		{"baseline", baseline, true},
		{"APP payload", withThumbnail, true},
		{"empty", nil, false},
		{"truncated SOI", []byte{0xff}, false},
		{"truncated marker", []byte{0xff, 0xd8, 0xff}, false},
		{"invalid length", []byte{0xff, 0xd8, 0xff, 0xe1, 0, 1}, false},
		{"truncated APP", []byte{0xff, 0xd8, 0xff, 0xe1, 0, 8}, false},
		{"truncated SOF", []byte{0xff, 0xd8, 0xff, 0xc0}, false},
		{"progressive after thumbnail", []byte{0xff, 0xd8, 0xff, 0xe1, 0, 4, 0xff, 0xc0, 0xff, 0xc2, 0, 2}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBaselineJPEG(tt.data); got != tt.want {
				t.Fatalf("baseline = %t, want %t", got, tt.want)
			}
		})
	}
}
