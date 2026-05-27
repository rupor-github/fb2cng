package pdf

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"fbc/fb2"
)

func TestPDFDropcapInlineRunSplitPreservesFirstRuneStyle(t *testing.T) {
	var blocks []pdfTextBlock
	paragraph := &fb2.Paragraph{
		Style: "intro has-dropcap",
		Text: []fb2.InlineSegment{{
			Kind: fb2.InlineStrong,
			Children: []fb2.InlineSegment{{
				Kind:     fb2.InlineLink,
				Href:     "https://example.com",
				Children: []fb2.InlineSegment{{Text: "Lorem"}},
			}},
		}, {Text: " ipsum"}},
	}

	appendParagraphBlock(&blocks, pdfParagraphBlockOptions{Kind: pdfBlockParagraph, Paragraph: paragraph})

	if len(blocks) != 1 {
		t.Fatalf("blocks = %#v, want one paragraph", blocks)
	}
	runs := blocks[0].Runs
	if len(runs) < 3 {
		t.Fatalf("runs = %#v, want dropcap, rest of linked text, trailing text", runs)
	}
	if runs[0].Text != "L" || !inlineRunHasStyleClass(runs[0], pdfStyleDropcap) {
		t.Fatalf("dropcap run = %#v, want first rune with dropcap class", runs[0])
	}
	if !runs[0].Bold || runs[0].LinkHref != "https://example.com" {
		t.Fatalf("dropcap run style = %#v, want inherited bold/link", runs[0])
	}
	if runs[1].Text != "orem" || inlineRunHasStyleClass(runs[1], pdfStyleDropcap) || !runs[1].Bold || runs[1].LinkHref != "https://example.com" {
		t.Fatalf("body linked run = %#v, want rest without dropcap class but with inherited style", runs[1])
	}
	if blocks[0].Text != "Lorem ipsum" {
		t.Fatalf("block text = %q, want original text", blocks[0].Text)
	}
}

func TestPDFDropcapDetectionAndLayoutTracing(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	tracer := newPDFStyleTracer(t.TempDir())
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `
		p.has-dropcap .dropcap {
			float: left;
			font-size: 4.1em;
			line-height: 1;
			font-weight: bold;
			padding-right: 0.2em;
		}
	`}}}

	resolver := newPDFStyleResolver(book, zap.New(core), tracer)

	logEntries := logs.FilterMessage("Detected drop cap pattern").All()
	if len(logEntries) != 1 {
		t.Fatalf("dropcap detection log entries = %d, want 1", len(logEntries))
	}
	logFields := pdfZapStringFields(logEntries[0])
	if logFields["parent"] != "has-dropcap" || logFields["unit"] != "em" {
		t.Fatalf("dropcap detection log fields = %#v, want parent has-dropcap and unit em", logFields)
	}
	if resolver.dropcaps["has-dropcap"].Lines != 4 {
		t.Fatalf("dropcap lines = %d, want rounded 4", resolver.dropcaps["has-dropcap"].Lines)
	}
	cssEntries := pdfTraceEntriesByOperation(tracer, "DROPCAP CSS")
	if len(cssEntries) != 1 {
		t.Fatalf("dropcap CSS trace entries = %d, want 1", len(cssEntries))
	}
	if !strings.Contains(cssEntries[0].Details, "parent=has-dropcap") || !strings.Contains(cssEntries[0].Details, "font-size=4.1em") {
		t.Fatalf("dropcap CSS trace details = %q", cssEntries[0].Details)
	}

	_, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 220,
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockParagraph,
			Text:         "Lorem ipsum dolor sit amet consectetur",
			Runs:         addPDFDropcapInlineRun([]pdfInlineRun{{Text: "Lorem ipsum dolor sit amet consectetur"}}),
			StyleClasses: "has-dropcap",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	resolvedEntries := pdfTraceEntriesByOperation(tracer, "DROPCAP")
	if len(resolvedEntries) != 1 {
		t.Fatalf("resolved dropcap trace entries = %d, want 1", len(resolvedEntries))
	}
	if !strings.Contains(resolvedEntries[0].Details, `char="L"`) || !strings.Contains(resolvedEntries[0].Details, "lines=4") {
		t.Fatalf("resolved dropcap trace details = %q", resolvedEntries[0].Details)
	}
}

func TestPDFDropcapFlowsFollowingParagraphAroundActiveExclusion(t *testing.T) {
	resolver := newPDFStyleResolverWithCSS(t, `
		p { font-size: 10pt; line-height: 12pt; margin: 0; text-indent: 6pt; }
		p.has-dropcap { text-indent: 0; margin: 0; }
		p.has-dropcap .dropcap { float: left; font-size: 3.2em; line-height: 1; font-weight: bold; padding-right: 1pt; }
	`)
	first := "Lorem."
	second := "Second paragraph keeps flowing beside the active dropcap before normal text resumes."

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 220,
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockParagraph,
			Text:         first,
			Runs:         addPDFDropcapInlineRun([]pdfInlineRun{{Text: first}}),
			StyleClasses: "has-dropcap",
		}, {
			Kind: pdfBlockParagraph,
			Text: second,
			Runs: []pdfInlineRun{{Text: second}},
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want one page", len(pages))
	}
	var dropcapLine, firstBodyLine, secondParagraphLine, secondParagraphContinuation *pdfPageLine
	for i := range pages[0].Lines {
		line := &pages[0].Lines[i]
		text := pdfPageLineText(*line)
		switch {
		case text == "L":
			dropcapLine = line
		case strings.HasPrefix(text, "orem"):
			firstBodyLine = line
		case strings.HasPrefix(text, "Second"):
			secondParagraphLine = line
		case secondParagraphLine != nil && secondParagraphContinuation == nil:
			secondParagraphContinuation = line
		}
	}
	if dropcapLine == nil || firstBodyLine == nil || secondParagraphLine == nil {
		t.Fatalf("lines = %#v, want dropcap, first body, and second paragraph lines", pages[0].Lines)
	}
	if dropcapLine.FontSize <= firstBodyLine.FontSize*2 {
		t.Fatalf("dropcap font size = %g, body font size = %g", dropcapLine.FontSize, firstBodyLine.FontSize)
	}
	if firstBodyLine.X <= dropcapLine.X+dropcapLine.WidthFromText() {
		t.Fatalf("first body X = %g, dropcap X/width = %g/%g, want body beside dropcap", firstBodyLine.X, dropcapLine.X, dropcapLine.WidthFromText())
	}
	if secondParagraphLine.X <= firstBodyLine.X {
		t.Fatalf(
			"second paragraph X = %g, first body X = %g, want following paragraph to preserve its indent while avoiding dropcap",
			secondParagraphLine.X,
			firstBodyLine.X,
		)
	}
	if secondParagraphContinuation == nil {
		t.Fatalf("lines = %#v, want second paragraph continuation line", pages[0].Lines)
	}
	if secondParagraphContinuation.X >= secondParagraphLine.X {
		t.Fatalf(
			"second paragraph continuation X = %g, first line X = %g, want line-by-line dropcap exclusion to expire",
			secondParagraphContinuation.X,
			secondParagraphLine.X,
		)
	}
}

func TestPDFDropcapFollowingParagraphStillExcludesWhenBaselinePassedVisualBottom(t *testing.T) {
	resolver := newPDFStyleResolverWithCSS(t, `
		p { font-size: 10pt; line-height: 10pt; margin: 0; text-indent: 0; }
		p.has-dropcap { text-indent: 0; margin: 0; }
		p.has-dropcap .dropcap { float: left; font-size: 3em; line-height: 1; font-weight: bold; padding-right: 1pt; }
	`)
	first := "Lorem ipsum dolor sit amet."
	second := "Second paragraph must still flow around the dropcap."

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  190,
		PageHeight: 180,
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockParagraph,
			Text:         first,
			Runs:         addPDFDropcapInlineRun([]pdfInlineRun{{Text: first}}),
			StyleClasses: "has-dropcap",
		}, {
			Kind: pdfBlockParagraph,
			Text: second,
			Runs: []pdfInlineRun{{Text: second}},
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want one page", len(pages))
	}
	var dropcapLine, secondParagraphLine *pdfPageLine
	firstBodyLines := 0
	for i := range pages[0].Lines {
		line := &pages[0].Lines[i]
		text := pdfPageLineText(*line)
		switch {
		case text == "L":
			dropcapLine = line
		case strings.HasPrefix(text, "orem") || strings.Contains(text, "amet"):
			firstBodyLines++
		case strings.HasPrefix(text, "Second"):
			secondParagraphLine = line
		}
	}
	if dropcapLine == nil || secondParagraphLine == nil {
		t.Fatalf("lines = %#v, want dropcap and following paragraph", pages[0].Lines)
	}
	if firstBodyLines != 2 {
		t.Fatalf("first body lines = %d, want two-line dropcap paragraph reproducer; lines = %#v", firstBodyLines, pages[0].Lines)
	}
	if secondParagraphLine.X <= dropcapLine.X+dropcapLine.WidthFromText() {
		t.Fatalf(
			"second paragraph X = %g, dropcap X/width = %g/%g, want line box to keep excluding active dropcap",
			secondParagraphLine.X,
			dropcapLine.X,
			dropcapLine.WidthFromText(),
		)
	}
}

func TestPDFDropcapStartsNextPageWhenShortBodyWouldFitButDropcapWouldNot(t *testing.T) {
	resolver := newPDFStyleResolverWithCSS(t, `
		p { font-size: 10pt; line-height: 12pt; margin: 0; text-indent: 0; }
		p.has-dropcap { text-indent: 0; margin: 0; }
		p.has-dropcap .dropcap { float: left; font-size: 3.2em; line-height: 1; font-weight: bold; padding-right: 0.1em; }
	`)
	lead := strings.Repeat("Previous paragraph consumes vertical space before the dropcap starts. ", 3)
	dropcapText := "Test."

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 150,
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: lead,
			Runs: []pdfInlineRun{{Text: lead}},
		}, {
			Kind:         pdfBlockParagraph,
			Text:         dropcapText,
			Runs:         addPDFDropcapInlineRun([]pdfInlineRun{{Text: dropcapText}}),
			StyleClasses: "has-dropcap",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("pages = %d, text=%q, want short dropcap paragraph moved to a following page", len(pages), pageText(pages[0]))
	}
	if strings.Contains(pageText(pages[0]), "T") {
		t.Fatalf("first page text = %q, want no short dropcap paragraph at page bottom", pageText(pages[0]))
	}
	if !strings.Contains(pageText(pages[1]), "T") {
		t.Fatalf("second page text = %q, want short dropcap paragraph", pageText(pages[1]))
	}
}

func TestPDFDropcapStartsNextPageWhenWrapLinesWouldSplit(t *testing.T) {
	resolver := newPDFStyleResolverWithCSS(t, `
		p { font-size: 10pt; line-height: 12pt; margin: 0; text-indent: 0; }
		p.has-dropcap { text-indent: 0; margin: 0; }
		p.has-dropcap .dropcap { float: left; font-size: 3.2em; line-height: 1; font-weight: bold; padding-right: 0.1em; }
	`)
	lead := strings.Repeat("Previous paragraph consumes vertical space before the dropcap starts. ", 3)
	dropcapText := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore."

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 150,
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: lead,
			Runs: []pdfInlineRun{{Text: lead}},
		}, {
			Kind:         pdfBlockParagraph,
			Text:         dropcapText,
			Runs:         addPDFDropcapInlineRun([]pdfInlineRun{{Text: dropcapText}}),
			StyleClasses: "has-dropcap",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("pages = %d, text=%q, want dropcap paragraph moved to a following page", len(pages), pageText(pages[0]))
	}
	if strings.Contains(pageText(pages[0]), "L") {
		t.Fatalf("first page text = %q, want no dropcap paragraph split at page bottom", pageText(pages[0]))
	}
	if !strings.Contains(pageText(pages[1]), "L") {
		t.Fatalf("second page text = %q, want dropcap paragraph", pageText(pages[1]))
	}
}

func pdfTraceEntriesByOperation(tracer *pdfStyleTracer, operation string) []pdfStyleTraceEntry {
	if tracer == nil {
		return nil
	}
	var entries []pdfStyleTraceEntry
	for _, entry := range tracer.entries {
		if entry.Operation == operation {
			entries = append(entries, entry)
		}
	}
	return entries
}

func pdfZapStringFields(entry observer.LoggedEntry) map[string]string {
	fields := make(map[string]string)
	for _, field := range entry.Context {
		if field.Type == zapcore.StringType {
			fields[field.Key] = field.String
		}
	}
	return fields
}
