package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAZW3CompressionConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name, yaml, want string
		invalid          bool
	}{
		{"default", "document: {}", "standard", false},
		{"standard", "document:\n  azw3:\n    compression: standard", "standard", false},
		{"none", "document:\n  azw3:\n    compression: none", "none", false},
		{"huffdic", "document:\n  azw3:\n    compression: huffdic", "huffdic", false},
		{"invalid", "document:\n  azw3:\n    compression: lzma", "", true},
		{"no numeric aliases", "document:\n  azw3:\n    compression: c2", "", true},
		{"exact spelling", "document:\n  azw3:\n    compression: huffcdic", "", true},
		{"unknown field", "document:\n  azw3:\n    dictionary: 4096", "", true},
		{"empty", "document:\n  azw3:\n    compression: ''", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfiguration(path)
			if tt.invalid {
				if err == nil {
					t.Fatal("accepted invalid AZW3 compression configuration")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Document.AZW3.Compression != tt.want || cfg.Document.KFX.Compression != "none" {
				t.Fatal("wrong AZW3 mode or changed KFX default")
			}
			cfg.Document.AZW3.CompressionSource = "cli"
			data, err := Dump(cfg)
			if err != nil || strings.Contains(string(data), "compression_source") || strings.Contains(string(data), "cli") {
				t.Fatal("runtime source leaked into YAML", err)
			}
		})
	}
}
