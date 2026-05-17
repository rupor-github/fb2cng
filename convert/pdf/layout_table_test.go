package pdf

import (
	"strings"
	"testing"

	"fbc/content"
	"fbc/fb2"
)

func TestCollectTextBlocksKeepsTableAsNativeBlock(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			Content: []fb2.FlowItem{{Kind: fb2.FlowTable, Table: &fb2.Table{
				ID:   "tbl",
				Rows: []fb2.TableRow{{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "A"}}}}}},
			}}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var tableBlocks int
	for _, block := range blocks {
		if block.Kind == pdfBlockTable {
			tableBlocks++
			if block.ID != "tbl" || block.Table == nil {
				t.Fatalf("table block = %#v, want native table with id", block)
			}
		}
		if block.Kind == pdfBlockParagraph && block.Text == "A" {
			t.Fatalf("table was flattened into paragraph block: %#v", block)
		}
	}
	if tableBlocks != 1 {
		t.Fatalf("table blocks = %d, want 1 in %#v", tableBlocks, blocks)
	}
}

func TestLayoutPDFPagesRendersTableCellsBordersAndSpans(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolverWithCSS(t, `
		table { margin: 0; width: 100%; }
		th { padding: 2pt; border: 1pt solid black; background-color: #ccc; }
		td { padding: 2pt; border: 1pt solid black; }
	`)
	table := &fb2.Table{Rows: []fb2.TableRow{{
		Cells: []fb2.TableCell{
			{Header: true, Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Head A"}}},
			{Header: true, Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Head B"}}},
		},
	}, {
		Cells: []fb2.TableCell{{ColSpan: 2, Align: "center", Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Spanned body"}}}},
	}}}

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks:     []pdfTextBlock{{Kind: pdfBlockTable, StyleName: pdfStyleTable, Table: table}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	text := pageText(pages[0])
	for _, want := range []string{"Head A", "Head B", "Spanned body"} {
		if !strings.Contains(text, want) {
			t.Fatalf("page text = %q, missing %q", text, want)
		}
	}
	if len(pages[0].Borders) != 3 {
		t.Fatalf("borders = %d, want one per rendered cell", len(pages[0].Borders))
	}
	if len(pages[0].Backgrounds) != 2 {
		t.Fatalf("backgrounds = %d, want header cell backgrounds", len(pages[0].Backgrounds))
	}
	if pages[0].Borders[2].Width <= pages[0].Borders[0].Width*1.9 {
		t.Fatalf("spanned cell width = %v, first column width = %v", pages[0].Borders[2].Width, pages[0].Borders[0].Width)
	}
}

func TestLayoutPDFTableHonorsHeaderHyphenationNoneAndNoWrap(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `th { hyphens: none; white-space: nowrap; }`)
	cellStyle := resolver.styleForTableCell(fb2.TableRow{}, fb2.TableCell{Header: true})
	if cellStyle.Paragraph.Hyphenation != paragraphHyphenationNone || !cellStyle.Paragraph.NoWrap {
		t.Fatalf("th hyphenation/nowrap = %v/%t, want none/true", cellStyle.Paragraph.Hyphenation, cellStyle.Paragraph.NoWrap)
	}
	table := &fb2.Table{Rows: []fb2.TableRow{{Cells: []fb2.TableCell{{Header: true, Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Строка 1"}}}}}}}
	block := pdfTextBlock{Kind: pdfBlockTable, StyleName: pdfStyleTable, Table: table}
	style := resolver.styleForBlock(block)
	layout, err := layoutPDFTable(skeletonDocument{PageWidth: 120, PageHeight: 120, Styles: resolver, Hyphenator: fakeHyphenator{"Строка": "Стро\u00adка"}}, resolver, block, style, 26)
	if err != nil {
		t.Fatalf("layoutPDFTable() error = %v", err)
	}
	if len(layout.Cells) != 1 || len(layout.Cells[0].Lines) == 0 {
		t.Fatalf("layout table = %#v, want rendered header cell", layout)
	}
	foundNoWrapLine := false
	for _, line := range layout.Cells[0].Lines {
		got := shapedRunes(line.Text)
		if strings.Contains(got, "Стро-") {
			t.Fatalf("hyphens:none header was hyphenated: line %q in %#v", got, layout.Cells[0].Lines)
		}
		if strings.Contains(got, "Строка 1") {
			foundNoWrapLine = true
		}
	}
	if !foundNoWrapLine {
		t.Fatalf("white-space:nowrap header did not keep text together: %#v", layout.Cells[0].Lines)
	}
}

func TestLayoutPDFTableScalesFootnoteLinkStylesWithWideTables(t *testing.T) {
	resolver := newPDFStyleResolverWithDefaultCSS(t, `td { padding: 2pt; }`)
	cells := make([]fb2.TableCell, 10)
	cells[0] = fb2.TableCell{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Cell"}, {Kind: fb2.InlineLink, Href: "#n", LinkType: "note", Children: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "1.12"}}}}}
	for i := 1; i < len(cells); i++ {
		cells[i] = fb2.TableCell{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Column"}}}
	}
	table := &fb2.Table{Rows: []fb2.TableRow{{Cells: cells}}}
	block := pdfTextBlock{Kind: pdfBlockTable, StyleName: pdfStyleTable, Table: table}
	style := resolver.styleForBlock(block)

	layout, err := layoutPDFTable(skeletonDocument{PageWidth: 180, PageHeight: 120, Styles: resolver}, resolver, block, style, 120)
	if err != nil {
		t.Fatalf("layoutPDFTable() error = %v", err)
	}
	if len(layout.Cells) == 0 || len(layout.Cells[0].Lines) == 0 {
		t.Fatalf("layout table cells = %#v, want rendered link cell", layout.Cells)
	}
	var linkFragment *paragraphLineFragment
	for i := range layout.Cells[0].Lines[0].Fragments {
		fragment := &layout.Cells[0].Lines[0].Fragments[i]
		if fragment.LinkHref == "#n" {
			linkFragment = fragment
			break
		}
	}
	if linkFragment == nil {
		t.Fatalf("missing link fragment in %#v", layout.Cells[0].Lines[0].Fragments)
	}
	wantMax := layout.Cells[0].Style.Paragraph.FontSize
	if linkFragment.FontSize >= wantMax {
		t.Fatalf("scaled footnote link font = %v, want smaller than scaled base %v", linkFragment.FontSize, wantMax)
	}
	if linkFragment.FontSize > pdfFootnoteLinkFontSize*0.75 {
		t.Fatalf("scaled footnote link font = %v, want table scale applied below unscaled link font %v", linkFragment.FontSize, pdfFootnoteLinkFontSize)
	}
}

func TestLayoutPDFPagesSplitsTablesBetweenRows(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolverWithCSS(t, `
		table { margin: 0; page-break-inside: auto; }
		td { padding: 2pt; border: 1pt solid black; }
	`)
	table := &fb2.Table{}
	for _, text := range []string{"row one", "row two", "row three", "row four"} {
		table.Rows = append(table.Rows, fb2.TableRow{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: text}}}}})
	}

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 120,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks:     []pdfTextBlock{{Kind: pdfBlockTable, StyleName: pdfStyleTable, Table: table}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("pages = %d, want table split across pages", len(pages))
	}
	if !strings.Contains(pageText(pages[0]), "row one") || strings.Contains(pageText(pages[0]), "row four") {
		t.Fatalf("first page text = %q, want early rows only", pageText(pages[0]))
	}
	if !strings.Contains(pageText(pages[len(pages)-1]), "row four") {
		t.Fatalf("last page text = %q, want final row", pageText(pages[len(pages)-1]))
	}
}
