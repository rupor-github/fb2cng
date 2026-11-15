package config

import (
	"bytes"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"

	"github.com/rupor-github/gencfg"

	"fbc/common"
)

type Config struct {
	Version        int               `yaml:"version" validate:"eq=1"`
	LogDestination string            `yaml:"log_destination,omitempty" sanitize:"path_clean,assure_dir_exists_for_file" validate:"omitempty,filepath"`
	Debug          bool              `yaml:"debug"`
	OutputFormat   *common.OutputFmt `yaml:"output_format,omitempty" validate:"omitempty,oneof=0 1 2 3 4 5 9"`
	KindleEbook    bool              `yaml:"kindle_ebook"`
}

func unmarshalConfig(data []byte, cfg *Config, process bool) (*Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode configuration data: %w", err)
	}
	if process {
		if err := gencfg.Sanitize(cfg); err != nil {
			return nil, err
		}
		if err := gencfg.Validate(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// Load overlays the configuration file on the expanded template, then sanitizes
// and validates the result. An empty path uses only the template defaults.
func Load(path string, template []byte, options ...func(*gencfg.ProcessingOptions)) (*Config, error) {
	haveFile := len(path) > 0

	data, err := gencfg.Process(template, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration template: %w", err)
	}
	cfg, err := unmarshalConfig(data, &Config{}, !haveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration template: %w", err)
	}
	if !haveFile {
		return cfg, nil
	}

	data, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	cfg, err = unmarshalConfig(data, cfg, haveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration file: %w", err)
	}
	return cfg, nil
}
