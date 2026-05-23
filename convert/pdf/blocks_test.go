package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/structure"
	"fbc/fb2"
)

func findPDFTOCEntry(entries []*structure.TOCEntry, title string) *structure.TOCEntry {
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Title == title {
			return entry
		}
		if found := findPDFTOCEntry(entry.Children, title); found != nil {
			return found
		}
	}
	return nil
}

func TestCollectPDFContentAddsAnnotationPage(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Book annotation."}}},
			}}},
		}},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:    "chapter-1",
				Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			}},
		}},
	}
	plan, err := collectPDFContent(&content.Content{Book: book}, &config.DocumentConfig{
		Annotation: config.AnnotationConfig{Enable: true, Title: "About", InTOC: true},
	})
	if err != nil {
		t.Fatalf("collectPDFContent() error = %v", err)
	}
	if len(plan.Blocks) < 4 {
		t.Fatalf("blocks = %#v, want annotation and chapter blocks", plan.Blocks)
	}
	if got := plan.Blocks[0]; got.Kind != pdfBlockPageBreak || got.ID != "annotation-page" {
		t.Fatalf("first block = %#v, want annotation page break", got)
	}
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "About" || got.StyleName != pdfStyleAnnotationTitle || got.StyleClasses != pdfStyleAnnotationTitle+"-first" || got.ContextClasses != pdfStyleAnnotationTitle {
		t.Fatalf("second block = %#v, want annotation heading via title helper", got)
	}
	if got := plan.Blocks[2]; got.Kind != pdfBlockParagraph || got.Text != "Book annotation." || got.StyleClasses != pdfStyleAnnotation {
		t.Fatalf("annotation paragraph = %#v", got)
	}
	if plan.Blocks[2].ContextClasses != pdfStyleAnnotation {
		t.Fatalf("annotation paragraph context = %q, want %q", plan.Blocks[2].ContextClasses, pdfStyleAnnotation)
	}
	if plan.Blocks[2].StripRootHorizontalMargins {
		t.Fatalf("generated annotation page paragraph should keep normal root margins")
	}
	if len(plan.TOC) == 0 || plan.TOC[0].ID != "annotation-page" || plan.TOC[0].Title != "About" {
		t.Fatalf("TOC = %#v, want annotation entry first", plan.TOC)
	}
}

func TestCollectTextBlocksKeepsMultiBlockSectionAnnotationOnNormalRootMargins(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "First annotation."}}}},
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Second annotation."}}}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var annotationBlocks []pdfTextBlock
	for _, block := range blocks {
		if block.Kind == pdfBlockParagraph && block.StyleClasses == pdfStyleAnnotation {
			annotationBlocks = append(annotationBlocks, block)
		}
	}
	if len(annotationBlocks) != 2 {
		t.Fatalf("annotation blocks = %#v, want 2 section annotation paragraphs", annotationBlocks)
	}
	for i, block := range annotationBlocks {
		if block.StripRootHorizontalMargins {
			t.Fatalf("annotation block %d should keep normal root margins like KFX annotation children: %#v", i, block)
		}
	}
}

func TestCollectTextBlocksKeepsSingleBlockSectionAnnotationOnNormalRootMargins(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Only annotation."}}},
			}}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	for _, block := range blocks {
		if block.Kind == pdfBlockParagraph && block.Text == "Only annotation." {
			if block.StripRootHorizontalMargins {
				t.Fatalf("single-block section annotation should keep normal root margins: %#v", block)
			}
			return
		}
	}
	t.Fatalf("single-block annotation paragraph not found in %#v", blocks)
}

func TestCollectTextBlocksPreservesRootHorizontalMarginsInAnnotationNestedCite(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Lead annotation."}}}},
				{Kind: fb2.FlowCite, Cite: &fb2.Cite{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested cite."}}},
				}}}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	for _, block := range blocks {
		if block.Kind == pdfBlockParagraph && block.Text == "Nested cite." {
			if block.StyleClasses != pdfStyleCite {
				t.Fatalf("nested cite block classes = %q, want %q", block.StyleClasses, pdfStyleCite)
			}
			if block.ContextClasses != joinStyleClasses(pdfStyleAnnotation, pdfStyleCite) {
				t.Fatalf("nested cite block context = %q, want %q", block.ContextClasses, joinStyleClasses(pdfStyleAnnotation, pdfStyleCite))
			}
			if block.StripRootHorizontalMargins {
				t.Fatalf("nested cite block should preserve normal root margins like KFX annotation children: %#v", block)
			}
			return
		}
	}
	t.Fatalf("nested cite block not found in %#v", blocks)
}

func TestCollectTextBlocksPreservesRootHorizontalMarginsInAnnotationNestedPoem(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Lead annotation."}}}},
				{Kind: fb2.FlowPoem, Poem: &fb2.Poem{
					Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Poem title"}}}}}},
					Stanzas: []fb2.Stanza{{Verses: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Verse line."}}}}}},
				}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	seenTitle := false
	seenVerse := false
	for _, block := range blocks {
		switch {
		case block.Text == "Poem title":
			seenTitle = true
			if block.Kind != pdfBlockParagraph {
				t.Fatalf("poem title kind = %v, want paragraph: %#v", block.Kind, block)
			}
			if block.StyleClasses != pdfStylePoemTitle+" "+pdfStylePoemTitle+"-first" {
				t.Fatalf("poem title classes = %q, want poem-title first variant", block.StyleClasses)
			}
			if block.ContextClasses != pdfStyleAnnotation+" "+pdfStylePoem {
				t.Fatalf("poem title context = %q, want annotation poem context", block.ContextClasses)
			}
			if block.StripRootHorizontalMargins {
				t.Fatalf("poem title should preserve normal root margins like KFX annotation children: %#v", block)
			}
		case block.Kind == pdfBlockPoem && block.Text == "Verse line.":
			seenVerse = true
			if block.StyleClasses != joinStyleClasses(pdfStylePoem, pdfStyleStanza) {
				t.Fatalf("verse block classes = %q, want poem plus stanza wrapper", block.StyleClasses)
			}
			if block.StripRootHorizontalMargins {
				t.Fatalf("verse block should preserve normal root margins like KFX annotation children: %#v", block)
			}
		}
	}
	if !seenTitle || !seenVerse {
		t.Fatalf("expected poem title and verse, got %#v", blocks)
	}
}

func TestCollectTextBlocksPreservesRootHorizontalMarginsInAnnotationNestedSection(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Lead annotation."}}}},
				{Kind: fb2.FlowSection, Section: &fb2.Section{
					Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested section note."}}},
					}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested section body."}}},
					}},
				}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	seenBody := false
	seenAnnotation := false
	for _, block := range blocks {
		switch {
		case block.Kind == pdfBlockParagraph && block.Text == "Nested section body.":
			seenBody = true
			if block.ContextClasses != "" {
				t.Fatalf("nested section body context = %q, want reset empty context", block.ContextClasses)
			}
			if block.StripRootHorizontalMargins {
				t.Fatalf("nested section body should preserve normal root margins like KFX annotation children: %#v", block)
			}
		case block.Kind == pdfBlockParagraph && block.Text == "Nested section note.":
			seenAnnotation = true
			if block.StyleClasses != pdfStyleAnnotation {
				t.Fatalf("nested section annotation classes = %q, want %q", block.StyleClasses, pdfStyleAnnotation)
			}
			if block.ContextClasses != pdfStyleAnnotation {
				t.Fatalf("nested section annotation context = %q, want %q", block.ContextClasses, pdfStyleAnnotation)
			}
			if block.StripRootHorizontalMargins {
				t.Fatalf("nested section annotation should preserve normal root margins like KFX annotation children: %#v", block)
			}
		}
	}
	if !seenBody || !seenAnnotation {
		t.Fatalf("expected nested section body and annotation, got %#v", blocks)
	}
}

func TestCollectPDFContentAddsTOCPageBeforeContent(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
		}},
	}}}
	plan, err := collectPDFContent(&content.Content{Book: book}, &config.DocumentConfig{
		TOCPage: config.TOCPageConfig{Placement: common.TOCPagePlacementBefore},
	})
	if err != nil {
		t.Fatalf("collectPDFContent() error = %v", err)
	}
	if len(plan.Blocks) < 4 {
		t.Fatalf("blocks = %#v, want TOC and chapter blocks", plan.Blocks)
	}
	if got := plan.Blocks[0]; got.Kind != pdfBlockPageBreak || got.ID != "toc-page" {
		t.Fatalf("first block = %#v, want TOC page break", got)
	}
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "Contents" || got.StyleName != pdfStyleTOCTitle || got.StyleClasses != pdfStyleTOCTitle+"-first" {
		t.Fatalf("second block = %#v, want TOC heading via title helper", got)
	}
	if got := plan.Blocks[2]; got.Kind != pdfBlockTOCEntry || got.Text != "Chapter 1" || len(got.Links) != 1 || got.Links[0].Href != "#chapter-1" {
		t.Fatalf("TOC entry block = %#v", got)
	}
}

func TestCollectPDFContentTOCPageUsesBookTitleAndAuthorsTemplate(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			BookTitle: fb2.TextField{Value: "My Great Book"},
			Authors:   []fb2.Author{{FirstName: "Ada", LastName: "Lovelace"}},
		}},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:    "chapter-1",
				Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			}},
		}},
	}
	plan, err := collectPDFContent(&content.Content{Book: book, SrcName: "book.fb2", OutputFormat: common.OutputFmtPdf}, &config.DocumentConfig{
		TOCPage: config.TOCPageConfig{
			Placement:       common.TOCPagePlacementBefore,
			AuthorsTemplate: "{{ (index .Authors 0).FirstName }} {{ (index .Authors 0).LastName }}",
		},
	})
	if err != nil {
		t.Fatalf("collectPDFContent() error = %v", err)
	}
	if len(plan.Blocks) < 5 {
		t.Fatalf("blocks = %#v, want TOC title lines, entry, and chapter", plan.Blocks)
	}
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "My Great Book" || got.StyleName != pdfStyleTOCTitle || got.StyleClasses != pdfStyleTOCTitle+"-first" {
		t.Fatalf("first TOC title block = %#v, want book-title first line", got)
	}
	if got := plan.Blocks[2]; got.Kind != pdfBlockHeading || got.Text != "Ada Lovelace" || got.StyleName != pdfStyleTOCTitle || got.StyleClasses != pdfStyleTOCTitle+"-next" {
		t.Fatalf("second TOC title block = %#v, want author next line", got)
	}
	if got := plan.Blocks[3]; got.Kind != pdfBlockTOCEntry || got.Text != "Chapter 1" {
		t.Fatalf("TOC entry block = %#v", got)
	}
}

func TestCollectTextBlocksUsesFootnoteSectionSemantics(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{
		{Kind: fb2.BodyMain, Sections: []fb2.Section{{ID: "chapter-1", Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}}}}},
		{Kind: fb2.BodyFootnotes, Name: "notes", Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Notes"}}}}}}, Sections: []fb2.Section{{
			ID:    "note-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "1"}}}}}},
			Content: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
			}},
		}}},
	}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	foundTitle := false
	foundBody := false
	for _, block := range blocks {
		switch {
		case block.Text == "1":
			foundTitle = true
			if block.Kind != pdfBlockParagraph {
				t.Fatalf("footnote title kind = %v, want paragraph: %#v", block.Kind, block)
			}
			if block.ID != "note-1" {
				t.Fatalf("footnote title ID = %q, want note-1", block.ID)
			}
			if block.StyleClasses != pdfStyleFootnoteTitle+" "+pdfStyleFootnoteTitle+"-first" {
				t.Fatalf("footnote title classes = %q, want footnote-title first variant", block.StyleClasses)
			}
			if block.ContextClasses != pdfStyleFootnoteTitle {
				t.Fatalf("footnote title context = %q, want %q", block.ContextClasses, pdfStyleFootnoteTitle)
			}
		case block.Text == "Footnote body.":
			foundBody = true
			if block.Kind != pdfBlockParagraph {
				t.Fatalf("footnote body kind = %v, want paragraph: %#v", block.Kind, block)
			}
			if block.StyleClasses != pdfStyleFootnote {
				t.Fatalf("footnote body classes = %q, want %q", block.StyleClasses, pdfStyleFootnote)
			}
			if block.ContextClasses != pdfStyleFootnote {
				t.Fatalf("footnote body context = %q, want %q", block.ContextClasses, pdfStyleFootnote)
			}
		}
	}
	if !foundTitle || !foundBody {
		t.Fatalf("expected footnote title/body blocks, got %#v", blocks)
	}
}

func TestCollectTextBlocksPDFPrintedFootnotesSkipNormalFlowBody(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{
		{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:    "chapter-1",
				Title: pdfTitleFromStrings("Chapter 1"),
			}},
		},
		{
			Kind:  fb2.BodyFootnotes,
			Name:  "notes",
			Title: pdfTitleFromStrings("Notes"),
			Sections: []fb2.Section{{
				ID:    "note-1",
				Title: pdfTitleFromStrings("1"),
				Content: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
				}},
			}},
		},
	}}

	plan, err := collectPDFContent(&content.Content{
		Book:          book,
		OutputFormat:  common.OutputFmtPdf,
		FootnotesMode: common.FootnotesModeFloat,
	}, nil)
	if err != nil {
		t.Fatalf("collectPDFContent() error = %v", err)
	}
	for _, block := range plan.Blocks {
		if block.Text == "Notes" || block.Text == "1" || block.Text == "Footnote body." {
			t.Fatalf("PDF printed footnotes should skip normal-flow footnote body block: %#v", block)
		}
	}
	if findPDFTOCEntry(plan.TOC, "Notes") != nil {
		t.Fatalf("PDF printed footnotes should skip footnote TOC entry: %#v", plan.TOC)
	}
}

func TestCollectTextBlocksFootnoteSectionsDoNotEmitChapterEndVignette(t *testing.T) {
	book := &fb2.FictionBook{
		VignetteIDs: map[common.VignettePos]string{common.VignettePosChapterEnd: "chapter-end"},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyFootnotes,
			Name: "notes",
			Sections: []fb2.Section{{
				ID:      "note-1",
				Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "1"}}}}}},
				Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}}}},
			}},
		}},
	}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	for _, block := range blocks {
		if block.Kind == pdfBlockImage && block.ImageID == "chapter-end" {
			t.Fatalf("footnote section should not emit chapter-end vignette: %#v", block)
		}
	}
}

func TestCollectTextBlocksUsesStructuralPageBreaks(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowSection,
				Section: &fb2.Section{
					ID:    "nested",
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested"}}}}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested body."}}},
					}},
				},
			}},
		}},
	}}}
	book.SetSectionPageBreaks(map[int]bool{2: true})

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var pageBreaks int
	var texts []string
	for _, block := range blocks {
		if block.Kind == pdfBlockPageBreak {
			pageBreaks++
			continue
		}
		texts = append(texts, block.Text)
	}
	if pageBreaks != 2 {
		t.Fatalf("page breaks = %d, want 2", pageBreaks)
	}
	wantTexts := []string{"Chapter", "Nested", "Nested body."}
	if len(texts) != len(wantTexts) {
		t.Fatalf("texts = %#v, want %#v", texts, wantTexts)
	}
	for i := range wantTexts {
		if texts[i] != wantTexts[i] {
			t.Fatalf("texts = %#v, want %#v", texts, wantTexts)
		}
	}
}

func TestCollectTextBlocksPreservesContainerStyleClasses(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			Epigraphs: []fb2.Epigraph{{
				Flow: fb2.Flow{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Epigraph text."}}},
				}, {
					Kind:     fb2.FlowSubtitle,
					Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Epigraph subtitle."}}},
				}}},
			}},
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowCite,
				Cite: &fb2.Cite{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Style: "source-class", Text: []fb2.InlineSegment{{Text: "Cite text."}}},
				}, {
					Kind:     fb2.FlowSubtitle,
					Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Cite subtitle."}}},
				}}},
			}, {
				Kind: fb2.FlowPoem,
				Poem: &fb2.Poem{
					Subtitles: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Poem subtitle."}}}},
					Stanzas: []fb2.Stanza{{
						Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Stanza subtitle."}}},
						Verses:   []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Verse line."}}}},
					}},
				},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}

	wantClasses := map[string]string{
		"Epigraph text.":     pdfStyleEpigraph,
		"Epigraph subtitle.": pdfStyleEpigraphSubtitle,
		"Cite text.":         "source-class " + pdfStyleCite,
		"Cite subtitle.":     pdfStyleCiteSubtitle,
		"Poem subtitle.":     pdfStylePoemSubtitle,
		"Stanza subtitle.":   joinStyleClasses(pdfStyleStanzaSubtitle, pdfStyleStanza),
		"Verse line.":        joinStyleClasses(pdfStylePoem, pdfStyleStanza),
	}
	for text, want := range wantClasses {
		found := false
		for _, block := range blocks {
			if block.Text != text {
				continue
			}
			found = true
			if block.StyleClasses != want {
				t.Fatalf("block %q classes = %q, want %q", text, block.StyleClasses, want)
			}
		}
		if !found {
			t.Fatalf("missing block %q in %#v", text, blocks)
		}
	}
}

func textBlocksOnly(blocks []pdfTextBlock) []pdfTextBlock {
	out := make([]pdfTextBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind != pdfBlockPageBreak {
			out = append(out, block)
		}
	}
	return out
}

func TestCollectTextBlocksTransfersChapterEndVignetteToLastSplitDescendant(t *testing.T) {
	title := func(text string) *fb2.Title {
		return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}}}
	}
	paragraph := func(text string) fb2.FlowItem {
		return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}
	}
	sectionItem := func(section *fb2.Section) fb2.FlowItem {
		return fb2.FlowItem{Kind: fb2.FlowSection, Section: section}
	}

	valid := &fb2.Section{
		ID:      "valid",
		Title:   title("Валидное вложение секций"),
		Content: []fb2.FlowItem{paragraph("Valid body.")},
	}
	validWrapper := &fb2.Section{
		ID:      "valid-wrapper",
		Content: []fb2.FlowItem{sectionItem(valid)},
	}
	invalid := &fb2.Section{
		ID:      "invalid",
		Title:   title("Невалидное вложение секций"),
		Content: []fb2.FlowItem{paragraph("Invalid body.")},
	}
	book := &fb2.FictionBook{
		VignetteIDs: map[common.VignettePos]string{common.VignettePosChapterEnd: "chapter-end"},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:      "chapter",
				Title:   title("Chapter"),
				Content: []fb2.FlowItem{sectionItem(validWrapper), sectionItem(invalid)},
			}},
		}},
	}
	book.SetSectionPageBreaks(map[int]bool{2: true})

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	indexOfText := func(text string) int {
		for i, block := range blocks {
			if block.Text == text {
				return i
			}
		}
		return -1
	}
	validIndex := indexOfText("Валидное вложение секций")
	invalidIndex := indexOfText("Невалидное вложение секций")
	chapterEnd := -1
	chapterEndCount := 0
	for i, block := range blocks {
		if block.Kind == pdfBlockImage && block.ImageID == "chapter-end" {
			chapterEnd = i
			chapterEndCount++
		}
	}
	if validIndex < 0 || invalidIndex < 0 || chapterEnd < 0 {
		t.Fatalf("missing expected blocks valid=%d invalid=%d chapterEnd=%d: %#v", validIndex, invalidIndex, chapterEnd, blocks)
	}
	if chapterEndCount != 1 {
		t.Fatalf("chapter-end count = %d, want 1", chapterEndCount)
	}
	if !(validIndex < invalidIndex && invalidIndex < chapterEnd) {
		t.Fatalf("chapter-end order valid=%d invalid=%d chapterEnd=%d, want after invalid section", validIndex, invalidIndex, chapterEnd)
	}
}

func TestCollectTextBlocksIncludesVignettes(t *testing.T) {
	book := &fb2.FictionBook{
		VignetteIDs: map[common.VignettePos]string{
			common.VignettePosBookTitleTop:       "book-top",
			common.VignettePosBookTitleBottom:    "book-bottom",
			common.VignettePosChapterTitleTop:    "chapter-top",
			common.VignettePosChapterTitleBottom: "chapter-bottom",
			common.VignettePosChapterEnd:         "chapter-end",
		},
		Bodies: []fb2.Body{{
			Kind:  fb2.BodyMain,
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Book title"}}}}}},
			Sections: []fb2.Section{{
				Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
				Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Body."}}}}},
			}},
		}},
	}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var got []string
	for _, block := range textBlocksOnly(blocks) {
		if block.Kind == pdfBlockImage && strings.Contains(block.StyleClasses, "vignette") {
			flag := "keep"
			if block.StripRootHorizontalMargins {
				flag = "strip"
			}
			got = append(got, flag+":"+block.ImageID+":"+block.StyleClasses)
		}
	}
	want := []string{
		"strip:book-top:image-vignette vignette vignette-book-title-top body-title",
		"strip:book-bottom:image-vignette vignette vignette-book-title-bottom body-title",
		"strip:chapter-top:image-vignette vignette vignette-chapter-title-top chapter-title",
		"strip:chapter-bottom:image-vignette vignette vignette-chapter-title-bottom chapter-title",
		"keep:chapter-end:image-vignette vignette vignette-chapter-end",
	}
	if len(got) != len(want) {
		t.Fatalf("vignette blocks = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("vignette block %d = %q, want %q (all: %#v)", i, got[i], want[i], got)
		}
	}
}

func TestCollectTextBlocksIncludesBlockImages(t *testing.T) {
	blocks, err := collectTextBlocks(&content.Content{
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind:  fb2.BodyMain,
			Image: &fb2.Image{Href: "#body-image", Alt: "Body image"},
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Body title"}}}}}},
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind:  fb2.FlowImage,
				Image: &fb2.Image{Href: "#flow-image", ID: "image-anchor", Alt: "Flow image"},
			}}}},
		}}},
	})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	blocks = textBlocksOnly(blocks)
	imageBlocks := make([]pdfTextBlock, 0, 2)
	for _, block := range blocks {
		if block.Kind == pdfBlockImage {
			imageBlocks = append(imageBlocks, block)
		}
	}
	if len(imageBlocks) != 2 {
		t.Fatalf("image blocks = %d, want 2: %#v", len(imageBlocks), blocks)
	}
	if got := imageBlocks[0]; got.ImageID != "body-image" || got.Text != "Body image" {
		t.Fatalf("body image block = %#v", got)
	}
	if got := imageBlocks[1]; got.ImageID != "flow-image" || got.ID != "image-anchor" {
		t.Fatalf("flow image block = %#v", got)
	}
}
