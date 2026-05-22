package pdf

import (
	"strings"
	"testing"
)

func TestPageContentDrawsBackgroundsBordersAndImages(t *testing.T) {
	content := string(pageContent(pdfPage{Backgrounds: []pdfPageRect{{
		X:      1,
		Y:      2,
		Width:  3,
		Height: 4,
		Color:  pdfColor{G: 1},
	}}, Borders: []pdfPageBorder{{
		X:         5,
		Y:         6,
		Width:     7,
		Height:    8,
		LineWidth: 1.5,
		Color:     pdfColor{R: 1},
	}}, Images: []pdfPageImage{{
		Name:   "Im1",
		X:      10,
		Y:      20,
		Width:  30,
		Height: 40,
	}}}))
	for _, want := range []string{"0 1 0 rg", "1 2 3 4 re f", "1 0 0 RG", "1.5 w", "5 6 7 8 re S", "30 0 0 40 10 20 cm", "/Im1 Do"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}

func TestPageContentDecoratesLinkedInlineImages(t *testing.T) {
	content := string(pageContent(pdfPage{Lines: []pdfPageLine{{
		X: 10,
		Y: 20,
		Fragments: []pdfPageLineFragment{{
			Width:         20,
			FontSize:      10,
			Color:         pdfColor{R: 1},
			Underline:     true,
			BaselineShift: -1,
			ImageID:       "inline",
			ImageHeight:   12,
		}},
	}}}))
	for _, want := range []string{"1 0 0 RG", "10 17.8 m 30 17.8 l S"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}

func TestMissingPDFGlyphBoxOccupiesFullCell(t *testing.T) {
	box := missingPDFGlyphBox(shapedGlyph{Rune: 'á', Width: 500, Missing: pdfMissingGlyphPrintable}, 10, 20, 30, pdfColor{})
	if box.X != 20 || box.Y != 27.5 || box.Width != 5 || box.Height != 10 {
		t.Fatalf("missing glyph box = %#v, want full 5x10 cell centered around baseline", box)
	}
}

func TestPageContentUsesSyntheticMissingGlyphBoxes(t *testing.T) {
	content := string(pageContent(pdfPage{Lines: []pdfPageLine{{
		X:        10,
		Y:        20,
		FontSize: 10,
		FontName: "F1",
		Color:    pdfColor{},
		Text: shapedText{Glyphs: []shapedGlyph{
			{GlyphID: 1, Rune: 'A', Width: 600},
			{Rune: 'á', Width: 500, Missing: pdfMissingGlyphPrintable},
			{Rune: ' ', Width: 500, Missing: pdfMissingGlyphSpace},
			{Rune: '\u0301', Width: 0, Missing: pdfMissingGlyphCombining},
		}},
	}}}))
	if strings.Contains(content, "<0000>") {
		t.Fatalf("page content emitted CID 0 for missing glyph: %q", content)
	}
	for _, want := range []string{"<0001>", "-500", " re S"} {
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
	for _, want := range []string{
		"/F1 10 Tf",
		"1.5 Tc",
		"1 0 0 rg",
		"/F2 10 Tf",
		"0 Tc",
		"0 0 1 rg",
		"<0001> Tj",
		"<0002> Tj",
		"1 0 0 RG",
		"10 18.8 m 16 18.8 l S",
		"0 0 1 RG",
		"10 11 m 15 11 l S",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}
