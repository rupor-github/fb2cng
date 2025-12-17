package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"

	"github.com/rupor-github/gencfg"

	"fbc/common"
)

type DoubleQuoteString string

// MarshalYAML implements the yaml.Marshaler interface.
func (s DoubleQuoteString) MarshalYAML() (any, error) {
	node := yaml.Node{
		Kind:  yaml.ScalarNode,
		Style: yaml.DoubleQuotedStyle,
		Value: string(s),
	}
	return &node, nil
}

//go:embed config.yaml.tmpl
var ConfigTmpl []byte

type (
	TemplateFieldName string

	CoverConfig struct {
		Generate         bool                   `yaml:"generate"`
		DefaultImagePath string                 `yaml:"default_image_path,omitempty" sanitize:"assure_file_access"`
		Resize           common.ImageResizeMode `yaml:"resize" validate:"gte=0"`
		Width            int                    `yaml:"width" validate:"min=600"`
		Height           int                    `yaml:"height" validate:"min=800"`
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
		Mode           common.FootnotesMode `yaml:"mode" validate:"gte=0"`
		BodyNames      []string             `yaml:"bodies" validate:"dive,required"`
		Backlinks      DoubleQuoteString    `yaml:"backlinks" validate:"gt=0"`
		MoreParagraphs DoubleQuoteString    `yaml:"more_paragraphs" validate:"gt=0"`
		LabelTemplate  string               `yaml:"label_template" validate:"required_if=Mode 2"`
	}

	AnnotationConfig struct {
		Enable bool   `yaml:"enable"`
		Title  string `yaml:"title" validate:"required_unless=Enable false"`
		InTOC  bool   `yaml:"in_toc"`
	}

	TOCPageConfig struct {
		Placement            common.TOCPagePlacement `yaml:"placement" validate:"oneof=0 1 2"`
		AuthorsTemplate      string                  `yaml:"authors_template"`
		ChaptersWithoutTitle bool                    `yaml:"include_chapters_without_title"`
	}

	MetainformationConfig struct {
		TitleTemplate       string `yaml:"title_template"`
		CreatorNameTemplate string `yaml:"creator_name_template"`
		Transliterate       bool   `yaml:"transliterate"`
	}

	VignettesConfig struct {
		Book    VignettePositions `yaml:"book"`
		Chapter VignettePositions `yaml:"chapter"`
		Section VignettePositions `yaml:"section"`
	}

	// VignettePositions defines where vignettes can be placed
	VignettePositions struct {
		TitleTop    string `yaml:"title_top,omitempty" sanitize:"oneof_or_tag=builtin assure_file_access"`
		TitleBottom string `yaml:"title_bottom,omitempty" sanitize:"oneof_or_tag=builtin assure_file_access"`
		End         string `yaml:"end,omitempty" sanitize:"oneof_or_tag=builtin assure_file_access"`
	}

	DropcapsConfig struct {
		Enable        bool              `yaml:"enable"`
		IgnoreSymbols DoubleQuoteString `yaml:"ignore_symbols,omitempty"`
	}

	PageMapConfig struct {
		Enable  bool `yaml:"enable"`
		Size    int  `yaml:"size" validate:"required_unless=Enable false,min=500"`
		AdobeDE bool `yaml:"adobe_de"`
	}

	TextTransformConfig struct {
		Speech   TextTransform `yaml:"speech"`
		Dashes   TextTransform `yaml:"dashes"`
		Dialogue TextTransform `yaml:"dialogue"`
	}

	TextTransform struct {
		Enable bool              `yaml:"enable"`
		From   DoubleQuoteString `yaml:"from" validate:"required_if=Enable true"`
		To     DoubleQuoteString `yaml:"to" validate:"required_if=Enable true"`
	}

	DocumentConfig struct {
		FixZip                bool                  `yaml:"fix_zip"`
		OpenFromCover         bool                  `yaml:"open_from_cover"`
		StylesheetPath        string                `yaml:"stylesheet_path,omitempty" sanitize:"assure_file_access"`
		OutputNameTemplate    string                `yaml:"output_name_template"`
		FileNameTransliterate bool                  `yaml:"file_name_transliterate"`
		InsertSoftHyphen      bool                  `yaml:"insert_soft_hyphen"`
		Images                ImagesConfig          `yaml:"images"`
		Footnotes             FootnotesConfig       `yaml:"footnotes"`
		Annotation            AnnotationConfig      `yaml:"annotation"`
		TOCPage               TOCPageConfig         `yaml:"toc_page"`
		Metainformation       MetainformationConfig `yaml:"metainformation"`
		Vignettes             VignettesConfig       `yaml:"vignettes"`
		Dropcaps              DropcapsConfig        `yaml:"dropcaps"`
		PageMap               PageMapConfig         `yaml:"page_map"`
		Transformations       TextTransformConfig   `yaml:"text_transformations"`
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
	LabelTemplateFieldName           TemplateFieldName = "label_template"
)

var requiredOptions = append([]func(*gencfg.ProcessingOptions){},
	gencfg.WithDoNotExpandField(string(OutputNameTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(MetaTitleTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(MetaCreatorNameTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(AuthorsTemplateFieldName)),
	gencfg.WithDoNotExpandField(string(LabelTemplateFieldName)),
)

// IsEmpty returns true if no vignette positions are defined
func (v *VignettePositions) IsEmpty() bool {
	return v.TitleTop == "" && v.TitleBottom == "" && v.End == ""
}

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
