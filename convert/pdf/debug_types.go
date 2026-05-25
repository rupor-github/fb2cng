package pdf

import "fbc/convert/pdf/structure"

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
