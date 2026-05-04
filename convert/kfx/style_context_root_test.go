package kfx

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"fbc/content"
	"fbc/fb2"
)

func resolvedStyleProps(t *testing.T, registry *StyleRegistry, tag, classes string) map[KFXSymbol]any {
	t.Helper()
	styleName := NewStyleContext(registry).Resolve(tag, classes)
	def, ok := registry.Get(styleName)
	if !ok {
		t.Fatalf("resolved style %q not registered", styleName)
	}
	return def.Properties
}

func requireMeasure(t *testing.T, props map[KFXSymbol]any, sym KFXSymbol, want float64, unit KFXSymbol) {
	t.Helper()
	got, ok := props[sym]
	if !ok {
		t.Fatalf("expected property %s", traceSymbolName(sym))
	}
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

func TestNewStyleContext_BodyHorizontalMargins(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		body { margin-left: 2em; margin-right: 3em; }
	`), nil, zap.NewNop())

	props := resolvedStyleProps(t, registry, "", "")
	requireMeasure(t, props, SymMarginLeft, 2, SymUnitEm)
	requireMeasure(t, props, SymMarginRight, 3, SymUnitEm)
}

func TestNewStyleContext_HTMLHorizontalMargins(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-left: 1em; margin-right: 1.5em; }
	`), nil, zap.NewNop())

	props := resolvedStyleProps(t, registry, "", "")
	requireMeasure(t, props, SymMarginLeft, 1, SymUnitEm)
	requireMeasure(t, props, SymMarginRight, 1.5, SymUnitEm)
}

func TestNewStyleContext_RootHorizontalMarginsAccumulateWithParagraphMargins(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-left: -1em; margin-right: -1em; }
		p { margin: 0 0 0 3em; }
	`), nil, zap.NewNop())

	props := resolvedStyleProps(t, registry, "p", "")
	requireMeasure(t, props, SymMarginLeft, 2, SymUnitEm)
	requireMeasure(t, props, SymMarginRight, -1, SymUnitEm)
}

func TestNewStyleContext_RootDescendantSelector(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		body p { text-align: center; }
		html p { text-indent: 0; }
	`), nil, zap.NewNop())

	props := resolvedStyleProps(t, registry, "p", "")
	if !isSymbol(props[SymTextAlignment], SymCenter) {
		t.Fatalf("body p text-align not applied, got %v", props[SymTextAlignment])
	}
	if got, ok := props[SymTextIndent]; !ok || !isZeroMargin(got) {
		t.Fatalf("html p text-indent not applied, got %v", got)
	}
}

func TestNewStyleContext_RootVerticalMarginsIgnored(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		body { margin-top: 5em; margin-bottom: 6em; }
	`), nil, zap.NewNop())

	props := resolvedStyleProps(t, registry, "", "")
	if _, ok := props[SymMarginTop]; ok {
		t.Fatalf("root margin-top should not be propagated: %v", props[SymMarginTop])
	}
	if _, ok := props[SymMarginBottom]; ok {
		t.Fatalf("root margin-bottom should not be propagated: %v", props[SymMarginBottom])
	}
}

func TestStartBlock_DoesNotApplyRootMarginsToTitleChildren(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-left: -0.5em; margin-right: -0.5em; }
		body { margin-left: -0.25em; margin-right: -0.25em; }
	`), nil, zap.NewNop())

	title := &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{
		Kind: fb2.InlineText,
		Text: "Wide title text",
	}}}}}}
	sb := NewStorylineBuilder("l1", "c1", 1, registry)
	sb.StartBlock("chapter-title", registry, nil)
	titleCtx := NewStyleContext(registry).Push("div", "chapter-title")
	addTitleAsHeading(&content.Content{}, title, titleCtx, "chapter-title-header", 1, sb, registry, nil, NewContentAccumulator(1), nil)
	sb.EndBlock()
	sb.Build()

	rootProps := resolvedStyleProps(t, registry, "", "")
	requireMeasure(t, rootProps, SymMarginLeft, -0.75, SymUnitEm)
	requireMeasure(t, rootProps, SymMarginRight, -0.75, SymUnitEm)

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one title wrapper, got %d", len(sb.contentEntries))
	}
	wrapper := sb.contentEntries[0]
	if len(wrapper.childRefs) != 1 {
		t.Fatalf("expected one title child, got %d", len(wrapper.childRefs))
	}
	childStyle, ok := registry.Get(wrapper.childRefs[0].Style)
	if !ok {
		t.Fatalf("child style %q not registered", wrapper.childRefs[0].Style)
	}
	if _, ok := childStyle.Properties[SymMarginLeft]; ok {
		t.Fatalf("root margin-left leaked into title child: %v", childStyle.Properties[SymMarginLeft])
	}
	if _, ok := childStyle.Properties[SymMarginRight]; ok {
		t.Fatalf("root margin-right leaked into title child: %v", childStyle.Properties[SymMarginRight])
	}
}

func TestAddTitleWithInlineImage_DoesNotApplyRootMarginsToTitleParagraph(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-left: -0.5em; margin-right: -0.5em; }
		body { margin-left: -0.25em; margin-right: -0.25em; }
	`), nil, zap.NewNop())

	rootProps := resolvedStyleProps(t, registry, "", "")
	requireMeasure(t, rootProps, SymMarginLeft, -0.75, SymUnitEm)
	requireMeasure(t, rootProps, SymMarginRight, -0.75, SymUnitEm)

	title := &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{
		Kind:  fb2.InlineImageSegment,
		Image: &fb2.InlineImage{Href: "#title"},
	}}}}}}
	images := imageResourceInfoByID{
		"title": {ResourceName: "e1", Width: 640, Height: 80},
	}
	sb := NewStorylineBuilder("l1", "c1", 1, registry)
	ctx := NewStyleContext(registry).Push("div", "chapter-title")
	addTitleAsHeading(&content.Content{}, title, ctx, "title-image-test", 1, sb, registry, images, NewContentAccumulator(1), nil)

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one title paragraph entry, got %d", len(sb.contentEntries))
	}
	styleName, ok := sb.contentEntries[0].RawEntry.Get(SymStyle).(SymbolByNameValue)
	if !ok {
		t.Fatalf("expected title paragraph style, got %T", sb.contentEntries[0].RawEntry.Get(SymStyle))
	}
	style, ok := registry.Get(string(styleName))
	if !ok {
		t.Fatalf("title paragraph style %q not registered", styleName)
	}
	if _, ok := style.Properties[SymMarginLeft]; ok {
		t.Fatalf("root margin-left leaked into title image paragraph: %v", style.Properties[SymMarginLeft])
	}
	if _, ok := style.Properties[SymMarginRight]; ok {
		t.Fatalf("root margin-right leaked into title image paragraph: %v", style.Properties[SymMarginRight])
	}

	contentList, ok := sb.contentEntries[0].RawEntry.GetList(SymContentList)
	if !ok || len(contentList) != 1 {
		t.Fatalf("expected mixed title content list with one image, got %#v", sb.contentEntries[0].RawEntry.Get(SymContentList))
	}
	imageEntry, ok := contentList[0].(StructValue)
	if !ok {
		t.Fatalf("expected inline image entry, got %T", contentList[0])
	}
	imageStyleName, ok := imageEntry.Get(SymStyle).(SymbolByNameValue)
	if !ok {
		t.Fatalf("expected inline image style, got %T", imageEntry.Get(SymStyle))
	}
	imageStyle, ok := registry.Get(string(imageStyleName))
	if !ok {
		t.Fatalf("inline image style %q not registered", imageStyleName)
	}
	if _, ok := imageStyle.Properties[SymMarginLeft]; ok {
		t.Fatalf("root margin-left leaked into inline title image: %v", imageStyle.Properties[SymMarginLeft])
	}
	if _, ok := imageStyle.Properties[SymMarginRight]; ok {
		t.Fatalf("root margin-right leaked into inline title image: %v", imageStyle.Properties[SymMarginRight])
	}
	requireMeasure(t, imageStyle.Properties, SymWidth, 40, SymUnitRem)
	requireMeasure(t, imageStyle.Properties, SymHeight, 5, SymUnitRem)
}

func TestAddBacklinkParagraph_InlineEventStyleDropsHorizontalMargins(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-left: -0.5em; margin-right: -0.5em; }
		body { margin-left: -0.25em; margin-right: -0.25em; }
		.link-backlink { margin-left: -2em; margin-right: -2em; font-weight: bold; color: gray; }
	`), nil, zap.NewNop())

	sb := NewStorylineBuilder("l1", "c1", 1, registry)
	c := &content.Content{BacklinkStr: "[<]"}
	refs := []content.BackLinkRef{
		{RefID: "ref-note-1"},
		{RefID: "ref-note-2"},
		{RefID: "ref-note-3"},
	}
	addBacklinkParagraph(c, refs, sb, registry, NewContentAccumulator(1), nil)

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one backlink paragraph, got %d", len(sb.contentEntries))
	}
	events := sb.contentEntries[0].StyleEvents
	if len(events) != len(refs) {
		t.Fatalf("expected %d backlink style events, got %d", len(refs), len(events))
	}
	for i, event := range events {
		if event.Style == "" {
			t.Fatalf("event %d has empty style", i)
		}
		style, ok := registry.Get(event.Style)
		if !ok {
			t.Fatalf("event %d style %q not registered", i, event.Style)
		}
		if _, ok := style.Properties[SymMarginLeft]; ok {
			t.Fatalf("event %d margin-left leaked into backlink inline style: %v", i, style.Properties[SymMarginLeft])
		}
		if _, ok := style.Properties[SymMarginRight]; ok {
			t.Fatalf("event %d margin-right leaked into backlink inline style: %v", i, style.Properties[SymMarginRight])
		}
		if event.LinkTo != refs[i].RefID {
			t.Fatalf("event %d link target = %q, want %q", i, event.LinkTo, refs[i].RefID)
		}
	}
}

func TestAddTable_DoesNotApplyRootMarginsInsideCells(t *testing.T) {
	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-left: -0.5em; margin-right: -0.5em; }
		body { margin-left: -0.25em; margin-right: -0.25em; }
	`), nil, zap.NewNop())

	table := &fb2.Table{
		Rows: []fb2.TableRow{{
			Cells: []fb2.TableCell{{
				Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "cell"}},
			}},
		}},
	}
	sb := NewStorylineBuilder("l1", "c1", 1, registry)
	sb.AddTable(&content.Content{}, table, registry, NewContentAccumulator(1), nil, nil)

	if len(sb.contentEntries) != 1 {
		t.Fatalf("expected one table content entry, got %d", len(sb.contentEntries))
	}
	tableEntry := sb.contentEntries[0].RawEntry
	tableList, ok := tableEntry.GetList(SymContentList)
	if !ok || len(tableList) != 1 {
		t.Fatalf("expected table body content list, got %#v", tableEntry.Get(SymContentList))
	}
	bodyEntry, ok := tableList[0].(StructValue)
	if !ok {
		t.Fatalf("expected body entry, got %T", tableList[0])
	}
	rowList, ok := bodyEntry.GetList(SymContentList)
	if !ok || len(rowList) != 1 {
		t.Fatalf("expected row content list, got %#v", bodyEntry.Get(SymContentList))
	}
	rowEntry, ok := rowList[0].(StructValue)
	if !ok {
		t.Fatalf("expected row entry, got %T", rowList[0])
	}
	cellList, ok := rowEntry.GetList(SymContentList)
	if !ok || len(cellList) != 1 {
		t.Fatalf("expected cell content list, got %#v", rowEntry.Get(SymContentList))
	}
	cellEntry, ok := cellList[0].(StructValue)
	if !ok {
		t.Fatalf("expected cell entry, got %T", cellList[0])
	}

	cellStyleName, ok := cellEntry.Get(SymStyle).(SymbolByNameValue)
	if !ok {
		t.Fatalf("expected cell style name, got %T", cellEntry.Get(SymStyle))
	}
	cellStyle, ok := registry.Get(string(cellStyleName))
	if !ok {
		t.Fatalf("cell style %q not registered", cellStyleName)
	}
	if _, ok := cellStyle.Properties[SymMarginLeft]; ok {
		t.Fatalf("root margin-left leaked into table cell container: %v", cellStyle.Properties[SymMarginLeft])
	}
	if _, ok := cellStyle.Properties[SymMarginRight]; ok {
		t.Fatalf("root margin-right leaked into table cell container: %v", cellStyle.Properties[SymMarginRight])
	}

	textList, ok := cellEntry.GetList(SymContentList)
	if !ok || len(textList) != 1 {
		t.Fatalf("expected cell text content list, got %#v", cellEntry.Get(SymContentList))
	}
	textEntry, ok := textList[0].(StructValue)
	if !ok {
		t.Fatalf("expected text entry, got %T", textList[0])
	}
	textStyleName, ok := textEntry.Get(SymStyle).(SymbolByNameValue)
	if !ok {
		t.Fatalf("expected text style name, got %T", textEntry.Get(SymStyle))
	}
	textStyle, ok := registry.Get(string(textStyleName))
	if !ok {
		t.Fatalf("text style %q not registered", textStyleName)
	}
	if _, ok := textStyle.Properties[SymMarginLeft]; ok {
		t.Fatalf("root margin-left leaked into table cell text: %v", textStyle.Properties[SymMarginLeft])
	}
	if _, ok := textStyle.Properties[SymMarginRight]; ok {
		t.Fatalf("root margin-right leaked into table cell text: %v", textStyle.Properties[SymMarginRight])
	}
}

func TestNewStyleRegistryFromCSS_LogsIgnoredRootVerticalMargins(t *testing.T) {
	var buf bytes.Buffer
	encoderCfg := zap.NewProductionEncoderConfig()
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(&buf), zap.DebugLevel)
	log := zap.New(core)

	_, _ = parseAndCreateRegistry([]byte(`
		html { margin-top: 1em; margin-bottom: 0; }
		body { margin-top: 0; margin-bottom: 2em; }
	`), nil, log)

	output := buf.String()
	if got := strings.Count(output, "Ignoring KFX root vertical margin"); got != 2 {
		t.Fatalf("expected 2 ignored root margin logs, got %d:\n%s", got, output)
	}
	for _, want := range []string{
		`"selector":"html"`,
		`"property":"margin-top"`,
		`"selector":"body"`,
		`"property":"margin-bottom"`,
		`"reason":"vertical margins on root elements are not modeled for split KFX storylines"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected log output to contain %s:\n%s", want, output)
		}
	}
	for _, notWant := range []string{
		`"selector":"html","property":"margin-bottom"`,
		`"selector":"body","property":"margin-top"`,
	} {
		if strings.Contains(output, notWant) {
			t.Fatalf("did not expect zero root margin log %s:\n%s", notWant, output)
		}
	}
}

func TestNewStyleRegistryFromCSS_DoesNotLogZeroRootVerticalMargins(t *testing.T) {
	var buf bytes.Buffer
	encoderCfg := zap.NewProductionEncoderConfig()
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(&buf), zap.DebugLevel)
	log := zap.New(core)

	registry, _ := parseAndCreateRegistry([]byte(`
		html { margin-top: 0; margin-bottom: 0; }
		body { margin-top: 0; margin-bottom: 0; }
	`), nil, log)

	props := resolvedStyleProps(t, registry, "", "")
	if _, ok := props[SymMarginTop]; ok {
		t.Fatalf("zero root margin-top should not be propagated: %v", props[SymMarginTop])
	}
	if _, ok := props[SymMarginBottom]; ok {
		t.Fatalf("zero root margin-bottom should not be propagated: %v", props[SymMarginBottom])
	}
	if output := buf.String(); strings.Contains(output, "Ignoring KFX root vertical margin") {
		t.Fatalf("zero root vertical margins should not be logged as ignored:\n%s", output)
	}
}
