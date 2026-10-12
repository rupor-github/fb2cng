package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKFXCompressionConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, yaml, want string
		invalid          bool
	}{
		{"default", "document: {}", "none", false},
		{"lzma", "document:\n  kfx:\n    compression: lzma", "lzma", false},
		{"none", "document:\n  kfx:\n    compression: none", "none", false},
		{"invalid", "document:\n  kfx:\n    compression: xz", "", true},
		{"unknown", "document:\n  kfx:\n    dictionary: 4096", "", true},
		{"empty", "document:\n  kfx:\n    compression: ''", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.yaml+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfiguration(path)
			if test.invalid {
				if err == nil {
					t.Fatal("accepted invalid configuration")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Document.KFX.Compression != test.want {
				t.Fatal(cfg.Document.KFX.Compression)
			}
			cfg.Document.KFX.CompressionSource = "cli"
			data, err := Dump(cfg)
			if err != nil || strings.Contains(string(data), "compression_source") || strings.Contains(string(data), "cli") {
				t.Fatal("runtime source leaked into YAML", err)
			}
		})
	}
}
