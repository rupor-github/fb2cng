package config

import (
	"os"
	"path/filepath"
	"testing"

	"fbc/common"
)

func TestArtifactTemplatesOutputDirectory(t *testing.T) {
	for _, test := range []struct{ name, directory string }{
		{"default", ""}, {"relative", "results"}, {"absolute", "absolute"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cwd := t.TempDir()
			t.Chdir(cwd)
			directory := test.directory
			if test.name == "absolute" {
				directory = filepath.Join(cwd, "nested", "my output")
			}
			cfg := &Config{
				Logging: LoggingConfig{FileLogger: LoggerConfig{
					DestinationTemplate:      `{{ .OutputDir }}/{{ .AppName }}.log`,
					PanicDestinationTemplate: `{{ .OutputDir }}/{{ .AppName }}-panic.log`,
				}},
				Reporting: ReporterConfig{DestinationTemplate: `{{ .OutputDir }}/{{ .AppName }}-report.zip`},
			}
			if err := cfg.ResolveArtifactTemplates(ArtifactTemplateValues{
				Command: "convert", Format: common.OutputFmtAzw8, Source: "book.fb2", OutputDir: directory,
			}); err != nil {
				t.Fatal(err)
			}
			wantDir, err := filepath.Abs(directory)
			if err != nil {
				t.Fatal(err)
			}
			for _, artifact := range []struct{ path, name string }{
				{cfg.Logging.FileLogger.Destination(), "fbc.log"},
				{cfg.Logging.FileLogger.PanicDestination(), "fbc-panic.log"},
				{cfg.Reporting.Destination(), "fbc-report.zip"},
			} {
				if want := filepath.Join(wantDir, artifact.name); artifact.path != want {
					t.Fatalf("got %q, want %q", artifact.path, want)
				}
			}
			if info, err := os.Stat(wantDir); err != nil || !info.IsDir() {
				t.Fatal("output directory not created", err)
			}
		})
	}
}
