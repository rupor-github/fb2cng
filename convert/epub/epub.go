package epub

import (
	"context"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/fb2"
)

// Generator implements format-specific EPUB generation logic.
// It handles epub2, epub3, and kepub variants.
type Generator struct {
	log    *zap.Logger
	format config.OutputFmt
}

// contentReader provides read-only access to parsed FB2 content.
// Generators receive this interface to access prepared book data.
type contentReader interface {
	Book() *fb2.FictionBook
	CoverID() string
	ImagesIndex() fb2.BookImages
	FootnotesIndex() fb2.FootnoteRefs
	IDsIndex() fb2.IDIndex
	LinksRevIndex() fb2.ReverseLinkIndex
}

// New creates a new EPUB generator for the specified format variant.
func New(format config.OutputFmt, log *zap.Logger) *Generator {
	return &Generator{
		log:    log,
		format: format,
	}
}

// Generate creates the EPUB output file.
func (g *Generator) Generate(ctx context.Context, content contentReader, outputPath string) error {
	g.log.Info("Generating EPUB", zap.Stringer("format", g.format), zap.String("output", outputPath))

	// TODO: Implement EPUB generation logic
	// - Create EPUB structure (mimetype, META-INF, OEBPS)
	// - Generate OPF manifest
	// - Generate NCX/NAV navigation
	// - Convert FB2 content to XHTML
	// - Write images
	// - Package as ZIP

	_ = content.Book()
	_ = content.CoverID()
	_ = content.ImagesIndex()

	return nil
}
