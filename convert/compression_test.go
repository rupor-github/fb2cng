package convert

import (
	"context"
	"strings"
	"testing"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
)

func TestCompressionPrecedence(t *testing.T) {
	for _, profile := range []struct {
		format                       common.OutputFmt
		initial, configured, invalid string
		modes                        []string
	}{
		{common.OutputFmtKfx, "none", "lzma", "huffdic", []string{"none", "lzma"}},
		{common.OutputFmtAzw8, "none", "lzma", "standard", []string{"none", "lzma"}},
		{common.OutputFmtAzw3, "standard", "huffdic", "lzma", []string{"none", "standard", "huffdic"}},
	} {
		t.Run(profile.format.String(), func(t *testing.T) {
			type testCase struct {
				name, configured, override, want, source string
				configFile, supplied, invalid            bool
			}
			tests := []testCase{
				{"default", profile.initial, "", profile.initial, "default", false, false, false},
				{"configured", profile.configured, "", profile.configured, "configuration", true, false, false},
				{"other format mode", profile.initial, profile.invalid, profile.initial, "", false, true, true},
				{"unknown mode", profile.initial, "xz", profile.initial, "", false, true, true},
				{"empty mode", profile.initial, "", profile.initial, "", false, true, true},
				{"case-sensitive mode", profile.initial, "NONE", profile.initial, "", false, true, true},
			}
			for _, mode := range profile.modes {
				tests = append(tests, testCase{"override/" + mode, profile.configured, mode, mode, "cli", true, true, false})
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					cfg := config.DocumentConfig{
						KFX:  config.KFXConfig{Compression: "lzma"},
						AZW3: config.AZW3Config{Compression: "huffdic", APNX: "eink"},
					}
					compression, source, target := &cfg.KFX.Compression, &cfg.KFX.CompressionSource, "KFX"
					if profile.format == common.OutputFmtAzw3 {
						compression, source, target = &cfg.AZW3.Compression, &cfg.AZW3.CompressionSource, "AZW3"
					}
					*compression = test.configured
					before := cfg
					core, observed := observer.New(zap.DebugLevel)
					app := &cli.Command{
						Flags: []cli.Flag{&cli.StringFlag{Name: "compression"}, &cli.StringFlag{Name: "config"}},
						Action: func(_ context.Context, cmd *cli.Command) error {
							return configureCompression(cmd, profile.format, &cfg, zap.New(core))
						},
					}
					args := []string{"test"}
					if test.configFile {
						args = append(args, "--config", "settings.yaml")
					}
					if test.supplied {
						args = append(args, "--compression="+test.override)
					}
					err := app.Run(t.Context(), args)
					if (err != nil) != test.invalid {
						t.Fatalf("unexpected compression result: %v", err)
					}
					if *compression != test.want || *source != test.source || cfg.AZW3.APNX != before.AZW3.APNX {
						t.Fatalf("compression=%q, source=%q, APNX=%q", *compression, *source, cfg.AZW3.APNX)
					}
					if profile.format == common.OutputFmtAzw3 && cfg.KFX != before.KFX ||
						profile.format != common.OutputFmtAzw3 && cfg.AZW3 != before.AZW3 {
						t.Fatal("compression selection modified unrelated format configuration")
					}
					if test.invalid {
						if !strings.Contains(err.Error(), "invalid --compression") ||
							!strings.Contains(err.Error(), profile.format.String()) ||
							!strings.Contains(err.Error(), strings.Join(profile.modes, " or ")) {
							t.Fatal("error lost format-specific mode guidance", err)
						}
						if observed.Len() != 0 {
							t.Fatal("invalid override was logged as selected")
						}
						return
					}
					entries := observed.FilterMessage(target + " compression selected").All()
					if len(entries) != 1 || entries[0].ContextMap()["source"] != test.source ||
						entries[0].ContextMap()["compression"] != test.want {
						t.Fatal("missing effective compression/source diagnostic", entries)
					}
					if observed.FilterLevelExact(zap.WarnLevel).Len() != 0 {
						t.Fatal("valid compression selection produced warnings")
					}
				})
			}
		})
	}
}

func TestCompressionIgnoredForNonKindle(t *testing.T) {
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
		common.OutputFmtPdf, common.OutputFmtTxt, common.OutputFmtMd,
	} {
		for _, supplied := range []bool{false, true} {
			name := "omitted"
			if supplied {
				name = "supplied"
			}
			t.Run(format.String()+"/"+name, func(t *testing.T) {
				cfg := config.DocumentConfig{
					KFX:  config.KFXConfig{Compression: "lzma"},
					AZW3: config.AZW3Config{Compression: "huffdic", APNX: "app"},
				}
				before := cfg
				core, observed := observer.New(zap.DebugLevel)
				app := &cli.Command{
					Flags: []cli.Flag{&cli.StringFlag{Name: "compression"}},
					Action: func(_ context.Context, cmd *cli.Command) error {
						return configureCompression(cmd, format, &cfg, zap.New(core))
					},
				}
				args := []string{"test"}
				if supplied {
					args = append(args, "--compression=xz")
				}
				if err := app.Run(t.Context(), args); err != nil {
					t.Fatal(err)
				}
				if cfg.KFX != before.KFX || cfg.AZW3 != before.AZW3 {
					t.Fatal("ignored flag modified compression or APNX configuration")
				}
				if supplied {
					entries := observed.FilterLevelExact(zap.WarnLevel).All()
					if len(entries) != 1 || entries[0].Message != "Ignoring --compression for non-Kindle output" ||
						entries[0].ContextMap()["format"] != format.String() {
						t.Fatal("missing ignored-option warning", entries)
					}
				} else if observed.Len() != 0 {
					t.Fatal("omitted flag produced diagnostics for non-Kindle output")
				}
			})
		}
	}
}
