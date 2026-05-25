package pdf

import (
	"os"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/fb2"
)

const (
	pdfDefaultCSSRootFontSize   = pdfBaseFontSize * 1.20
	pdfDefaultCSSRootLineHeight = pdfDefaultCSSRootFontSize * 1.30
)

func newPDFStyleResolverWithDefaultCSS(t *testing.T, extraCSS ...string) *pdfStyleResolver {
	t.Helper()
	data, err := os.ReadFile("../default.css")
	if err != nil {
		t.Fatalf("read default.css: %v", err)
	}
	stylesheets := []fb2.Stylesheet{{Type: "text/css", Data: string(data)}}
	for _, css := range extraCSS {
		stylesheets = append(stylesheets, fb2.Stylesheet{Type: "text/css", Data: css})
	}
	book := &fb2.FictionBook{Stylesheets: stylesheets}
	return newPDFStyleResolver(book, zaptest.NewLogger(t))
}

func newPDFStyleResolverWithCSS(t *testing.T, css string) *pdfStyleResolver {
	t.Helper()
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: css}}}
	return newPDFStyleResolver(book, zaptest.NewLogger(t))
}

func textWithParagraphLineCount(t *testing.T, face *builtinFontFace, style paragraphStyle, width float64, wantLines int, word string) string {
	t.Helper()
	for words := 1; words <= 80; words++ {
		parts := make([]string, words)
		for i := range parts {
			parts[i] = word
		}
		text := strings.Join(parts, " ")
		lines, err := layoutParagraph(face, text, style, width)
		if err != nil {
			t.Fatalf("layoutParagraph() error = %v", err)
		}
		if len(lines) == wantLines {
			return text
		}
	}
	t.Fatalf("could not build paragraph with %d lines", wantLines)
	return ""
}

func pageText(page pdfPage) string {
	parts := make([]string, 0, len(page.Lines))
	for _, line := range page.Lines {
		parts = append(parts, shapedRunes(line.Text))
	}
	return strings.Join(parts, "\n")
}
