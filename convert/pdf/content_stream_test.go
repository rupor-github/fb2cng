package pdf

import (
	"strings"
	"testing"
)

func TestPageContentDrawsImages(t *testing.T) {
	content := string(pageContent(pdfPage{Images: []pdfPageImage{{
		Name:   "Im1",
		X:      10,
		Y:      20,
		Width:  30,
		Height: 40,
	}}}))
	for _, want := range []string{"30 0 0 40 10 20 cm", "/Im1 Do"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}

func TestPageContentSwitchesFontResourcesAndColors(t *testing.T) {
	content := string(pageContent(pdfPage{Lines: []pdfPageLine{{
		X:        10,
		Y:        20,
		FontSize: 10,
		FontName: "F1",
		Color:    pdfColor{R: 1},
		Text:     shapedText{Glyphs: []shapedGlyph{{GlyphID: 1, Rune: 'A'}}},
	}, {
		X:        10,
		Y:        8,
		FontSize: 10,
		FontName: "F2",
		Color:    pdfColor{B: 1},
		Text:     shapedText{Glyphs: []shapedGlyph{{GlyphID: 2, Rune: 'B'}}},
	}}}))
	for _, want := range []string{"/F1 10 Tf", "1 0 0 rg", "/F2 10 Tf", "0 0 1 rg", "<0001> Tj", "<0002> Tj"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}
