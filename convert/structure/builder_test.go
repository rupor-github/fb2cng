package structure

import (
	"reflect"
	"testing"

	"golang.org/x/text/language"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

type unitSummary struct {
	Kind       UnitKind
	ID         string
	Depth      int
	TitleDepth int
}

func summarizeUnits(plan *Plan) []unitSummary {
	out := make([]unitSummary, 0, len(plan.Units))
	for _, u := range plan.Units {
		out = append(out, unitSummary{
			Kind:       u.Kind,
			ID:         u.ID,
			Depth:      u.Depth,
			TitleDepth: u.TitleDepth,
		})
	}
	return out
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

func TestBuildPlan_BodyImageSplit_WithPageBreak(t *testing.T) {
	book := createBodyIntroBook(true)
	book.SetBodyTitlePageBreak(true)

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		CoverID:      "cover.jpg",
	}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitCover, ID: "cover.jpg"},
		{Kind: UnitBodyImage, ID: "a-body-image-0"},
		{Kind: UnitBodyIntro, ID: "a-body-0"},
		{Kind: UnitSection, ID: "chap1", Depth: 1, TitleDepth: 1},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	if got, want := plan.Landmarks.CoverID, "cover.jpg"; got != want {
		t.Fatalf("CoverID = %q, want %q", got, want)
	}
	if got, want := plan.Landmarks.StartID, "a-body-image-0"; got != want {
		t.Fatalf("StartID = %q, want %q", got, want)
	}

	bodyIntro := findTOCEntry(plan.TOC, "Book Title")
	if bodyIntro == nil {
		t.Fatal("missing body intro TOC entry")
	}
}

func TestBuildPlan_BodyImageNoSplit_WithoutPageBreak(t *testing.T) {
	book := createBodyIntroBook(true)

	c := &content.Content{
		Book:         book,
		OutputFormat: common.OutputFmtKfx,
		CoverID:      "cover.jpg",
	}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitCover, ID: "cover.jpg"},
		{Kind: UnitBodyIntro, ID: "a-body-0"},
		{Kind: UnitSection, ID: "chap1", Depth: 1, TitleDepth: 1},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	if got, want := plan.Landmarks.StartID, "a-body-0"; got != want {
		t.Fatalf("StartID = %q, want %q", got, want)
	}

	if bodyIntro := findTOCEntry(plan.TOC, "Book Title"); bodyIntro == nil {
		t.Fatal("missing body intro TOC entry")
	}
}

func TestBuildPlan_SectionBreakDepthThree(t *testing.T) {
	book := createSectionBreakTestBook()
	book.SetSectionPageBreaks(map[int]bool{3: true})

	c := &content.Content{Book: book, OutputFormat: common.OutputFmtKfx}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitSection, ID: "chap", Depth: 1, TitleDepth: 1},
		{Kind: UnitSection, ID: "deep", Depth: 3, TitleDepth: 2},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	chapter := findTOCEntry(plan.TOC, "Chapter")
	if chapter == nil {
		t.Fatal("missing Chapter TOC entry")
	}
	if findTOCEntry(chapter.Children, "Deep Section") == nil {
		t.Fatal("missing Deep Section TOC entry")
	}
	if findTOCEntry(chapter.Children, "Sibling Section") == nil {
		t.Fatal("missing Sibling Section TOC entry")
	}
}

func TestBuildPlan_SectionBreakDepthTwoOnly(t *testing.T) {
	book := createSectionBreakTestBook()
	book.SetSectionPageBreaks(map[int]bool{2: true})

	c := &content.Content{Book: book, OutputFormat: common.OutputFmtKfx}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitSection, ID: "chap", Depth: 1, TitleDepth: 1},
		{Kind: UnitSection, ID: "sibling", Depth: 2, TitleDepth: 2},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	chapter := findTOCEntry(plan.TOC, "Chapter")
	if chapter == nil {
		t.Fatal("missing Chapter TOC entry")
	}
	if findTOCEntry(chapter.Children, "Deep Section") == nil {
		t.Fatal("missing Deep Section TOC entry")
	}
	if findTOCEntry(chapter.Children, "Sibling Section") == nil {
		t.Fatal("missing Sibling Section TOC entry")
	}
}

func TestBuildPlan_TitledChildOfUntitledTopLevelWrapper_KeepsTitleDepthOne(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo:    fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Test"}, Lang: language.English},
			DocumentInfo: fb2.DocumentInfo{ID: "test-untitled-wrapper"},
		},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID: "wrap",
				Content: []fb2.FlowItem{{
					Kind: fb2.FlowSection,
					Section: &fb2.Section{
						ID:    "child",
						Title: simpleTitle("Chapter 1"),
					},
				}},
			}},
		}},
	}
	book.SetSectionPageBreaks(map[int]bool{2: true})

	c := &content.Content{Book: book, OutputFormat: common.OutputFmtKfx}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitSection, ID: "wrap", Depth: 1, TitleDepth: 1},
		{Kind: UnitSection, ID: "child", Depth: 2, TitleDepth: 1},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	if gotTitles := topLevelTOCTitles(plan.TOC); !reflect.DeepEqual(gotTitles, []string{"Chapter 1"}) {
		t.Fatalf("top-level TOC = %#v, want %#v", gotTitles, []string{"Chapter 1"})
	}
	if findTOCEntry(plan.TOC, "Chapter 1") == nil {
		t.Fatal("missing promoted TOC entry for titled child")
	}
}

func TestBuildPlan_TitledChildOfDoubleUntitledWrapper_KeepsTitleDepthOne(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo:    fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Test"}, Lang: language.English},
			DocumentInfo: fb2.DocumentInfo{ID: "test-double-untitled-wrapper"},
		},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID: "outer",
				Content: []fb2.FlowItem{{
					Kind: fb2.FlowSection,
					Section: &fb2.Section{
						ID: "inner",
						Content: []fb2.FlowItem{{
							Kind: fb2.FlowSection,
							Section: &fb2.Section{
								ID:    "child",
								Title: simpleTitle("Chapter 1"),
							},
						}},
					},
				}},
			}},
		}},
	}
	book.SetSectionPageBreaks(map[int]bool{3: true})

	c := &content.Content{Book: book, OutputFormat: common.OutputFmtKfx}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitSection, ID: "outer", Depth: 1, TitleDepth: 1},
		{Kind: UnitSection, ID: "child", Depth: 3, TitleDepth: 1},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	if gotTitles := topLevelTOCTitles(plan.TOC); !reflect.DeepEqual(gotTitles, []string{"Chapter 1"}) {
		t.Fatalf("top-level TOC = %#v, want %#v", gotTitles, []string{"Chapter 1"})
	}
}

func TestBuildPlan_FootnoteBodiesAppendedAfterMainContent(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo:    fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Test"}, Lang: language.English},
			DocumentInfo: fb2.DocumentInfo{ID: "test-footnotes"},
		},
		Bodies: []fb2.Body{
			{
				Kind:     fb2.BodyMain,
				Sections: []fb2.Section{{ID: "chap1", Title: simpleTitle("Chapter 1")}},
			},
			{
				Kind:  fb2.BodyFootnotes,
				Name:  "notes",
				Title: simpleTitle("Notes"),
				Sections: []fb2.Section{{
					ID:    "note1",
					Title: simpleTitle("1"),
				}},
			},
		},
	}

	c := &content.Content{Book: book, OutputFormat: common.OutputFmtKfx, FootnotesMode: common.FootnotesModeDefault}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	want := []unitSummary{
		{Kind: UnitSection, ID: "chap1", Depth: 1, TitleDepth: 1},
		{Kind: UnitFootnotesBody, ID: "a-notes-0"},
	}
	if got := summarizeUnits(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %#v, want %#v", got, want)
	}

	notes := findTOCEntry(plan.TOC, "Notes")
	if notes == nil {
		t.Fatal("missing Notes TOC entry")
	}
	if findTOCEntry(notes.Children, "1") == nil {
		t.Fatal("missing footnote child TOC entry")
	}
}

func TestBuildPlan_FootnoteBodyTOC_FloatModeOmitsChildren(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo:    fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Test"}, Lang: language.English},
			DocumentInfo: fb2.DocumentInfo{ID: "test-footnotes-float"},
		},
		Bodies: []fb2.Body{
			{
				Kind:     fb2.BodyMain,
				Sections: []fb2.Section{{ID: "chap1", Title: simpleTitle("Chapter 1")}},
			},
			{
				Kind:  fb2.BodyFootnotes,
				Name:  "notes",
				Title: simpleTitle("Notes"),
				Sections: []fb2.Section{{
					ID:    "note1",
					Title: simpleTitle("1"),
				}},
			},
		},
	}

	c := &content.Content{Book: book, OutputFormat: common.OutputFmtKfx, FootnotesMode: common.FootnotesModeFloat}

	plan, err := BuildPlan(c)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	notes := findTOCEntry(plan.TOC, "Notes")
	if notes == nil {
		t.Fatal("missing Notes TOC entry")
	}
	if len(notes.Children) != 0 {
		t.Fatalf("float mode should omit child note TOC entries, got %#v", notes.Children)
	}
}

func createBodyIntroBook(withBodyImage bool) *fb2.FictionBook {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
				Coverpage: []fb2.InlineImage{{Href: "#cover.jpg"}},
			},
			DocumentInfo: fb2.DocumentInfo{ID: "test-body-intro"},
		},
		Bodies: []fb2.Body{{
			Kind:  fb2.BodyMain,
			Title: simpleTitle("Book Title"),
			Sections: []fb2.Section{{
				ID:    "chap1",
				Title: simpleTitle("Chapter 1"),
			}},
		}},
	}

	if withBodyImage {
		book.Bodies[0].Image = &fb2.Image{Href: "#body-img.jpg", Alt: "Body image"}
	}

	return book
}

func createSectionBreakTestBook() *fb2.FictionBook {
	return &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Test Book"},
				Authors:   []fb2.Author{{LastName: "Author"}},
				Lang:      language.English,
			},
			DocumentInfo: fb2.DocumentInfo{ID: "test-kfx-section-breaks"},
		},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:    "chap",
				Title: simpleTitle("Chapter"),
				Content: []fb2.FlowItem{
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							Content: []fb2.FlowItem{
								{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Inner intro"}}}},
								{
									Kind: fb2.FlowSection,
									Section: &fb2.Section{
										ID:    "deep",
										Title: simpleTitle("Deep Section"),
									},
								},
							},
						},
					},
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							ID:    "sibling",
							Title: simpleTitle("Sibling Section"),
						},
					},
				},
			}},
		}},
	}
}

func simpleTitle(text string) *fb2.Title {
	return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}}}
}

func topLevelTOCTitles(entries []*TOCEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Title)
	}
	return out
}
