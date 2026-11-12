// Package publicationtest shares source fixtures between publication and adapter tests.
package publicationtest

import (
	"strings"
	"testing"

	"github.com/beevik/etree"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func Content(format common.OutputFmt, mode common.FootnotesMode, pageSize int) *content.Content {
	paragraph := func(id, text string) fb2.FlowItem {
		return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
			ID: id, Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: text}},
		}}
	}
	main := fb2.Section{ID: "main", Title: &fb2.Title{Items: []fb2.TitleItem{{
		Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Main chapter"}}},
	}}}}
	main.Content = []fb2.FlowItem{
		paragraph("text", "A paragraph with enough text to create several approximate pages. Кириллица тоже считается."),
		{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
			{Kind: fb2.InlineLink, Href: "#note1", Children: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "1"}}},
		}}},
		{Kind: fb2.FlowImage, Image: &fb2.Image{Href: "#art"}},
	}
	return &content.Content{
		OutputFormat: format, FootnotesMode: mode, PageSize: pageSize,
		BacklinkTemplate: "{{.Format}}", CoverID: "cover",
		IDsIndex:       fb2.IDIndex{"toc-page": {}, "annotation-page": {}},
		FootnotesIndex: fb2.FootnoteRefs{"note1": {BodyIdx: 1, SectionIdx: 0}},
		ImagesIndex: fb2.BookImages{
			"cover": {Filename: "images/cover.jpg"}, "art": {Filename: "images/art.png"}, "unused": {Filename: "images/unused.png"},
		},
		Book: &fb2.FictionBook{
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Publication"},
				Annotation: &fb2.Flow{Items: []fb2.FlowItem{
					paragraph("annotation-paragraph", "Annotation text that must not add in-document page markers."),
				}},
			}},
			Bodies: []fb2.Body{
				{Sections: []fb2.Section{main}},
				{Name: "notes", Kind: fb2.BodyFootnotes, Sections: []fb2.Section{
					{ID: "note1", Content: []fb2.FlowItem{paragraph("", "First note")}},
				}},
				{Name: "comments", Kind: fb2.BodyFootnotes, Sections: []fb2.Section{
					{ID: "note2", Content: []fb2.FlowItem{paragraph("", "Second note")}},
				}},
			},
			Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: "@font-face { src: url('#font'); }", Resources: []fb2.StylesheetResource{
				{OriginalURL: "#font", Filename: "fonts/probe.ttf", MimeType: "font/ttf", Data: []byte("font bytes")},
			}}},
		},
	}
}

func AssertLinks(t *testing.T, documents map[string]*etree.Document, rendered map[string]string) {
	t.Helper()
	for currentFile, document := range documents {
		for _, link := range document.FindElements("//a[@href]") {
			filename, id, _ := strings.Cut(link.SelectAttrValue("href", ""), "#")
			if filename == "" {
				filename = currentFile
			}
			if documents[filename] == nil || (id != "" && rendered[id] != filename) {
				t.Errorf("unresolved link in %s: %s", currentFile, link.SelectAttrValue("href", ""))
			}
		}
	}
}
