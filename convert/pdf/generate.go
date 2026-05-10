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
// This initial implementation creates a valid PDF 1.4 file with one fixed-size
// page, embedded Unicode font resources, selectable title/author text, and Info
// dictionary metadata. Later milestones will replace the title page scaffold
// with the real book layout pipeline.
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
		catalogID        = 1
		pagesID          = 2
		pageID           = 3
		contentID        = 4
		infoID           = 5
		type0FontID      = 6
		cidFontID        = 7
		fontDescriptorID = 8
		fontFileID       = 9
		toUnicodeID      = 10
	)

	fontFace, err := builtinFont("sans-serif", false, false)
	if err != nil {
		return nil, err
	}
	titleText := strings.TrimSpace(doc.Title)
	if titleText == "" {
		titleText = "Untitled"
	}
	authorText := strings.TrimSpace(doc.Author)
	if authorText == "" {
		authorText = "fbc"
	}
	title, err := shapeText(fontFace, titleText)
	if err != nil {
		return nil, fmt.Errorf("shape title: %w", err)
	}
	authorLines, err := wrapText(fontFace, authorText, 9, doc.PageWidth-48)
	if err != nil {
		return nil, fmt.Errorf("shape author: %w", err)
	}
	usedText := append([]shapedText{title}, authorLines...)
	fontObjs, err := fontResourceObjects(fontFace, mergeUsedGlyphs(usedText...), fontObjectIDs{
		Type0Font:      type0FontID,
		CIDFont:        cidFontID,
		FontDescriptor: fontDescriptorID,
		FontFile:       fontFileID,
		ToUnicode:      toUnicodeID,
	})
	if err != nil {
		return nil, err
	}

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
		"Parent": pdfdoc.Ref{ObjectNumber: pagesID},
		"Resources": pdfdoc.Dict{
			"Font": pdfdoc.Dict{
				"F1": pdfdoc.Ref{ObjectNumber: type0FontID},
			},
		},
		"Type": pdfdoc.Name("Page"),
	}); err != nil {
		return nil, err
	}

	stream, err := flateStream(titlePageContent(doc, title, authorLines))
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
	if err := writer.Object(type0FontID, fontObjs.Type0Font); err != nil {
		return nil, err
	}
	if err := writer.Object(cidFontID, fontObjs.CIDFont); err != nil {
		return nil, err
	}
	if err := writer.Object(fontDescriptorID, fontObjs.FontDescriptor); err != nil {
		return nil, err
	}
	if err := writer.StreamObject(fontFileID, fontObjs.FontFile, fontObjs.FontFileData); err != nil {
		return nil, err
	}
	if err := writer.StreamObject(toUnicodeID, pdfdoc.Dict{}, fontObjs.ToUnicode); err != nil {
		return nil, err
	}

	infoRef := pdfdoc.Ref{ObjectNumber: infoID}
	return writer.Finish(pdfdoc.Trailer{
		Root: pdfdoc.Ref{ObjectNumber: catalogID},
		Info: &infoRef,
	})
}

func titlePageContent(doc skeletonDocument, title shapedText, authorLines []shapedText) []byte {
	x := 24.0
	titleY := doc.PageHeight - 54.0
	authorY := titleY - 20.0
	if authorY < 24.0 {
		authorY = 24.0
	}

	var buf bytes.Buffer
	buf.WriteString("q\nBT\n")
	fmt.Fprintf(&buf, "/F1 14 Tf\n1 0 0 1 %s %s Tm\n%s Tj\n",
		pdfdoc.FormatNumber(x),
		pdfdoc.FormatNumber(titleY),
		pdfdoc.Format(glyphHex(title.Glyphs)),
	)
	buf.WriteString("/F1 9 Tf\n")
	for i, line := range authorLines {
		y := authorY - float64(i)*11.0
		if y < 24.0 {
			break
		}
		fmt.Fprintf(&buf, "1 0 0 1 %s %s Tm\n%s Tj\n",
			pdfdoc.FormatNumber(x),
			pdfdoc.FormatNumber(y),
			pdfdoc.Format(glyphHex(line.Glyphs)),
		)
	}
	buf.WriteString("ET\nQ\n")
	return buf.Bytes()
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
