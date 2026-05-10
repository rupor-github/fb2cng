package pdf

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
)

const (
	pdfVersion              = "1.4"
	defaultDPI              = 300
	metadataExcerptMaxRunes = 500
)

// Generate writes a native PDF document.
//
// The current native renderer writes fixed-size PDF 1.4 pages with embedded
// Unicode font resources, selectable title/author text, initial FB2 text body
// pagination, and Info dictionary metadata. Later milestones will replace the
// fixed default styles with the KFX-aligned CSS pipeline.
func Generate(ctx context.Context, c *content.Content, outputName string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil {
		return errors.New("content is required")
	}
	if cfg == nil {
		return errors.New("document config is required")
	}

	pageWidth, pageHeight, err := pageSizePoints(cfg.Images.Screen)
	if err != nil {
		return err
	}

	contentPlan, err := collectPDFContent(c, cfg)
	if err != nil {
		return fmt.Errorf("collect pdf content: %w", err)
	}

	styleTracer := newPDFStyleTracer("")
	if c.Debug {
		styleTracer = newPDFStyleTracer(c.WorkDir)
	}

	data, err := buildSkeletonPDF(skeletonDocument{
		PageWidth:      pageWidth,
		PageHeight:     pageHeight,
		ScreenWidthPx:  cfg.Images.Screen.Width,
		ScreenHeightPx: cfg.Images.Screen.Height,
		Title:          bookTitle(c),
		Author:         bookAuthors(c),
		Subject:        bookSubject(c),
		Keywords:       bookKeywords(c),
		Blocks:         contentPlan.Blocks,
		TOC:            contentPlan.TOC,
		DebugPlan:      contentPlan.DebugPlan,
		Styles:         newPDFStyleResolver(c.Book, log, styleTracer),
		Images:         c.ImagesIndex,
		CoverID:        c.CoverID,
		Hyphenator:     pdfHyphenator(c, log),
		Fonts:          newPDFFontRegistry(c.Book, log),
		Debug:          c.Debug,
		WorkDir:        c.WorkDir,
	})
	if err != nil {
		return fmt.Errorf("build pdf: %w", err)
	}

	if log != nil {
		log.Debug("Writing PDF",
			zap.String("file", outputName),
			zap.Float64("page_width_pt", pageWidth),
			zap.Float64("page_height_pt", pageHeight),
			zap.Int("bytes", len(data)))
	}

	if err := os.WriteFile(outputName, data, 0644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}
	return nil
}
