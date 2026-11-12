package kfx

import (
	"slices"
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestKFXSelectiveBacklinkTextAndOffsets(t *testing.T) {
	for _, tt := range []struct {
		name, template, want string
		wantRefs             []string
		wantOffsets          []int
	}{
		{name: "empty", template: `{{- "" -}}`},
		{name: "whitespace", template: "{{ ` \n\t ` }}"},
		{
			name: "middle only", template: `{{- if eq .RefNumber 2 -}}↩🙂{{- end -}}`, want: "↩🙂",
			wantRefs: []string{"ref-n1-2"}, wantOffsets: []int{0},
		},
		{
			name: "first and last", template: `{{- if ne .RefNumber 2 -}}↩🙂{{- end -}}`, want: "↩🙂\u00a0↩🙂",
			wantRefs: []string{"ref-n1-1", "ref-n1-3"}, wantOffsets: []int{0, 3},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &content.Content{OutputFormat: common.OutputFmtKfx, BacklinkTemplate: tt.template}
			refs := make([]content.BackLinkRef, 3)
			for i := range refs {
				refs[i] = c.AddFootnoteBackLinkRef("n1")
			}
			text, events := kfxBacklinkParagraphContent(c, refs, nil)
			if text != tt.want || len(events) != len(tt.wantRefs) {
				t.Fatalf("content = %q, events = %+v, want %q with targets %v", text, events, tt.want, tt.wantRefs)
			}
			for i, event := range events {
				if event.LinkTo != tt.wantRefs[i] || event.Offset != tt.wantOffsets[i] || event.Length != 2 || event.IsFootnoteLink {
					t.Fatalf("event %d = %+v, want target %s at rune offset %d, length 2, no popup marker",
						i, event, tt.wantRefs[i], tt.wantOffsets[i])
				}
			}
		})
	}
}

func TestKFXBacklinkSuppressionModes(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []common.OutputFmt{common.OutputFmtKfx, common.OutputFmtAzw8} {
		for _, mode := range []common.FootnotesMode{
			common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
		} {
			for _, tt := range []struct {
				name, template string
				wantReturns    int
			}{
				{name: "default template", template: cfg.Document.Footnotes.BacklinkTemplate, wantReturns: 3},
				{name: "nonempty", template: " RETURN ", wantReturns: 3},
				{name: "empty", template: `{{- "" -}}`},
				{name: "whitespace", template: "{{ ` \n\t ` }}"},
				{name: "selective", template: `{{- if eq .RefNumber 2 -}}RETURN{{- end -}}`, wantReturns: 1},
				{name: "location dependent", template: `{{- if .LocationNumber -}}LOC{{- end -}}`, wantReturns: 3},
				{name: "missing template", wantReturns: 3},
				{name: "invalid syntax", template: "{{", wantReturns: 3},
				{name: "execution failure", template: "{{.MissingField}}", wantReturns: 3},
			} {
				t.Run(format.String()+"/"+mode.String()+"/"+tt.name, func(t *testing.T) {
					book := kfxDefaultBacklinkBook()
					paragraph := book.Bodies[0].Sections[0].Content[0].Paragraph
					paragraph.Text = append(paragraph.Text,
						fb2.InlineSegment{Kind: fb2.InlineText, Text: " repeated "},
						fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#n2", Text: "2"},
					)
					c := &content.Content{
						Book: book, OutputFormat: format, FootnotesMode: mode,
						FootnotesIndex: fb2.FootnoteRefs{"n1": {}, "n2": {}}, BacklinkTemplate: tt.template,
					}
					fragments, _, sections, _, sectionEIDs, targets, _, chapters, err := generateStoryline(
						t.Context(), c, NewStyleRegistry(), nil, 1000,
					)
					if err != nil {
						t.Fatal(err)
					}
					links := kfxNamedLinkCounts(fragments)
					if links["n1"] != 1 || links["n2"] != 2 {
						t.Fatalf("forward links = %v, want n1=1 n2=2", links)
					}
					returns := 0
					for target, count := range links {
						if strings.HasPrefix(target, "ref-") {
							returns += count
						}
					}
					if returns != tt.wantReturns {
						t.Fatalf("returns = %d, want %d; links=%v", returns, tt.wantReturns, links)
					}
					if tt.name == "selective" && links["ref-n2-2"] != 1 {
						t.Fatalf("selective return lost its occurrence target: %v", links)
					}
					wantPopups := 0
					if mode.IsFloat() {
						wantPopups = 3
					}
					if count := kfxSymbolFieldCount(fragments, SymYjDisplay, SymbolValue(SymYjNote)); count != wantPopups {
						t.Fatalf("popup reference markers = %d, want %d", count, wantPopups)
					}
					wantFootnotes := 0
					if mode.IsFloat() {
						wantFootnotes = 2
					}
					if count := kfxSymbolFieldCount(fragments, SymYjClassification, SymbolValue(SymFootnote)); count != wantFootnotes {
						t.Fatalf("footnote body markers = %d, want %d", count, wantFootnotes)
					}
					positions := newKFXPositionLookup(CollectPositionItems(fragments, sections, chapters))
					for _, note := range []string{"n1", "n2"} {
						if _, ok := positions.pidForEIDOffset(targets[note].EID, targets[note].Offset); !ok {
							t.Fatalf("note %s has no addressable target: %+v; sections=%v", note, targets[note], sectionEIDs)
						}
						for _, ref := range c.BackLinkIndex[note] {
							if ref.LocationNumber == 0 {
								t.Fatalf("reference location not resolved: %+v", ref)
							}
						}
					}
					if tt.wantReturns == 0 || tt.name == "selective" || tt.name == "location dependent" {
						if contentFragmentText(fragments, "[<]") != "" {
							t.Fatal("empty template result became fallback text")
						}
					}
				})
			}
		}
	}
}

func TestKFXEmptyNoteBacklinkSuppression(t *testing.T) {
	for _, format := range []common.OutputFmt{common.OutputFmtKfx, common.OutputFmtAzw8} {
		for _, mode := range []common.FootnotesMode{
			common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
		} {
			for _, hasTitle := range []bool{false, true} {
				name := "untitled"
				if hasTitle {
					name = "titled"
				}
				t.Run(format.String()+"/"+mode.String()+"/"+name, func(t *testing.T) {
					book := kfxDefaultBacklinkBook()
					note := &book.Bodies[1].Sections[0]
					note.Content = nil
					if hasTitle {
						note.Title = &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{
							Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Empty note"}},
						}}}}
					}
					c := &content.Content{
						Book: book, OutputFormat: format, FootnotesMode: mode,
						FootnotesIndex: fb2.FootnoteRefs{"n1": {}, "n2": {}}, BacklinkTemplate: `{{- "" -}}`,
					}
					fragments, _, sections, _, _, targets, _, chapters, err := generateStoryline(
						t.Context(), c, NewStyleRegistry(), nil, 1000,
					)
					if err != nil {
						t.Fatal(err)
					}
					positions := newKFXPositionLookup(CollectPositionItems(fragments, sections, chapters))
					if _, ok := positions.pidForEIDOffset(targets["n1"].EID, 0); !ok {
						t.Fatalf("empty note target disappeared: %+v", targets["n1"])
					}
					links := kfxNamedLinkCounts(fragments)
					if links["n1"] != 1 || links["n2"] != 1 || len(links) != 2 {
						t.Fatalf("empty note lost forward links or retained returns: %v", links)
					}
				})
			}
		}
	}
}

func TestKFXBacklinkVisibilityReevaluatesLocations(t *testing.T) {
	c := &content.Content{
		OutputFormat: common.OutputFmtKfx, BacklinkTemplate: `{{- if gt .LocationNumber 1 -}}RETURN{{- end -}}`,
	}
	ref := c.AddFootnoteBackLinkRef("n1")
	sb := NewStorylineBuilder("l1", "c0", 1000, nil)
	ca := NewContentAccumulator(1)
	paragraph, ok := addBacklinkParagraph(c, []content.BackLinkRef{ref}, false, sb, nil, ca)
	if !ok || slices.Contains(sb.AllEIDs(), paragraph.EID) {
		t.Fatal("initially empty candidate was lost or included in positions")
	}
	for _, tt := range []struct {
		name     string
		location int
		visible  bool
	}{
		{name: "appears", location: 2, visible: true},
		{name: "disappears", location: 1},
		{name: "reappears", location: 3, visible: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c.SetBackLinkRefLocation(ref.RefID, tt.location)
			if !rewriteKFXBacklinkParagraphs(c, []kfxBacklinkParagraph{paragraph}, sb, nil, ca) {
				t.Fatal("expected backlink content/visibility update")
			}
			if slices.Contains(sb.AllEIDs(), paragraph.EID) != tt.visible {
				t.Fatal("suppressed candidate incorrectly included/excluded from section positions")
			}
			story, section := storylineSnapshotFragments(sb)
			fragments := NewFragmentList()
			if err := fragments.Add(story); err != nil {
				t.Fatal(err)
			}
			if err := fragments.Add(section); err != nil {
				t.Fatal(err)
			}
			if (kfxNamedLinkCounts(fragments)[ref.RefID] == 1) != tt.visible {
				t.Fatal("suppressed candidate incorrectly included/excluded from storyline snapshot")
			}
		})
	}
}

func kfxNamedLinkCounts(fragments *FragmentList) map[string]int {
	counts := make(map[string]int)
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case StructValue:
			if target, ok := v[SymLinkTo].(SymbolByNameValue); ok {
				counts[string(target)]++
			}
			for _, child := range v {
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		case map[string]any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	for _, fragment := range fragments.All() {
		visit(fragment.Value)
	}
	return counts
}
