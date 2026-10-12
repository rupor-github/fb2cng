package convert

import (
	"context"
	"testing"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
)

func TestKFXCompressionPrecedence(t *testing.T) {
	for _, test := range []struct {
		name, configured, override, want, source string
		format                                   common.OutputFmt
		invalid, warning                         bool
	}{
		{"default", "none", "", "none", "default", common.OutputFmtKfx, false, false},
		{"configured", "lzma", "", "lzma", "configuration", common.OutputFmtAzw8, false, false},
		{"enable", "none", "lzma", "lzma", "cli", common.OutputFmtKfx, false, false},
		{"disable", "lzma", "none", "none", "cli", common.OutputFmtAzw8, false, false},
		{"invalid", "none", "xz", "none", "", common.OutputFmtKfx, true, false},
		{"ignored", "none", "xz", "none", "", common.OutputFmtPdf, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DocumentConfig{KFX: config.KFXConfig{Compression: test.configured}}
			core, observed := observer.New(zap.DebugLevel)
			app := &cli.Command{
				Flags: []cli.Flag{&cli.StringFlag{Name: "kfx-compression"}, &cli.StringFlag{Name: "config"}},
				Action: func(_ context.Context, cmd *cli.Command) error {
					return configureKFXCompression(cmd, test.format, &cfg, zap.New(core))
				},
			}
			args := []string{"test"}
			if test.name == "configured" {
				args = append(args, "--config", "settings.yaml")
			}
			if test.override != "" {
				args = append(args, "--kfx-compression", test.override)
			}
			err := app.Run(t.Context(), args)
			if (err != nil) != test.invalid {
				t.Fatal(err)
			}
			if cfg.KFX.Compression != test.want || cfg.KFX.CompressionSource != test.source {
				t.Fatalf("got %+v", cfg.KFX)
			}
			if test.warning && observed.FilterLevelExact(zap.WarnLevel).Len() != 1 {
				t.Fatal("missing ignored-option warning")
			}
		})
	}
}
