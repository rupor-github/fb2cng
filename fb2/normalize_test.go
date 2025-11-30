package fb2

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestNormalizeFootnotes(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	t.Run("non_footnotes_body_unchanged", func(t *testing.T) {
		body := &Body{
			Kind: BodyMain,
			Sections: []Section{
				{ID: "s1", Title: &Title{Lang: "en"}},
				{ID: "s2", Title: &Title{Lang: "en"}},
			},
		}

		result := body.normalizeFootnotes(log)
		if len(result.Sections) != 2 {
			t.Fatalf("expected 2 sections, got %d", len(result.Sections))
		}
		if result.Sections[0].ID != "s1" || result.Sections[1].ID != "s2" {
			t.Fatalf("sections changed unexpectedly")
		}
	})

	t.Run("skips_sections_without_id", func(t *testing.T) {
		body := &Body{
			Kind: BodyFootnotes,
			Sections: []Section{
				{ID: "note1", Content: []FlowItem{{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "First"}}}}}},
				{ID: "", Content: []FlowItem{{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Invalid"}}}}}},
				{ID: "note2", Content: []FlowItem{{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Second"}}}}}},
			},
		}

		result := body.normalizeFootnotes(log)
		if len(result.Sections) != 2 {
			t.Fatalf("expected 2 sections (skipping one without ID), got %d", len(result.Sections))
		}
		if result.Sections[0].ID != "note1" || result.Sections[1].ID != "note2" {
			t.Fatalf("wrong sections kept: %v, %v", result.Sections[0].ID, result.Sections[1].ID)
		}
	})

	t.Run("flattens_nested_sections", func(t *testing.T) {
		nestedSection := Section{
			ID:    "nested",
			Lang:  "en",
			Title: &Title{Items: []TitleItem{{Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Nested Title"}}}}}},
			Content: []FlowItem{
				{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Nested content"}}}},
			},
		}

		body := &Body{
			Kind: BodyFootnotes,
			Sections: []Section{
				{
					ID:   "note1",
					Lang: "en",
					Content: []FlowItem{
						{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Before"}}}},
						{Kind: FlowSection, Section: &nestedSection},
						{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "After"}}}},
					},
				},
			},
		}

		result := body.normalizeFootnotes(log)
		if len(result.Sections) != 1 {
			t.Fatalf("expected 1 section, got %d", len(result.Sections))
		}

		note := result.Sections[0]
		if note.ID != "note1" {
			t.Fatalf("wrong section ID: %q", note.ID)
		}

		// Should have: "Before" paragraph, nested title as subtitle, nested content paragraph, "After" paragraph
		expectedItems := 4
		if len(note.Content) != expectedItems {
			t.Fatalf("expected %d content items after flattening, got %d", expectedItems, len(note.Content))
		}

		// Check first item
		if note.Content[0].Kind != FlowParagraph {
			t.Fatalf("expected first item to be paragraph")
		}

		// Check second item (flattened title becomes subtitle)
		if note.Content[1].Kind != FlowSubtitle {
			t.Fatalf("expected second item to be subtitle from nested title, got %v", note.Content[1].Kind)
		}

		// Check third item (nested content)
		if note.Content[2].Kind != FlowParagraph {
			t.Fatalf("expected third item to be paragraph from nested content")
		}

		// Check fourth item
		if note.Content[3].Kind != FlowParagraph {
			t.Fatalf("expected fourth item to be paragraph")
		}
	})

	t.Run("converts_section_metadata_to_flow", func(t *testing.T) {
		nestedImg := Image{Href: "#pic", ID: "img1"}
		nestedAnnotation := Flow{
			Items: []FlowItem{
				{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Annotation"}}}},
			},
		}
		nestedEpigraph := Epigraph{
			Flow: Flow{
				ID: "epi1",
				Items: []FlowItem{
					{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Quote"}}}},
				},
			},
			TextAuthors: []Paragraph{
				{Text: []InlineSegment{{Kind: InlineText, Text: "Author"}}},
			},
		}

		nestedSection := Section{
			ID:         "nested",
			Image:      &nestedImg,
			Epigraphs:  []Epigraph{nestedEpigraph},
			Annotation: &nestedAnnotation,
			Content: []FlowItem{
				{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Content"}}}},
			},
		}

		body := &Body{
			Kind: BodyFootnotes,
			Sections: []Section{
				{
					ID:      "note1",
					Content: []FlowItem{{Kind: FlowSection, Section: &nestedSection}},
				},
			},
		}

		result := body.normalizeFootnotes(log)
		note := result.Sections[0]

		// Should have: image, cite (from epigraph), annotation paragraph, content paragraph
		expectedItems := 4
		if len(note.Content) != expectedItems {
			t.Fatalf("expected %d content items, got %d", expectedItems, len(note.Content))
		}

		// Check image
		if note.Content[0].Kind != FlowImage || note.Content[0].Image.Href != "#pic" {
			t.Fatalf("expected first item to be image")
		}

		// Check cite (from epigraph)
		if note.Content[1].Kind != FlowCite {
			t.Fatalf("expected second item to be cite from epigraph")
		}

		// Check annotation content
		if note.Content[2].Kind != FlowParagraph {
			t.Fatalf("expected third item to be paragraph from annotation")
		}

		// Check main content
		if note.Content[3].Kind != FlowParagraph {
			t.Fatalf("expected fourth item to be main content paragraph")
		}
	})

	t.Run("deeply_nested_sections_flattened", func(t *testing.T) {
		deeplyNested := Section{
			ID: "deep",
			Content: []FlowItem{
				{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Deep"}}}},
			},
		}

		middleNested := Section{
			ID: "middle",
			Content: []FlowItem{
				{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Middle"}}}},
				{Kind: FlowSection, Section: &deeplyNested},
			},
		}

		body := &Body{
			Kind: BodyFootnotes,
			Sections: []Section{
				{
					ID: "note1",
					Content: []FlowItem{
						{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Top"}}}},
						{Kind: FlowSection, Section: &middleNested},
					},
				},
			},
		}

		result := body.normalizeFootnotes(log)
		note := result.Sections[0]

		// Should have all content flattened: Top, Middle, Deep
		if len(note.Content) != 3 {
			t.Fatalf("expected 3 content items after deep flattening, got %d", len(note.Content))
		}

		// Verify all are paragraphs with expected text
		texts := []string{"Top", "Middle", "Deep"}
		for i, expected := range texts {
			if note.Content[i].Kind != FlowParagraph {
				t.Fatalf("item %d: expected paragraph", i)
			}
			if len(note.Content[i].Paragraph.Text) == 0 {
				t.Fatalf("item %d: empty text", i)
			}
			if note.Content[i].Paragraph.Text[0].Text != expected {
				t.Fatalf("item %d: expected %q, got %q", i, expected, note.Content[i].Paragraph.Text[0].Text)
			}
		}
	})

	t.Run("adds_title_from_id_when_missing", func(t *testing.T) {
		body := &Body{
			Kind:     BodyFootnotes,
			Sections: []Section{{ID: "noteX", Content: []FlowItem{}}},
		}
		result := body.normalizeFootnotes(log)
		if len(result.Sections) != 1 {
			t.Fatalf("expected 1 section")
		}
		if result.Sections[0].Title == nil {
			t.Fatalf("expected fabricated title")
		}
		para := result.Sections[0].Title.Items[0].Paragraph
		if para == nil || len(para.Text) == 0 || para.Text[0].Text != "~ noteX ~" {
			f := ""
			if para != nil && len(para.Text) > 0 {
				f = para.Text[0].Text
			}
			t.Fatalf("expected title paragraph text noteX, got %q", f)
		}
	})

	t.Run("replaces_empty_title_with_id", func(t *testing.T) {
		// Title exists but only empty line or paragraph without text
		emptyPara := &Paragraph{Text: []InlineSegment{}} // no text segments
		emptyTitle := &Title{Items: []TitleItem{{Paragraph: emptyPara}}}
		body := &Body{Kind: BodyFootnotes, Sections: []Section{{ID: "fn42", Title: emptyTitle}}}
		result := body.normalizeFootnotes(log)
		if result.Sections[0].Title == nil {
			t.Fatalf("expected fabricated title when original empty")
		}
		para := result.Sections[0].Title.Items[0].Paragraph
		if para == nil || len(para.Text) == 0 || para.Text[0].Text != "~ fn42 ~" {
			f := ""
			if para != nil && len(para.Text) > 0 {
				f = para.Text[0].Text
			}
			t.Fatalf("expected fallback title text fn42, got %q", f)
		}
	})
}

func TestFictionBookNormalizeFootnoteBodies(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Build a main body that should remain unchanged.
	mainBody := Body{
		Kind: BodyMain,
		Sections: []Section{
			{ID: "main1", Content: []FlowItem{{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Main"}}}}}},
		},
	}

	// Build nested footnote section structure to exercise flattening.
	deepNested := Section{ID: "deep", Content: []FlowItem{{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Deep"}}}}}}
	midNested := Section{ID: "mid", Content: []FlowItem{
		{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Middle"}}}},
		{Kind: FlowSection, Section: &deepNested},
	}}
	topFootnote := Section{ID: "note1", Content: []FlowItem{
		{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Top"}}}},
		{Kind: FlowSection, Section: &midNested},
	}}

	footnoteBody := Body{Kind: BodyFootnotes, Sections: []Section{topFootnote}}

	book := &FictionBook{
		Bodies: []Body{mainBody, footnoteBody},
	}

	// Invoke normalization across footnote bodies.
	book, footnotesIndex := book.NormalizeFootnoteBodies(log)

	if len(book.Bodies) != 2 {
		t.Fatalf("expected 2 bodies, got %d", len(book.Bodies))
	}

	// Main body remains unchanged.
	if len(book.Bodies[0].Sections) != 1 || book.Bodies[0].Sections[0].ID != "main1" {
		t.Fatalf("main body sections altered unexpectedly")
	}

	// Footnote body should be normalized: one section (note1) whose content is flattened.
	fnBody := book.Bodies[1]
	if !fnBody.Footnotes() {
		t.Fatalf("second body expected to be footnotes kind")
	}
	if len(fnBody.Sections) != 1 || fnBody.Sections[0].ID != "note1" {
		t.Fatalf("footnote body sections unexpected after normalization")
	}
	// Expect flattened content sequence: Top, Middle, Deep (all paragraphs)
	expectedTexts := []string{"Top", "Middle", "Deep"}
	content := fnBody.Sections[0].Content
	if len(content) != len(expectedTexts) {
		t.Fatalf("expected %d flattened items, got %d", len(expectedTexts), len(content))
	}
	for i, exp := range expectedTexts {
		if content[i].Kind != FlowParagraph || len(content[i].Paragraph.Text) == 0 || content[i].Paragraph.Text[0].Text != exp {
			t.Fatalf("content item %d expected paragraph %q, got %+v", i, exp, content[i])
		}
	}

	// Verify the returned footnote index is correct
	if len(footnotesIndex) != 1 {
		t.Fatalf("expected 1 footnote in index, got %d", len(footnotesIndex))
	}
	ref, exists := footnotesIndex["note1"]
	if !exists {
		t.Fatalf("note1 not found in footnote index")
	}
	if ref.BodyIdx != 1 || ref.SectionIdx != 0 {
		t.Errorf("note1 has wrong index: BodyIdx=%d, SectionIdx=%d", ref.BodyIdx, ref.SectionIdx)
	}
}

func TestNormalizeLinks(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Create a book with a broken internal link
	original := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Content: []FlowItem{
							{
								Kind: FlowParagraph,
								Paragraph: &Paragraph{
									Text: []InlineSegment{
										{
											Kind:     InlineLink,
											Href:     "#nonexistent",
											LinkType: "note",
											Children: []InlineSegment{
												{Kind: InlineText, Text: "Broken link"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Normalize links
	normalized, ids, links := original.NormalizeLinks(log)

	// Verify original is unchanged
	if original.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Kind != InlineLink {
		t.Errorf("original link was mutated: kind = %v", original.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Kind)
	}
	if original.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Href != "#nonexistent" {
		t.Errorf("original link href was mutated: href = %v", original.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Href)
	}

	// Verify normalized has the link replaced with text
	if normalized.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Kind != InlineText {
		t.Errorf("normalized link was not replaced: kind = %v", normalized.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Kind)
	}
	normalizedText := normalized.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Text
	if !strings.Contains(normalizedText, "Broken link") || !strings.Contains(normalizedText, "broken link") {
		t.Errorf("normalized text doesn't contain expected content: %q", normalizedText)
	}

	// Verify returned link index doesn't contain the broken link anymore
	if refs, exists := links["nonexistent"]; exists {
		t.Errorf("broken link 'nonexistent' still in link index after normalization: %+v", refs)
	}

	// Verify IDs index is still valid
	if ids["section1"].Type != "section" {
		t.Errorf("section1 not properly indexed")
	}
}

func TestNormalizeLinks_BrokenImageLinks(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	original := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				Coverpage: []InlineImage{
					{Href: "#missing-cover", Type: "image/jpeg"},
				},
			},
		},
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Content: []FlowItem{
							{
								Kind: FlowImage,
								Image: &Image{
									Href: "#missing-block-img",
									Alt:  "Missing block image",
								},
							},
							{
								Kind: FlowParagraph,
								Paragraph: &Paragraph{
									Text: []InlineSegment{
										{
											Kind: InlineText,
											Text: "Text with ",
										},
										{
											Kind:  InlineImageSegment,
											Image: &InlineImage{Href: "#missing-inline-img", Alt: "Missing inline"},
										},
										{
											Kind: InlineText,
											Text: " image",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Normalize links
	normalized, _, _ := original.NormalizeLinks(log)

	// Verify not-found image binary was added
	foundNotFoundBinary := false
	for i := range normalized.Binaries {
		if normalized.Binaries[i].ID == NotFoundImageID {
			foundNotFoundBinary = true
			if normalized.Binaries[i].ContentType != "image/png" {
				t.Errorf("notFoundImage has wrong content type: %s", normalized.Binaries[i].ContentType)
			}
			if len(normalized.Binaries[i].Data) == 0 {
				t.Errorf("notFoundImage has no data")
			}
			break
		}
	}
	if !foundNotFoundBinary {
		t.Error("not-found image binary was not added")
	}

	// Verify coverpage image points to not-found image
	if normalized.Description.TitleInfo.Coverpage[0].Href != "#"+NotFoundImageID {
		t.Errorf("coverpage href not redirected: got %q, want %q",
			normalized.Description.TitleInfo.Coverpage[0].Href, "#"+NotFoundImageID)
	}

	// Verify block image points to not-found image
	if normalized.Bodies[0].Sections[0].Content[0].Image.Href != "#"+NotFoundImageID {
		t.Errorf("block image href not redirected: got %q, want %q",
			normalized.Bodies[0].Sections[0].Content[0].Image.Href, "#"+NotFoundImageID)
	}

	// Verify inline image points to not-found image
	inlineImg := normalized.Bodies[0].Sections[0].Content[1].Paragraph.Text[1].Image
	if inlineImg.Href != "#"+NotFoundImageID {
		t.Errorf("inline image href not redirected: got %q, want %q",
			inlineImg.Href, "#"+NotFoundImageID)
	}

	// Verify original is unchanged
	if original.Description.TitleInfo.Coverpage[0].Href == "#"+NotFoundImageID {
		t.Error("original coverpage was mutated")
	}
	if original.Bodies[0].Sections[0].Content[0].Image.Href == "#"+NotFoundImageID {
		t.Error("original block image was mutated")
	}
}

func TestEnsureNotFoundImageBinary_Idempotent(t *testing.T) {
	book := &FictionBook{
		Binaries: []BinaryObject{
			{ID: "existing-image", ContentType: "image/jpeg", Data: []byte{1, 2, 3}},
		},
	}

	// First call should add the binary
	book.ensureNotFoundImageBinary()
	if len(book.Binaries) != 2 {
		t.Fatalf("expected 2 binaries after first call, got %d", len(book.Binaries))
	}

	// Second call should not add duplicate
	book.ensureNotFoundImageBinary()
	if len(book.Binaries) != 2 {
		t.Errorf("expected 2 binaries after second call (idempotent), got %d", len(book.Binaries))
	}

	// Verify the not-found binary is present
	found := false
	for i := range book.Binaries {
		if book.Binaries[i].ID == NotFoundImageID {
			found = true
			break
		}
	}
	if !found {
		t.Error("not-found image binary not found in binaries list")
	}
}

func TestNormalizeIDs_AvoidCollisions(t *testing.T) {
	log := zaptest.NewLogger(t)

	// Create a book with existing IDs that will collide with generated ones
	book := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "sect_1", // This exists
						Content: []FlowItem{
							{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Section 1"}}}},
						},
					},
					{
						// No ID - should get sect_2
						Content: []FlowItem{
							{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Section 2"}}}},
						},
					},
					{
						ID: "sect_2", // This exists - so next generated should skip to sect_3
						Content: []FlowItem{
							{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Section 3"}}}},
						},
					},
					{
						// No ID - should get sect_3
						Content: []FlowItem{
							{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Section 4"}}}},
						},
					},
				},
			},
		},
	}

	// Build ID index
	ids := book.buildIDIndex(log)

	// Normalize IDs
	result, updatedIDs := book.NormalizeIDs(ids, log)

	// Check results - all sections should have unique IDs
	seenIDs := make(map[string]int)
	for i, section := range result.Bodies[0].Sections {
		if prevIdx, exists := seenIDs[section.ID]; exists {
			t.Errorf("Duplicate ID %q found at sections %d and %d", section.ID, prevIdx, i)
		}
		seenIDs[section.ID] = i

		if section.ID == "" {
			t.Errorf("Section %d has empty ID", i)
		}
	}

	// Verify we have exactly 4 unique IDs
	if len(seenIDs) != 4 {
		t.Errorf("Expected 4 unique IDs, got %d", len(seenIDs))
	}

	// Verify the IDs are correct
	sections := result.Bodies[0].Sections
	if sections[0].ID != "sect_1" {
		t.Errorf("Section 0 should keep ID 'sect_1', got %q", sections[0].ID)
	}
	if sections[1].ID != "sect_3" {
		t.Errorf("Section 1 should get ID 'sect_3' (avoiding collision with sect_2), got %q", sections[1].ID)
	}
	if sections[2].ID != "sect_2" {
		t.Errorf("Section 2 should keep ID 'sect_2', got %q", sections[2].ID)
	}
	if sections[3].ID != "sect_4" {
		t.Errorf("Section 3 should get ID 'sect_4', got %q", sections[3].ID)
	}
	_ = updatedIDs // use it
}

func TestNormalizeIDs_UpdatesIndex(t *testing.T) {
	log := zaptest.NewLogger(t)

	book := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						// No ID
						Content: []FlowItem{
							{Kind: FlowParagraph, Paragraph: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Section 1"}}}},
							{Kind: FlowSubtitle, Subtitle: &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "Subtitle 1"}}}},
						},
					},
				},
			},
		},
	}

	ids := book.buildIDIndex(log)
	result, updatedIDs := book.NormalizeIDs(ids, log)

	// Check that generated section ID is in updated index
	sectionID := result.Bodies[0].Sections[0].ID
	if sectionID == "" {
		t.Fatal("Section should have been assigned an ID")
	}

	if ref, exists := updatedIDs[sectionID]; !exists {
		t.Errorf("Updated ID index should contain generated section ID %q", sectionID)
	} else if ref.Type != "section-generated" {
		t.Errorf("Generated section ID %q should have type 'section-generated', got %q", sectionID, ref.Type)
	}

	// Check that generated subtitle ID is in updated index
	var subtitleID string
	for _, item := range result.Bodies[0].Sections[0].Content {
		if item.Kind == FlowSubtitle && item.Subtitle != nil {
			subtitleID = item.Subtitle.ID
			break
		}
	}

	if subtitleID == "" {
		t.Fatal("Subtitle should have been assigned an ID")
	}

	if ref, exists := updatedIDs[subtitleID]; !exists {
		t.Errorf("Updated ID index should contain generated subtitle ID %q", subtitleID)
	} else if ref.Type != "subtitle-generated" {
		t.Errorf("Generated subtitle ID %q should have type 'subtitle-generated', got %q", subtitleID, ref.Type)
	}
}
