package kfx

import (
	"testing"

	"fbc/content"
	"fbc/content/text"
	"fbc/fb2"
)

func TestInsertSpacerAfterFirstRune(t *testing.T) {
	got := insertSpacerAfterFirstRune("Abc", text.NNBSP)
	if got != "A\u202Fbc" {
		t.Fatalf("expected narrow no-break space after first rune, got %q", got)
	}
}

func TestAdjustStyleEventsForInsertedRune(t *testing.T) {
	events := []StyleEventRef{
		{Offset: 0, Length: 1, Style: "dropcap"},
		{Offset: 1, Length: 4, Style: "strong"},
		{Offset: 0, Length: 5, Style: "outer"},
	}

	adjustStyleEventsForInsertedRune(events, 1)

	if events[0].Offset != 0 || events[0].Length != 1 {
		t.Fatalf("dropcap event changed unexpectedly: %+v", events[0])
	}
	if events[1].Offset != 2 || events[1].Length != 4 {
		t.Fatalf("expected later event to shift right, got %+v", events[1])
	}
	if events[2].Offset != 0 || events[2].Length != 6 {
		t.Fatalf("expected spanning event to expand, got %+v", events[2])
	}
}

func TestAddParagraphWithImages_DropcapNegativeMarginInjectsSpacer(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("p").Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).MarginLeft(-1, SymUnitEm).Build())
	sr.Register(NewStyle("has-dropcap--dropcap").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	para := &fb2.Paragraph{
		Style: "has-dropcap",
		Text:  []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Abc"}},
	}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	ca := NewContentAccumulator(1)
	addParagraphWithImages(&content.Content{}, para, ctx, para.Style, 0, sb, sr, nil, ca, nil)

	contentFrags := ca.Finish()
	list := contentFrags["content_1"]
	if len(list) != 1 {
		t.Fatalf("expected one content entry, got %d", len(list))
	}
	if list[0] != "A\u202Fbc" {
		t.Fatalf("expected injected narrow no-break space content, got %q", list[0])
	}

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one storyline entry, got %d", len(sb.contentEntries))
	}
	entry := sb.contentEntries[0]
	if entry.Style == "" {
		t.Fatal("expected resolved style")
	}
	def, ok := sr.Get(entry.Style)
	if !ok {
		t.Fatalf("style %q not found", entry.Style)
	}
	chars, ok := numericFromAny(def.Properties[SymDropcapChars])
	if !ok || chars != 2 {
		t.Fatalf("expected dropcap chars = 2, got %v", def.Properties[SymDropcapChars])
	}
	if len(entry.StyleEvents) != 1 {
		t.Fatalf("expected one style event, got %d", len(entry.StyleEvents))
	}
	if entry.StyleEvents[0].Offset != 0 || entry.StyleEvents[0].Length != 1 {
		t.Fatalf("expected glyph event to stay on first rune, got %+v", entry.StyleEvents[0])
	}
}

func TestAddParagraphWithImages_PendingFootnoteMorePreservesInlineImages(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("p").Build())
	sr.Register(NewStyle("footnote-more").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	para := &fb2.Paragraph{
		Text: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "Lead "},
			{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#img1", Alt: "img"}},
			{Kind: fb2.InlineText, Text: " tail"},
		},
	}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	sb.SetPendingFootnoteMore()
	ca := NewContentAccumulator(1)
	images := imageResourceInfoByID{
		"img1": {ResourceName: "resource/img1", Width: 10, Height: 10},
	}
	addParagraphWithImages(&content.Content{MoreParaStr: "(~) "}, para, ctx, "", 0, sb, sr, images, ca, nil)

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one storyline entry, got %d", len(sb.contentEntries))
	}
	entry := sb.contentEntries[0]
	if entry.RawEntry == nil {
		t.Fatal("expected mixed-content raw entry")
	}
	items, ok := entry.RawEntry.GetList(SymContentList)
	if !ok {
		t.Fatal("expected content_list on mixed-content entry")
	}
	if len(items) != 3 {
		t.Fatalf("expected text-image-text content_list, got %d items", len(items))
	}
	firstText, ok := items[0].(string)
	if !ok {
		t.Fatalf("expected first content_list item to be text, got %T", items[0])
	}
	if firstText != "(~) Lead " {
		t.Fatalf("expected marker-prefixed first text item, got %q", firstText)
	}
	if _, ok := items[1].(StructValue); !ok {
		t.Fatalf("expected second content_list item to be image struct, got %T", items[1])
	}
	lastText, ok := items[2].(string)
	if !ok {
		t.Fatalf("expected last content_list item to be text, got %T", items[2])
	}
	if lastText != " tail" {
		t.Fatalf("expected trailing text after inline image, got %q", lastText)
	}
	if sb.HasPendingFootnoteMore() {
		t.Fatal("expected pending footnote-more flag to be consumed")
	}
	if entry.Style == "" && len(entry.StyleEvents) == 0 {
		t.Fatal("expected marker styling to be preserved on the rendered paragraph")
	}
}
