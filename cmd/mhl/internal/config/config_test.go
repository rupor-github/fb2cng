package config

import (
	"os"
	"path/filepath"
	"testing"

	"fbc/common"
)

func TestLoad(t *testing.T) {
	template, err := os.ReadFile("../../connector.yaml.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		data       string
		wantFormat common.OutputFmt
		wantError  bool
	}{
		{name: "defaults"},
		{name: "epub2", data: "output_format: epub2\n", wantFormat: common.OutputFmtEpub2},
		{name: "epub3", data: "output_format: epub3\n", wantFormat: common.OutputFmtEpub3},
		{name: "kepub", data: "output_format: kepub\n", wantFormat: common.OutputFmtKepub},
		{name: "kepub3", data: "output_format: kepub3\n", wantFormat: common.OutputFmtKepub3},
		{name: "kfx", data: "output_format: kfx\n", wantFormat: common.OutputFmtKfx},
		{name: "azw8", data: "output_format: azw8\n", wantFormat: common.OutputFmtAzw8},
		{name: "azw3", data: "output_format: azw3\n", wantFormat: common.OutputFmtAzw3},
		{name: "pdf rejected", data: "output_format: pdf\n", wantError: true},
		{name: "txt rejected", data: "output_format: txt\n", wantError: true},
		{name: "md rejected", data: "output_format: md\n", wantError: true},
		{name: "unknown format rejected", data: "output_format: mobi\n", wantError: true},
		{name: "unknown field rejected", data: "unknown: true\n", wantError: true},
		{name: "invalid version rejected", data: "version: 2\n", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.data != "" {
				path = filepath.Join(t.TempDir(), "connector.yaml")
				if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := Load(path, template)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected invalid configuration to be rejected")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Version != 1 || cfg.Debug || cfg.KindleEbook {
				t.Fatalf("unexpected defaults: %+v", cfg)
			}
			if tt.data == "" {
				if cfg.OutputFormat != nil {
					t.Fatalf("default output format = %v, want nil", cfg.OutputFormat)
				}
			} else if cfg.OutputFormat == nil || *cfg.OutputFormat != tt.wantFormat {
				t.Fatalf("output format = %v, want %s", cfg.OutputFormat, tt.wantFormat)
			}
		})
	}
}
