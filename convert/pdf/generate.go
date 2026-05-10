package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
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
	writer := newPDFWriter()
	writer.writeHeader()

	const (
		catalogID = 1
		pagesID   = 2
		pageID    = 3
		contentID = 4
		infoID    = 5
	)

	writer.object(catalogID, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesID))
	writer.object(pagesID, fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1 >>", pageID))
	writer.object(pageID, fmt.Sprintf(
		"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources << >> /Contents %d 0 R >>",
		pagesID,
		formatPDFNumber(doc.PageWidth),
		formatPDFNumber(doc.PageHeight),
		contentID,
	))

	stream, err := flateStream([]byte("q\nQ\n"))
	if err != nil {
		return nil, err
	}
	writer.streamObject(contentID, "<< /Filter /FlateDecode", stream)
	writer.object(infoID, infoDictionary(doc))

	writer.writeXrefAndTrailer(catalogID, infoID)
	return writer.bytes(), nil
}

func infoDictionary(doc skeletonDocument) string {
	parts := []string{
		"/Creator " + pdfUnicodeString("fbc"),
		"/Producer " + pdfUnicodeString("fbc"),
	}
	if doc.Title != "" {
		parts = append(parts, "/Title "+pdfUnicodeString(doc.Title))
	}
	if doc.Author != "" {
		parts = append(parts, "/Author "+pdfUnicodeString(doc.Author))
	}
	return "<< " + strings.Join(parts, " ") + " >>"
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

func formatPDFNumber(n float64) string {
	formatted := strconv.FormatFloat(n, 'f', 4, 64)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "-0" {
		return "0"
	}
	return formatted
}

func pdfUnicodeString(s string) string {
	words := utf16.Encode([]rune(s))
	data := make([]byte, 2, 2+len(words)*2)
	data[0] = 0xfe
	data[1] = 0xff
	for _, word := range words {
		data = append(data, byte(word>>8), byte(word))
	}
	return "<" + strings.ToUpper(hex.EncodeToString(data)) + ">"
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

type pdfWriter struct {
	buf     bytes.Buffer
	offsets map[int]int
}

func newPDFWriter() *pdfWriter {
	return &pdfWriter{offsets: make(map[int]int)}
}

func (w *pdfWriter) writeHeader() {
	fmt.Fprintf(&w.buf, "%%PDF-%s\n", pdfVersion)
	w.buf.WriteString("%\xE2\xE3\xCF\xD3\n")
}

func (w *pdfWriter) object(id int, body string) {
	w.offsets[id] = w.buf.Len()
	fmt.Fprintf(&w.buf, "%d 0 obj\n%s\nendobj\n", id, body)
}

func (w *pdfWriter) streamObject(id int, dictPrefix string, data []byte) {
	w.offsets[id] = w.buf.Len()
	fmt.Fprintf(&w.buf, "%d 0 obj\n%s /Length %d >>\nstream\n", id, dictPrefix, len(data))
	w.buf.Write(data)
	w.buf.WriteString("\nendstream\nendobj\n")
}

func (w *pdfWriter) writeXrefAndTrailer(rootID, infoID int) {
	startXref := w.buf.Len()
	maxID := 0
	for id := range w.offsets {
		maxID = max(maxID, id)
	}

	fmt.Fprintf(&w.buf, "xref\n0 %d\n", maxID+1)
	w.buf.WriteString("0000000000 65535 f \n")
	for id := 1; id <= maxID; id++ {
		fmt.Fprintf(&w.buf, "%010d 00000 n \n", w.offsets[id])
	}
	fmt.Fprintf(&w.buf, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxID+1, rootID, infoID, startXref)
}

func (w *pdfWriter) bytes() []byte {
	return w.buf.Bytes()
}
