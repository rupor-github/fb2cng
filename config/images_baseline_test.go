package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfiguration_BaselineJPEG(t *testing.T) {
	cfg, err := LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Document.Images.BaselineJPEG {
		t.Fatal("baseline JPEG conversion must default to disabled")
	}
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("document:\n  images:\n    baseline_jpeg: true\n    optimize: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfiguration(file)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Document.Images.BaselineJPEG || cfg.Document.Images.Optimize {
		t.Fatal("baseline conversion must be independent of optimization")
	}
	dumped, err := Dump(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dumped), "baseline_jpeg: true") {
		t.Fatal("baseline option missing from configuration dump")
	}
}
