package kfx

import (
	"strings"
	"testing"

	"golang.org/x/text/language"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

// TestBodyImageSplit_WithPageBreak verifies that when CSS has
// page-break-before:always on .body-title and the body has both an image
// and a title, the image is placed in a separate storyline from the title.
//
// Expected storylines:
//
//	l1 = cover
//	l2 = body image (separate — due to page-break-before: always)
//	l3 = body title + epigraphs
//	l4 = chapter 1
func TestBodyImageSplit_WithPageBreak(t *testing.T) {
	book := createBodyIntroBook(true) // has body image
	book.SetBodyTitlePageBreak(true)

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  createBodyIntroImages(),
	}
	imageResources := createBodyIntroResources(c.ImagesIndex)
	styles := NewStyleRegistry()

	fragments, _, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	storylineImages := collectStorylineImages(fragments)
	storylineNames := collectStorylineNames(fragments)

	// Expect 4 storylines: cover, body-image, body-title, chapter1
	if len(storylineNames) != 4 {
		t.Fatalf("expected 4 storylines, got %d: %v", len(storylineNames), storylineNames)
	}

	// l1 = cover
	if !containsImage(storylineImages["l1"], "rsrc-cover.jpg") {
		t.Errorf("l1 (cover) should contain cover image, got %v", storylineImages["l1"])
	}

	// l2 = body image only
	if !containsImage(storylineImages["l2"], "rsrc-body-img.jpg") {
		t.Errorf("l2 (body image) should contain body image, got %v", storylineImages["l2"])
	}
	// l2 should NOT contain any text content (title), verify by absence of heading resources
	// The body image storyline should have exactly one image
	imgCount := 0
	for _, name := range storylineImages["l2"] {
		if name == "rsrc-body-img.jpg" {
			imgCount++
		}
	}
	if imgCount != 1 {
		t.Errorf("l2 should have exactly 1 body image, got %d in %v", imgCount, storylineImages["l2"])
	}

	// l3 = body title (no body image since it was split out)
	if containsImage(storylineImages["l3"], "rsrc-body-img.jpg") {
		t.Errorf("l3 (body title) should NOT contain body image, got %v", storylineImages["l3"])
	}

	// l4 = chapter 1
	if storylineNames[3] != "l4" {
		t.Errorf("expected 4th storyline to be l4, got %s", storylineNames[3])
	}
}

// TestBodyImageNoSplit_WithoutPageBreak verifies that when CSS does NOT have
// page-break-before:always on .body-title, the body image and title remain
// in the same storyline (original behavior preserved).
//
// Expected storylines:
//
//	l1 = cover
//	l2 = body image + title + epigraphs (combined)
//	l3 = chapter 1
func TestBodyImageNoSplit_WithoutPageBreak(t *testing.T) {
	book := createBodyIntroBook(true) // has body image
	// bodyTitlePageBreak is false by default — no split

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  createBodyIntroImages(),
	}
	imageResources := createBodyIntroResources(c.ImagesIndex)
	styles := NewStyleRegistry()

	fragments, _, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	storylineImages := collectStorylineImages(fragments)
	storylineNames := collectStorylineNames(fragments)

	// Expect 3 storylines: cover, body-intro (combined), chapter1
	if len(storylineNames) != 3 {
		t.Fatalf("expected 3 storylines, got %d: %v", len(storylineNames), storylineNames)
	}

	// l1 = cover
	if !containsImage(storylineImages["l1"], "rsrc-cover.jpg") {
		t.Errorf("l1 (cover) should contain cover image, got %v", storylineImages["l1"])
	}

	// l2 = combined body intro (image + title)
	if !containsImage(storylineImages["l2"], "rsrc-body-img.jpg") {
		t.Errorf("l2 (body intro) should contain body image, got %v", storylineImages["l2"])
	}

	// l3 = chapter 1
	if storylineNames[2] != "l3" {
		t.Errorf("expected 3rd storyline to be l3, got %s", storylineNames[2])
	}
}

// TestBodyNoImage_WithPageBreak verifies that when CSS has page-break-before:always
// on .body-title but the body has no image, no extra storyline is created.
//
// Expected storylines:
//
//	l1 = cover
//	l2 = body title only
//	l3 = chapter 1
func TestBodyNoImage_WithPageBreak(t *testing.T) {
	book := createBodyIntroBook(false) // no body image
	book.SetBodyTitlePageBreak(true)

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  createBodyIntroImages(),
	}
	imageResources := createBodyIntroResources(c.ImagesIndex)
	styles := NewStyleRegistry()

	fragments, _, _, _, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	storylineNames := collectStorylineNames(fragments)

	// Expect 3 storylines: cover, body-title, chapter1 (no extra image storyline)
	if len(storylineNames) != 3 {
		t.Fatalf("expected 3 storylines, got %d: %v", len(storylineNames), storylineNames)
	}
}

// ============================================================================
// Helper functions for body intro tests
// ============================================================================

// createBodyIntroBook creates a test FictionBook with body title and optionally body image.
func createBodyIntroBook(withBodyImage bool) *fb2.FictionBook {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
				Coverpage: []fb2.InlineImage{{Href: "#cover.jpg"}},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-body-intro",
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
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1 content."}}}},
						},
					},
				},
			},
		},
	}

	if withBodyImage {
		book.Bodies[0].Image = &fb2.Image{
			Href: "#body-img.jpg",
			Alt:  "Body image",
		}
	}

	return book
}

// createBodyIntroImages creates the image index for body intro tests.
func createBodyIntroImages() fb2.BookImages {
	return fb2.BookImages{
		"cover.jpg":    {Dim: struct{ Width, Height int }{1000, 1500}},
		"body-img.jpg": {Dim: struct{ Width, Height int }{450, 450}},
	}
}

// createBodyIntroResources creates image resource info from book images.
func createBodyIntroResources(images fb2.BookImages) imageResourceInfoByID {
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

// collectStorylineNames returns ordered storyline names from fragments.
func collectStorylineNames(fragments *FragmentList) []string {
	var names []string
	for _, frag := range fragments.fragments {
		if frag.FType == SymStoryline {
			name := extractStorylineNameFromFrag(frag)
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// collectStorylineImages extracts all image resource names per storyline.
func collectStorylineImages(fragments *FragmentList) map[string][]string {
	result := make(map[string][]string)
	for _, frag := range fragments.fragments {
		if frag.FType == SymStoryline {
			name := extractStorylineNameFromFrag(frag)
			images := extractImageResourcesFromFrag(frag)
			result[name] = images
		}
	}
	return result
}

// extractImageResourcesFromFrag extracts all image resource names from a storyline fragment.
func extractImageResourcesFromFrag(frag *Fragment) []string {
	var images []string
	if frag.Value == nil {
		return images
	}

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

	var search func(v any)
	search = func(v any) {
		switch val := v.(type) {
		case StructValue:
			if resName, ok := val[SymResourceName]; ok {
				name := extractName(resName)
				if name != "" && strings.HasPrefix(name, "rsrc-") {
					images = append(images, name)
				}
			}
			for _, child := range val {
				search(child)
			}
		case map[KFXSymbol]any:
			if resName, ok := val[SymResourceName]; ok {
				name := extractName(resName)
				if name != "" && strings.HasPrefix(name, "rsrc-") {
					images = append(images, name)
				}
			}
			for _, child := range val {
				search(child)
			}
		case []any:
			for _, child := range val {
				search(child)
			}
		}
	}

	search(frag.Value)
	return images
}

// containsImage checks if an image resource name is in the list.
func containsImage(images []string, target string) bool {
	for _, img := range images {
		if img == target {
			return true
		}
	}
	return false
}
