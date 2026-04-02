package kfx

import (
	"testing"

	"golang.org/x/text/language"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func TestGenerateStoryline_SectionBreakDepthThree(t *testing.T) {
	book := createSectionBreakTestBook()
	book.SetSectionPageBreaks(map[int]bool{3: true})

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  fb2.BookImages{},
	}

	styles := NewStyleRegistry()
	imageResources := make(imageResourceInfoByID)

	_, _, sectionNames, tocEntries, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	if len(sectionNames) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(sectionNames), sectionNames)
	}
	if sectionNames[0] != "c0" || sectionNames[1] != "c1" {
		t.Fatalf("unexpected section names: %v", sectionNames)
	}

	if len(tocEntries) != 1 {
		t.Fatalf("expected 1 top-level TOC entry, got %d: %+v", len(tocEntries), tocEntries)
	}

	chapter := tocEntries[0]
	if chapter.Title != "Chapter" {
		t.Fatalf("expected chapter title 'Chapter', got %q", chapter.Title)
	}
	if chapter.StoryName != "l1" || chapter.SectionName != "c0" {
		t.Fatalf("unexpected chapter location: story=%q section=%q", chapter.StoryName, chapter.SectionName)
	}

	if len(chapter.Children) != 2 {
		t.Fatalf("expected 2 child entries under chapter, got %d: %+v", len(chapter.Children), chapter.Children)
	}

	deep := findTOCEntry(chapter.Children, "Deep Section")
	if deep == nil {
		t.Fatalf("expected child title 'Deep Section', got %+v", chapter.Children)
	}
	if deep.StoryName != "l2" || deep.SectionName != "c1" {
		t.Fatalf("unexpected deep section location: story=%q section=%q", deep.StoryName, deep.SectionName)
	}
	if len(deep.Children) != 0 {
		t.Fatalf("expected no nested TOC entries under deep section, got %+v", deep.Children)
	}

	sibling := findTOCEntry(chapter.Children, "Sibling Section")
	if sibling == nil {
		t.Fatalf("expected child title 'Sibling Section', got %+v", chapter.Children)
	}
	if sibling.StoryName != "" || sibling.SectionName != "" {
		t.Fatalf("expected sibling depth-2 section to remain inline, got story=%q section=%q", sibling.StoryName, sibling.SectionName)
	}
	if len(sibling.Children) != 0 {
		t.Fatalf("expected no nested TOC entries under sibling section, got %+v", sibling.Children)
	}
}

func TestGenerateStoryline_SectionBreakDepthTwoOnly(t *testing.T) {
	book := createSectionBreakTestBook()
	book.SetSectionPageBreaks(map[int]bool{2: true})

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		ScreenWidth:  1264,
		ImagesIndex:  fb2.BookImages{},
	}

	styles := NewStyleRegistry()
	imageResources := make(imageResourceInfoByID)

	_, _, sectionNames, tocEntries, _, _, _, _, err := generateStoryline(c, styles, imageResources, 1000)
	if err != nil {
		t.Fatalf("generateStoryline failed: %v", err)
	}

	if len(sectionNames) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(sectionNames), sectionNames)
	}
	if sectionNames[0] != "c0" || sectionNames[1] != "c1" {
		t.Fatalf("unexpected section names: %v", sectionNames)
	}

	if len(tocEntries) != 1 {
		t.Fatalf("expected 1 top-level TOC entry, got %d: %+v", len(tocEntries), tocEntries)
	}

	chapter := tocEntries[0]
	if chapter.Title != "Chapter" {
		t.Fatalf("expected chapter title 'Chapter', got %q", chapter.Title)
	}
	if chapter.StoryName != "l1" || chapter.SectionName != "c0" {
		t.Fatalf("unexpected chapter location: story=%q section=%q", chapter.StoryName, chapter.SectionName)
	}

	if len(chapter.Children) != 2 {
		t.Fatalf("expected 2 child entries under chapter, got %d: %+v", len(chapter.Children), chapter.Children)
	}

	sibling := findTOCEntry(chapter.Children, "Sibling Section")
	if sibling == nil {
		t.Fatalf("expected child title 'Sibling Section', got %+v", chapter.Children)
	}
	if sibling.StoryName != "l2" || sibling.SectionName != "c1" {
		t.Fatalf("unexpected sibling section location: story=%q section=%q", sibling.StoryName, sibling.SectionName)
	}
	if len(sibling.Children) != 0 {
		t.Fatalf("expected no nested TOC entries under sibling section, got %+v", sibling.Children)
	}

	deep := findTOCEntry(chapter.Children, "Deep Section")
	if deep == nil {
		t.Fatalf("expected child title 'Deep Section', got %+v", chapter.Children)
	}
	if deep.StoryName != "" || deep.SectionName != "" {
		t.Fatalf("expected depth-3 section to remain inline, got story=%q section=%q", deep.StoryName, deep.SectionName)
	}
	if len(deep.Children) != 0 {
		t.Fatalf("expected no nested TOC entries under deep section, got %+v", deep.Children)
	}
}

func createSectionBreakTestBook() *fb2.FictionBook {
	return &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-kfx-section-breaks",
			},
		},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:    "chap",
				Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
				Content: []fb2.FlowItem{
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							Content: []fb2.FlowItem{
								{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Inner intro"}}}},
								{
									Kind: fb2.FlowSection,
									Section: &fb2.Section{
										ID:      "deep",
										Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Deep Section"}}}}}},
										Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Deep content"}}}}},
									},
								},
							},
						},
					},
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							ID:      "sibling",
							Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Sibling Section"}}}}}},
							Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Sibling content"}}}}},
						},
					},
				},
			}},
		}},
	}
}

func findTOCEntry(entries []*TOCEntry, title string) *TOCEntry {
	for _, entry := range entries {
		if entry.Title == title {
			return entry
		}
		if found := findTOCEntry(entry.Children, title); found != nil {
			return found
		}
	}
	return nil
}
