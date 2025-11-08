package kfx

import (
	"context"

	"go.uber.org/zap"

	"fbc/fb2"
)

// Generator implements KFX format generation logic.
// KFX is Amazon's proprietary format for Kindle devices.
type Generator struct {
	log *zap.Logger
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
	WorkDir() string
}

// New creates a new KFX generator.
func New(log *zap.Logger) *Generator {
	return &Generator{
		log: log,
	}
}

// Generate creates the KFX output file.
func (g *Generator) Generate(ctx context.Context, content contentReader, outputPath string) error {
	g.log.Info("Generating KFX", zap.String("output", outputPath))

	// TODO: Implement KFX generation logic
	// - Convert to EPUB first (as intermediate format)
	// - Use Amazon's tools or libraries to convert EPUB to KFX
	// - Handle Kindle-specific features

	_ = content.Book()
	_ = content.CoverID()

	return nil
}
