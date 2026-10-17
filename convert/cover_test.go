package convert

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	imgutil "fbc/utils/images"
)

func TestRunDefaultCover(t *testing.T) {
	for _, tt := range []struct {
		name, format, subtype string
		force, remove         bool
	}{
		{"native PDF", "pdf", "Form", false, false},
		{"forced raster PDF", "pdf", "Image", true, false},
		{"opaque PDF", "pdf", "Image", false, true},
		{"EPUB JPEG compatibility", "epub2", "", false, false},
		{"Kindle JPEG compatibility", "azw8", "", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, env := setupTestEnv(t)
			env.Cfg.Document.Images.Cover.Generate = true
			env.Cfg.Document.Images.Screen.Width = 300
			env.Cfg.Document.Images.Screen.Height = 400
			env.Cfg.Document.Images.RasterizeSVG = tt.force
			env.Cfg.Document.Images.RemoveTransparency = tt.remove
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "book.fb2"), filepath.Join(dir, "book."+tt.format)
			if err := os.WriteFile(src, []byte(minimalFB2Content), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := &cli.Command{
				Name: "convert", Action: Run,
				Flags: []cli.Flag{&cli.StringFlag{Name: "to"}, &cli.StringFlag{Name: "output-file"}},
			}
			if err := cmd.Run(ctx, []string{"convert", "--to", tt.format, "--output-file", dst, src}); err != nil {
				t.Fatal(err)
			}
			if tt.format != "pdf" {
				if !bytes.HasPrefix(env.DefaultCover, []byte{0xff, 0xd8}) {
					t.Fatal("non-PDF built-in cover must remain JPEG")
				}
				return
			}
			if !bytes.Equal(env.DefaultCover, defaultCoverSVG) {
				t.Fatal("PDF must retain the SVG source until image preparation")
			}
			data, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte("/Subtype /"+tt.subtype)) {
				t.Fatalf("PDF is missing cover resource subtype %s", tt.subtype)
			}
			if tt.subtype == "Form" && bytes.Contains(data, []byte("/Subtype /Image")) {
				t.Fatal("native default cover unexpectedly contains raster artwork")
			}
		})
	}
}

func TestDefaultCoverVectorScene(t *testing.T) {
	scene, err := imgutil.ReadVectorSVG(defaultCoverSVG)
	if err != nil {
		t.Fatal(err)
	}
	if scene.Width != 1264 || scene.Height != 1680 || len(scene.Paths) != 40 {
		t.Fatalf("unexpected default-cover scene: %gx%g, %d paths", scene.Width, scene.Height, len(scene.Paths))
	}
}

func TestRunNoCover(t *testing.T) {
	formats := []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
		common.OutputFmtPdf, common.OutputFmtTxt, common.OutputFmtMd, common.OutputFmtKfx, common.OutputFmtAzw8,
	}
	for _, format := range formats {
		t.Run(format.String(), func(t *testing.T) {
			for _, noCover := range []bool{false, true} {
				name := "without flag"
				if noCover {
					name = "with flag"
				}
				t.Run(name, func(t *testing.T) {
					ctx, env := setupTestEnv(t)
					core, logs := observer.New(zap.WarnLevel)
					env.Log = zap.New(core)
					env.Cfg.Document.Images.Screen.Width = 400
					env.Cfg.Document.Images.Screen.Height = 600
					if noCover && !format.IsKindle() {
						env.Cfg.Document.Images.Cover.Generate = true
						env.Cfg.Document.Images.Cover.DefaultImagePath = filepath.Join(t.TempDir(), "missing-cover.jpg")
					}
					cmd := &cli.Command{
						Name:   "convert",
						Action: Run,
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "to"},
							&cli.BoolFlag{Name: "no-cover"},
						},
					}
					args := []string{"convert", "--to", format.String()}
					if noCover {
						args = append(args, "--no-cover")
					}
					args = append(args, filepath.Join(t.TempDir(), "missing.fb2"))
					if err := cmd.Run(ctx, args); err == nil || !strings.Contains(err.Error(), "input source was not found") {
						t.Fatalf("expected missing-source error after option handling, got %v", err)
					}
					if got, want := env.NoCover, noCover && !format.IsKindle(); got != want {
						t.Errorf("NoCover = %v, want %v", got, want)
					}
					warnings := logs.FilterMessage("Ignoring --no-cover for Kindle output: a cover is required").All()
					wantWarnings := 0
					if noCover && format.IsKindle() {
						wantWarnings = 1
					}
					if len(warnings) != wantWarnings {
						t.Fatalf("ignored-flag warnings = %d, want %d", len(warnings), wantWarnings)
					}
					if len(warnings) > 0 && warnings[0].ContextMap()["format"] != format.String() {
						t.Errorf("warning does not identify format: %v", warnings[0].ContextMap())
					}
					if got, want := len(env.DefaultCover) > 0, format.IsKindle(); got != want {
						t.Errorf("default cover loaded = %v, want %v", got, want)
					}
				})
			}
		})
	}
}
