package pdf

import (
	"strings"
	"testing"
)

func TestAppendPDFPrintedFootnotePagePlansInsertsContinuationBeforeNextMainPage(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{PageWidth: 260, PageHeight: 180}
	mainPages := []pdfPage{
		{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main page 1", 130)}},
		{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main page 2", 130)}},
	}
	plan := pdfPrintedFootnotePagePlan{
		PageIndex: 0,
		QueuePages: []pdfPage{
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote first part", 60)}},
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote continuation", 60)}},
		},
	}

	out := appendPDFPrintedFootnotePagePlans(doc, mainPages, []pdfPrintedFootnotePagePlan{plan}, 80, nil)
	if len(out) != 3 {
		t.Fatalf("pages = %d, want main page, continuation page, next main page", len(out))
	}
	if got := pageText(out[0]); !strings.Contains(got, "Main page 1") || !strings.Contains(got, "Footnote first part") {
		t.Fatalf("first page text = %q, want main and first footnote queue page", got)
	}
	if got := pageText(out[1]); !strings.Contains(got, "Footnote continuation") || strings.Contains(got, "Main page 2") {
		t.Fatalf("continuation page text = %q, want only footnote continuation before next main page", got)
	}
	if got := pageText(out[2]); !strings.Contains(got, "Main page 2") {
		t.Fatalf("third page text = %q, want next main page after footnote continuation", got)
	}
	if len(out[0].Backgrounds) == 0 || len(out[1].Backgrounds) == 0 {
		t.Fatalf("backgrounds first=%#v continuation=%#v, want separator on rendered footnote pages", out[0].Backgrounds, out[1].Backgrounds)
	}
}

func TestAppendPDFPrintedFootnotePagePlansMergesUsedGlyphs(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	line := testPDFPageLine(t, face, "Z", 60)
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)
	planUsed := make(map[pdfFontKey]map[uint16]shapedGlyph)
	collectPDFLineUsedGlyphs(planUsed, line)
	out := appendPDFPrintedFootnotePagePlans(
		pdfDocumentSpec{PageWidth: 260, PageHeight: 180},
		[]pdfPage{{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main", 130)}}},
		[]pdfPrintedFootnotePagePlan{{PageIndex: 0, QueuePages: []pdfPage{{Lines: []pdfPageLine{line}}}, UsedGlyphs: planUsed}},
		80,
		used,
	)
	if len(out) != 1 || !strings.Contains(pageText(out[0]), "Z") {
		t.Fatalf("rendered pages = %#v, want footnote text appended", out)
	}
	merged := 0
	for _, glyphs := range used {
		merged += len(glyphs)
	}
	if merged == 0 {
		t.Fatalf("used glyphs = %#v, want merged footnote glyphs", used)
	}
}

func TestPDFPrintedFootnoteSeparatorMetricsUsesContentWidth(t *testing.T) {
	doc := pdfDocumentSpec{PageWidth: 260, PageHeight: 180}
	metrics := pdfPrintedFootnoteSeparatorMetricsForArea(doc, nil, 24, 212, 24, 80)
	if metrics.Width <= 0 || metrics.Width > 212 {
		t.Fatalf("separator width = %v, want within content width", metrics.Width)
	}
	if metrics.Y <= 24+80 {
		t.Fatalf("separator y = %v, want above footnote text area", metrics.Y)
	}
}

func testPDFPageLine(t *testing.T, face *builtinFontFace, text string, y float64) pdfPageLine {
	t.Helper()
	shaped, err := shapeText(face, text)
	if err != nil {
		t.Fatalf("shapeText(%q) error = %v", text, err)
	}
	return pdfPageLine{
		X:        24,
		Y:        y,
		FontSize: pdfBaseFontSize,
		FontKey:  face.Key,
		Text:     shaped,
	}
}
