package kfx

import (
	"math"
	"testing"

	"fbc/content"
	"fbc/fb2"
)

func TestAddParagraphWithImages_DropcapWithoutNegativeMarginPreservesBottomMargin(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("p").Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).MarginBottom(1, SymUnitLh).Build())
	sr.Register(NewStyle("has-dropcap--dropcap").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	para := &fb2.Paragraph{
		Style: "has-dropcap",
		Text:  []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Abc"}},
	}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	ca := NewContentAccumulator(1)
	addParagraphWithImages(&content.Content{}, para, ctx, para.Style, 0, sb, sr, nil, ca, nil)

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one storyline entry, got %d", len(sb.contentEntries))
	}
	entry := sb.contentEntries[0]
	if entry.Style == "" {
		t.Fatal("expected resolved dropcap style")
	}
	def, ok := sr.Get(entry.Style)
	if !ok {
		t.Fatalf("style %q not found", entry.Style)
	}
	assertMeasure(t, def.Properties[SymMarginBottom], SymMarginBottom, 1, SymUnitLh)
	if entry.StyleSpec == dropcapMarginWrapperStyle || len(entry.childRefs) > 0 {
		t.Fatal("did not expect wrapper for dropcap paragraph without negative margins")
	}
}

func TestAddParagraphWithImages_DropcapNegativeRootMarginWrapsParagraphWithoutSpacer(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("html").MarginLeft(-1, SymUnitEm).MarginRight(-2, SymUnitEm).Build())
	sr.Register(NewStyle("p").Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).Build())
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
	if list[0] != "Abc" {
		t.Fatalf("expected original content without injected spacer, got %q", list[0])
	}

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one storyline wrapper entry, got %d", len(sb.contentEntries))
	}
	wrapper := &sb.contentEntries[0]
	if wrapper.Style == "" {
		t.Fatal("expected resolved dropcap margin wrapper style")
	}
	if len(wrapper.childRefs) != 1 {
		t.Fatalf("expected one wrapped dropcap child, got %d", len(wrapper.childRefs))
	}
	child := wrapper.childRefs[0]
	if child.Style == "" {
		t.Fatal("expected resolved child style")
	}
	def, ok := sr.Get(child.Style)
	if !ok {
		t.Fatalf("style %q not found", child.Style)
	}
	chars, ok := numericFromAny(def.Properties[SymDropcapChars])
	if !ok || chars != 1 {
		t.Fatalf("expected dropcap chars = 1, got %v", def.Properties[SymDropcapChars])
	}
	if _, ok := def.Properties[SymMarginLeft]; ok {
		t.Fatalf("expected wrapped dropcap child to have no margin-left, got %v", def.Properties[SymMarginLeft])
	}
	if _, ok := def.Properties[SymMarginRight]; ok {
		t.Fatalf("expected wrapped dropcap child to have no margin-right, got %v", def.Properties[SymMarginRight])
	}
	if _, ok := def.Properties[SymMarginBottom]; ok {
		t.Fatalf("expected wrapped dropcap child to preserve natural absent margin-bottom, got %v", def.Properties[SymMarginBottom])
	}
	if len(child.StyleEvents) != 1 {
		t.Fatalf("expected one style event, got %d", len(child.StyleEvents))
	}
	if child.StyleEvents[0].Offset != 0 || child.StyleEvents[0].Length != 1 {
		t.Fatalf("expected glyph event to stay on first rune, got %+v", child.StyleEvents[0])
	}

	beforeBuildWrapperStyle := wrapper.Style
	sb.Build()
	if wrapper.Style != beforeBuildWrapperStyle {
		t.Fatalf("expected wrapper style to remain pre-resolved, got %q want %q", wrapper.Style, beforeBuildWrapperStyle)
	}
	wrapperDef, ok := sr.Get(wrapper.Style)
	if !ok {
		t.Fatalf("wrapper style %q not found", wrapper.Style)
	}
	assertMeasure(t, wrapperDef.Properties[SymMarginLeft], SymMarginLeft, -1, SymUnitEm)
	assertMeasure(t, wrapperDef.Properties[SymMarginRight], SymMarginRight, -2, SymUnitEm)
}

func TestProcessDropcapParagraphPair_WrapsFollowingParagraphAndClampsChildMargins(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("html").MarginLeft(-1, SymUnitEm).MarginRight(-2, SymUnitEm).Build())
	sr.Register(NewStyle("p").MarginLeft(-0.5, SymUnitEm).MarginRight(-0.25, SymUnitEm).Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).MarginBottom(0.75, SymUnitLh).Build())
	sr.Register(NewStyle("has-dropcap--dropcap").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	first := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Style: "has-dropcap",
		Text:  []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Abc"}},
	}}
	second := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Second"}},
	}}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	ca := NewContentAccumulator(1)
	if !processDropcapParagraphPair(&content.Content{}, first, second, ctx, sb, sr, nil, ca, nil) {
		t.Fatal("expected dropcap pair wrapper to be used")
	}

	contentFrags := ca.Finish()
	list := contentFrags["content_1"]
	if len(list) != 2 || list[0] != "Abc" || list[1] != "Second" {
		t.Fatalf("expected two paragraph content entries in wrapper, got %#v", list)
	}

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one wrapper entry, got %d", len(sb.contentEntries))
	}
	wrapper := &sb.contentEntries[0]
	if len(wrapper.childRefs) != 2 {
		t.Fatalf("expected dropcap wrapper to contain first and following paragraphs, got %d children", len(wrapper.childRefs))
	}

	wrapperDef, ok := sr.Get(wrapper.Style)
	if !ok {
		t.Fatalf("wrapper style %q not found", wrapper.Style)
	}
	assertMeasure(t, wrapperDef.Properties[SymMarginLeft], SymMarginLeft, -1.5, SymUnitEm)
	assertMeasure(t, wrapperDef.Properties[SymMarginRight], SymMarginRight, -2.25, SymUnitEm)

	for i, child := range wrapper.childRefs {
		def, ok := sr.Get(child.Style)
		if !ok {
			t.Fatalf("child %d style %q not found", i, child.Style)
		}
		assertMeasure(t, def.Properties[SymMarginLeft], SymMarginLeft, 0, SymUnitEm)
		assertMeasure(t, def.Properties[SymMarginRight], SymMarginRight, 0, SymUnitEm)
	}

	sb.Build()
	firstDef, ok := sr.Get(wrapper.childRefs[0].Style)
	if !ok {
		t.Fatalf("first child style %q not found after build", wrapper.childRefs[0].Style)
	}
	if _, ok := firstDef.Properties[SymMarginBottom]; ok {
		t.Fatalf("expected dropcap pair boundary to strip first paragraph margin-bottom, got %v", firstDef.Properties[SymMarginBottom])
	}
	secondDef, ok := sr.Get(wrapper.childRefs[1].Style)
	if !ok {
		t.Fatalf("second child style %q not found after build", wrapper.childRefs[1].Style)
	}
	assertMeasure(t, secondDef.Properties[SymMarginTop], SymMarginTop, 0.75, SymUnitLh)
}

func TestProcessDropcapParagraphPair_UsesNormalParagraphGapInsteadOfDefaultDropcapMargin(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("p").MarginLeft(-0.5, SymUnitEm).MarginRight(-0.25, SymUnitEm).MarginBottom(0.25, SymUnitLh).Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).MarginBottom(defaultParagraphMarginLH(), SymUnitLh).Build())
	sr.Register(NewStyle("has-dropcap--dropcap").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	first := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Style: "has-dropcap",
		Text:  []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Abc"}},
	}}
	second := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Second"}},
	}}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	ca := NewContentAccumulator(1)
	if !processDropcapParagraphPair(&content.Content{}, first, second, ctx, sb, sr, nil, ca, nil) {
		t.Fatal("expected dropcap pair wrapper to be used")
	}

	sb.Build()
	wrapper := &sb.contentEntries[0]
	firstDef, ok := sr.Get(wrapper.childRefs[0].Style)
	if !ok {
		t.Fatalf("first child style %q not found after build", wrapper.childRefs[0].Style)
	}
	if _, ok := firstDef.Properties[SymMarginBottom]; ok {
		t.Fatalf("expected first paragraph margin-bottom to be stripped, got %v", firstDef.Properties[SymMarginBottom])
	}
	secondDef, ok := sr.Get(wrapper.childRefs[1].Style)
	if !ok {
		t.Fatalf("second child style %q not found after build", wrapper.childRefs[1].Style)
	}
	assertMeasure(t, secondDef.Properties[SymMarginTop], SymMarginTop, 0.25, SymUnitLh)
}

func TestProcessDropcapParagraphPair_DoesNotTransferDefaultDropcapMarginWhenParagraphGapIsZero(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("p").MarginLeft(-0.5, SymUnitEm).MarginRight(-0.25, SymUnitEm).Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).MarginBottom(defaultParagraphMarginLH(), SymUnitLh).Build())
	sr.Register(NewStyle("has-dropcap--dropcap").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	first := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Style: "has-dropcap",
		Text:  []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Abc"}},
	}}
	second := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Second"}},
	}}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	ca := NewContentAccumulator(1)
	if !processDropcapParagraphPair(&content.Content{}, first, second, ctx, sb, sr, nil, ca, nil) {
		t.Fatal("expected dropcap pair wrapper to be used")
	}

	sb.Build()
	wrapper := &sb.contentEntries[0]
	secondDef, ok := sr.Get(wrapper.childRefs[1].Style)
	if !ok {
		t.Fatalf("second child style %q not found after build", wrapper.childRefs[1].Style)
	}
	if _, ok := secondDef.Properties[SymMarginTop]; ok {
		t.Fatalf("expected zero paragraph gap not to gain default dropcap margin, got %v", secondDef.Properties[SymMarginTop])
	}
}

func TestProcessDropcapParagraphPair_WrapsWhenOnlyFollowingParagraphHasNegativeMargins(t *testing.T) {
	sr := NewStyleRegistry()
	sr.Register(NewStyle("p").MarginLeft(-0.5, SymUnitEm).MarginRight(-0.25, SymUnitEm).Build())
	sr.Register(NewStyle("has-dropcap").Dropcap(1, 3).MarginLeft(0, SymUnitEm).MarginRight(0, SymUnitEm).MarginBottom(0.75, SymUnitLh).Build())
	sr.Register(NewStyle("has-dropcap--dropcap").FontWeight(SymBold).Build())

	ctx := NewStyleContext(sr)
	first := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Style: "has-dropcap",
		Text:  []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Abc"}},
	}}
	second := &fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Second"}},
	}}

	sb := NewStorylineBuilder("l1", "c0", 1, sr)
	ca := NewContentAccumulator(1)
	if !processDropcapParagraphPair(&content.Content{}, first, second, ctx, sb, sr, nil, ca, nil) {
		t.Fatal("expected pair wrapper when the following paragraph has negative margins")
	}
	if len(sb.contentEntries) != 1 || len(sb.contentEntries[0].childRefs) != 2 {
		t.Fatalf("expected one two-child wrapper, got %#v", sb.contentEntries)
	}

	wrapper := &sb.contentEntries[0]
	wrapperDef, ok := sr.Get(wrapper.Style)
	if !ok {
		t.Fatalf("wrapper style %q not found", wrapper.Style)
	}
	assertMeasure(t, wrapperDef.Properties[SymMarginLeft], SymMarginLeft, -0.5, SymUnitEm)
	assertMeasure(t, wrapperDef.Properties[SymMarginRight], SymMarginRight, -0.25, SymUnitEm)

	sb.Build()
	firstDef, ok := sr.Get(wrapper.childRefs[0].Style)
	if !ok {
		t.Fatalf("first child style %q not found after build", wrapper.childRefs[0].Style)
	}
	if _, ok := firstDef.Properties[SymMarginBottom]; ok {
		t.Fatalf("expected first paragraph margin-bottom to move to following paragraph, got %v", firstDef.Properties[SymMarginBottom])
	}
	secondDef, ok := sr.Get(wrapper.childRefs[1].Style)
	if !ok {
		t.Fatalf("second child style %q not found after build", wrapper.childRefs[1].Style)
	}
	assertMeasure(t, secondDef.Properties[SymMarginTop], SymMarginTop, 0.75, SymUnitLh)
	assertMeasure(t, secondDef.Properties[SymMarginLeft], SymMarginLeft, 0, SymUnitEm)
	assertMeasure(t, secondDef.Properties[SymMarginRight], SymMarginRight, 0, SymUnitEm)
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

func assertMeasure(t *testing.T, got any, sym KFXSymbol, want float64, unit KFXSymbol) {
	t.Helper()
	value, gotUnit, ok := measureParts(got)
	if !ok {
		t.Fatalf("property %s is not a measure: %v", traceSymbolName(sym), got)
	}
	if gotUnit != unit {
		t.Fatalf("property %s unit = %s, want %s", traceSymbolName(sym), traceSymbolName(gotUnit), traceSymbolName(unit))
	}
	if math.Abs(value-want) > 1e-6 {
		t.Fatalf("property %s value = %v, want %v", traceSymbolName(sym), value, want)
	}
}
