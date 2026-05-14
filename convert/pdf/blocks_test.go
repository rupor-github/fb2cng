package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

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
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "About" {
		t.Fatalf("second block = %#v, want annotation heading", got)
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

func TestCollectTextBlocksMarksMultiBlockSectionAnnotationForRootHorizontalStripping(t *testing.T) {
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
		if !block.StripRootHorizontalMargins {
			t.Fatalf("annotation block %d should strip root horizontal margins: %#v", i, block)
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

func TestCollectTextBlocksPropagatesWrappedAnnotationRootHorizontalStrippingIntoCite(t *testing.T) {
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
			if !block.StripRootHorizontalMargins {
				t.Fatalf("nested cite block should inherit wrapped-annotation root stripping: %#v", block)
			}
			return
		}
	}
	t.Fatalf("nested cite block not found in %#v", blocks)
}

func TestCollectTextBlocksPropagatesWrappedAnnotationRootHorizontalStrippingIntoPoem(t *testing.T) {
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
			if !block.StripRootHorizontalMargins {
				t.Fatalf("poem title should inherit wrapped-annotation root stripping: %#v", block)
			}
		case block.Kind == pdfBlockPoem && block.Text == "Verse line.":
			seenVerse = true
			if block.StyleClasses != pdfStylePoem {
				t.Fatalf("verse block classes = %q, want %q", block.StyleClasses, pdfStylePoem)
			}
			if !block.StripRootHorizontalMargins {
				t.Fatalf("verse block should inherit wrapped-annotation root stripping: %#v", block)
			}
		}
	}
	if !seenTitle || !seenVerse {
		t.Fatalf("expected stripped poem title and verse, got %#v", blocks)
	}
}

func TestCollectTextBlocksPropagatesWrappedAnnotationRootHorizontalStrippingIntoNestedSection(t *testing.T) {
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
			if !block.StripRootHorizontalMargins {
				t.Fatalf("nested section body should inherit wrapped-annotation root stripping: %#v", block)
			}
		case block.Kind == pdfBlockParagraph && block.Text == "Nested section note.":
			seenAnnotation = true
			if block.StyleClasses != pdfStyleAnnotation {
				t.Fatalf("nested section annotation classes = %q, want %q", block.StyleClasses, pdfStyleAnnotation)
			}
			if block.ContextClasses != pdfStyleAnnotation {
				t.Fatalf("nested section annotation context = %q, want %q", block.ContextClasses, pdfStyleAnnotation)
			}
			if !block.StripRootHorizontalMargins {
				t.Fatalf("nested section annotation should inherit wrapped-annotation root stripping: %#v", block)
			}
		}
	}
	if !seenBody || !seenAnnotation {
		t.Fatalf("expected stripped nested section body and annotation, got %#v", blocks)
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
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "Contents" {
		t.Fatalf("second block = %#v, want TOC heading", got)
	}
	if got := plan.Blocks[2]; got.Kind != pdfBlockTOCEntry || got.Text != "Chapter 1" || len(got.Links) != 1 || got.Links[0].Href != "#chapter-1" {
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
		"Stanza subtitle.":   pdfStyleStanzaSubtitle,
		"Verse line.":        pdfStylePoem,
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
