package convert

import (
	"path/filepath"
	"strings"
	"testing"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
)

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
					if noCover && !format.ForKindle() {
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
					if got, want := env.NoCover, noCover && !format.ForKindle(); got != want {
						t.Errorf("NoCover = %v, want %v", got, want)
					}
					warnings := logs.FilterMessage("Ignoring --no-cover for Kindle output: a cover is required").All()
					wantWarnings := 0
					if noCover && format.ForKindle() {
						wantWarnings = 1
					}
					if len(warnings) != wantWarnings {
						t.Fatalf("ignored-flag warnings = %d, want %d", len(warnings), wantWarnings)
					}
					if len(warnings) > 0 && warnings[0].ContextMap()["format"] != format.String() {
						t.Errorf("warning does not identify format: %v", warnings[0].ContextMap())
					}
					if got, want := len(env.DefaultCover) > 0, format.ForKindle(); got != want {
						t.Errorf("default cover loaded = %v, want %v", got, want)
					}
				})
			}
		})
	}
}
