package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fbc/common"
)

func TestLoadConfiguration_CoverFill(t *testing.T) {
	for _, mode := range []string{"fill", "crop"} {
		t.Run(mode, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(file, []byte("document:\n  images:\n    cover:\n      resize: "+mode+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfiguration(file)
			if mode == "crop" {
				if err == nil {
					t.Fatal("unsupported crop mode was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Document.Images.Cover.Resize != common.ImageResizeModeFill {
				t.Fatal("fill mode was not loaded")
			}
			data, err := Dump(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "resize: fill") {
				t.Fatal("fill mode was not preserved in config dump")
			}
		})
	}
	cfg, err := LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Document.Images.Cover.Resize != common.ImageResizeModeStretch {
		t.Fatal("default cover resizing changed")
	}
}
