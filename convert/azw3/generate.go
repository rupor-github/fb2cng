package azw3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
)

// Result identifies the final files published by a successful conversion.
type Result struct {
	BookPath string
	APNXPath string
}

// Generate builds native standalone KF8 and publishes the final AZW3/APNX bundle.
// Unlike single-file adapters, it owns staging and publication: outputPath is the
// final book filename, not an outer temporary file. APNX placement is config-only.
func Generate(
	ctx context.Context, c *content.Content, outputPath string, overwrite bool, cfg *config.DocumentConfig, log *zap.Logger,
) (result Result, rerr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if c == nil || c.Book == nil || cfg == nil {
		return result, errors.New("AZW3 generation requires prepared content and document configuration")
	}
	if c.OutputFormat != common.OutputFmtAzw3 {
		return result, fmt.Errorf("AZW3 generation cannot use output identity %q", c.OutputFormat)
	}
	bundle, err := preflightOutputBundle(ctx, outputPath, nil, overwrite)
	if err != nil {
		return result, err
	}
	outputPath = bundle.book.path
	started := time.Now()
	log.Info("AZW3 generation starting", zap.String("output", outputPath), zap.Stringer("format", c.OutputFormat),
		zap.String("compression", cfg.AZW3.Compression), zap.String("apnx", cfg.AZW3.APNX))
	defer func() {
		if rerr == nil {
			log.Info("AZW3 generation completed", zap.String("output", result.BookPath), zap.String("apnx_path", result.APNXPath),
				zap.Duration("elapsed", time.Since(started)))
		}
	}()
	pub, err := publication.Build(ctx, c, cfg, publication.HTML5, log)
	if err != nil {
		return result, fmt.Errorf("build AZW3 publication: %w", err)
	}
	pub, err = preparePublicationImages(ctx, pub, &cfg.Images, log)
	if err != nil {
		return result, err
	}
	pub, err = prepareCoverPublication(ctx, pub, &cfg.Images)
	if err != nil {
		return result, err
	}
	pub, text, err := compileBacklinks(ctx, c, pub)
	if err != nil {
		return result, fmt.Errorf("compile AZW3 text: %w", err)
	}
	navigation, err := compileNavigation(ctx, pub, text, cfg.TOCType)
	if err != nil {
		return result, fmt.Errorf("compile AZW3 navigation: %w", err)
	}
	metadata, err := compileMetadata(ctx, c, cfg, pub, text, navigation, log)
	if err != nil {
		return result, fmt.Errorf("compile AZW3 metadata: %w", err)
	}
	book, err := compileBook(ctx, text, navigation, metadata, cfg.AZW3.Compression)
	if err != nil {
		return result, fmt.Errorf("assemble AZW3 book: %w", err)
	}
	apnx, err := compileAPNX(ctx, outputPath, pub, text, metadata, cfg, log)
	if err != nil {
		return result, err
	}
	bundle, err = preflightOutputBundle(ctx, outputPath, apnx, overwrite)
	if err != nil {
		return result, err
	}
	if c.Debug {
		if err := writeDebugArtifacts(c, pub, text, navigation, metadata, book, apnx, cfg); err != nil {
			log.Warn("Unable to write AZW3 diagnostics", zap.Error(err))
		}
	}
	if err := bundle.publish(ctx, book, started); err != nil {
		return result, err
	}
	result.BookPath = bundle.book.path
	if bundle.apnx != nil {
		result.APNXPath = bundle.apnx.path
	}
	return result, nil
}
