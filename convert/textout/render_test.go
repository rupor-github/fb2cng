package textout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/language"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestRenderMarkdownSemanticOutput(t *testing.T) {
	noteNum := 1
	c := testContent(common.OutputFmtMd)
	c.Book.Description.TitleInfo.Authors = []fb2.Author{{FirstName: "Ada", LastName: "Lovelace"}}
	c.Book.Description.TitleInfo.Sequences = []fb2.Sequence{{Name: "Series", Number: &noteNum}}
	c.Book.Description.TitleInfo.Genres = []fb2.GenreRef{{Value: "sf"}}
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: title("Chapter *One*"),
			Image: &fb2.Image{Alt: "Map"},
			Content: []fb2.FlowItem{
				paragraphItem(
					fb2.InlineSegment{Kind: fb2.InlineText, Text: "Hello "},
					fb2.InlineSegment{Kind: fb2.InlineStrong, Text: "world"},
					fb2.InlineSegment{Kind: fb2.InlineText, Text: " "},
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "https://example.com", Text: "site"},
				),
				paragraphItem(
					fb2.InlineSegment{Kind: fb2.InlineText, Text: "See\n    "},
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#chapter-1", Text: "Chapter"},
					fb2.InlineSegment{Kind: fb2.InlineText, Text: "\n    end"},
				),
				paragraphItem(
					fb2.InlineSegment{Kind: fb2.InlineText, Text: "Author"},
					fb2.InlineSegment{Kind: fb2.InlineSup, Text: "1"},
				),
				{Kind: fb2.FlowTable, Table: &fb2.Table{Rows: []fb2.TableRow{
					{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "A"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "B"}}}}},
					{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "1"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "2"}}}}},
				}}},
			},
		}},
	}}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "# Test Book")
	assertContains(t, got, "Authors: Ada Lovelace")
	assertContains(t, got, "Series: Series #1")
	assertContains(t, got, "## Chapter \\*One\\*")
	assertContains(t, got, "Hello **world** [site](https://example.com)")
	assertContains(t, got, "See [Chapter](#chapter-1) end")
	assertContains(t, got, "Author<sup>1</sup>")
	assertContains(t, got, "[Image: Map]")
	assertContains(t, got, "| A | B |")
	assertContains(t, got, "| --- | --- |")
}

func TestRenderMarkdownTOCLinksToStableAnchors(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "s_1",
			Title: title("Chapter One"),
			Content: []fb2.FlowItem{{Kind: fb2.FlowSection, Section: &fb2.Section{
				ID:      "s_2",
				Title:   title("Nested Chapter"),
				Content: []fb2.FlowItem{paragraphItem(fb2.InlineSegment{Kind: fb2.InlineText, Text: "Text"})},
			}}},
		}},
	}}
	cfg := testConfig()
	cfg.TOCPage.Placement = common.TOCPagePlacementBefore

	got, err := renderForTest(c, cfg)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "- [Chapter One](#s_1)")
	assertContains(t, got, "  - [Nested Chapter](#s_2)")
	assertContains(t, got, "<a id=\"s_1\"></a>\n## Chapter One")
	assertContains(t, got, "<a id=\"s_2\"></a>\n### Nested Chapter")
}

func TestRenderTXTReadableOutput(t *testing.T) {
	c := testContent(common.OutputFmtTxt)
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: title("Chapter One"),
			Content: []fb2.FlowItem{
				paragraphItem(fb2.InlineSegment{Kind: fb2.InlineText, Text: "A paragraph."}),
				{Kind: fb2.FlowTable, Table: &fb2.Table{Rows: []fb2.TableRow{
					{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Long"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "B"}}}}},
					{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "1"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "2"}}}}},
				}}},
			},
		}},
	}}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "TEST BOOK")
	assertContains(t, got, "Chapter One\n-----------")
	assertContains(t, got, "A paragraph.")
	assertContains(t, got, "Long  B\n1     2")
}

func TestRenderMarkdownExternalImageWritesAsset(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.Book.Description.DocumentInfo.ID = "book-id"
	c.ImagesIndex = fb2.BookImages{"img-1": {MimeType: "image/png", Data: []byte("png-data"), Filename: "images/pic.png"}}
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:      "chapter-1",
			Title:   title("Chapter"),
			Content: []fb2.FlowItem{{Kind: fb2.FlowImage, Image: &fb2.Image{Href: "#img-1", Alt: "Picture"}}},
		}},
	}}
	cfg := testConfig()
	cfg.Images.Markdown = common.MarkdownImageModeExternal
	outPath := filepath.Join(t.TempDir(), "book.md")

	data, err := RenderWithOptions(c, cfg, RenderOptions{OutputPath: outPath})
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}
	got := strings.TrimSpace(string(data))

	assertContains(t, got, "![Picture](images-book-id/pic.png)")
	assetPath := filepath.Join(filepath.Dir(outPath), "images-book-id", "pic.png")
	asset, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read external image asset: %v", err)
	}
	if string(asset) != "png-data" {
		t.Fatalf("external image asset = %q, want png-data", string(asset))
	}
}

func TestRenderMarkdownEmbeddedImageUsesDataURI(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.ImagesIndex = fb2.BookImages{"img-1": {MimeType: "image/png", Data: []byte("png-data"), Filename: "images/pic.png"}}
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:      "chapter-1",
			Title:   title("Chapter"),
			Content: []fb2.FlowItem{{Kind: fb2.FlowImage, Image: &fb2.Image{Href: "#img-1", Alt: "Picture"}}},
		}},
	}}
	cfg := testConfig()
	cfg.Images.Markdown = common.MarkdownImageModeEmbedded

	got, err := renderForTest(c, cfg)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "![Picture](data:image/png;base64,cG5nLWRhdGE=)")
}

func TestRenderFootnotesDefaultRendersFootnoteBody(t *testing.T) {
	c := footnoteContent(common.OutputFmtMd, common.FootnotesModeDefault)

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "Text [\\[note\\]](#note-1)")
	assertContains(t, got, "## Notes")
	assertContains(t, got, "### Note 1")
	assertContains(t, got, "<a id=\"note-1\"></a>\n### Note 1")
	assertContains(t, got, "Footnote text.")
}

func TestRenderFootnotesFloatCollectsEndnotes(t *testing.T) {
	c := footnoteContent(common.OutputFmtMd, common.FootnotesModeFloat)

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "Text [\\[note\\]](#note-note-1)")
	assertContains(t, got, "## Notes")
	assertContains(t, got, "<a id=\"note-note-1\"></a>\n### 1. Note 1")
	assertContains(t, got, "Footnote text.")
	if strings.Contains(got, "### Note 1") {
		t.Fatalf("float mode rendered footnote body and endnote:\n%s", got)
	}
}

func TestRenderFootnotesFloatDoesNotDoubleWrapBracketedLabels(t *testing.T) {
	c := footnoteContent(common.OutputFmtMd, common.FootnotesModeFloat)
	c.Book.Bodies[0].Sections[0].Content = []fb2.FlowItem{paragraphItem(
		fb2.InlineSegment{Kind: fb2.InlineText, Text: "Text "},
		fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note-1", Text: "[1]"},
	)}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "Text [\\[1\\]](#note-note-1)")
	if strings.Contains(got, "[\\[1\\]]]") || strings.Contains(got, "[\\[1\\]] ") {
		t.Fatalf("bracketed footnote label was wrapped twice:\n%s", got)
	}
}

func TestRenderFootnotesFloatPreservesMarkdownTable(t *testing.T) {
	c := footnoteContent(common.OutputFmtMd, common.FootnotesModeFloat)
	c.Book.Bodies[1].Sections[0].Content = append(c.Book.Bodies[1].Sections[0].Content, fb2.FlowItem{
		Kind: fb2.FlowTable,
		Table: &fb2.Table{Rows: []fb2.TableRow{
			{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "A"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "B"}}}}},
			{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "1"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "2"}}}}},
		}},
	})

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "<a id=\"note-note-1\"></a>\n### 1. Note 1")
	assertContains(t, got, "Footnote text.\n\n| A | B |\n| --- | --- |\n| 1 | 2 |")
}

func TestRenderFootnotesFloatRenumberedUsesDisplayText(t *testing.T) {
	c := footnoteContent(common.OutputFmtMd, common.FootnotesModeFloatRenumbered)
	c.FootnotesIndex["note-1"] = fb2.FootnoteRef{BodyIdx: 1, SectionIdx: 0, DisplayText: "7"}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "Text [\\[7\\]](#note-note-1)")
	assertContains(t, got, "<a id=\"note-note-1\"></a>\n### 1. 7")
	if strings.Contains(got, "Text [note]") || strings.Contains(got, "### 1. Note 1") {
		t.Fatalf("floatRenumbered did not prefer DisplayText:\n%s", got)
	}
}

func TestRenderFootnotesFloatIncludesNestedReferencedNotes(t *testing.T) {
	c := footnoteContent(common.OutputFmtMd, common.FootnotesModeFloatRenumbered)
	c.FootnotesIndex["note-1"] = fb2.FootnoteRef{BodyIdx: 1, SectionIdx: 0, DisplayText: "1.1"}
	c.FootnotesIndex["note-2"] = fb2.FootnoteRef{BodyIdx: 1, SectionIdx: 1, DisplayText: "1.2"}
	c.Book.Bodies[1].Sections = append(c.Book.Bodies[1].Sections, fb2.Section{
		ID:      "note-2",
		Title:   title("1.2"),
		Content: []fb2.FlowItem{paragraphItem(fb2.InlineSegment{Kind: fb2.InlineText, Text: "Nested note."})},
	})
	c.Book.Bodies[1].Sections[0].Content = []fb2.FlowItem{paragraphItem(
		fb2.InlineSegment{Kind: fb2.InlineText, Text: "First note references "},
		fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note-2", Text: "old"},
	)}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "Text [\\[1.1\\]](#note-note-1)")
	assertContains(t, got, "First note references [\\[1.2\\]](#note-note-2)")
	assertContains(t, got, "<a id=\"note-note-2\"></a>\n### 2. 1.2")
	assertContains(t, got, "Nested note.")
}

func TestRenderMarkdownDoesNotTurnTextAuthorIntoList(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: title("Chapter"),
			Epigraphs: []fb2.Epigraph{{
				Flow: fb2.Flow{Items: []fb2.FlowItem{paragraphItem(fb2.InlineSegment{Kind: fb2.InlineText, Text: "Quote"})}},
				TextAuthors: []fb2.Paragraph{{Text: []fb2.InlineSegment{
					{Kind: fb2.InlineText, Text: "Author"},
					{Kind: fb2.InlineSup, Text: "1"},
				}}},
			}},
		}},
	}}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "> Quote")
	assertContains(t, got, "> \\- Author<sup>1</sup>")
}

func TestRenderMarkdownCodeDoesNotLeakNestedFormatting(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: title("Chapter"),
			Content: []fb2.FlowItem{paragraphItem(
				fb2.InlineSegment{Kind: fb2.InlineText, Text: "Element "},
				fb2.InlineSegment{Kind: fb2.InlineCode, Children: []fb2.InlineSegment{{Kind: fb2.InlineStrong, Text: "<code>"}}},
				fb2.InlineSegment{Kind: fb2.InlineText, Text: " is code."},
			)},
		}},
	}}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "Element `<code>` is code.")
	if strings.Contains(got, "*\\<code\\>*") || strings.Contains(got, "**") {
		t.Fatalf("code leaked nested Markdown formatting:\n%s", got)
	}
}

func TestRenderMarkdownCodeParagraphUsesFence(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: title("Chapter"),
			Content: []fb2.FlowItem{paragraphItem(
				fb2.InlineSegment{Kind: fb2.InlineCode, Text: "line 1\n    line 2\nline 3"},
			)},
		}},
	}}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "```\nline 1\n    line 2\nline 3\n```")
}

func TestRenderMarkdownInlineMarkersIgnoreSurroundingWhitespace(t *testing.T) {
	c := testContent(common.OutputFmtMd)
	c.Book.Bodies = []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID: "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
				{Kind: fb2.InlineEmphasis, Text: " Подстрочник в заголовке. "},
			}}}}},
			Content: []fb2.FlowItem{paragraphItem(
				fb2.InlineSegment{Kind: fb2.InlineText, Text: "Before"},
				fb2.InlineSegment{Kind: fb2.InlineStrong, Text: " strong "},
				fb2.InlineSegment{Kind: fb2.InlineText, Text: "after"},
			)},
		}},
	}}

	got, err := renderForTest(c, testConfig())
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	assertContains(t, got, "## *Подстрочник в заголовке.*")
	assertContains(t, got, "Before **strong** after")
	if strings.Contains(got, "* Подстрочник") || strings.Contains(got, "strong **") {
		t.Fatalf("Markdown markers include surrounding whitespace:\n%s", got)
	}
}

func testContent(format common.OutputFmt) *content.Content {
	return &content.Content{
		SrcName:      "test.fb2",
		OutputFormat: format,
		Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			BookTitle: fb2.TextField{Value: "Test Book"},
			Lang:      language.English,
		}}},
		ImagesIndex: fb2.BookImages{},
	}
}

func footnoteContent(format common.OutputFmt, mode common.FootnotesMode) *content.Content {
	c := testContent(format)
	c.FootnotesMode = mode
	c.FootnotesIndex = fb2.FootnoteRefs{"note-1": {BodyIdx: 1, SectionIdx: 0, DisplayText: "Note 1"}}
	c.Book.Bodies = []fb2.Body{
		{Kind: fb2.BodyMain, Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: title("Chapter"),
			Content: []fb2.FlowItem{paragraphItem(
				fb2.InlineSegment{Kind: fb2.InlineText, Text: "Text "},
				fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note-1", Text: "note"},
			)},
		}}},
		{Name: "notes", Kind: fb2.BodyFootnotes, Title: title("Notes"), Sections: []fb2.Section{{
			ID:      "note-1",
			Title:   title("Note 1"),
			Content: []fb2.FlowItem{paragraphItem(fb2.InlineSegment{Kind: fb2.InlineText, Text: "Footnote text."})},
		}}},
	}
	return c
}

func testConfig() *config.DocumentConfig {
	return &config.DocumentConfig{}
}

func renderForTest(c *content.Content, cfg *config.DocumentConfig) (string, error) {
	data, err := Render(c, cfg)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func title(text string) *fb2.Title {
	return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: text}}}}}}
}

func paragraphItem(segments ...fb2.InlineSegment) fb2.FlowItem {
	return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: segments}}
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output does not contain %q:\n%s", want, got)
	}
}
