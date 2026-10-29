package pdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
func Generate(
	ctx context.Context,
	c *content.Content,
	outputName string,
	finalOutputName string,
	cfg *config.DocumentConfig,
	log *zap.Logger,
) error {
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
	pageFrameWidth, err := cfg.PDF.PageFramePoints(cfg.Images.Screen)
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

	parsedStylesheets := parsePDFStylesheets(c.Book, log)
	styleResolver := newPDFStyleResolverFromParsed(parsedStylesheets, styleTracer, log)
	writePDFParsedStylesheetDebug(c, styleResolver, log)

	title := bookTitle(c, cfg, log)
	author := bookAuthors(c, cfg, log)
	subject := bookSubject(c)
	keywords := bookKeywords(c)

	data, err := buildPDFDocument(pdfDocumentSpec{
		PageWidth:        pageWidth,
		PageHeight:       pageHeight,
		PageFrameWidth:   pageFrameWidth,
		ScreenWidthPx:    cfg.Images.Screen.Width,
		ScreenHeightPx:   cfg.Images.Screen.Height,
		ScreenDPI:        cfg.Images.Screen.DPI,
		Title:            title,
		Author:           author,
		Subject:          subject,
		Keywords:         keywords,
		XMP:              pdfXMPMetadata(c, cfg, title, subject, keywords, log),
		Blocks:           contentPlan.Blocks,
		TOC:              contentPlan.TOC,
		PrintedFootnotes: contentPlan.PrintedFootnotes,
		DebugPlan:        contentPlan.DebugPlan,
		Content:          c,
		Styles:           styleResolver,
		Images:           c.ImagesIndex,
		ImageConfig:      cfg.Images,
		CoverID:          c.CoverID,
		Hyphenator:       pdfHyphenator(c, log),
		Fonts:            newPDFFontRegistryFromParsed(parsedStylesheets, log),
		Debug:            c.Debug,
		WorkDir:          c.WorkDir,
		Log:              log,
	})
	if err != nil {
		return fmt.Errorf("build pdf: %w", err)
	}

	if log != nil {
		fields := []zap.Field{
			zap.String("file", pdfFinalOutputName(outputName, finalOutputName)),
			zap.Float64("page_width_pt", pageWidth),
			zap.Float64("page_height_pt", pageHeight),
			zap.Int("bytes", len(data)),
		}
		if final := pdfFinalOutputName(outputName, finalOutputName); final != outputName {
			fields = append(fields, zap.String("temporary_file", outputName))
		}
		log.Debug("Writing PDF", fields...)
	}

	if err := os.WriteFile(outputName, data, 0644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}
	return nil
}

func writePDFParsedStylesheetDebug(c *content.Content, styles *pdfStyleResolver, log *zap.Logger) {
	if c == nil || styles == nil || !c.Debug || c.WorkDir == "" || !styles.hasParsedStylesheet {
		return
	}
	if err := os.WriteFile(filepath.Join(c.WorkDir, "book.stylesheet.css"), []byte(styles.parsedStylesheetCSS), 0644); err != nil && log != nil {
		log.Warn("Failed to write parsed stylesheet for debug", zap.Error(err))
	}
}

func pdfFinalOutputName(outputName string, finalOutputName string) string {
	if finalOutputName != "" {
		return finalOutputName
	}
	return outputName
}
