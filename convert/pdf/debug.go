package pdf

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fbc/convert/pdf/docwriter"
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
	VisualOverflow   float64            `json:"visual_overflow,omitempty"`
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

type pdfDebugGlyphLine struct {
	Page         int             `json:"page"`
	Line         int             `json:"line"`
	Text         string          `json:"text"`
	X            float64         `json:"x"`
	Y            float64         `json:"y"`
	FontSize     float64         `json:"font_size"`
	FontResource string          `json:"font_resource,omitempty"`
	Glyphs       []pdfDebugGlyph `json:"glyphs"`
}

type pdfDebugGlyph struct {
	Index        int     `json:"index"`
	Fragment     int     `json:"fragment,omitempty"`
	PDFCID       uint16  `json:"pdf_cid,omitempty"`
	Source       string  `json:"source,omitempty"`
	Rune         string  `json:"rune,omitempty"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Advance      float64 `json:"advance"`
	XOffset      float64 `json:"x_offset,omitempty"`
	YOffset      float64 `json:"y_offset,omitempty"`
	MissingGlyph string  `json:"missing_glyph,omitempty"`
}

type pdfDebugJustificationLine struct {
	Page                       int                `json:"page"`
	Line                       int                `json:"line"`
	Text                       string             `json:"text"`
	Decision                   string             `json:"decision"`
	NaturalWidth               float64            `json:"natural_width"`
	DrawnWidth                 float64            `json:"drawn_width"`
	AvailableWidth             float64            `json:"available_width"`
	Residual                   float64            `json:"residual"`
	JustificationGaps          int                `json:"justification_gaps"`
	GlyphCount                 int                `json:"glyph_count"`
	ExtraWordSpacing           float64            `json:"extra_word_spacing,omitempty"`
	ExtraCharSpacing           float64            `json:"extra_char_spacing,omitempty"`
	WordSpacingCap             float64            `json:"word_spacing_cap,omitempty"`
	CharSpacingCap             float64            `json:"char_spacing_cap,omitempty"`
	WordSpacingCapped          bool               `json:"word_spacing_capped,omitempty"`
	CharSpacingCapped          bool               `json:"char_spacing_capped,omitempty"`
	ResidualAfterWordSpacing   float64            `json:"residual_after_word_spacing,omitempty"`
	ResidualAfterCharSpacing   float64            `json:"residual_after_char_spacing,omitempty"`
	BreakCandidateSummary      string             `json:"break_candidate_summary"`
	RejectedCandidatesRecorded bool               `json:"rejected_candidates_recorded"`
	LineBreak                  *pdfDebugLineBreak `json:"line_break,omitempty"`
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
	ResourceName               string                   `json:"resource_name"`
	Family                     string                   `json:"family"`
	Bold                       bool                     `json:"bold,omitempty"`
	Italic                     bool                     `json:"italic,omitempty"`
	PostScriptName             string                   `json:"post_script_name"`
	PDFBaseFont                string                   `json:"pdf_base_font,omitempty"`
	OutlineKind                string                   `json:"outline_kind,omitempty"`
	PDFCIDFontSubtype          string                   `json:"pdf_cid_font_subtype,omitempty"`
	PDFEmbeddedFontFile        string                   `json:"pdf_embedded_font_file,omitempty"`
	UnitsPerEm                 int                      `json:"units_per_em"`
	Ascent                     int                      `json:"ascent"`
	Descent                    int                      `json:"descent"`
	CapHeight                  int                      `json:"cap_height"`
	BBox                       [4]int                   `json:"bbox"`
	Flags                      int                      `json:"flags"`
	ItalicAngle                int                      `json:"italic_angle"`
	OriginalFontFileSize       int                      `json:"original_font_file_size"`
	EmbeddedFontFileSize       int                      `json:"embedded_font_file_size"`
	EmbeddedFontFileStreamSize int                      `json:"embedded_font_file_stream_size,omitempty"`
	ToUnicodeStreamSize        int                      `json:"to_unicode_stream_size,omitempty"`
	Subset                     bool                     `json:"subset,omitempty"`
	UsedGlyphCount             int                      `json:"used_glyph_count"`
	UsedGlyphIDs               []uint16                 `json:"used_glyph_ids"`
	OriginalGlyphIDs           []uint16                 `json:"original_glyph_ids,omitempty"`
	PDFCIDs                    []uint16                 `json:"pdf_cids,omitempty"`
	SubsetGlyphIDs             []uint16                 `json:"subset_glyph_ids,omitempty"`
	GlyphIDMap                 []pdfDebugFontGlyphIDMap `json:"glyph_id_map,omitempty"`
}

type pdfDebugFontGlyphIDMap struct {
	OriginalGlyphID uint16 `json:"original_glyph_id"`
	PDFCID          uint16 `json:"pdf_cid"`
	SubsetGlyphID   uint16 `json:"subset_glyph_id,omitempty"`
	Used            bool   `json:"used,omitempty"`
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
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-line-glyphs.json"), pdfDebugLineGlyphs(pages)); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-justification.json"), pdfDebugJustificationLines(pages)); err != nil {
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

func pdfDebugLineGlyphs(pages []pdfPage) []pdfDebugGlyphLine {
	out := make([]pdfDebugGlyphLine, 0)
	for pageIndex, page := range pages {
		for lineIndex, line := range page.Lines {
			glyphs := pdfDebugGlyphsForLine(line)
			if len(glyphs) == 0 {
				continue
			}
			out = append(out, pdfDebugGlyphLine{
				Page:         pageIndex + 1,
				Line:         lineIndex + 1,
				Text:         pdfPageLineText(line),
				X:            line.X,
				Y:            line.Y,
				FontSize:     line.FontSize,
				FontResource: line.FontName,
				Glyphs:       glyphs,
			})
		}
	}
	return out
}

func pdfDebugGlyphsForLine(line pdfPageLine) []pdfDebugGlyph {
	if len(line.Fragments) == 0 {
		return pdfDebugGlyphs(line.Text.Glyphs, 0, line.X, line.Y, line.FontSize, line.LetterSpacing+line.ExtraCharSpacing, line.ExtraWordSpacing)
	}

	glyphs := make([]pdfDebugGlyph, 0)
	currentX := line.X
	for fragmentIndex, fragment := range line.Fragments {
		fragmentGlyphs := pdfDebugGlyphs(
			fragment.Text.Glyphs,
			fragmentIndex+1,
			currentX,
			line.Y+fragment.BaselineShift,
			fragment.FontSize,
			fragment.LetterSpacing+line.ExtraCharSpacing,
			line.ExtraWordSpacing,
		)
		glyphs = append(glyphs, fragmentGlyphs...)
		currentX += fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
		if fragmentIndex != len(line.Fragments)-1 {
			currentX += line.ExtraCharSpacing
		}
		currentX += line.ExtraWordSpacing * float64(pdfPageFragmentJustificationSpaceCount(fragment, fragmentIndex != len(line.Fragments)-1))
	}
	return glyphs
}

func pdfDebugGlyphs(glyphs []shapedGlyph, fragment int, x float64, y float64, fontSize float64, letterSpacing float64, extraWordSpacing float64) []pdfDebugGlyph {
	out := make([]pdfDebugGlyph, 0, len(glyphs))
	currentX := x
	for i, glyph := range glyphs {
		glyphXOffset := glyphOffsetPoints(glyph.XOffset, fontSize)
		glyphYOffset := glyphOffsetPoints(glyph.YOffset, fontSize)
		entry := pdfDebugGlyph{
			Index:    i,
			Fragment: fragment,
			PDFCID:   glyph.GlyphID,
			Source:   glyphUnicodeText(glyph),
			Rune:     pdfDebugRune(glyph.Rune),
			X:        currentX + glyphXOffset,
			Y:        y + glyphYOffset,
			Advance:  glyphAdvancePoints(glyph, fontSize),
			XOffset:  glyphXOffset,
			YOffset:  glyphYOffset,
		}
		if glyph.Missing != pdfMissingGlyphNone {
			entry.MissingGlyph = glyph.Missing.String()
		}
		out = append(out, entry)
		currentX += glyphAdvancePoints(glyph, fontSize)
		if i != len(glyphs)-1 {
			currentX += letterSpacing
		}
		if glyph.Rune == ' ' && i != len(glyphs)-1 {
			currentX += extraWordSpacing
		}
	}
	return out
}

func pdfDebugRune(r rune) string {
	if r == 0 {
		return ""
	}
	return fmt.Sprintf("U+%04X", r)
}

func pdfDebugJustificationLines(pages []pdfPage) []pdfDebugJustificationLine {
	out := make([]pdfDebugJustificationLine, 0)
	for pageIndex, page := range pages {
		for lineIndex, line := range page.Lines {
			debugLine, ok := pdfDebugJustificationLineFor(pageIndex+1, lineIndex+1, line)
			if ok {
				out = append(out, debugLine)
			}
		}
	}
	return out
}

func pdfDebugJustificationLineFor(pageNumber int, lineNumber int, line pdfPageLine) (pdfDebugJustificationLine, bool) {
	justified := pdfPageLineIsJustified(line)
	if !justified && !line.BreakStats.Emergency && !line.BreakStats.Hyphenated && pdfPageLineOverflow(line) == 0 && pdfPageLineVisualOverflow(line) == 0 {
		return pdfDebugJustificationLine{}, false
	}
	naturalWidth := pdfPageLineAdvanceWidth(line)
	drawnWidth := pdfPageLineDrawnWidth(line)
	available := pdfPageLineAvailableWidth(line)
	glyphCount := pdfDebugLineGlyphCount(line)
	gaps := pdfPageLineJustificationSpaceCount(line)
	debugLine := pdfDebugJustificationLine{
		Page:                       pageNumber,
		Line:                       lineNumber,
		Text:                       pdfPageLineText(line),
		Decision:                   pdfDebugJustificationDecision(line, naturalWidth, available, glyphCount, gaps),
		NaturalWidth:               naturalWidth,
		DrawnWidth:                 drawnWidth,
		AvailableWidth:             available,
		Residual:                   available - naturalWidth,
		JustificationGaps:          gaps,
		GlyphCount:                 glyphCount,
		ExtraWordSpacing:           line.ExtraWordSpacing,
		ExtraCharSpacing:           line.ExtraCharSpacing,
		BreakCandidateSummary:      "selected_break_recorded; rejected_candidates_not_retained",
		RejectedCandidatesRecorded: false,
		LineBreak:                  pdfDebugLineBreakStats(line.BreakStats),
	}
	pdfDebugPopulateJustificationCaps(&debugLine, line.FontSize)
	return debugLine, true
}

func pdfDebugLineGlyphCount(line pdfPageLine) int {
	if len(line.Fragments) == 0 {
		return len(line.Text.Glyphs)
	}
	count := 0
	for _, fragment := range line.Fragments {
		count += len(fragment.Text.Glyphs)
	}
	return count
}

func pdfDebugJustificationDecision(line pdfPageLine, naturalWidth float64, available float64, glyphCount int, gaps int) string {
	if !pdfPageLineIsJustified(line) {
		switch {
		case line.BreakStats.Emergency:
			return "emergency_break_unjustified"
		case line.BreakStats.Hyphenated:
			return "hyphenated_break_unjustified"
		case pdfPageLineOverflow(line) > 0 || pdfPageLineVisualOverflow(line) > 0:
			return "overflow_unjustified"
		default:
			return "not_justified"
		}
	}
	if naturalWidth > available+pdfLineWidthTolerance {
		return pdfDebugJustificationShrinkDecision(line, naturalWidth-available, glyphCount, gaps)
	}
	return pdfDebugJustificationStretchDecision(line, available-naturalWidth, glyphCount, gaps)
}

func pdfDebugJustificationStretchDecision(line pdfPageLine, residual float64, glyphCount int, gaps int) string {
	if gaps <= 0 || residual <= pdfLineWidthTolerance {
		return "justified_no_adjustment"
	}
	wordCap := max(line.FontSize*0.40, 3.0)
	wordCapped := residual/float64(gaps) > wordCap
	remaining := residual - min(residual/float64(gaps), wordCap)*float64(gaps)
	if remaining <= pdfLineWidthTolerance || glyphCount < 2 {
		if wordCapped {
			return "stretch_word_spacing_capped"
		}
		return "stretch_word_spacing"
	}
	charCap := 0.25
	charCapped := remaining/float64(glyphCount-1) > charCap
	if wordCapped && charCapped {
		return "stretch_word_and_char_spacing_capped"
	}
	if wordCapped {
		return "stretch_word_spacing_capped_with_tracking"
	}
	if charCapped {
		return "stretch_char_spacing_capped"
	}
	return "stretch_word_and_char_spacing"
}

func pdfDebugJustificationShrinkDecision(line pdfPageLine, overflow float64, glyphCount int, gaps int) string {
	if gaps <= 0 || overflow <= pdfLineWidthTolerance {
		return "justified_no_adjustment"
	}
	wordCap := max(line.FontSize*0.18, 1.0)
	wordCapped := overflow/float64(gaps) > wordCap
	remaining := overflow - min(overflow/float64(gaps), wordCap)*float64(gaps)
	if remaining <= pdfLineWidthTolerance || glyphCount < 2 {
		if wordCapped {
			return "shrink_word_spacing_capped"
		}
		return "shrink_word_spacing"
	}
	charCap := min(max(line.FontSize*0.025, 0.12), 0.35)
	charCapped := remaining/float64(glyphCount-1) > charCap
	if wordCapped && charCapped {
		return "shrink_word_and_char_spacing_capped"
	}
	if wordCapped {
		return "shrink_word_spacing_capped_with_tracking"
	}
	if charCapped {
		return "shrink_char_spacing_capped"
	}
	return "shrink_word_and_char_spacing"
}

func pdfDebugPopulateJustificationCaps(line *pdfDebugJustificationLine, fontSize float64) {
	if line == nil || line.JustificationGaps <= 0 {
		return
	}
	if line.Residual >= 0 {
		line.WordSpacingCap = max(fontSize*0.40, 3.0)
		wordSpacing := min(line.Residual/float64(line.JustificationGaps), line.WordSpacingCap)
		line.WordSpacingCapped = line.Residual/float64(line.JustificationGaps) > line.WordSpacingCap
		line.ResidualAfterWordSpacing = line.Residual - wordSpacing*float64(line.JustificationGaps)
		if line.ResidualAfterWordSpacing > pdfLineWidthTolerance && line.GlyphCount > 1 {
			line.CharSpacingCap = 0.25
			charSpacing := min(line.ResidualAfterWordSpacing/float64(line.GlyphCount-1), line.CharSpacingCap)
			line.CharSpacingCapped = line.ResidualAfterWordSpacing/float64(line.GlyphCount-1) > line.CharSpacingCap
			line.ResidualAfterCharSpacing = line.ResidualAfterWordSpacing - charSpacing*float64(line.GlyphCount-1)
		}
		return
	}
	overflow := -line.Residual
	line.WordSpacingCap = max(fontSize*0.18, 1.0)
	wordShrink := min(overflow/float64(line.JustificationGaps), line.WordSpacingCap)
	line.WordSpacingCapped = overflow/float64(line.JustificationGaps) > line.WordSpacingCap
	line.ResidualAfterWordSpacing = -(overflow - wordShrink*float64(line.JustificationGaps))
	if -line.ResidualAfterWordSpacing > pdfLineWidthTolerance && line.GlyphCount > 1 {
		line.CharSpacingCap = min(max(fontSize*0.025, 0.12), 0.35)
		charShrink := min((-line.ResidualAfterWordSpacing)/float64(line.GlyphCount-1), line.CharSpacingCap)
		line.CharSpacingCapped = (-line.ResidualAfterWordSpacing)/float64(line.GlyphCount-1) > line.CharSpacingCap
		line.ResidualAfterCharSpacing = line.ResidualAfterWordSpacing + charShrink*float64(line.GlyphCount-1)
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
			visualLeft, visualRight, visualOK := pdfPageLineVisualBounds(line)
			if !visualOK {
				visualLeft, visualRight = 0, 0
			}
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
				VisualOverflow:   pdfPageLineVisualOverflow(line),
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
		originalSize := len(resource.Face.Data)
		embeddedSize := len(resource.Objects.FontFileData)
		subset := embeddedSize > 0 && embeddedSize < originalSize
		program := pdfDebugFontProgram(resource.Face)
		pdfCIDs, subsetGlyphIDs, glyphIDMap := pdfDebugFontGlyphIDMapping(resource, usedGlyphIDs, subset && program.TrueTypeOutlines)
		out = append(out, pdfDebugFont{
			ResourceName:               resource.Name,
			Family:                     resource.Key.Family,
			Bold:                       resource.Key.Bold,
			Italic:                     resource.Key.Italic,
			PostScriptName:             resource.Face.PostScriptName,
			PDFBaseFont:                pdfDebugFontBaseName(resource),
			OutlineKind:                program.OutlineKind,
			PDFCIDFontSubtype:          string(program.CIDFontSubtype),
			PDFEmbeddedFontFile:        program.FontFileKey,
			UnitsPerEm:                 resource.Face.UnitsPerEm,
			Ascent:                     resource.Face.Ascent,
			Descent:                    resource.Face.Descent,
			CapHeight:                  resource.Face.CapHeight,
			BBox:                       resource.Face.BBox,
			Flags:                      resource.Face.Flags,
			ItalicAngle:                resource.Face.ItalicAngle,
			OriginalFontFileSize:       originalSize,
			EmbeddedFontFileSize:       embeddedSize,
			EmbeddedFontFileStreamSize: compressedPDFStreamSize(resource.Objects.FontFileData),
			ToUnicodeStreamSize:        compressedPDFStreamSize(resource.Objects.ToUnicode),
			Subset:                     subset,
			UsedGlyphCount:             len(usedGlyphIDs),
			UsedGlyphIDs:               usedGlyphIDs,
			OriginalGlyphIDs:           usedGlyphIDs,
			PDFCIDs:                    pdfCIDs,
			SubsetGlyphIDs:             subsetGlyphIDs,
			GlyphIDMap:                 glyphIDMap,
		})
	}
	return out
}

func pdfDebugFontGlyphIDMapping(
	resource pdfFontResource,
	usedGlyphIDs []uint16,
	includeSubsetGIDs bool,
) ([]uint16, []uint16, []pdfDebugFontGlyphIDMap) {
	used := make(map[uint16]bool, len(usedGlyphIDs))
	pdfCIDs := make([]uint16, 0, len(usedGlyphIDs))
	for _, originalGlyphID := range usedGlyphIDs {
		used[originalGlyphID] = true
		if cid, ok := resource.CIDMap[originalGlyphID]; ok {
			pdfCIDs = append(pdfCIDs, cid)
		}
	}
	slices.Sort(pdfCIDs)
	pdfCIDs = compactSortedUint16s(pdfCIDs)

	originalIDs := make([]int, 0, len(resource.CIDMap))
	for originalGlyphID := range resource.CIDMap {
		originalIDs = append(originalIDs, int(originalGlyphID))
	}
	slices.Sort(originalIDs)

	glyphIDMap := make([]pdfDebugFontGlyphIDMap, 0, len(originalIDs))
	subsetGlyphIDs := make([]uint16, 0, len(originalIDs))
	for _, originalGlyphIDInt := range originalIDs {
		originalGlyphID := uint16(originalGlyphIDInt)
		cid := resource.CIDMap[originalGlyphID]
		entry := pdfDebugFontGlyphIDMap{
			OriginalGlyphID: originalGlyphID,
			PDFCID:          cid,
			Used:            used[originalGlyphID],
		}
		if includeSubsetGIDs {
			entry.SubsetGlyphID = cid
			subsetGlyphIDs = append(subsetGlyphIDs, cid)
		}
		glyphIDMap = append(glyphIDMap, entry)
	}
	slices.Sort(subsetGlyphIDs)
	subsetGlyphIDs = compactSortedUint16s(subsetGlyphIDs)
	return pdfCIDs, subsetGlyphIDs, glyphIDMap
}

func compactSortedUint16s(values []uint16) []uint16 {
	if len(values) < 2 {
		return values
	}
	out := values[:0]
	var previous uint16
	for i, value := range values {
		if i > 0 && value == previous {
			continue
		}
		out = append(out, value)
		previous = value
	}
	return out
}

func pdfDebugFontProgram(face *builtinFontFace) pdfFontProgramInfo {
	if face == nil {
		return pdfFontProgramInfo{}
	}
	program, err := pdfFontProgram(face.Data)
	if err != nil {
		return pdfFontProgramInfo{}
	}
	return program
}

func pdfDebugFontBaseName(resource pdfFontResource) string {
	baseFont, ok := resource.Objects.Type0Font["BaseFont"].(docwriter.Name)
	if !ok {
		return ""
	}
	return string(baseFont)
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
