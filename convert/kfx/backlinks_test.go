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
