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
	metadataExcerptMaxRunes = 500
)

// Generate writes a native PDF document.
//
// The current native renderer writes fixed-size PDF 1.4 pages with embedded
// Unicode font resources, selectable title/author text, initial FB2 text body
// pagination, and Info dictionary metadata. Later milestones will replace the
// fixed default styles with the KFX-aligned CSS pipeline.
func Generate(ctx context.Context, c *content.Content, outputName string, cfg *config.DocumentConfig, log *zap.Logger, finalOutputName ...string) error {
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

	data, err := buildPDFDocument(pdfDocumentSpec{
		PageWidth:      pageWidth,
		PageHeight:     pageHeight,
		ScreenWidthPx:  cfg.Images.Screen.Width,
		ScreenHeightPx: cfg.Images.Screen.Height,
		ScreenDPI:      cfg.Images.Screen.DPI,
		Title:          bookTitle(c, cfg, log),
		Author:         bookAuthors(c, cfg, log),
		Subject:        bookSubject(c),
		Keywords:       bookKeywords(c),
		Blocks:         contentPlan.Blocks,
		TOC:            contentPlan.TOC,
		DebugPlan:      contentPlan.DebugPlan,
		Content:        c,
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
		fields := []zap.Field{
			zap.String("file", pdfFinalOutputName(outputName, finalOutputName...)),
			zap.Float64("page_width_pt", pageWidth),
			zap.Float64("page_height_pt", pageHeight),
			zap.Int("bytes", len(data)),
		}
		if final := pdfFinalOutputName(outputName, finalOutputName...); final != outputName {
			fields = append(fields, zap.String("temporary_file", outputName))
		}
		log.Debug("Writing PDF", fields...)
	}

	if err := os.WriteFile(outputName, data, 0644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}
	return nil
}

func pdfFinalOutputName(outputName string, finalOutputName ...string) string {
	if len(finalOutputName) > 0 && finalOutputName[0] != "" {
		return finalOutputName[0]
	}
	return outputName
}
