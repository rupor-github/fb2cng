package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestPDFPageFramePreservesPageContent(t *testing.T) {
	cover := &fb2.BookImage{MimeType: "image/png", Data: testPNG(t, 2, 3)}
	cover.Dim.Width, cover.Dim.Height = 2, 3
	for _, test := range []struct {
		name string
		doc  pdfDocumentSpec
	}{
		{name: "blank", doc: pdfDocumentSpec{PageWidth: 220, PageHeight: 180}},
		{name: "cover", doc: pdfDocumentSpec{
			PageWidth: 220, PageHeight: 180, ScreenDPI: 300,
			CoverID: "cover", Images: fb2.BookImages{"cover": cover},
		}},
		{name: "negative margins and multiple pages", doc: pdfDocumentSpec{
			PageWidth: 220, PageHeight: 180,
			Styles: newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `
				html { margin: 0 -20pt; }
				p { margin: 0 -8pt; background-color: #eee; border: 1pt solid red; }
			`}}}, nil, nil),
			Blocks: []pdfTextBlock{
				{Kind: pdfBlockParagraph, Text: "Text extending into the margins."},
				{Kind: pdfBlockPageBreak},
				{Kind: pdfBlockParagraph, Text: "Another page with unchanged text and positioning."},
			},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseline, err := buildPDFDocument(test.doc)
			if err != nil {
				t.Fatal(err)
			}
			test.doc.PageFrameWidth = 0.5
			framed, err := buildPDFDocument(test.doc)
			if err != nil {
				t.Fatal(err)
			}
			originalStreams := pdfPageContentStreams(t, baseline)
			framedStreams := pdfPageContentStreams(t, framed)
			if len(framedStreams) != len(originalStreams) {
				t.Fatalf("page count changed: %d to %d", len(originalStreams), len(framedStreams))
			}
			const frame = "q\n0.996 0.996 0.996 RG\n0.5 w\n0.25 0.25 219.5 179.5 re S\nQ\n"
			for i, stream := range framedStreams {
				if !bytes.HasPrefix(stream, []byte(frame)) {
					t.Fatalf("page %d missing underlaid frame: %s", i+1, stream)
				}
				if !bytes.Equal(stream[len(frame):], originalStreams[i]) {
					t.Fatalf("page %d content changed beyond adding the frame", i+1)
				}
			}
			if count := bytes.Count(framed, []byte("/MediaBox [0 0 220 180]")); count != len(framedStreams) {
				t.Fatalf("page dimensions changed: %d matching MediaBoxes", count)
			}
		})
	}
}

func TestGeneratePDFPageFrame(t *testing.T) {
	for _, test := range []struct {
		value, prefix string
	}{
		{value: "0", prefix: "q\nBT\n"},
		{value: "0.5pt", prefix: "q\n0.996 0.996 0.996 RG\n0.5 w\n0.25 0.25 302.86 402.7 re S\nQ\n"},
		{value: "1px", prefix: "q\n0.996 0.996 0.996 RG\n0.24 w\n0.12 0.12 303.12 402.96 re S\nQ\n"},
	} {
		t.Run(test.value, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "book.pdf")
			cfg := &config.DocumentConfig{
				Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300}},
				PDF:    config.PDFConfig{PageFrame: test.value},
			}
			c := &content.Content{Book: &fb2.FictionBook{}}
			if err := Generate(context.Background(), c, path, "", cfg, nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			streams := pdfPageContentStreams(t, data)
			if len(streams) != 1 || !bytes.HasPrefix(streams[0], []byte(test.prefix)) {
				t.Fatalf("unexpected page content: %q", streams)
			}
		})
	}
}

// pdfPageContentStreams decodes only the streams referenced by page Contents,
// avoiding compressed image/font streams in the same generated document.
func pdfPageContentStreams(t *testing.T, data []byte) [][]byte {
	t.Helper()
	refs := regexp.MustCompile(`/Contents (\d+) 0 R`).FindAllSubmatch(data, -1)
	if len(refs) == 0 {
		t.Fatal("no PDF page Contents references")
	}
	streams := make([][]byte, 0, len(refs))
	for _, ref := range refs {
		header := "\n" + string(ref[1]) + " 0 obj\n"
		_, object, found := strings.Cut(string(data), header)
		if !found {
			t.Fatalf("missing object %s", ref[1])
		}
		_, compressed, found := strings.Cut(object, "\nstream\n")
		if !found {
			t.Fatalf("missing stream for object %s", ref[1])
		}
		reader, err := zlib.NewReader(strings.NewReader(compressed))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("decode page stream: %v, close: %v", err, closeErr)
		}
		streams = append(streams, decoded)
	}
	return streams
}
