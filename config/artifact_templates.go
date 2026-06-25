package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	sprig "github.com/go-task/slim-sprig/v3"

	"fbc/common"
	"fbc/misc"
)

const (
	LoggingFileDestinationTemplateFieldName  TemplateFieldName = "logging.file.destination_template"
	LoggingPanicDestinationTemplateFieldName TemplateFieldName = "logging.file.panic_destination_template"
	ReportingDestinationTemplateFieldName    TemplateFieldName = "reporting.destination_template"
)

type ArtifactTemplateContext struct {
	Context    string
	AppName    string
	PID        int
	Hostname   string
	Started    time.Time
	Unique     string
	Command    string
	Format     string
	SourceFile string
}

type ArtifactTemplateValues struct {
	Started time.Time
	Command string
	Format  common.OutputFmt
	Source  string
}

func (cfg *Config) ResolveArtifactTemplates(values ArtifactTemplateValues) error {
	values.withDefaults()
	if cfg.Logging.FileLogger.PanicDestinationTemplate == "" && cfg.Logging.FileLogger.DestinationTemplate != "" {
		cfg.Logging.FileLogger.PanicDestinationTemplate = "{{ .AppName }}-panic.log"
	}

	loggingDestination, err := expandArtifactTemplate(
		LoggingFileDestinationTemplateFieldName,
		cfg.Logging.FileLogger.DestinationTemplate,
		values,
	)
	if err != nil {
		return err
	}
	cfg.Logging.FileLogger.destination = loggingDestination

	panicDestination, err := expandArtifactTemplate(
		LoggingPanicDestinationTemplateFieldName,
		cfg.Logging.FileLogger.PanicDestinationTemplate,
		values,
	)
	if err != nil {
		return err
	}
	cfg.Logging.FileLogger.panicDestination = panicDestination

	reportDestination, err := expandArtifactTemplate(
		ReportingDestinationTemplateFieldName,
		cfg.Reporting.DestinationTemplate,
		values,
	)
	if err != nil {
		return err
	}
	cfg.Reporting.destination = reportDestination

	return nil
}

func expandArtifactTemplate(name TemplateFieldName, field string, values ArtifactTemplateValues) (string, error) {
	values.withDefaults()

	tmpl, err := template.New(string(name)).Funcs(sprig.FuncMap()).Parse(field)
	if err != nil {
		return "", fmt.Errorf("unable to parse template field %s: %w", name, err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	format := ""
	if values.Command == "convert" {
		if values.Format.IsValid() {
			format = values.Format.String()
		}
	}

	ctx := ArtifactTemplateContext{
		Context:    string(name),
		AppName:    misc.GetAppName(),
		PID:        os.Getpid(),
		Hostname:   hostname,
		Started:    values.Started,
		Unique:     fmt.Sprintf("%s.%d", values.Started.Format("20060102T150405"), os.Getpid()),
		Command:    values.Command,
		Format:     format,
		SourceFile: sourceFile(values.Source),
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, &ctx); err != nil {
		return "", fmt.Errorf("unable to execute template field %s: %w", name, err)
	}
	result := strings.TrimSpace(buf.String())
	if result != "" {
		result = filepath.Clean(result)
		if err := os.MkdirAll(filepath.Dir(result), 0755); err != nil {
			return "", fmt.Errorf("unable to create directory for template field %s: %w", name, err)
		}
	}
	return result, nil
}

func (values *ArtifactTemplateValues) withDefaults() {
	if values.Started.IsZero() {
		values.Started = time.Now()
	}
	if values.Command == "" {
		values.Command = "unknown"
	}
}

func sourceFile(source string) string {
	if source == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
}
