package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rupor-github/gencfg"
)

func TestLoadConfiguration_NoFile(t *testing.T) {
	cfg, err := LoadConfiguration("")
	if err != nil {
		t.Fatalf("LoadConfiguration() with empty path error = %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfiguration() returned nil config")
	}

	if cfg.Version != 1 {
		t.Errorf("Default config version = %d, want 1", cfg.Version)
	}
}

func TestLoadConfiguration_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `version: 1
document:
  fix_zip: true
  images:
    use_broken: false
    remove_png_transparency: true
    scale_factor: 1.5
    optimize: true
    jpeq_quality_level: 85
  footnotes:
    mode: inline
    bodies: ["notes", "comments"]
logging:
  console:
    level: normal
  file:
    level: debug
    destination: /tmp/test.log
    mode: append
reporting:
  destination: /tmp/test-report.zip
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		t.Fatalf("LoadConfiguration() error = %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}

	if !cfg.Document.FixZip {
		t.Error("Expected FixZip to be true")
	}

	if cfg.Document.Images.ScaleFactor != 1.5 {
		t.Errorf("ScaleFactor = %f, want 1.5", cfg.Document.Images.ScaleFactor)
	}

	if cfg.Document.Images.JPEGQuality != 85 {
		t.Errorf("JPEGQuality = %d, want 85", cfg.Document.Images.JPEGQuality)
	}

	if cfg.Document.Footnotes.Mode != FootnotesModeInline {
		t.Errorf("FootnotesMode = %d, want FootnotesModeInline", cfg.Document.Footnotes.Mode)
	}

	if len(cfg.Document.Footnotes.BodyNames) != 2 {
		t.Errorf("BodyNames length = %d, want 2", len(cfg.Document.Footnotes.BodyNames))
	}
}

func TestLoadConfiguration_NonExistentFile(t *testing.T) {
	_, err := LoadConfiguration("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadConfiguration_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `version: 1
document:
  fix_zip: true
  invalid indent
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfiguration(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfiguration_UnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unknown.yaml")

	configWithUnknown := `version: 1
unknown_field: value
document:
  fix_zip: true
`

	if err := os.WriteFile(configPath, []byte(configWithUnknown), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfiguration(configPath)
	if err == nil {
		t.Error("Expected error for unknown fields")
	}
}

func TestLoadConfiguration_ValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid_values.yaml")

	// Invalid version number
	configWithInvalidVersion := `version: 2
document:
  fix_zip: true
`

	if err := os.WriteFile(configPath, []byte(configWithInvalidVersion), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfiguration(configPath)
	if err == nil {
		t.Error("Expected validation error for invalid version")
	}
}

func TestLoadConfiguration_WithOptions(t *testing.T) {
	option := func(opts *gencfg.ProcessingOptions) {
		// Options are opaque, just test that we can pass them
	}

	cfg, err := LoadConfiguration("", option)
	if err != nil {
		t.Fatalf("LoadConfiguration() with options error = %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfiguration() returned nil config")
	}
}

func TestPrepare(t *testing.T) {
	data, err := Prepare()
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Prepare() returned empty data")
	}

	// Verify it's valid YAML by trying to unmarshal
	cfg := &Config{}
	_, err = unmarshalConfig(data, cfg, true)
	if err != nil {
		t.Errorf("Prepared config is not valid: %v", err)
	}
}

func TestDump(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Document: DocumentConfig{
			FixZip: true,
			Images: ImagesConfig{
				UseBroken:             false,
				RemovePNGTransparency: true,
				ScaleFactor:           1.0,
				Optimize:              true,
				JPEGQuality:           80,
			},
			Footnotes: FootnotesConfig{
				Mode:      0,
				BodyNames: []string{"notes"},
			},
		},
	}

	data, err := Dump(cfg)
	if err != nil {
		t.Fatalf("Dump() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Dump() returned empty data")
	}

	// Verify we can load it back
	cfg2 := &Config{}
	_, err = unmarshalConfig(data, cfg2, false)
	if err != nil {
		t.Errorf("Dumped config cannot be loaded: %v", err)
	}

	if cfg2.Version != cfg.Version {
		t.Errorf("Version mismatch after dump/load: got %d, want %d", cfg2.Version, cfg.Version)
	}
}

func TestUnmarshalConfig(t *testing.T) {
	t.Run("valid config without processing", func(t *testing.T) {
		data := []byte(`version: 1`)
		cfg := &Config{}

		result, err := unmarshalConfig(data, cfg, false)
		if err != nil {
			t.Errorf("unmarshalConfig() error = %v", err)
		}

		if result == nil {
			t.Fatal("unmarshalConfig() returned nil")
		}

		if result.Version != 1 {
			t.Errorf("Version = %d, want 1", result.Version)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		data := []byte(`invalid: [yaml`)
		cfg := &Config{}

		_, err := unmarshalConfig(data, cfg, false)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg, err := LoadConfiguration("")
	if err != nil {
		t.Fatalf("LoadConfiguration() error = %v", err)
	}

	// Check that default values are reasonable
	if cfg.Document.Images.ScaleFactor < 0 {
		t.Error("ScaleFactor should not be negative")
	}

	if cfg.Document.Images.JPEGQuality < 40 || cfg.Document.Images.JPEGQuality > 100 {
		t.Errorf("JPEGQuality = %d, should be between 40 and 100", cfg.Document.Images.JPEGQuality)
	}

	if cfg.Document.Footnotes.BodyNames == nil {
		t.Error("BodyNames should not be nil")
	}
}

func TestImagesConfig(t *testing.T) {
	img := ImagesConfig{
		UseBroken:             true,
		RemovePNGTransparency: false,
		ScaleFactor:           2.0,
		Optimize:              true,
		JPEGQuality:           90,
	}

	if !img.UseBroken {
		t.Error("UseBroken should be true")
	}
	if img.RemovePNGTransparency {
		t.Error("RemovePNGTransparency should be false")
	}
	if img.ScaleFactor != 2.0 {
		t.Errorf("ScaleFactor = %f, want 2.0", img.ScaleFactor)
	}
	if !img.Optimize {
		t.Error("Optimize should be true")
	}
	if img.JPEGQuality != 90 {
		t.Errorf("JPEGQuality = %d, want 90", img.JPEGQuality)
	}
}

func TestFootnotesConfig(t *testing.T) {
	fn := FootnotesConfig{
		Mode:      1,
		BodyNames: []string{"notes", "comments", "footnotes"},
	}

	if fn.Mode != 1 {
		t.Errorf("Mode = %d, want 1", fn.Mode)
	}

	if len(fn.BodyNames) != 3 {
		t.Errorf("BodyNames length = %d, want 3", len(fn.BodyNames))
	}

	expected := []string{"notes", "comments", "footnotes"}
	for i, name := range fn.BodyNames {
		if name != expected[i] {
			t.Errorf("BodyNames[%d] = %s, want %s", i, name, expected[i])
		}
	}
}

func TestDocumentConfig(t *testing.T) {
	doc := DocumentConfig{
		FixZip: true,
		Images: ImagesConfig{
			ScaleFactor: 1.5,
		},
		Footnotes: FootnotesConfig{
			Mode: 2,
		},
	}

	if !doc.FixZip {
		t.Error("FixZip should be true")
	}
	if doc.Images.ScaleFactor != 1.5 {
		t.Errorf("Images.ScaleFactor = %f, want 1.5", doc.Images.ScaleFactor)
	}
	if doc.Footnotes.Mode != 2 {
		t.Errorf("Footnotes.Mode = %d, want 2", doc.Footnotes.Mode)
	}
}

func TestLoadConfiguration_MergeWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")

	// Partial config that only overrides some values
	partialConfig := `version: 1
document:
  fix_zip: false
`

	if err := os.WriteFile(configPath, []byte(partialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		t.Fatalf("LoadConfiguration() error = %v", err)
	}

	// Check that explicitly set value is used
	if cfg.Document.FixZip {
		t.Error("Expected FixZip to be false from config file")
	}

	// Check that default values are still present for unspecified fields
	if cfg.Document.Images.ScaleFactor < 0 {
		t.Error("ScaleFactor should have default value")
	}
}
