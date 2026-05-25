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

	out := appendPDFPrintedFootnotePagePlans(doc, mainPages, []pdfPrintedFootnotePagePlan{plan}, 80, nil, nil)
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
	if len(out[0].Backgrounds) == 0 {
		t.Fatalf("first page backgrounds=%#v, want separator on source page", out[0].Backgrounds)
	}
	if len(out[1].Backgrounds) == 0 {
		t.Fatalf("continuation backgrounds=%#v, want top continuation separator", out[1].Backgrounds)
	}
	if len(out[1].Strokes) != pdfPrintedFootnoteContinuationMarkerChevrons*2 {
		t.Fatalf("continuation strokes=%#v, want vector chevron marker", out[1].Strokes)
	}
	markerTop, markerBottom, ok := pageStrokesYBounds(out[1].Strokes)
	if !ok {
		t.Fatalf("continuation strokes = %#v, want marker strokes", out[1].Strokes)
	}
	footnoteLine, ok := pageLineByText(out[1], "Footnote continuation")
	if !ok {
		t.Fatalf("continuation page text = %q, want footnote line", pageText(out[1]))
	}
	if markerTop <= footnoteLine.Y || markerBottom <= footnoteLine.Y {
		t.Fatalf("continuation marker y=%v..%v footnote y=%v, want separator marker above footnote", markerBottom, markerTop, footnoteLine.Y)
	}
	if gotBottom := footnoteLine.Y - footnoteLine.FontSize*0.2; gotBottom < 23.999 || gotBottom > 24.001 {
		t.Fatalf("continuation footnote visual bottom = %v, want content bottom 24", gotBottom)
	}
}

func TestAppendPDFPrintedFootnotePagePlansBottomAlignsSourceFootnote(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{PageWidth: 260, PageHeight: 180}
	mainPages := []pdfPage{{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main", 130)}}}
	footnoteLine := testPDFPageLine(t, face, "Footnote", 60)
	plan := pdfPrintedFootnotePagePlan{PageIndex: 0, QueuePages: []pdfPage{{Lines: []pdfPageLine{footnoteLine}}}}
	separatorBefore := pdfPrintedFootnoteSeparatorMetricsForArea(doc, nil, 24, 212, 24, 80)

	out := appendPDFPrintedFootnotePagePlans(doc, mainPages, []pdfPrintedFootnotePagePlan{plan}, 80, nil, nil)
	if len(out) != 1 {
		t.Fatalf("pages = %d, want one source page", len(out))
	}
	footnoteY, ok := pageLineYByText(out[0], "Footnote")
	if !ok {
		t.Fatalf("page text = %q, want footnote line", pageText(out[0]))
	}
	if gotBottom := footnoteY - footnoteLine.FontSize*0.2; gotBottom < 23.999 || gotBottom > 24.001 {
		t.Fatalf("footnote visual bottom = %v, want content bottom 24", gotBottom)
	}
	if len(out[0].Backgrounds) == 0 || out[0].Backgrounds[0].Y >= separatorBefore.Y {
		t.Fatalf("separator y = %#v, want source separator shifted down from max reserved area %v", out[0].Backgrounds, separatorBefore.Y)
	}
}

func TestAppendPDFPrintedFootnotePagePlansBottomAlignsPackedContinuationChunks(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	plan := pdfPrintedFootnotePagePlan{
		PageIndex: 0,
		QueuePages: []pdfPage{
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote first part", 60)}},
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote continuation one", 60)}},
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote continuation two", 60)}},
		},
	}

	out := appendPDFPrintedFootnotePagePlans(
		pdfDocumentSpec{PageWidth: 260, PageHeight: 180},
		[]pdfPage{{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main page 1", 130)}}, {Lines: []pdfPageLine{testPDFPageLine(t, face, "Main page 2", 130)}}},
		[]pdfPrintedFootnotePagePlan{plan},
		80,
		nil,
		nil,
	)
	if len(out) != 3 {
		t.Fatalf("pages = %d, want source page, packed continuation page, next main page", len(out))
	}
	if got := pageText(out[1]); !strings.Contains(got, "Footnote continuation one") || !strings.Contains(got, "Footnote continuation two") || strings.Contains(got, "Main page 2") {
		t.Fatalf("packed continuation text = %q, want both continuation chunks before next main page", got)
	}
	firstLine, firstOK := pageLineByText(out[1], "Footnote continuation one")
	secondLine, secondOK := pageLineByText(out[1], "Footnote continuation two")
	if !firstOK || !secondOK || firstLine.Y <= secondLine.Y {
		t.Fatalf("packed continuation lines = %#v, want chunks stacked top-down", out[1].Lines)
	}
	if gotBottom := secondLine.Y - secondLine.FontSize*0.2; gotBottom < 23.999 || gotBottom > 24.001 {
		t.Fatalf("packed continuation visual bottom = %v, want content bottom 24", gotBottom)
	}
	if len(out[1].Backgrounds) == 0 || out[1].Backgrounds[0].Y <= firstLine.Y {
		t.Fatalf("packed continuation separator = %#v first line y=%v, want separator above bottom-anchored group", out[1].Backgrounds, firstLine.Y)
	}
}

func TestAppendPDFPrintedFootnotePagePlansBottomAlignsTallContinuationChunk(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	tallContinuation := pdfPage{Lines: []pdfPageLine{
		testPDFPageLine(t, face, "Tall continuation top", 140),
		testPDFPageLine(t, face, "Tall continuation bottom", 40),
	}}
	out := appendPDFPrintedFootnotePagePlans(
		pdfDocumentSpec{PageWidth: 260, PageHeight: 180},
		[]pdfPage{{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main", 130)}}},
		[]pdfPrintedFootnotePagePlan{{PageIndex: 0, QueuePages: []pdfPage{
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote first", 60)}},
			tallContinuation,
		}}},
		80,
		nil,
		nil,
	)
	if len(out) != 2 {
		t.Fatalf("pages = %d, want source and tall continuation page", len(out))
	}
	bottomLine, ok := pageLineByText(out[1], "Tall continuation bottom")
	if !ok {
		t.Fatalf("continuation page text = %q, want tall continuation", pageText(out[1]))
	}
	if gotBottom := bottomLine.Y - bottomLine.FontSize*0.2; gotBottom < 23.999 || gotBottom > 24.001 {
		t.Fatalf("tall continuation visual bottom = %v, want content bottom 24 despite top overflow", gotBottom)
	}
}

func TestAppendPDFPrintedFootnotePagePlansDrawsVectorContinuationMarkerWithoutGlyphs(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)
	out := appendPDFPrintedFootnotePagePlans(
		pdfDocumentSpec{PageWidth: 260, PageHeight: 180},
		[]pdfPage{{Lines: []pdfPageLine{testPDFPageLine(t, face, "Main", 130)}}},
		[]pdfPrintedFootnotePagePlan{{PageIndex: 0, QueuePages: []pdfPage{
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote first", 60)}},
			{Lines: []pdfPageLine{testPDFPageLine(t, face, "Footnote continuation", 60)}},
		}}},
		80,
		used,
		nil,
	)
	if len(out) != 2 || len(out[1].Strokes) != pdfPrintedFootnoteContinuationMarkerChevrons*2 {
		t.Fatalf("rendered pages = %#v, want vector continuation marker", out)
	}
	if strings.Contains(pageText(out[1]), ">") {
		t.Fatalf("continuation page text = %q, want vector marker not text", pageText(out[1]))
	}
	if usedGlyphCount(used) != 0 {
		t.Fatalf("used glyphs = %#v, want no marker glyph usage", used)
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
		nil,
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

func pageLineYByText(page pdfPage, text string) (float64, bool) {
	line, ok := pageLineByText(page, text)
	if !ok {
		return 0, false
	}
	return line.Y, true
}

func pageLineByText(page pdfPage, text string) (pdfPageLine, bool) {
	for _, line := range page.Lines {
		if shapedRunes(line.Text) == text {
			return line, true
		}
	}
	return pdfPageLine{}, false
}

func pageStrokesYBounds(strokes []pdfPageStroke) (float64, float64, bool) {
	if len(strokes) == 0 {
		return 0, 0, false
	}
	top := max(strokes[0].Y1, strokes[0].Y2)
	bottom := min(strokes[0].Y1, strokes[0].Y2)
	for _, stroke := range strokes[1:] {
		top = max(top, max(stroke.Y1, stroke.Y2))
		bottom = min(bottom, min(stroke.Y1, stroke.Y2))
	}
	return top, bottom, true
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
