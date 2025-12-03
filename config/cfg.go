package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"

	"github.com/rupor-github/gencfg"
)

//go:embed config.yaml.tmpl
var ConfigTmpl []byte

type (
	TemplateFieldName string

	CoverConfig struct {
		Generate         bool            `yaml:"generate"`
		DefaultImagePath string          `yaml:"default_image_path" sanitize:"assure_file_access"`
		Resize           ImageResizeMode `yaml:"resize" validate:"gte=0"`
		Width            int             `yaml:"width" validate:"min=600"`
		Height           int             `yaml:"height" validate:"min=800"`
	}

	ImagesConfig struct {
		UseBroken             bool        `yaml:"use_broken"`
		RemovePNGTransparency bool        `yaml:"remove_png_transparency"`
		ScaleFactor           float64     `yaml:"scale_factor" validate:"gte=0.0"`
		Optimize              bool        `yaml:"optimize"`
		JPEGQuality           int         `yaml:"jpeq_quality_level" validate:"min=40,max=100"`
		Cover                 CoverConfig `yaml:"cover"`
	}

	FootnotesConfig struct {
		Mode      FootnotesMode `yaml:"mode" validate:"gte=0"`
		BodyNames []string      `yaml:"bodies" validate:"dive,required"`
	}

	AnnotationConfig struct {
		Enable bool   `yaml:"enable"`
		Title  string `yaml:"title" validate:"required_unless=Enable false"`
		TOC    bool   `yaml:"toc"`
	}

	TOCPageConfig struct {
		Placement       TOCPagePlacement `yaml:"placement" validate:"oneof=0 1 2"`
		Title           string           `yaml:"title" validate:"required_unless=Placement 0"`
		AuthorsTemplate string           `yaml:"authors_template"`
	}

	MetainformationConfig struct {
		TitleTemplate       string `yaml:"title_template"`
		CreatorNameTemplate string `yaml:"creator_name_template"`
		Transliterate       bool   `yaml:"transliterate"`
	}

	DocumentConfig struct {
		FixZip                bool                  `yaml:"fix_zip"`
		StylesheetPath        string                `yaml:"stylesheet_path" sanitize:"assure_file_access"`
		OutputNameTemplate    string                `yaml:"output_name_template"`
		FileNameTransliterate bool                  `yaml:"file_name_transliterate"`
		InsertSoftHyphen      bool                  `yaml:"insert_soft_hyphen"`
		Images                ImagesConfig          `yaml:"images"`
		Footnotes             FootnotesConfig       `yaml:"footnotes"`
		Annotation            AnnotationConfig      `yaml:"annotation"`
		TOCPage               TOCPageConfig         `yaml:"toc_page"`
		Metainformation       MetainformationConfig `yaml:"metainformation"`
	}

	Config struct {
		Version   int            `yaml:"version" validate:"eq=1"`
		Document  DocumentConfig `yaml:"document"`
		Logging   LoggingConfig  `yaml:"logging"`
		Reporting ReporterConfig `yaml:"reporting"`
	}
)

const (
	// NOTE: must match yaml field name above, alternative is to use struct
	// field name and reflection which I want to avoid for now
	OutputNameTemplateFieldName      TemplateFieldName = "output_name_template"
	MetaTitleTemplateFieldName       TemplateFieldName = "title_template"
	MetaCreatorNameTemplateFieldName TemplateFieldName = "creator_name_template"
	AuthorsTemplateFieldName         TemplateFieldName = "authors_template"
)

var requiredOptions = append([]func(*gencfg.ProcessingOptions){},
	gencfg.WithDoNotExpandField(string(OutputNameTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(MetaTitleTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(MetaCreatorNameTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(AuthorsTemplateFieldName)),
)

func unmarshalConfig(data []byte, cfg *Config, process bool) (*Config, error) {
	// We want to use only fields we defined so we cannot use yaml.Unmarshal
	// directly here
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode configuration data: %w", err)
	}
	if process {
		// sanitize and validate what has been loaded
		if err := gencfg.Sanitize(cfg); err != nil {
			return nil, err
		}
		if err := gencfg.Validate(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// LoadConfiguration reads the configuration from the file at the given path,
// superimposes its values on top of expanded configuration tamplate to provide
// sane defaults and performs validation.
func LoadConfiguration(path string, options ...func(*gencfg.ProcessingOptions)) (*Config, error) {
	haveFile := len(path) > 0

	data, err := gencfg.Process(ConfigTmpl, append(requiredOptions, options...)...)
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

	// overwrite cfg values with values from the file
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

// Prepare generates configuration file from template and returns it as a byte
// slice.
func Prepare() ([]byte, error) {
	return gencfg.Process(ConfigTmpl, requiredOptions...)
}

func Dump(cfg *Config) ([]byte, error) {
	data, err := yaml.Marshal(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to yaml: %v", err)
	}
	return data, nil
}
