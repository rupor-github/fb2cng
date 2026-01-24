package kfx

import (
	"strings"
	"testing"

	"golang.org/x/text/language"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

// TestVignettePlacement_ValidNesting verifies vignette placement for "Валидное" (valid) nesting.
// Valid nesting: titled sections are inside UNTITLED wrapper sections.
// Pattern: <section>(untitled) -> <section><title>... (titled at depth 3+)
//
// Structure:
//
//	<body> (main body with title)
//	  <section id="chap1"> (depth=1, "Chapter 1") - simple chapter
//	  <section id="chap2"> (depth=1, "Chapter 2" - like "Аннотации")
//	    <section> (depth=2, UNTITLED wrapper) <- key difference from invalid
//	      <section id="valid1"> (depth=3, "Valid 1") - inline, section vignettes
//	    </section>
//	    <section id="nested2"> (depth=2, "Nested 2") - separate storyline
//	  </section>
//
// Expected behavior:
//   - valid1 at depth=3 is INLINE (processed in same storyline as chap2)
//   - valid1 gets section-title-top, section-title-bottom (depth > 1)
//   - valid1 gets section-end vignette (inline titled section at depth > 1)
//   - chapter-end transfers to nested2 (last storyline of chapter)
func TestVignettePlacement_ValidNesting(t *testing.T) {
	vignetteIDs := map[common.VignettePos]string{
		common.VignettePosBookTitleTop:       "vig-book-top",
		common.VignettePosBookTitleBottom:    "vig-book-bottom",
		common.VignettePosChapterTitleTop:    "vig-chapter-top",
		common.VignettePosChapterTitleBottom: "vig-chapter-bottom",
		common.VignettePosChapterEnd:         "vig-chapter-end",
		common.VignettePosSectionTitleTop:    "vig-section-top",
		common.VignettePosSectionTitleBottom: "vig-section-bottom",
		common.VignettePosSectionEnd:         "vig-section-end",
	}

	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
				Coverpage: []fb2.InlineImage{{Href: "#cover.jpg"}},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-valid-nesting",
			},
		},
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Title: &fb2.Title{
					Items: []fb2.TitleItem{
						{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Book Title"}}}},
					},
				},
				Sections: []fb2.Section{
					// Chapter 1 - simple chapter
					{
						ID: "chap1",
						Title: &fb2.Title{
							Items: []fb2.TitleItem{
								{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}},
							},
						},
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1 content."}}}},
						},
					},
					// Chapter 2 - Valid nesting pattern (like "Валидное")
					{
						ID: "chap2",
						Title: &fb2.Title{
							Items: []fb2.TitleItem{
								{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 2"}}}},
							},
						},
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 2 intro."}}}},
							// VALID: Untitled wrapper containing titled section
							// The untitled wrapper at depth=2 means the titled child is at depth=3 (inline)
							{
								Kind: fb2.FlowSection,
								Section: &fb2.Section{
									// No title - UNTITLED wrapper at depth=2
									Content: []fb2.FlowItem{
										{
											Kind: fb2.FlowSection,
											Section: &fb2.Section{
												ID: "valid1",
												Title: &fb2.Title{
													Items: []fb2.TitleItem{
														{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Valid 1"}}}},
													},
												},
												Content: []fb2.FlowItem{
													{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Valid 1 content."}}}},
												},
											},
										},
									},
								},
							},
							// Titled section at depth=2 - becomes separate storyline
							{
								Kind: fb2.FlowSection,
								Section: &fb2.Section{
									ID: "nested2",
									Title: &fb2.Title{
										Items: []fb2.TitleItem{
											{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested 2"}}}},
										},
									},
									Content: []fb2.FlowItem{
										{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested 2 content."}}}},
									},
								},
							},
						},
					},
				},
			},
		},
		VignetteIDs: vignetteIDs,
	}

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  createVignetteImages(),
	}
	imageResources := createVignetteResources(c.ImagesIndex)
	styles := NewStyleRegistry()

	fragments, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	storylineVignettes := collectStorylineVignettes(fragments)

	// l1 = cover, l2 = body intro, l3 = chap1, l4 = chap2+valid1 (inline), l5 = nested2
	tests := []struct {
		name     string
		expected []string
		excluded []string
	}{
		{
			name:     "l1",
			expected: []string{}, // Cover - no vignettes
			excluded: []string{"rsrc-vig-chapter-end", "rsrc-vig-section-end"},
		},
		{
			name:     "l2",
			expected: []string{"rsrc-vig-book-top", "rsrc-vig-book-bottom"}, // Body intro
			excluded: []string{"rsrc-vig-chapter-end", "rsrc-vig-section-end", "rsrc-vig-chapter-top"},
		},
		{
			name:     "l3",
			expected: []string{"rsrc-vig-chapter-top", "rsrc-vig-chapter-bottom", "rsrc-vig-chapter-end"}, // Chapter 1
			excluded: []string{"rsrc-vig-section-end", "rsrc-vig-book-top"},
		},
		{
			name: "l4",
			// Chapter 2 + Valid1 inline: chapter title vignettes + section title vignettes + section-end
			// NO chapter-end because nested2 follows as separate storyline
			expected: []string{"rsrc-vig-chapter-top", "rsrc-vig-chapter-bottom", "rsrc-vig-section-top", "rsrc-vig-section-bottom", "rsrc-vig-section-end"},
			excluded: []string{"rsrc-vig-chapter-end"}, // chapter-end transferred to l5
		},
		{
			name: "l5",
			// Nested 2: section title vignettes + chapter-end (inherited from chap2)
			expected: []string{"rsrc-vig-section-top", "rsrc-vig-section-bottom", "rsrc-vig-chapter-end"},
			excluded: []string{"rsrc-vig-section-end", "rsrc-vig-chapter-top"}, // No section-end (it's a separate storyline, not inline)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, exists := storylineVignettes[tc.name]
			if !exists {
				t.Fatalf("storyline %s not found in %v", tc.name, storylineVignettes)
			}

			// Check expected vignettes are present
			for _, exp := range tc.expected {
				if !containsVignette(actual, exp) {
					t.Errorf("expected vignette %s not found, got %v", exp, actual)
				}
			}

			// Check excluded vignettes are NOT present
			for _, excl := range tc.excluded {
				if containsVignette(actual, excl) {
					t.Errorf("vignette %s should NOT be present, got %v", excl, actual)
				}
			}

			// Check no other unexpected vignettes
			for _, v := range actual {
				if !strings.HasPrefix(v, "rsrc-vig-") {
					continue
				}
				if !containsVignette(tc.expected, v) {
					t.Errorf("unexpected vignette %s found", v)
				}
			}
		})
	}
}

// TestVignettePlacement_InvalidNesting verifies vignette placement for "Невалидное" (invalid) nesting.
// Invalid nesting: titled sections are DIRECTLY nested within titled sections.
// Pattern: <section><title>... -> <section><title>... (titled at depth 2)
//
// Structure:
//
//	<section id="chap"> (depth=1, "Chapter" - titled)
//	  <section id="invalid1"> (depth=2, "Invalid 1" - TITLED, separate storyline)
//	    <section id="invalid2"> (depth=3, "Invalid 2" - inline within invalid1)
//
// Expected behavior:
//   - invalid1 at depth=2 is TITLED, becomes separate storyline
//   - invalid1 gets section-title-top, section-title-bottom (depth > 1)
//   - invalid2 at depth=3 is inline within invalid1's storyline
//   - invalid2 gets section-title-top, section-title-bottom, section-end
//   - chapter-end goes to the LAST storyline in the chapter
func TestVignettePlacement_InvalidNesting(t *testing.T) {
	vignetteIDs := map[common.VignettePos]string{
		common.VignettePosChapterTitleTop:    "vig-chapter-top",
		common.VignettePosChapterTitleBottom: "vig-chapter-bottom",
		common.VignettePosChapterEnd:         "vig-chapter-end",
		common.VignettePosSectionTitleTop:    "vig-section-top",
		common.VignettePosSectionTitleBottom: "vig-section-bottom",
		common.VignettePosSectionEnd:         "vig-section-end",
	}

	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-invalid-nesting",
			},
		},
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{
					// Chapter with invalid nesting (like "Невалидное")
					{
						ID: "chap",
						Title: &fb2.Title{
							Items: []fb2.TitleItem{
								{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}},
							},
						},
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter intro."}}}},
							// INVALID: Titled section directly nested in titled chapter
							{
								Kind: fb2.FlowSection,
								Section: &fb2.Section{
									ID: "invalid1",
									Title: &fb2.Title{
										Items: []fb2.TitleItem{
											{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Invalid 1"}}}},
										},
									},
									Content: []fb2.FlowItem{
										{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Invalid 1 content."}}}},
										// Another titled section nested within titled section (depth=3, inline)
										{
											Kind: fb2.FlowSection,
											Section: &fb2.Section{
												ID: "invalid2",
												Title: &fb2.Title{
													Items: []fb2.TitleItem{
														{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Invalid 2"}}}},
													},
												},
												Content: []fb2.FlowItem{
													{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Invalid 2 content."}}}},
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
		},
		VignetteIDs: vignetteIDs,
	}

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex: fb2.BookImages{
			"vig-chapter-top":    {Dim: struct{ Width, Height int }{800, 100}},
			"vig-chapter-bottom": {Dim: struct{ Width, Height int }{800, 100}},
			"vig-chapter-end":    {Dim: struct{ Width, Height int }{800, 100}},
			"vig-section-top":    {Dim: struct{ Width, Height int }{800, 100}},
			"vig-section-bottom": {Dim: struct{ Width, Height int }{800, 100}},
			"vig-section-end":    {Dim: struct{ Width, Height int }{800, 100}},
		},
	}
	imageResources := createVignetteResources(c.ImagesIndex)
	styles := NewStyleRegistry()

	fragments, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	storylineVignettes := collectStorylineVignettes(fragments)

	// l1 = chap (chapter title + content, NO chapter-end because invalid1 follows)
	// l2 = invalid1 + invalid2 inline (section titles + section-end for invalid2 + chapter-end)
	tests := []struct {
		name     string
		expected []string
		excluded []string
	}{
		{
			name: "l1",
			// Chapter: chapter title vignettes only, NO chapter-end (transferred to l2)
			expected: []string{"rsrc-vig-chapter-top", "rsrc-vig-chapter-bottom"},
			excluded: []string{"rsrc-vig-chapter-end", "rsrc-vig-section-end"},
		},
		{
			name: "l2",
			// Invalid1 (separate storyline) + Invalid2 (inline):
			// - invalid1 gets section-title-top, section-title-bottom
			// - invalid2 gets section-title-top, section-title-bottom, section-end
			// - chapter-end inherited from chap
			expected: []string{
				"rsrc-vig-section-top", "rsrc-vig-section-bottom", // invalid1 title
				"rsrc-vig-section-top", "rsrc-vig-section-bottom", // invalid2 title (duplicate expected)
				"rsrc-vig-section-end", // invalid2 is inline titled section
				"rsrc-vig-chapter-end", // inherited from chap
			},
			excluded: []string{"rsrc-vig-chapter-top"}, // No chapter title vignettes in section storyline
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, exists := storylineVignettes[tc.name]
			if !exists {
				t.Fatalf("storyline %s not found in %v", tc.name, storylineVignettes)
			}

			// For l2, we expect 2x section-top and 2x section-bottom (from invalid1 and invalid2)
			// Count occurrences instead of just checking presence
			if tc.name == "l2" {
				sectionTopCount := countVignette(actual, "rsrc-vig-section-top")
				sectionBottomCount := countVignette(actual, "rsrc-vig-section-bottom")
				if sectionTopCount != 2 {
					t.Errorf("expected 2 section-top vignettes, got %d in %v", sectionTopCount, actual)
				}
				if sectionBottomCount != 2 {
					t.Errorf("expected 2 section-bottom vignettes, got %d in %v", sectionBottomCount, actual)
				}
				if !containsVignette(actual, "rsrc-vig-section-end") {
					t.Errorf("expected section-end vignette not found in %v", actual)
				}
				if !containsVignette(actual, "rsrc-vig-chapter-end") {
					t.Errorf("expected chapter-end vignette not found in %v", actual)
				}
			} else {
				// Standard checks for other storylines
				for _, exp := range tc.expected {
					if !containsVignette(actual, exp) {
						t.Errorf("expected vignette %s not found, got %v", exp, actual)
					}
				}
			}

			// Check excluded vignettes are NOT present
			for _, excl := range tc.excluded {
				if containsVignette(actual, excl) {
					t.Errorf("vignette %s should NOT be present, got %v", excl, actual)
				}
			}
		})
	}
}

// extractStorylineNameFromFrag extracts the storyline name from a fragment
func extractStorylineNameFromFrag(frag *Fragment) string {
	if frag.Value == nil {
		return ""
	}
	// Look for story_name (SymStoryName = 176) in the fragment value
	// The value is a StructValue (which is map[KFXSymbol]any), so try both types
	var dataMap map[KFXSymbol]any
	switch v := frag.Value.(type) {
	case StructValue:
		dataMap = v
	case map[KFXSymbol]any:
		dataMap = v
	default:
		return ""
	}
	if name, ok := dataMap[SymStoryName]; ok {
		// story_name is stored as SymbolByNameValue (string) before serialization
		switch v := name.(type) {
		case SymbolByNameValue:
			return string(v)
		case SymbolValue:
			return KFXSymbol(v).Name()
		case string:
			return v
		}
	}
	return ""
}

// extractVignetteResourcesFromFrag extracts vignette resource names from a storyline fragment
func extractVignetteResourcesFromFrag(frag *Fragment) []string {
	var vignettes []string
	if frag.Value == nil {
		return vignettes
	}

	// Helper to extract name from different symbol value types
	extractName := func(v any) string {
		switch sym := v.(type) {
		case SymbolByNameValue:
			return string(sym)
		case SymbolValue:
			return KFXSymbol(sym).Name()
		case string:
			return sym
		}
		return ""
	}

	// Recursively search for resource_name entries that are vignettes
	var search func(v any)
	search = func(v any) {
		switch val := v.(type) {
		case StructValue:
			// Check if this is an image entry with a vignette resource
			if resName, ok := val[SymResourceName]; ok {
				name := extractName(resName)
				if strings.HasPrefix(name, "rsrc-vig-") {
					vignettes = append(vignettes, name)
				}
			}
			// Recurse into map values
			for _, child := range val {
				search(child)
			}
		case map[KFXSymbol]any:
			// Check if this is an image entry with a vignette resource
			if resName, ok := val[SymResourceName]; ok {
				name := extractName(resName)
				if strings.HasPrefix(name, "rsrc-vig-") {
					vignettes = append(vignettes, name)
				}
			}
			// Recurse into map values
			for _, child := range val {
				search(child)
			}
		case []any:
			// Recurse into array elements
			for _, child := range val {
				search(child)
			}
		}
	}

	search(frag.Value)
	return vignettes
}

// TestVignettePlacement_NoVignettes verifies behavior when no vignettes are configured
func TestVignettePlacement_NoVignettes(t *testing.T) {
	// Create test FB2 book without vignettes
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-no-vignettes",
			},
		},
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Title: &fb2.Title{
					Items: []fb2.TitleItem{
						{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Book Title"}}}},
					},
				},
				Sections: []fb2.Section{
					{
						ID: "chap1",
						Title: &fb2.Title{
							Items: []fb2.TitleItem{
								{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}},
							},
						},
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Content."}}}},
						},
					},
				},
			},
		},
		VignetteIDs: nil, // No vignettes
	}

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  fb2.BookImages{},
	}

	styles := NewStyleRegistry()
	imageResources := make(imageResourceInfoByID)

	fragments, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	// Verify no vignette resources in any storyline
	for _, frag := range fragments.fragments {
		if frag.FType == SymStoryline {
			vignettes := extractVignetteResourcesFromFrag(frag)
			if len(vignettes) > 0 {
				t.Errorf("unexpected vignettes found when none configured: %v", vignettes)
			}
		}
	}
}

// TestChapterEndVignetteTransfer verifies that chapter-end vignette is correctly
// transferred to the last nested storyline when a chapter has split sections
func TestChapterEndVignetteTransfer(t *testing.T) {
	vignetteIDs := map[common.VignettePos]string{
		common.VignettePosChapterEnd: "vig-chapter-end",
	}

	// Create a chapter with multiple nested titled sections at depth=2
	// All should become separate storylines, only the LAST one gets the chapter-end vignette
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-vignette-transfer",
			},
		},
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{
					{
						ID: "chap1",
						Title: &fb2.Title{
							Items: []fb2.TitleItem{
								{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}},
							},
						},
						Content: []fb2.FlowItem{
							// Three titled sections at depth=2 - all become separate storylines
							{
								Kind: fb2.FlowSection,
								Section: &fb2.Section{
									ID: "sec1",
									Title: &fb2.Title{
										Items: []fb2.TitleItem{
											{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Section 1"}}}},
										},
									},
									Content: []fb2.FlowItem{
										{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Sec 1."}}}},
									},
								},
							},
							{
								Kind: fb2.FlowSection,
								Section: &fb2.Section{
									ID: "sec2",
									Title: &fb2.Title{
										Items: []fb2.TitleItem{
											{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Section 2"}}}},
										},
									},
									Content: []fb2.FlowItem{
										{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Sec 2."}}}},
									},
								},
							},
							{
								Kind: fb2.FlowSection,
								Section: &fb2.Section{
									ID: "sec3",
									Title: &fb2.Title{
										Items: []fb2.TitleItem{
											{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Section 3"}}}},
										},
									},
									Content: []fb2.FlowItem{
										{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Sec 3."}}}},
									},
								},
							},
						},
					},
				},
			},
		},
		VignetteIDs: vignetteIDs,
	}

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex: fb2.BookImages{
			"vig-chapter-end": {Dim: struct{ Width, Height int }{800, 100}},
		},
	}

	imageResources := make(imageResourceInfoByID)
	imageResources["vig-chapter-end"] = imageResourceInfo{
		ResourceName: "rsrc-vig-chapter-end",
		Width:        800,
		Height:       100,
	}

	styles := NewStyleRegistry()

	fragments, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	// Collect vignettes per storyline
	storylineVignettes := collectStorylineVignettes(fragments)

	// Expected: l1=chap1 (no vignette), l2=sec1 (no vignette), l3=sec2 (no vignette), l4=sec3 (HAS chapter-end)
	// Only the last nested storyline should have the chapter-end vignette
	expectedWithVignette := "l4" // sec3 is the last
	storylinesWithoutVignette := []string{"l1", "l2", "l3"}

	for _, name := range storylinesWithoutVignette {
		if containsVignette(storylineVignettes[name], "rsrc-vig-chapter-end") {
			t.Errorf("storyline %s should NOT have chapter-end vignette", name)
		}
	}

	if !containsVignette(storylineVignettes[expectedWithVignette], "rsrc-vig-chapter-end") {
		t.Errorf("storyline %s should have chapter-end vignette, got %v", expectedWithVignette, storylineVignettes[expectedWithVignette])
	}
}

// ============================================================================
// Helper functions for vignette tests
// ============================================================================

// createVignetteImages creates a standard set of vignette images for testing
func createVignetteImages() fb2.BookImages {
	return fb2.BookImages{
		"cover.jpg":          {Dim: struct{ Width, Height int }{1000, 1500}},
		"vig-book-top":       {Dim: struct{ Width, Height int }{800, 100}},
		"vig-book-bottom":    {Dim: struct{ Width, Height int }{800, 100}},
		"vig-chapter-top":    {Dim: struct{ Width, Height int }{800, 100}},
		"vig-chapter-bottom": {Dim: struct{ Width, Height int }{800, 100}},
		"vig-chapter-end":    {Dim: struct{ Width, Height int }{800, 100}},
		"vig-section-top":    {Dim: struct{ Width, Height int }{800, 100}},
		"vig-section-bottom": {Dim: struct{ Width, Height int }{800, 100}},
		"vig-section-end":    {Dim: struct{ Width, Height int }{800, 100}},
	}
}

// createVignetteResources creates image resources from book images
func createVignetteResources(images fb2.BookImages) imageResourceInfoByID {
	resources := make(imageResourceInfoByID)
	for id, img := range images {
		resources[id] = imageResourceInfo{
			ResourceName: "rsrc-" + id,
			Width:        img.Dim.Width,
			Height:       img.Dim.Height,
		}
	}
	return resources
}

// collectStorylineVignettes extracts vignettes from all storyline fragments
func collectStorylineVignettes(fragments *FragmentList) map[string][]string {
	result := make(map[string][]string)
	for _, frag := range fragments.fragments {
		if frag.FType == SymStoryline {
			name := extractStorylineNameFromFrag(frag)
			vignettes := extractVignetteResourcesFromFrag(frag)
			result[name] = vignettes
		}
	}
	return result
}

// containsVignette checks if a vignette is in the list
func containsVignette(vignettes []string, target string) bool {
	for _, v := range vignettes {
		if v == target {
			return true
		}
	}
	return false
}

// countVignette counts occurrences of a vignette in the list
func countVignette(vignettes []string, target string) int {
	count := 0
	for _, v := range vignettes {
		if v == target {
			count++
		}
	}
	return count
}
