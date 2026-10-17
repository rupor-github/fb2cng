package textout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
)

// Generate writes UTF-8 TXT or Markdown output.
func Generate(
	ctx context.Context,
	c *content.Content,
	outputName string,
	finalOutputName string,
	cfg *config.DocumentConfig,
	log *zap.Logger,
) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil {
		return errors.New("content is required")
	}
	if cfg == nil {
		return errors.New("document config is required")
	}
	if c.OutputFormat != common.OutputFmtTxt && c.OutputFormat != common.OutputFmtMd {
		return fmt.Errorf("unsupported text output format: %s", c.OutputFormat)
	}

	log.Info(
		"Text generation starting",
		append([]zap.Field{zap.Stringer("format", c.OutputFormat)}, textOutputLogFields(outputName, finalOutputName)...)...,
	)
	defer func(start time.Time) {
		if err == nil {
			log.Info("Text generation completed", zap.Duration("elapsed", time.Since(start)))
		}
	}(time.Now())

	renderOutputPath := outputName
	if finalOutputName != "" {
		renderOutputPath = finalOutputName
	}
	data, err := Render(c, cfg, renderOutputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputName), 0755); err != nil {
		return fmt.Errorf("unable to create output directory: %w", err)
	}
	if err := os.WriteFile(outputName, data, 0644); err != nil {
		return fmt.Errorf("unable to write output file: %w", err)
	}
	return nil
}

func textOutputLogFields(outputName string, finalOutputName string) []zap.Field {
	fields := []zap.Field{zap.String("output", outputName)}
	if finalOutputName != "" && finalOutputName != outputName {
		fields = append(fields, zap.String("final_output", finalOutputName))
	}
	return fields
}
