package kfx

import (
	"testing"

	"fbc/fb2"
)

func TestCollectUsedImageIDs_PoemRenderedSubstructures(t *testing.T) {
	book := &fb2.FictionBook{
		Bodies: []fb2.Body{
			{
				Sections: []fb2.Section{
					{
						Content: []fb2.FlowItem{
							{
								Kind: fb2.FlowPoem,
								Poem: &fb2.Poem{
									Epigraphs: []fb2.Epigraph{
										{
											Flow: fb2.Flow{Items: []fb2.FlowItem{
												{Kind: fb2.FlowParagraph, Paragraph: paragraphWithInlineImage("poem-epigraph-flow-image")},
											}},
											TextAuthors: []fb2.Paragraph{*paragraphWithInlineImage("poem-epigraph-author-image")},
										},
									},
									Subtitles: []fb2.Paragraph{*paragraphWithInlineImage("poem-subtitle-image")},
									Stanzas: []fb2.Stanza{
										{
											Subtitle: paragraphWithInlineImage("stanza-subtitle-image"),
											Verses:   []fb2.Paragraph{{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "verse"}}}},
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

	used := collectUsedImageIDs(book)
	for _, id := range []string{
		"poem-epigraph-flow-image",
		"poem-epigraph-author-image",
		"poem-subtitle-image",
		"stanza-subtitle-image",
	} {
		if !used[id] {
			t.Errorf("collectUsedImageIDs() did not collect rendered image %q", id)
		}
	}
}

func paragraphWithInlineImage(id string) *fb2.Paragraph {
	return &fb2.Paragraph{
		Text: []fb2.InlineSegment{
			{
				Kind:  fb2.InlineImageSegment,
				Image: &fb2.InlineImage{Href: "#" + id, Alt: id},
			},
		},
	}
}
