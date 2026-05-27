package pdf

import (
	"math"
	"strings"
	"testing"

	"fbc/fb2"
)

func TestPDFTypographyValidationFixtureShapesKerningLigaturesAndCombiningMarks(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}

	for _, sample := range []string{"AVATAR", "To Wa Yo"} {
		t.Run("kerning "+sample, func(t *testing.T) {
			simple, err := simplePDFTextShaper{face: face}.Shape(sample, pdfShapeOptions{})
			if err != nil {
				t.Fatalf("simple shape %q: %v", sample, err)
			}
			openType, err := shapeTextWithCache(nil, face, sample)
			if err != nil {
				t.Fatalf("OpenType shape %q: %v", sample, err)
			}
			if got, want := shapedRunes(openType), sample; got != want {
				t.Fatalf("shaped text = %q, want %q", got, want)
			}
			if simpleWidth, openTypeWidth := shapedWidthPoints(simple, 12), shapedWidthPoints(openType, 12); openTypeWidth >= simpleWidth {
				t.Fatalf("OpenType width = %v, simple width = %v, want kerning to reduce width", openTypeWidth, simpleWidth)
			}
		})
	}

	for _, sample := range []string{"office", "afflict", "file"} {
		t.Run("ligature "+sample, func(t *testing.T) {
			shaped, err := shapeTextWithCache(nil, face, sample)
			if err != nil {
				t.Fatalf("shape text %q: %v", sample, err)
			}
			if got, want := shapedRunes(shaped), sample; got != want {
				t.Fatalf("shaped text = %q, want %q", got, want)
			}
			if len(shaped.Glyphs) >= len([]rune(sample)) {
				t.Fatalf(
					"glyph count = %d, rune count = %d, want at least one standard ligature: %#v",
					len(shaped.Glyphs),
					len([]rune(sample)),
					shaped.Glyphs,
				)
			}
			if !shapedTextHasMultiRuneCluster(shaped) {
				t.Fatalf("shaped glyphs = %#v, want a multi-rune ligature cluster", shaped.Glyphs)
			}
		})
	}

	for _, sample := range []string{"áéó", "a\u0301e\u0301o\u0301"} {
		t.Run("combining "+sample, func(t *testing.T) {
			shaped, err := shapeTextWithCache(nil, face, sample)
			if err != nil {
				t.Fatalf("shape text %q: %v", sample, err)
			}
			if got := shapedRunes(shaped); got != sample {
				t.Fatalf("shaped text = %q, want %q; glyphs=%#v", got, sample, shaped.Glyphs)
			}
		})
	}
}

func TestPDFTypographyValidationFixtureLayoutsScriptsAndJustification(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{FontSize: 10.08, LineHeight: 12, Align: textAlignJustify, Hyphenation: paragraphHyphenationNone}
	samples := []struct {
		name string
		text string
	}{
		{
			name: "cyrillic prose",
			text: "Съешь ещё этих мягких французских булок, да выпей чаю. " +
				"Обычный русский текст должен переноситься и выравниваться без потери букв.",
		},
		{
			name: "greek prose",
			text: "Ταχύ καφέ αλεπού πηδά πάνω από νωχελικό σκύλο. " +
				"Το ελληνικό κείμενο πρέπει να τυλίγεται και να εξάγεται σωστά.",
		},
	}

	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			lines, err := layoutParagraph(face, sample.text, style, 165)
			if err != nil {
				t.Fatalf("layoutParagraph() error = %v", err)
			}
			if len(lines) < 2 {
				t.Fatalf("layoutParagraph() produced %d lines, want wrapping", len(lines))
			}
			joined := strings.ReplaceAll(strings.Join(paragraphLineTexts(lines), " "), "- ", "")
			for _, word := range strings.Fields(sample.text)[:3] {
				if !strings.Contains(joined, strings.Trim(word, ".,")) {
					t.Fatalf("joined text = %q, want to contain %q", joined, word)
				}
			}
			for i, line := range lines[:len(lines)-1] {
				if line.JustificationGaps == 0 || !pdfParagraphLineIsJustified(line) {
					continue
				}
				if got, want := pdfParagraphLineDrawnWidth(line), line.BreakStats.AvailableWidth; math.Abs(got-want) > pdfLineWidthTolerance {
					t.Fatalf("line %d drawn width = %v, available = %v, text=%q", i, got, want, shapedRunes(line.Text))
				}
			}
		})
	}
}

func TestPDFLayoutValidationFixtureCoversInlineDecorationsFallbackAndImages(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 80
	img.Dim.Height = 40

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     180,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Styles:         newPDFStyleResolverWithDefaultCSS(t),
		Images:         fb2.BookImages{"inline": img},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "Styled fallback inline validation.",
			Runs: []pdfInlineRun{
				{Text: "Link", LinkHref: "https://example.invalid", Underline: true},
				{Text: " strike", Strikethrough: true},
				{Text: " note"},
				{Text: "7", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n1", Superscript: true},
				{Text: " math ≤ arrow → bullet ● box █ "},
				{ImageID: "inline", LinkHref: "#image-target", StyleClasses: pdfStyleLinkInternal},
			},
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Lines) != 1 {
		t.Fatalf("pages = %#v, want one page with one validation line", pages)
	}
	line := pages[0].Lines[0]
	if len(line.Fragments) == 0 {
		t.Fatalf("line fragments = %#v, want styled fragments", line.Fragments)
	}
	wantFallbacks := map[pdfFontKey]bool{
		{Family: pdfBuiltinFontFamilyMath}:     false,
		{Family: pdfBuiltinFontFamilySymbols2}: false,
		{Family: "monospace"}:                  false,
	}
	seenUnderline := false
	seenStrike := false
	seenFootnote := false
	seenImageFragment := false
	for _, fragment := range line.Fragments {
		if _, ok := wantFallbacks[fragment.FontKey]; ok {
			wantFallbacks[fragment.FontKey] = true
		}
		seenUnderline = seenUnderline || fragment.Underline
		seenStrike = seenStrike || fragment.Strikethrough
		seenFootnote = seenFootnote || fragment.FootnoteID == "n1" && fragment.BaselineShift > 0
		seenImageFragment = seenImageFragment || fragment.ImageID == "inline" && fragment.Width > 0 && fragment.ImageHeight > 0
	}
	for key, seen := range wantFallbacks {
		if !seen {
			t.Fatalf("fragments = %#v, want fallback font key %#v", line.Fragments, key)
		}
	}
	if !seenUnderline || !seenStrike || !seenFootnote || !seenImageFragment {
		t.Fatalf(
			"fragments = %#v, want underline=%t strikethrough=%t footnote=%t image=%t",
			line.Fragments,
			seenUnderline,
			seenStrike,
			seenFootnote,
			seenImageFragment,
		)
	}
	if len(pages[0].Images) != 1 || pages[0].Images[0].ImageID != "inline" {
		t.Fatalf("images = %#v, want placed inline image", pages[0].Images)
	}
	if len(pages[0].Annotations) < 2 {
		t.Fatalf("annotations = %#v, want text link and image link annotations", pages[0].Annotations)
	}
}

func shapedTextHasMultiRuneCluster(text shapedText) bool {
	for _, glyph := range text.Glyphs {
		if glyph.ClusterEnd-glyph.ClusterStart > 1 || len([]rune(glyphUnicodeText(glyph))) > 1 {
			return true
		}
	}
	return false
}

func paragraphLineTexts(lines []paragraphLine) []string {
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, shapedRunes(line.Text))
	}
	return texts
}

func pdfParagraphLineIsJustified(line paragraphLine) bool {
	return line.ExtraWordSpacing != 0 || line.ExtraCharSpacing != 0
}

func pdfParagraphLineDrawnWidth(line paragraphLine) float64 {
	pageLine := pdfPageLine{
		Text:             line.Text,
		Fragments:        pageLineFragments(line.Fragments),
		FontSize:         10.08,
		ExtraWordSpacing: line.ExtraWordSpacing,
		ExtraCharSpacing: line.ExtraCharSpacing,
	}
	return pdfPageLineDrawnWidth(pageLine)
}
