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
    mode: float
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

	if cfg.Document.Footnotes.Mode != FootnotesModeFloat {
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

func TestOutputFmt_String(t *testing.T) {
	tests := []struct {
		fmt      OutputFmt
		expected string
	}{
		{OutputFmtEpub2, "epub2"},
		{OutputFmtEpub3, "epub3"},
		{OutputFmtKepub, "kepub"},
		{OutputFmtKfx, "kfx"},
		{OutputFmt(99), "OutputFmt(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.fmt.String()
			if got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOutputFmt_IsValid(t *testing.T) {
	tests := []struct {
		fmt   OutputFmt
		valid bool
	}{
		{OutputFmtEpub2, true},
		{OutputFmtEpub3, true},
		{OutputFmtKepub, true},
		{OutputFmtKfx, true},
		{OutputFmt(99), false},
		{OutputFmt(-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.fmt.String(), func(t *testing.T) {
			got := tt.fmt.IsValid()
			if got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestParseOutputFmt(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  OutputFmt
		shouldErr bool
	}{
		{"epub2 lowercase", "epub2", OutputFmtEpub2, false},
		{"EPUB2 uppercase", "EPUB2", OutputFmtEpub2, false},
		{"epub3", "epub3", OutputFmtEpub3, false},
		{"kepub", "kepub", OutputFmtKepub, false},
		{"kfx", "kfx", OutputFmtKfx, false},
		{"invalid", "invalid", OutputFmt(0), true},
		{"empty", "", OutputFmt(0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOutputFmt(tt.input)
			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("ParseOutputFmt(%q) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestMustParseOutputFmt(t *testing.T) {
	t.Run("valid value", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MustParseOutputFmt panicked unexpectedly: %v", r)
			}
		}()
		got := MustParseOutputFmt("epub2")
		if got != OutputFmtEpub2 {
			t.Errorf("MustParseOutputFmt(\"epub2\") = %v, want %v", got, OutputFmtEpub2)
		}
	})

	t.Run("invalid value panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustParseOutputFmt should have panicked")
			}
		}()
		MustParseOutputFmt("invalid")
	})
}

func TestOutputFmt_MarshalText(t *testing.T) {
	tests := []struct {
		fmt      OutputFmt
		expected string
	}{
		{OutputFmtEpub2, "epub2"},
		{OutputFmtEpub3, "epub3"},
		{OutputFmtKepub, "kepub"},
		{OutputFmtKfx, "kfx"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got, err := tt.fmt.MarshalText()
			if err != nil {
				t.Errorf("MarshalText() error = %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("MarshalText() = %q, want %q", string(got), tt.expected)
			}
		})
	}
}

func TestOutputFmt_UnmarshalText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  OutputFmt
		shouldErr bool
	}{
		{"epub2", "epub2", OutputFmtEpub2, false},
		{"epub3", "epub3", OutputFmtEpub3, false},
		{"kepub", "kepub", OutputFmtKepub, false},
		{"kfx", "kfx", OutputFmtKfx, false},
		{"invalid", "invalid", OutputFmt(0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fmt OutputFmt
			err := fmt.UnmarshalText([]byte(tt.input))
			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("UnmarshalText() error = %v", err)
				}
				if fmt != tt.expected {
					t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, fmt, tt.expected)
				}
			}
		})
	}
}

func TestOutputFmtNames(t *testing.T) {
	names := OutputFmtNames()
	expected := []string{"epub2", "epub3", "kepub", "kfx"}

	if len(names) != len(expected) {
		t.Errorf("OutputFmtNames() length = %d, want %d", len(names), len(expected))
	}

	for i, name := range expected {
		if names[i] != name {
			t.Errorf("OutputFmtNames()[%d] = %q, want %q", i, names[i], name)
		}
	}
}

func TestOutputFmt_ForKindle(t *testing.T) {
	tests := []struct {
		fmt      OutputFmt
		expected bool
	}{
		{OutputFmtEpub2, false},
		{OutputFmtEpub3, false},
		{OutputFmtKepub, false},
		{OutputFmtKfx, true},
	}

	for _, tt := range tests {
		t.Run(tt.fmt.String(), func(t *testing.T) {
			got := tt.fmt.ForKindle()
			if got != tt.expected {
				t.Errorf("ForKindle() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOutputFmt_Ext(t *testing.T) {
	tests := []struct {
		fmt      OutputFmt
		expected string
	}{
		{OutputFmtEpub2, ".epub"},
		{OutputFmtEpub3, ".epub"},
		{OutputFmtKepub, ".kepub.epub"},
		{OutputFmtKfx, ".kfx"},
	}

	for _, tt := range tests {
		t.Run(tt.fmt.String(), func(t *testing.T) {
			got := tt.fmt.Ext()
			if got != tt.expected {
				t.Errorf("Ext() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOutputFmt_Ext_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Ext() should panic for invalid format")
		}
	}()
	invalidFmt := OutputFmt(99)
	invalidFmt.Ext()
}
