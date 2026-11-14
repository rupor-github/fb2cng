package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfiguration_FitToScreen(t *testing.T) {
	cfg, err := LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Document.Images.FitToScreen {
		t.Fatal("screen limiting must default to disabled")
	}
	file := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("document:\n  images:\n    fit_to_screen: true\n    scale_factor: 2\n")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfiguration(file)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Document.Images.FitToScreen || cfg.Document.Images.ScaleFactor != 2 {
		t.Fatal("screen limiting and scaling must load independently")
	}
	dumped, err := Dump(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dumped), "fit_to_screen: true") {
		t.Fatal("screen limiting missing from configuration dump")
	}
}
