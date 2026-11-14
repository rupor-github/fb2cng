package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fbc/common"
)

func TestLoadConfiguration_Grayscale(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  common.GrayscaleMode
		valid bool
	}{
		{"none", common.GrayscaleModeNone, true},
		{"luma", common.GrayscaleModeLuma, true},
		{"average", 0, false},
		{"true", 0, false},
		{"false", 0, false},
		{`""`, 0, false},
		{"-1", 0, false},
		{"42", 0, false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.yaml")
			data := []byte("document:\n  images:\n    grayscale: " + tt.value + "\n    optimize: false\n")
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfiguration(file)
			if !tt.valid {
				if err == nil {
					t.Fatal("unsupported grayscale method accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Document.Images.Grayscale != tt.want || cfg.Document.Images.Optimize {
				t.Fatal("grayscale mode must load independently of optimization")
			}
			dumped, err := Dump(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(dumped), "grayscale: "+tt.value) {
				t.Fatal("grayscale mode missing from configuration dump")
			}
		})
	}
	cfg, err := LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Document.Images.Grayscale != common.GrayscaleModeNone {
		t.Fatal("grayscale conversion must default to disabled")
	}
}
