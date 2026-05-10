package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/internal/pdfdoc"
	"fbc/fb2"
)

const (
	pdfVersion = "1.4"
	defaultDPI = 300
)

// Generate writes a native PDF document.
//
// This initial implementation is the Milestone 1 skeleton: it creates a valid
// PDF 1.4 file with one fixed-size page and Info dictionary metadata. Later
// milestones will replace the blank page with the real book layout pipeline.
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

	data, err := buildSkeletonPDF(skeletonDocument{
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
		Title:      bookTitle(c),
		Author:     bookAuthors(c),
	})
	if err != nil {
		return fmt.Errorf("build skeleton pdf: %w", err)
	}

	if log != nil {
		log.Debug("Writing PDF skeleton",
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

type skeletonDocument struct {
	PageWidth  float64
	PageHeight float64
	Title      string
	Author     string
}

func pageSizePoints(screen config.ScreenConfig) (float64, float64, error) {
	dpi := screen.DPI
	if dpi == 0 {
		dpi = defaultDPI
	}
	if screen.Width <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen width: %d", screen.Width)
	}
	if screen.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen height: %d", screen.Height)
	}
	if dpi <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen dpi: %d", screen.DPI)
	}

	return float64(screen.Width) * 72.0 / float64(dpi), float64(screen.Height) * 72.0 / float64(dpi), nil
}

func buildSkeletonPDF(doc skeletonDocument) ([]byte, error) {
	writer := pdfdoc.NewWriter(pdfVersion)

	const (
		catalogID = 1
		pagesID   = 2
		pageID    = 3
		contentID = 4
		infoID    = 5
	)

	if err := writer.Object(catalogID, pdfdoc.Dict{
		"Pages": pdfdoc.Ref{ObjectNumber: pagesID},
		"Type":  pdfdoc.Name("Catalog"),
	}); err != nil {
		return nil, err
	}
	if err := writer.Object(pagesID, pdfdoc.Dict{
		"Count": pdfdoc.Integer(1),
		"Kids":  pdfdoc.Array{pdfdoc.Ref{ObjectNumber: pageID}},
		"Type":  pdfdoc.Name("Pages"),
	}); err != nil {
		return nil, err
	}
	if err := writer.Object(pageID, pdfdoc.Dict{
		"Contents": pdfdoc.Ref{ObjectNumber: contentID},
		"MediaBox": pdfdoc.Array{
			pdfdoc.Integer(0),
			pdfdoc.Integer(0),
			pdfdoc.Number(doc.PageWidth),
			pdfdoc.Number(doc.PageHeight),
		},
		"Parent":    pdfdoc.Ref{ObjectNumber: pagesID},
		"Resources": pdfdoc.Dict{},
		"Type":      pdfdoc.Name("Page"),
	}); err != nil {
		return nil, err
	}

	stream, err := flateStream([]byte("q\nQ\n"))
	if err != nil {
		return nil, err
	}
	if err := writer.StreamObject(contentID, pdfdoc.Dict{
		"Filter": pdfdoc.Name("FlateDecode"),
	}, stream); err != nil {
		return nil, err
	}
	if err := writer.Object(infoID, infoDictionary(doc)); err != nil {
		return nil, err
	}

	infoRef := pdfdoc.Ref{ObjectNumber: infoID}
	return writer.Finish(pdfdoc.Trailer{
		Root: pdfdoc.Ref{ObjectNumber: catalogID},
		Info: &infoRef,
	})
}

func infoDictionary(doc skeletonDocument) pdfdoc.Dict {
	info := pdfdoc.Dict{
		"Creator":  pdfdoc.UTF16TextString("fbc"),
		"Producer": pdfdoc.UTF16TextString("fbc"),
	}
	if doc.Title != "" {
		info["Title"] = pdfdoc.UTF16TextString(doc.Title)
	}
	if doc.Author != "" {
		info["Author"] = pdfdoc.UTF16TextString(doc.Author)
	}
	return info
}

func flateStream(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("compress content stream: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finish content stream compression: %w", err)
	}
	return buf.Bytes(), nil
}

func bookTitle(c *content.Content) string {
	if c.Book == nil {
		return strings.TrimSuffix(c.SrcName, ".fb2")
	}
	if title := strings.TrimSpace(c.Book.Description.TitleInfo.BookTitle.Value); title != "" {
		return title
	}
	return strings.TrimSuffix(c.SrcName, ".fb2")
}

func bookAuthors(c *content.Content) string {
	if c.Book == nil {
		return ""
	}

	authors := make([]string, 0, len(c.Book.Description.TitleInfo.Authors))
	for i := range c.Book.Description.TitleInfo.Authors {
		name := authorName(&c.Book.Description.TitleInfo.Authors[i])
		if name != "" {
			authors = append(authors, name)
		}
	}
	return strings.Join(authors, ", ")
}

func authorName(author *fb2.Author) string {
	if author == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, part := range []string{author.FirstName, author.MiddleName, author.LastName} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) != 0 {
		return strings.Join(parts, " ")
	}
	return strings.TrimSpace(author.Nickname)
}
