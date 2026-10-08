package kfx

import (
	"context"
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func TestKFXBacklinkTemplateUsesResolvedLocation(t *testing.T) {
	book := &fb2.FictionBook{
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{
					{
						ID: "s1",
						Title: &fb2.Title{Items: []fb2.TitleItem{
							{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Section"}}}},
						}},
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
								ID: "p1",
								Text: []fb2.InlineSegment{
									{Kind: fb2.InlineText, Text: "hello "},
									{Kind: fb2.InlineLink, Href: "#n1", Text: "1"},
									{Kind: fb2.InlineText, Text: " world"},
								},
							}},
						},
					},
				},
			},
			{
				Name: "notes",
				Kind: fb2.BodyFootnotes,
				Sections: []fb2.Section{
					{
						ID: "n1",
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "note"}}}},
						},
					},
				},
			},
		},
	}
	c := &content.Content{
		Book:             book,
		OutputFormat:     common.OutputFmtKfx,
		FootnotesMode:    common.FootnotesModeFloat,
		FootnotesIndex:   fb2.FootnoteRefs{"n1": {}},
		BacklinkTemplate: `[{{- if .LocationNumber -}}loc {{ .LocationNumber }}{{- else if .SectionTitle -}}{{ .SectionTitle }}{{- else -}}<{{- end -}}]`,
	}
	fragments, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), c, NewStyleRegistry(), nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	refs := c.BackLinkIndex["n1"]
	if len(refs) == 0 || refs[0].LocationNumber == 0 {
		t.Fatalf("no location: %+v", refs)
	}
	if got := contentFragmentText(fragments, "[loc 1]"); got == "" {
		t.Fatalf("generated footnote backlink did not use resolved location; refs=%+v", refs)
	}
}

func TestKFXNestedFootnoteBacklinkTemplateUsesResolvedLocation(t *testing.T) {
	book := &fb2.FictionBook{
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{
					{
						ID: "s1",
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
								Text: []fb2.InlineSegment{
									{Kind: fb2.InlineText, Text: "main "},
									{Kind: fb2.InlineLink, Href: "#n1", Text: "1"},
								},
							}},
						},
					},
				},
			},
			{
				Name: "notes",
				Kind: fb2.BodyFootnotes,
				Sections: []fb2.Section{
					{
						ID: "n1",
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
								Text: []fb2.InlineSegment{
									{Kind: fb2.InlineText, Text: "note one "},
									{Kind: fb2.InlineLink, Href: "#n2", Text: "2"},
								},
							}},
						},
					},
					{
						ID: "n2",
						Content: []fb2.FlowItem{
							{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "note two"}}}},
						},
					},
				},
			},
		},
	}
	c := &content.Content{
		Book:             book,
		OutputFormat:     common.OutputFmtKfx,
		FootnotesMode:    common.FootnotesModeFloat,
		FootnotesIndex:   fb2.FootnoteRefs{"n1": {}, "n2": {}},
		BacklinkTemplate: `[{{- if .LocationNumber -}}loc {{ .LocationNumber }}{{- else -}}<{{- end -}}]`,
	}
	fragments, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), c, NewStyleRegistry(), nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	refs := c.BackLinkIndex["n2"]
	if len(refs) == 0 || refs[0].LocationNumber == 0 {
		t.Fatalf("nested footnote reference has no location: %+v", refs)
	}
	if count := contentFragmentTextCount(fragments, "[loc 1]"); count != 2 {
		t.Fatalf("expected two resolved location backlinks, got %d; refs=%+v", count, c.BackLinkIndex)
	}
	if got := contentFragmentText(fragments, "[<]"); got != "" {
		t.Fatalf("nested footnote backlink fell back to %q", got)
	}
}

func TestKFXNestedFootnoteBacklinkChainUsesResolvedLocations(t *testing.T) {
	book := &fb2.FictionBook{
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{
					ID: "s1",
					Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
						Text: []fb2.InlineSegment{
							{Kind: fb2.InlineText, Text: "main "},
							{Kind: fb2.InlineLink, Href: "#n1", Text: "1"},
						},
					}}},
				}},
			},
			{
				Name: "notes",
				Kind: fb2.BodyFootnotes,
				Sections: []fb2.Section{
					{
						ID: "n1",
						Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
							{Kind: fb2.InlineText, Text: "note one "},
							{Kind: fb2.InlineLink, Href: "#n2", Text: "2"},
						}}}},
					},
					{
						ID: "n2",
						Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
							{Kind: fb2.InlineText, Text: "note two "},
							{Kind: fb2.InlineLink, Href: "#n3", Text: "3"},
						}}}},
					},
					{
						ID: "n3",
						Content: []fb2.FlowItem{
							{
								Kind:      fb2.FlowParagraph,
								Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "note three"}}},
							},
						},
					},
				},
			},
		},
	}
	c := &content.Content{
		Book:             book,
		OutputFormat:     common.OutputFmtKfx,
		FootnotesMode:    common.FootnotesModeFloat,
		FootnotesIndex:   fb2.FootnoteRefs{"n1": {}, "n2": {}, "n3": {}},
		BacklinkTemplate: `[{{- if .LocationNumber -}}loc {{ .LocationNumber }}{{- else -}}MISSING{{- end -}}]`,
	}
	fragments, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), c, NewStyleRegistry(), nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got := contentFragmentText(fragments, "[MISSING]"); got != "" {
		t.Fatalf("nested footnote backlink fell back to %q; refs=%+v", got, c.BackLinkIndex)
	}
	if count := contentFragmentTextPrefixCount(fragments, "[loc "); count != 3 {
		t.Fatalf("resolved backlink count = %d, want 3; refs=%+v", count, c.BackLinkIndex)
	}
}

func TestKFXDefaultBacklinksIncludeBackwardFootnoteReferences(t *testing.T) {
	book := kfxDefaultBacklinkBook()
	c := &content.Content{
		Book:             book,
		OutputFormat:     common.OutputFmtKfx,
		FootnotesMode:    common.FootnotesModeDefault,
		FootnotesIndex:   fb2.FootnoteRefs{"n1": {}, "n2": {}},
		BacklinkTemplate: `[{{- if .LocationNumber -}}loc {{ .LocationNumber }}{{- else -}}MISSING{{- end -}}]`,
	}
	fragments, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), c, NewStyleRegistry(), nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	refs := c.BackLinkIndex["n1"]
	if len(refs) == 0 || refs[0].LocationNumber == 0 {
		t.Fatalf("backward footnote reference has no resolved location: %+v", refs)
	}
	if got := contentFragmentText(fragments, "[MISSING]"); got != "" {
		t.Fatalf("generated default backlink fell back to %q; refs=%+v", got, c.BackLinkIndex)
	}
	if count := contentFragmentTextPrefixCount(fragments, "[loc "); count != 2 {
		t.Fatalf("resolved backlink count = %d, want 2; refs=%+v", count, c.BackLinkIndex)
	}
}

func TestKFXDefaultFootnotesDoNotEmitPopupMarkers(t *testing.T) {
	book := kfxDefaultBacklinkBook()
	defaultContent := &content.Content{
		Book:             book,
		OutputFormat:     common.OutputFmtKfx,
		FootnotesMode:    common.FootnotesModeDefault,
		FootnotesIndex:   fb2.FootnoteRefs{"n1": {}, "n2": {}},
		BacklinkTemplate: `[<]`,
	}
	defaultFragments, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), defaultContent, NewStyleRegistry(), nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if count := kfxSymbolFieldCount(defaultFragments, SymYjDisplay, SymbolValue(SymYjNote)); count != 0 {
		t.Fatalf("default mode yj.note display markers = %d, want 0", count)
	}
	if count := kfxSymbolFieldCount(defaultFragments, SymYjClassification, SymbolValue(SymFootnote)); count != 0 {
		t.Fatalf("default mode footnote classifications = %d, want 0", count)
	}

	floatContent := &content.Content{
		Book:             kfxDefaultBacklinkBook(),
		OutputFormat:     common.OutputFmtKfx,
		FootnotesMode:    common.FootnotesModeFloat,
		FootnotesIndex:   fb2.FootnoteRefs{"n1": {}, "n2": {}},
		BacklinkTemplate: `[<]`,
	}
	floatFragments, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), floatContent, NewStyleRegistry(), nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if count := kfxSymbolFieldCount(floatFragments, SymYjDisplay, SymbolValue(SymYjNote)); count == 0 {
		t.Fatal("float mode yj.note display markers = 0, want popup markers")
	}
	if count := kfxSymbolFieldCount(floatFragments, SymYjClassification, SymbolValue(SymFootnote)); count == 0 {
		t.Fatal("float mode footnote classifications = 0, want popup markers")
	}
}

func TestKFXBacklinksPreserveReferenceOffsets(t *testing.T) {
	for _, mode := range []common.FootnotesMode{common.FootnotesModeDefault, common.FootnotesModeFloat} {
		for _, kind := range []string{"paragraph", "mixed", "title"} {
			t.Run(mode.String()+"/"+kind, func(t *testing.T) {
				segments := []fb2.InlineSegment{{Kind: fb2.InlineText, Text: strings.Repeat("я", 80) + " "}}
				firstOffset := 81
				if kind == "mixed" {
					segments = append(segments, fb2.InlineSegment{
						Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#img"},
					})
					firstOffset++ // Inline images occupy one position.
				}
				segments = append(segments,
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{
						{Kind: fb2.InlineStrong, Text: "[1]"},
					}},
					fb2.InlineSegment{Kind: fb2.InlineText, Text: strings.Repeat("ю", 80) + " "},
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#n2", Text: "[2]"},
				)
				book := kfxDefaultBacklinkBook()
				section := &book.Bodies[0].Sections[0]
				para := &fb2.Paragraph{Text: segments}
				if kind == "title" {
					section.Title = &fb2.Title{Items: []fb2.TitleItem{{Paragraph: para}}}
					section.Content = nil
				} else {
					section.Content = []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: para}}
				}
				// Keep both footnotes free of references so each has one backlink.
				book.Bodies[1].Sections[1].Content[0].Paragraph.Text = []fb2.InlineSegment{
					{Kind: fb2.InlineText, Text: "note two"},
				}
				c := &content.Content{
					Book: book, OutputFormat: common.OutputFmtKfx, FootnotesMode: mode,
					FootnotesIndex: fb2.FootnoteRefs{"n1": {}, "n2": {}}, BacklinkTemplate: "[<]",
				}
				images := imageResourceInfoByID{"img": {ResourceName: "e1", Width: 10, Height: 10}}
				fragments, _, _, _, _, targets, _, _, err := generateStoryline(
					context.Background(), c, NewStyleRegistry(), images, 1000,
				)
				if err != nil {
					t.Fatal(err)
				}
				var firstEID int
				for i, noteID := range []string{"n1", "n2"} {
					refs := c.BackLinkIndex[noteID]
					if len(refs) != 1 {
						t.Fatalf("%s backlinks = %+v, want one", noteID, refs)
					}
					anchors := buildAnchorFragments(targets, map[string]bool{refs[0].RefID: true})
					if len(anchors) != 1 {
						t.Fatalf("%s anchors = %d, want one", noteID, len(anchors))
					}
					position := anchors[0].Value.(StructValue)[SymPosition].(StructValue)
					wantOffset := firstOffset + i*84
					if offset, ok := position.GetInt(SymOffset); !ok || offset != int64(wantOffset) {
						t.Errorf("%s anchor offset = %d (present=%v), want %d", noteID, offset, ok, wantOffset)
					}
					if i == 0 {
						firstEID = targets[refs[0].RefID].EID
					} else if targets[refs[0].RefID].EID != firstEID {
						t.Error("references should target the same content entry")
					}
				}
				if c.BackLinkIndex["n1"][0].LocationNumber == c.BackLinkIndex["n2"][0].LocationNumber {
					t.Error("separated references should have distinct location numbers")
				}
				if mode == common.FootnotesModeDefault &&
					kfxSymbolFieldCount(fragments, SymYjDisplay, SymbolValue(SymYjNote)) != 0 {
					t.Error("default-mode links should not enable footnote popups")
				}
			})
		}
	}
}

func kfxDefaultBacklinkBook() *fb2.FictionBook {
	book := &fb2.FictionBook{
		Bodies: []fb2.Body{
			{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{
					ID: "s1",
					Content: []fb2.FlowItem{{
						Kind: fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
							{Kind: fb2.InlineText, Text: "main "},
							{Kind: fb2.InlineLink, Href: "#n2", Text: "2"},
						}},
					}},
				}},
			},
			{
				Name: "notes",
				Kind: fb2.BodyFootnotes,
				Sections: []fb2.Section{
					{
						ID: "n1",
						Content: []fb2.FlowItem{{
							Kind: fb2.FlowParagraph,
							Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
								{Kind: fb2.InlineText, Text: "note one"},
							}},
						}},
					},
					{
						ID: "n2",
						Content: []fb2.FlowItem{{
							Kind: fb2.FlowParagraph,
							Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
								{Kind: fb2.InlineText, Text: "note two "},
								{Kind: fb2.InlineLink, Href: "#n1", Text: "1"},
							}},
						}},
					},
				},
			},
		},
	}
	return book
}

func kfxSymbolFieldCount(fragments *FragmentList, field KFXSymbol, value SymbolValue) int {
	if fragments == nil {
		return 0
	}
	count := 0
	for _, frag := range fragments.All() {
		count += kfxSymbolFieldCountInValue(frag.Value, field, value)
	}
	return count
}

func kfxSymbolFieldCountInValue(value any, field KFXSymbol, want SymbolValue) int {
	count := 0
	switch v := value.(type) {
	case StructValue:
		if got, ok := v[field].(SymbolValue); ok && got == want {
			count++
		}
		for _, child := range v {
			count += kfxSymbolFieldCountInValue(child, field, want)
		}
	case []any:
		for _, child := range v {
			count += kfxSymbolFieldCountInValue(child, field, want)
		}
	case map[string]any:
		for _, child := range v {
			count += kfxSymbolFieldCountInValue(child, field, want)
		}
	}
	return count
}

func contentFragmentText(fragments *FragmentList, want string) string {
	for _, frag := range fragments.GetByType(SymContent) {
		m, ok := frag.Value.(map[string]any)
		if !ok {
			continue
		}
		items, ok := m["$146"].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			text, ok := item.(string)
			if ok && text == want {
				return text
			}
		}
	}
	return ""
}

func contentFragmentTextPrefixCount(fragments *FragmentList, prefix string) int {
	count := 0
	for _, frag := range fragments.GetByType(SymContent) {
		m, ok := frag.Value.(map[string]any)
		if !ok {
			continue
		}
		items, ok := m["$146"].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			text, ok := item.(string)
			if ok && strings.HasPrefix(text, prefix) {
				count++
			}
		}
	}
	return count
}

func contentFragmentTextCount(fragments *FragmentList, want string) int {
	count := 0
	for _, frag := range fragments.GetByType(SymContent) {
		m, ok := frag.Value.(map[string]any)
		if !ok {
			continue
		}
		items, ok := m["$146"].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			text, ok := item.(string)
			if ok && text == want {
				count++
			}
		}
	}
	return count
}
