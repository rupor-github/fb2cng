package pdf

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fbc/convert/pdf/structure"
)

type pdfDebugStructurePlan struct {
	Units     []pdfDebugStructureUnit     `json:"units"`
	TOC       []pdfDebugStructureTOCEntry `json:"toc,omitempty"`
	Landmarks structure.LandmarkInfo      `json:"landmarks"`
	Generated pdfDebugStructureGenerated  `json:"generated"`
}

type pdfDebugStructureUnit struct {
	Index        int    `json:"index"`
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	Title        string `json:"title,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	TitleDepth   int    `json:"title_depth,omitempty"`
	ForceNewPage bool   `json:"force_new_page,omitempty"`
	BodyName     string `json:"body_name,omitempty"`
	SectionID    string `json:"section_id,omitempty"`
	IsTopLevel   bool   `json:"is_top_level,omitempty"`
}

type pdfDebugStructureTOCEntry struct {
	ID           string                      `json:"id,omitempty"`
	Title        string                      `json:"title,omitempty"`
	IncludeInTOC bool                        `json:"include_in_toc"`
	Children     []pdfDebugStructureTOCEntry `json:"children,omitempty"`
}

type pdfDebugStructureGenerated struct {
	AnnotationPage                 bool   `json:"annotation_page"`
	AnnotationInTOC                bool   `json:"annotation_in_toc"`
	TOCPagePlacement               string `json:"toc_page_placement,omitempty"`
	TOCIncludeChaptersWithoutTitle bool   `json:"toc_include_chapters_without_title,omitempty"`
	TOCType                        string `json:"toc_type,omitempty"`
}

type pdfDebugBlock struct {
	Index        int    `json:"index"`
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	StyleName    string `json:"style_name,omitempty"`
	StyleClasses string `json:"style_classes,omitempty"`
	ImageID      string `json:"image_id,omitempty"`
	Text         string `json:"text,omitempty"`
}

type pdfDebugPage struct {
	Number      int                  `json:"number"`
	ObjectID    int                  `json:"object_id,omitempty"`
	ContentID   int                  `json:"content_id,omitempty"`
	Anchors     []string             `json:"anchors,omitempty"`
	Lines       []pdfDebugLine       `json:"lines"`
	Images      []pdfDebugImage      `json:"images,omitempty"`
	Backgrounds []pdfDebugBackground `json:"backgrounds,omitempty"`
	Borders     []pdfDebugBorder     `json:"borders,omitempty"`
	Strokes     []pdfDebugStroke     `json:"strokes,omitempty"`
	Links       []pdfDebugLink       `json:"links,omitempty"`
}

type pdfDebugBackground struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Color  string  `json:"color"`
}

type pdfDebugBorder struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	LineWidth float64 `json:"line_width"`
	Color     string  `json:"color"`
}

type pdfDebugStroke struct {
	X1        float64 `json:"x1"`
	Y1        float64 `json:"y1"`
	X2        float64 `json:"x2"`
	Y2        float64 `json:"y2"`
	LineWidth float64 `json:"line_width"`
	Color     string  `json:"color"`
}

type pdfDebugLine struct {
	Text             string             `json:"text"`
	X                float64            `json:"x"`
	Y                float64            `json:"y"`
	FontSize         float64            `json:"font_size"`
	LetterSpacing    float64            `json:"letter_spacing,omitempty"`
	FontResource     string             `json:"font_resource,omitempty"`
	FontFamily       string             `json:"font_family,omitempty"`
	FontWeight       string             `json:"font_weight,omitempty"`
	FontStyle        string             `json:"font_style,omitempty"`
	Color            string             `json:"color,omitempty"`
	Underline        bool               `json:"underline,omitempty"`
	Strikethrough    bool               `json:"strikethrough,omitempty"`
	Width            float64            `json:"width"`
	AdvanceWidth     float64            `json:"advance_width"`
	DrawnWidth       float64            `json:"drawn_width"`
	AvailableWidth   float64            `json:"available_width,omitempty"`
	RightEdge        float64            `json:"right_edge"`
	VisualLeft       float64            `json:"visual_left,omitempty"`
	VisualRight      float64            `json:"visual_right,omitempty"`
	Overflow         float64            `json:"overflow,omitempty"`
	Justified        bool               `json:"justified,omitempty"`
	ExtraWordSpacing float64            `json:"extra_word_spacing,omitempty"`
	ExtraCharSpacing float64            `json:"extra_char_spacing,omitempty"`
	LineBreak        *pdfDebugLineBreak `json:"line_break,omitempty"`
}

type pdfDebugLineBreak struct {
	AvailableWidth  float64 `json:"available_width"`
	AdjustmentRatio float64 `json:"adjustment_ratio"`
	Badness         float64 `json:"badness"`
	Demerits        float64 `json:"demerits"`
	Fitness         string  `json:"fitness"`
	Hyphenated      bool    `json:"hyphenated,omitempty"`
	Emergency       bool    `json:"emergency,omitempty"`
	SingleWord      bool    `json:"single_word,omitempty"`
}

type pdfDebugImage struct {
	Page         int     `json:"page,omitempty"`
	ImageID      string  `json:"image_id"`
	ResourceName string  `json:"resource_name,omitempty"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
}

type pdfDebugLink struct {
	Page     int          `json:"page,omitempty"`
	ObjectID int          `json:"object_id,omitempty"`
	Href     string       `json:"href"`
	Internal bool         `json:"internal"`
	Rect     pdfDebugRect `json:"rect"`
}

type pdfDebugRect struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}

type pdfDebugFont struct {
	ResourceName   string   `json:"resource_name"`
	Family         string   `json:"family"`
	Bold           bool     `json:"bold,omitempty"`
	Italic         bool     `json:"italic,omitempty"`
	PostScriptName string   `json:"post_script_name"`
	UnitsPerEm     int      `json:"units_per_em"`
	Ascent         int      `json:"ascent"`
	Descent        int      `json:"descent"`
	CapHeight      int      `json:"cap_height"`
	BBox           [4]int   `json:"bbox"`
	Flags          int      `json:"flags"`
	ItalicAngle    int      `json:"italic_angle"`
	UsedGlyphCount int      `json:"used_glyph_count"`
	UsedGlyphIDs   []uint16 `json:"used_glyph_ids"`
}

type pdfDebugPrintedFootnotes struct {
	Enabled                  bool                                      `json:"enabled"`
	SourcePageCount          int                                       `json:"source_page_count"`
	FinalPageCount           int                                       `json:"final_page_count"`
	FootnoteTextHeight       float64                                   `json:"footnote_text_height,omitempty"`
	PlanCount                int                                       `json:"plan_count"`
	ReserveCount             int                                       `json:"reserve_count"`
	ContinuationPageCount    int                                       `json:"continuation_page_count"`
	PagesWithoutPrintedRefs  int                                       `json:"pages_without_printed_refs,omitempty"`
	SkippedCount             int                                       `json:"skipped_count"`
	OverflowCount            int                                       `json:"overflow_count"`
	Plans                    []pdfDebugPrintedFootnotePlan             `json:"plans,omitempty"`
	Reserves                 []pdfDebugPrintedFootnoteReserve          `json:"reserves,omitempty"`
	PackedContinuationChunks []pdfDebugPrintedFootnoteContinuationPack `json:"packed_continuation_chunks,omitempty"`
	Skipped                  []pdfDebugPrintedFootnoteCase             `json:"skipped,omitempty"`
	Overflow                 []pdfDebugPrintedFootnoteCase             `json:"overflow,omitempty"`
}

type pdfDebugPrintedFootnotePlan struct {
	PageIndex         int                                 `json:"page_index"`
	Refs              []pdfDebugPrintedFootnoteRef        `json:"refs"`
	Queue             []pdfDebugPrintedFootnoteQueueEntry `json:"queue"`
	QueuePageCount    int                                 `json:"queue_page_count"`
	ContinuationPages int                                 `json:"continuation_pages"`
	QueuePages        []pdfDebugPrintedFootnoteQueuePage  `json:"queue_pages,omitempty"`
	Reserve           float64                             `json:"reserve,omitempty"`
}

type pdfDebugPrintedFootnoteRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type pdfDebugPrintedFootnoteQueueEntry struct {
	ID        string `json:"id"`
	PageLabel string `json:"page_label"`
	Nested    bool   `json:"nested,omitempty"`
}

type pdfDebugPrintedFootnoteQueuePage struct {
	Index     int              `json:"index"`
	LineCount int              `json:"line_count"`
	Bounds    *pdfDebugYBounds `json:"bounds,omitempty"`
}

type pdfDebugPrintedFootnoteReserve struct {
	PageIndex int     `json:"page_index"`
	Reserve   float64 `json:"reserve"`
}

type pdfDebugPrintedFootnoteContinuationPack struct {
	SourcePageIndex       int     `json:"source_page_index"`
	QueuePageIndex        int     `json:"queue_page_index"`
	ContinuationPageIndex int     `json:"continuation_page_index"`
	ChunkTop              float64 `json:"chunk_top"`
	ChunkBottom           float64 `json:"chunk_bottom"`
	ChunkHeight           float64 `json:"chunk_height"`
	ShiftY                float64 `json:"shift_y"`
	PlacedTop             float64 `json:"placed_top"`
	PlacedBottom          float64 `json:"placed_bottom"`
}

type pdfDebugPrintedFootnoteCase struct {
	Kind           string  `json:"kind"`
	Reason         string  `json:"reason"`
	PageIndex      int     `json:"page_index,omitempty"`
	QueuePageIndex int     `json:"queue_page_index,omitempty"`
	Value          float64 `json:"value,omitempty"`
	Limit          float64 `json:"limit,omitempty"`
}

type pdfDebugYBounds struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Height float64 `json:"height"`
}

func writePDFDebugDumps(doc pdfDocumentSpec, pages []pdfPage, fontResources []pdfFontResource, printedFootnotes pdfDebugPrintedFootnotes) error {
	if !doc.Debug || doc.WorkDir == "" {
		return nil
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-structure-plan.json"), doc.DebugPlan); err != nil {
		return err
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-resolved-styles.json"), styles.debugStyles()); err != nil {
		return err
	}

	blocks := make([]pdfDebugBlock, 0, len(doc.Blocks))
	for i, block := range doc.Blocks {
		styleName := pdfStyleNameForBlock(block)
		styles.tracer.traceAssign(block, styleName, styles.styleForBlock(block))
		blocks = append(blocks, pdfDebugBlock{
			Index:        i,
			Kind:         block.Kind.String(),
			ID:           block.ID,
			Depth:        block.Depth,
			StyleName:    styleName,
			StyleClasses: strings.TrimSpace(block.StyleClasses),
			ImageID:      block.ImageID,
			Text:         block.Text,
		})
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-text-blocks.json"), blocks); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-printed-footnotes.json"), printedFootnotes); err != nil {
		return err
	}
	styles.tracer.flush()

	debugPages, debugImages, debugLinks := pdfDebugPages(pages)
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-layout-pages.json"), debugPages); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-images.json"), debugImages); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-links.json"), debugLinks); err != nil {
		return err
	}
	return writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-fonts.json"), pdfDebugFonts(fontResources))
}

func pdfPageLineText(line pdfPageLine) string {
	if len(line.Fragments) == 0 {
		return shapedRunes(line.Text)
	}
	var b strings.Builder
	for _, fragment := range line.Fragments {
		b.WriteString(shapedRunes(fragment.Text))
	}
	return b.String()
}

func pdfDebugLineBreakStats(stats paragraphLineBreakStats) *pdfDebugLineBreak {
	if stats.AvailableWidth <= 0 || math.IsInf(stats.AdjustmentRatio, 0) || math.IsInf(stats.Badness, 0) || math.IsInf(stats.Demerits, 0) {
		return nil
	}
	return &pdfDebugLineBreak{
		AvailableWidth:  stats.AvailableWidth,
		AdjustmentRatio: stats.AdjustmentRatio,
		Badness:         stats.Badness,
		Demerits:        stats.Demerits,
		Fitness:         paragraphFitnessString(stats.Fitness),
		Hyphenated:      stats.Hyphenated,
		Emergency:       stats.Emergency,
		SingleWord:      stats.SingleWord,
	}
}

func pdfDebugPages(pages []pdfPage) ([]pdfDebugPage, []pdfDebugImage, []pdfDebugLink) {
	debugPages := make([]pdfDebugPage, 0, len(pages))
	debugImages := make([]pdfDebugImage, 0)
	debugLinks := make([]pdfDebugLink, 0)
	for i, page := range pages {
		debugPage := pdfDebugPage{
			Number:      i + 1,
			ObjectID:    page.ObjectID,
			ContentID:   page.ContentID,
			Anchors:     slices.Clone(page.Anchors),
			Lines:       make([]pdfDebugLine, 0, len(page.Lines)),
			Images:      make([]pdfDebugImage, 0, len(page.Images)),
			Backgrounds: make([]pdfDebugBackground, 0, len(page.Backgrounds)),
			Borders:     make([]pdfDebugBorder, 0, len(page.Borders)),
			Strokes:     make([]pdfDebugStroke, 0, len(page.Strokes)),
			Links:       make([]pdfDebugLink, 0, len(page.Annotations)),
		}
		for _, line := range page.Lines {
			advanceWidth := pdfPageLineAdvanceWidth(line)
			drawnWidth := pdfPageLineDrawnWidth(line)
			visualLeft, visualRight, _ := pdfPageLineVisualBounds(line)
			debugPage.Lines = append(debugPage.Lines, pdfDebugLine{
				Text:             pdfPageLineText(line),
				X:                line.X,
				Y:                line.Y,
				FontSize:         line.FontSize,
				LetterSpacing:    line.LetterSpacing,
				FontResource:     line.FontName,
				FontFamily:       line.FontKey.Family,
				FontWeight:       pdfCSSFontWeightString(line.FontKey.Bold),
				FontStyle:        pdfCSSFontStyleString(line.FontKey.Italic),
				Color:            line.Color.String(),
				Underline:        line.Underline,
				Strikethrough:    line.Strikethrough,
				Width:            drawnWidth,
				AdvanceWidth:     advanceWidth,
				DrawnWidth:       drawnWidth,
				AvailableWidth:   pdfPageLineAvailableWidth(line),
				RightEdge:        line.X + drawnWidth,
				VisualLeft:       visualLeft,
				VisualRight:      visualRight,
				Overflow:         pdfPageLineOverflow(line),
				Justified:        pdfPageLineIsJustified(line),
				ExtraWordSpacing: line.ExtraWordSpacing,
				ExtraCharSpacing: line.ExtraCharSpacing,
				LineBreak:        pdfDebugLineBreakStats(line.BreakStats),
			})
		}
		for _, background := range page.Backgrounds {
			debugPage.Backgrounds = append(debugPage.Backgrounds, pdfDebugBackground{
				X:      background.X,
				Y:      background.Y,
				Width:  background.Width,
				Height: background.Height,
				Color:  background.Color.String(),
			})
		}
		for _, border := range page.Borders {
			debugPage.Borders = append(debugPage.Borders, pdfDebugBorder{
				X:         border.X,
				Y:         border.Y,
				Width:     border.Width,
				Height:    border.Height,
				LineWidth: border.LineWidth,
				Color:     border.Color.String(),
			})
		}
		for _, stroke := range page.Strokes {
			debugPage.Strokes = append(debugPage.Strokes, pdfDebugStroke{
				X1:        stroke.X1,
				Y1:        stroke.Y1,
				X2:        stroke.X2,
				Y2:        stroke.Y2,
				LineWidth: stroke.LineWidth,
				Color:     stroke.Color.String(),
			})
		}
		for _, image := range page.Images {
			debugImage := pdfDebugImage{
				Page:         i + 1,
				ImageID:      image.ImageID,
				ResourceName: image.Name,
				X:            image.X,
				Y:            image.Y,
				Width:        image.Width,
				Height:       image.Height,
			}
			debugPage.Images = append(debugPage.Images, debugImage)
			debugImages = append(debugImages, debugImage)
		}
		for _, link := range page.Annotations {
			debugLink := pdfDebugLink{
				Page:     i + 1,
				ObjectID: link.ObjectID,
				Href:     link.Href,
				Internal: strings.HasPrefix(link.Href, "#"),
				Rect: pdfDebugRect{
					X1: link.Rect.X1,
					Y1: link.Rect.Y1,
					X2: link.Rect.X2,
					Y2: link.Rect.Y2,
				},
			}
			debugPage.Links = append(debugPage.Links, debugLink)
			debugLinks = append(debugLinks, debugLink)
		}
		debugPages = append(debugPages, debugPage)
	}
	return debugPages, debugImages, debugLinks
}

func pdfDebugFonts(resources []pdfFontResource) []pdfDebugFont {
	out := make([]pdfDebugFont, 0, len(resources))
	for _, resource := range resources {
		if resource.Face == nil {
			continue
		}
		usedGlyphIDs := make([]uint16, 0, len(resource.Used))
		for glyphID := range resource.Used {
			usedGlyphIDs = append(usedGlyphIDs, glyphID)
		}
		slices.Sort(usedGlyphIDs)
		out = append(out, pdfDebugFont{
			ResourceName:   resource.Name,
			Family:         resource.Key.Family,
			Bold:           resource.Key.Bold,
			Italic:         resource.Key.Italic,
			PostScriptName: resource.Face.PostScriptName,
			UnitsPerEm:     resource.Face.UnitsPerEm,
			Ascent:         resource.Face.Ascent,
			Descent:        resource.Face.Descent,
			CapHeight:      resource.Face.CapHeight,
			BBox:           resource.Face.BBox,
			Flags:          resource.Face.Flags,
			ItalicAngle:    resource.Face.ItalicAngle,
			UsedGlyphCount: len(usedGlyphIDs),
			UsedGlyphIDs:   usedGlyphIDs,
		})
	}
	return out
}

func pdfDebugPrintedFootnotesFromReserved(doc pdfDocumentSpec, reserved pdfPrintedFootnoteReservedLayout) pdfDebugPrintedFootnotes {
	summary := pdfDebugPrintedFootnotes{
		Enabled:            pdfPrintedFootnotesEnabled(doc.Content) && len(doc.PrintedFootnotes) > 0,
		SourcePageCount:    len(reserved.Pages),
		FootnoteTextHeight: reserved.FootnoteTextHeight,
	}
	if !summary.Enabled {
		return summary
	}
	summary.Plans = make([]pdfDebugPrintedFootnotePlan, 0, len(reserved.Plans))
	plannedPages := make(map[int]bool, len(reserved.Plans))
	for _, plan := range reserved.Plans {
		debugPlan := pdfDebugPrintedFootnotePlan{
			PageIndex:         plan.PageIndex,
			Refs:              pdfDebugPrintedFootnoteRefs(plan.Refs),
			Queue:             pdfDebugPrintedFootnoteQueue(plan.Queue),
			QueuePageCount:    len(plan.QueuePages),
			ContinuationPages: plan.ContinuationPages,
			QueuePages:        pdfDebugPrintedFootnoteQueuePages(plan.QueuePages),
		}
		if plan.PageIndex >= 0 && plan.PageIndex < len(reserved.PageBottomReserves) {
			debugPlan.Reserve = reserved.PageBottomReserves[plan.PageIndex]
		}
		summary.Plans = append(summary.Plans, debugPlan)
		if plan.PageIndex >= 0 && plan.PageIndex < len(reserved.Pages) {
			plannedPages[plan.PageIndex] = true
		}
		summary.ContinuationPageCount += plan.ContinuationPages
	}
	summary.PlanCount = len(summary.Plans)
	summary.PagesWithoutPrintedRefs = max(len(reserved.Pages)-len(plannedPages), 0)
	for pageIndex, reserve := range reserved.PageBottomReserves {
		if reserve <= 0 {
			continue
		}
		summary.Reserves = append(summary.Reserves, pdfDebugPrintedFootnoteReserve{PageIndex: pageIndex, Reserve: reserve})
	}
	summary.ReserveCount = len(summary.Reserves)
	pdfDebugPrintedFootnotesReserveOverflow(doc, reserved, &summary)
	pdfDebugPrintedFootnotesSyncCounts(&summary)
	return summary
}

func pdfDebugPrintedFootnoteRefs(refs []pdfPrintedFootnoteRef) []pdfDebugPrintedFootnoteRef {
	out := make([]pdfDebugPrintedFootnoteRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, pdfDebugPrintedFootnoteRef(ref))
	}
	return out
}

func pdfDebugPrintedFootnoteQueue(queue []pdfPrintedFootnoteQueueEntry) []pdfDebugPrintedFootnoteQueueEntry {
	out := make([]pdfDebugPrintedFootnoteQueueEntry, 0, len(queue))
	for _, entry := range queue {
		out = append(out, pdfDebugPrintedFootnoteQueueEntry(entry))
	}
	return out
}

func pdfDebugPrintedFootnoteQueuePages(pages []pdfPage) []pdfDebugPrintedFootnoteQueuePage {
	out := make([]pdfDebugPrintedFootnoteQueuePage, 0, len(pages))
	for i, page := range pages {
		out = append(out, pdfDebugPrintedFootnoteQueuePage{
			Index:     i,
			LineCount: len(page.Lines),
			Bounds:    pdfDebugPageYBounds(page),
		})
	}
	return out
}

func pdfDebugPageYBounds(page pdfPage) *pdfDebugYBounds {
	top, bottom, ok := pdfPageYBounds(page)
	if !ok {
		return nil
	}
	return &pdfDebugYBounds{Top: top, Bottom: bottom, Height: max(top-bottom, 0)}
}

func pdfDebugPrintedFootnotesReserveOverflow(doc pdfDocumentSpec, reserved pdfPrintedFootnoteReservedLayout, summary *pdfDebugPrintedFootnotes) {
	if summary == nil || len(reserved.Plans) == 0 || reserved.FootnoteTextHeight <= 0 {
		return
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	const margin = 24.0
	contentLeft, contentRight, contentTop, contentBottom := pdfPageContentMargins(doc, styles, margin)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(doc, styles, contentLeft, contentWidth, contentBottom, reserved.FootnoteTextHeight)
	maxReserve := max(doc.PageHeight-contentTop-contentBottom-pdfBaseLineHeight, 0)
	if maxReserve <= 0 {
		return
	}
	for _, plan := range reserved.Plans {
		if plan.PageIndex < 0 || plan.PageIndex >= len(reserved.PageBottomReserves) || len(plan.QueuePages) == 0 {
			continue
		}
		requested := pdfPrintedFootnotePagePlanReserve(plan, reserved.FootnoteTextHeight, separator)
		actual := reserved.PageBottomReserves[plan.PageIndex]
		if requested > actual && actual == maxReserve {
			summary.Overflow = append(summary.Overflow, pdfDebugPrintedFootnoteCase{
				Kind:      "reserve",
				Reason:    "clamped_to_main_text_minimum",
				PageIndex: plan.PageIndex,
				Value:     requested,
				Limit:     actual,
			})
		}
	}
}

func pdfDebugPrintedFootnotesSyncCounts(summary *pdfDebugPrintedFootnotes) {
	if summary == nil {
		return
	}
	summary.PlanCount = len(summary.Plans)
	summary.ReserveCount = len(summary.Reserves)
	summary.SkippedCount = len(summary.Skipped)
	summary.OverflowCount = len(summary.Overflow)
}

func writeJSONDebugDump(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func shapedRunes(text shapedText) string {
	var b strings.Builder
	for _, glyph := range text.Glyphs {
		b.WriteString(glyphUnicodeText(glyph))
	}
	return b.String()
}
