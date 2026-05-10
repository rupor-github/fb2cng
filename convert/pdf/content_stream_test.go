package pdf

import (
	"strings"
	"testing"
)

func TestPageContentDrawsBackgroundsAndImages(t *testing.T) {
	content := string(pageContent(pdfPage{Backgrounds: []pdfPageRect{{
		X:      1,
		Y:      2,
		Width:  3,
		Height: 4,
		Color:  pdfColor{G: 1},
	}}, Images: []pdfPageImage{{
		Name:   "Im1",
		X:      10,
		Y:      20,
		Width:  30,
		Height: 40,
	}}}))
	for _, want := range []string{"0 1 0 rg", "1 2 3 4 re f", "30 0 0 40 10 20 cm", "/Im1 Do"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}

func TestPageContentSwitchesFontResourcesAndColors(t *testing.T) {
	content := string(pageContent(pdfPage{Lines: []pdfPageLine{{
		X:             10,
		Y:             20,
		FontSize:      10,
		LetterSpacing: 1.5,
		FontName:      "F1",
		Color:         pdfColor{R: 1},
		Underline:     true,
		Text:          shapedText{Glyphs: []shapedGlyph{{GlyphID: 1, Rune: 'A', Width: 600}}},
	}, {
		X:             10,
		Y:             8,
		FontSize:      10,
		LetterSpacing: 0,
		FontName:      "F2",
		Color:         pdfColor{B: 1},
		Strikethrough: true,
		Text:          shapedText{Glyphs: []shapedGlyph{{GlyphID: 2, Rune: 'B', Width: 500}}},
	}}}))
	for _, want := range []string{"/F1 10 Tf", "1.5 Tc", "1 0 0 rg", "/F2 10 Tf", "0 Tc", "0 0 1 rg", "<0001> Tj", "<0002> Tj", "1 0 0 RG", "10 18.8 m 16 18.8 l S", "0 0 1 RG", "10 11 m 15 11 l S"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}
