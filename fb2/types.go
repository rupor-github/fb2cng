package fb2

import (
	"strings"
	"time"

	"golang.org/x/text/language"

	"fbc/config"
)

// Type definitions for FictionBook2 format structures.

// FictionBook mirrors the root element defined in FictionBook.xsd.
type FictionBook struct {
	Stylesheets     []Stylesheet
	Description     Description
	Bodies          []Body
	Binaries        []BinaryObject
	NotFoundImageID string                        // ID used for placeholder image when broken image links are found
	VignetteIDs     map[config.VignettePos]string // IDs for vignette images by position
}

func (fb *FictionBook) IsVignetteEnabled(position config.VignettePos) bool {
	_, exists := fb.VignetteIDs[position]
	return exists
}

type FootnoteRef struct {
	BodyIdx    int
	SectionIdx int
}

type FootnoteRefs map[string]FootnoteRef

type BookImage struct {
	MimeType string
	Data     []byte
	Filename string
	Dim      struct {
		Width  int
		Height int
	}
}

type BookImages map[string]*BookImage

// ElementRef describes the location and type of an element
type ElementRef struct {
	Type string
	Path []any
}

type IDIndex map[string]ElementRef

type ReverseLinkIndex map[string][]ElementRef

// Stylesheet corresponds to the optional <stylesheet> element.
type Stylesheet struct {
	Type string
	Data string
}

// Description groups metadata about the book extracted from <description>.
type Description struct {
	TitleInfo    TitleInfo
	SrcTitleInfo *TitleInfo
	DocumentInfo DocumentInfo
	PublishInfo  *PublishInfo
	CustomInfo   []CustomInfo
	Output       []OutputInstruction
}

// TitleInfo aggregates the <title-info> block.
type TitleInfo struct {
	Genres      []GenreRef
	Authors     []Author
	BookTitle   TextField
	Annotation  *Flow
	Keywords    *TextField
	Date        *Date
	Coverpage   []InlineImage
	Lang        language.Tag
	SrcLang     language.Tag
	Translators []Author
	Sequences   []Sequence
}

// GenreRef represents a <genre> entry with an optional match percentage.
type GenreRef struct {
	Value string
	Match int
}

// TextField corresponds to textFieldType and stores the text value with an optional xml:lang.
type TextField struct {
	Value string
	Lang  string
}

// Date mirrors dateType storing both rendered text and an optional machine readable value.
type Date struct {
	Display string
	Value   time.Time
	Lang    string
}

// Flow is a heterogeneous ordered collection of flow items such as paragraphs and poems.
type Flow struct {
	ID    string
	Lang  string
	Items []FlowItem
}

// AsPlainText extracts plain text content from all flow items, excluding image alt text.
func (f *Flow) AsPlainText() string {
	var buf strings.Builder
	for _, item := range f.Items {
		text := item.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}
	return strings.TrimSpace(buf.String())
}

// FlowItemKind distinguishes the different kinds of flow content.
type FlowItemKind string

const (
	FlowParagraph FlowItemKind = "paragraph"
	FlowImage     FlowItemKind = "image"
	FlowPoem      FlowItemKind = "poem"
	FlowSubtitle  FlowItemKind = "subtitle"
	FlowCite      FlowItemKind = "cite"
	FlowEmptyLine FlowItemKind = "empty-line"
	FlowTable     FlowItemKind = "table"
	FlowSection   FlowItemKind = "section"
)

// FlowItem stores a single piece of flow content, keeping the original ordering.
type FlowItem struct {
	Kind      FlowItemKind
	Paragraph *Paragraph
	Poem      *Poem
	Cite      *Cite
	Subtitle  *Paragraph
	Table     *Table
	Image     *Image
	Section   *Section
}

// AsPlainText extracts plain text from the flow item based on its kind.
func (fi *FlowItem) AsPlainText() string {
	switch fi.Kind {
	case FlowParagraph:
		if fi.Paragraph != nil {
			return fi.Paragraph.AsPlainText()
		}
	case FlowSubtitle:
		if fi.Subtitle != nil {
			return fi.Subtitle.AsPlainText()
		}
	case FlowPoem:
		if fi.Poem != nil {
			return fi.Poem.AsPlainText()
		}
	case FlowCite:
		if fi.Cite != nil {
			return fi.Cite.AsPlainText()
		}
	case FlowTable:
		if fi.Table != nil {
			return fi.Table.AsPlainText()
		}
	case FlowSection:
		if fi.Section != nil {
			return fi.Section.AsPlainText()
		}
	}
	return ""
}

// Author mirrors authorType in the schema.
type Author struct {
	FirstName  string
	MiddleName string
	LastName   string
	Nickname   string
	HomePages  []string
	Emails     []string
	ID         string
}

// DocumentInfo captures the <document-info> block.
type DocumentInfo struct {
	Authors     []Author
	ProgramUsed *TextField
	Date        Date
	SourceURLs  []string
	SourceOCR   *TextField
	ID          string
	Version     string
	History     *Flow
	Publishers  []Author
}

// PublishInfo mirrors the optional <publish-info> element.
type PublishInfo struct {
	BookName  *TextField
	Publisher *TextField
	City      *TextField
	Year      string
	ISBN      *TextField
	Sequences []Sequence
}

// CustomInfo stores <custom-info info-type="..."> metadata entries.
type CustomInfo struct {
	Type  string
	Value TextField
}

// OutputInstruction represents one <output> element describing sharing rules.
type OutputInstruction struct {
	Mode       ShareMode
	IncludeAll ShareDirective
	Price      *float64
	Currency   string
	Parts      []PartInstruction
	Documents  []OutputDocument
}

// ShareMode enumerates the allowed values of the mode attribute.
type ShareMode string

const (
	ShareModeFree ShareMode = "free"
	ShareModePaid ShareMode = "paid"
)

// ShareDirective captures allow/deny/require semantics.
type ShareDirective string

const (
	ShareRequire ShareDirective = "require"
	ShareAllow   ShareDirective = "allow"
	ShareDeny    ShareDirective = "deny"
)

// PartInstruction mirrors partShareInstructionType.
type PartInstruction struct {
	Href    string
	Type    string
	Include ShareDirective
}

// OutputDocument mirrors outPutDocumentType.
type OutputDocument struct {
	Name   string
	Create *ShareDirective
	Price  *float64
	Parts  []PartInstruction
}

// Sequence represents a nested <sequence> tree.
type Sequence struct {
	Name     string
	Number   *int
	Lang     string
	Children []Sequence
}

type BodyKind int

const (
	BodyOther BodyKind = iota
	BodyMain
	BodyFootnotes
)

func (bk BodyKind) String() string {
	switch bk {
	case BodyMain:
		return "main"
	case BodyFootnotes:
		return "notes"
	default:
		return "other"
	}
}

// Body contains the main narrative or auxiliary materials.
type Body struct {
	Name      string
	Lang      string
	Kind      BodyKind
	Image     *Image
	Title     *Title
	Epigraphs []Epigraph
	Sections  []Section
}

func (b *Body) Main() bool {
	return b.Kind == BodyMain
}

func (b *Body) Footnotes() bool {
	return b.Kind == BodyFootnotes
}

func (b *Body) Other() bool {
	return b.Kind != BodyMain && b.Kind != BodyFootnotes
}

// AsTitleText extracts the title text from the body, with fallback.
func (b *Body) AsTitleText(fallback string) string {
	if b.Title != nil {
		for _, item := range b.Title.Items {
			if item.Paragraph != nil {
				if title := item.Paragraph.AsPlainText(); title != "" {
					return title
				}
			}
		}
	}
	return fallback
}

// Title aggregates one or more title paragraphs or empty lines.
type Title struct {
	Lang  string
	Items []TitleItem
}

// AsTOCText extracts text from title items for TOC display.
// Priority: 1) plain text, 2) image alt attributes, 3) fallback
func (t *Title) AsTOCText(fallback string) string {
	var buf strings.Builder
	var imageAltBuf strings.Builder
	hasText := false

	for _, item := range t.Items {
		if item.Paragraph != nil {
			text := item.Paragraph.AsPlainText()
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteString(" ")
				}
				buf.WriteString(text)
				hasText = true
			}

			// Also collect image alt text as fallback
			imageAlt := item.Paragraph.AsImageAlt()
			if imageAlt != "" {
				if imageAltBuf.Len() > 0 {
					imageAltBuf.WriteString(" ")
				}
				imageAltBuf.WriteString(imageAlt)
			}
		}
	}

	if hasText {
		return strings.TrimSpace(buf.String())
	}

	// Fallback to image alt text if no plain text
	imageAltText := strings.TrimSpace(imageAltBuf.String())
	if imageAltText != "" {
		return imageAltText
	}

	return fallback
}

// TitleItem is a single paragraph or empty line within a title.
type TitleItem struct {
	Paragraph *Paragraph
	EmptyLine bool
}

// Epigraph groups flow content plus optional text-author lines.
type Epigraph struct {
	Flow        Flow
	TextAuthors []Paragraph
}

// AsPlainText extracts plain text from the epigraph including flow content and text authors.
func (e *Epigraph) AsPlainText() string {
	var buf strings.Builder

	text := e.Flow.AsPlainText()
	if text != "" {
		buf.WriteString(text)
	}

	for _, author := range e.TextAuthors {
		text := author.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// Section corresponds to sectionType in the schema.
type Section struct {
	ID         string
	Lang       string
	Title      *Title
	Epigraphs  []Epigraph
	Image      *Image
	Annotation *Flow
	Content    []FlowItem
}

// AsTitleText extracts the title text from the section with fallback.
func (s *Section) AsTitleText(fallback string) string {
	if s.Title != nil {
		text := s.Title.AsTOCText(FormatIDToTOC(s.ID))
		if text != "" {
			return text
		}
	}
	return fallback
}

// AsPlainText extracts plain text from the section including title, epigraphs, annotation, and content.
func (s *Section) AsPlainText() string {
	var buf strings.Builder

	if s.Title != nil {
		text := s.Title.AsTOCText("")
		if text != "" {
			buf.WriteString(text)
		}
	}

	for _, epi := range s.Epigraphs {
		text := epi.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	if s.Annotation != nil {
		text := s.Annotation.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	for _, item := range s.Content {
		text := item.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// Paragraph corresponds to pType in the schema.
type Paragraph struct {
	ID      string
	Lang    string
	Style   string
	Special bool
	Text    []InlineSegment
}

// AsPlainText returns the plain text content of the paragraph by extracting text from all segments.
func (p *Paragraph) AsPlainText() string {
	var buf strings.Builder
	for _, seg := range p.Text {
		buf.WriteString(seg.AsText())
	}
	return strings.TrimSpace(buf.String())
}

// AsImageAlt extracts alt text from all images in the paragraph.
func (p *Paragraph) AsImageAlt() string {
	var buf strings.Builder
	for _, seg := range p.Text {
		alt := seg.AsImageAlt()
		if alt != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(alt)
		}
	}
	return strings.TrimSpace(buf.String())
}

// AsTOCText extracts text for TOC display from the paragraph.
// Priority: 1) plain text, 2) image alt attributes, 3) fallback text
func (p *Paragraph) AsTOCText(fallback string) string {
	// Try plain text first
	text := p.AsPlainText()
	if text != "" {
		return text
	}

	// Try image alt text as fallback
	imageAlt := p.AsImageAlt()
	if imageAlt != "" {
		return imageAlt
	}
	return fallback
}

// InlineSegmentKind distinguishes different inline content types.
type InlineSegmentKind string

const (
	InlineText          InlineSegmentKind = "text"
	InlineStrong        InlineSegmentKind = "strong"
	InlineEmphasis      InlineSegmentKind = "emphasis"
	InlineNamedStyle    InlineSegmentKind = "style"
	InlineLink          InlineSegmentKind = "link"
	InlineStrikethrough InlineSegmentKind = "strikethrough"
	InlineSub           InlineSegmentKind = "sub"
	InlineSup           InlineSegmentKind = "sup"
	InlineCode          InlineSegmentKind = "code"
	InlineImageSegment  InlineSegmentKind = "image"
)

// InlineSegment stores text or styled/linked inline content.
type InlineSegment struct {
	Kind     InlineSegmentKind
	Text     string
	Lang     string
	Name     string
	Style    string
	Href     string
	LinkType string
	Children []InlineSegment
	Image    *InlineImage
}

// AsText returns the plain text content of the inline segment, recursively extracting text from children.
// Link elements are skipped completely for text extraction (e.g., for TOC text).
func (seg *InlineSegment) AsText() string {
	// Skip link elements completely for text extraction
	if seg.Kind == InlineLink {
		return ""
	}

	var buf strings.Builder
	buf.WriteString(seg.Text)
	for _, child := range seg.Children {
		buf.WriteString(child.AsText())
	}
	return buf.String()
}

// AsImageAlt recursively extracts alt text from inline images in this segment.
func (seg *InlineSegment) AsImageAlt() string {
	if seg.Kind == InlineImageSegment && seg.Image != nil && seg.Image.Alt != "" {
		return seg.Image.Alt
	}

	// Recurse into children
	var buf strings.Builder
	for _, child := range seg.Children {
		alt := child.AsImageAlt()
		if alt != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(alt)
		}
	}
	return strings.TrimSpace(buf.String())
}

// InlineImage corresponds to the <image> element embedded in inline content.
type InlineImage struct {
	Href string
	Type string
	Alt  string
}

// Image corresponds to the standalone block-level <image> element.
type Image struct {
	Href  string
	Type  string
	Alt   string
	Title string
	ID    string
}

// Poem models poemType with a title, epigraphs, stanzas, and text authors.
type Poem struct {
	ID          string
	Lang        string
	Title       *Title
	Epigraphs   []Epigraph
	Subtitles   []Paragraph
	Stanzas     []Stanza
	TextAuthors []Paragraph
	Date        *Date
}

// AsPlainText extracts plain text from the poem including title, epigraphs, subtitles, stanzas, and text authors.
func (p *Poem) AsPlainText() string {
	var buf strings.Builder

	if p.Title != nil {
		text := p.Title.AsTOCText("")
		if text != "" {
			buf.WriteString(text)
		}
	}

	for _, epi := range p.Epigraphs {
		text := epi.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	for _, subtitle := range p.Subtitles {
		text := subtitle.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	for _, stanza := range p.Stanzas {
		text := stanza.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	for _, author := range p.TextAuthors {
		text := author.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// Stanza groups verses that form a single poem stanza.
type Stanza struct {
	Lang     string
	Title    *Title
	Subtitle *Paragraph
	Verses   []Paragraph
}

// AsPlainText extracts plain text from the stanza including title, subtitle, and verses.
func (s *Stanza) AsPlainText() string {
	var buf strings.Builder

	if s.Title != nil {
		text := s.Title.AsTOCText("")
		if text != "" {
			buf.WriteString(text)
		}
	}

	if s.Subtitle != nil {
		text := s.Subtitle.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	for _, verse := range s.Verses {
		text := verse.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// Cite corresponds to citeType in the schema.
type Cite struct {
	ID          string
	Lang        string
	Items       []FlowItem
	TextAuthors []Paragraph
}

// AsPlainText extracts plain text from the cite including items and text authors.
func (c *Cite) AsPlainText() string {
	var buf strings.Builder

	for _, item := range c.Items {
		text := item.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	for _, author := range c.TextAuthors {
		text := author.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// Table corresponds to tableType in the schema.
type Table struct {
	ID    string
	Style string
	Rows  []TableRow
}

// AsPlainText extracts plain text from all table cells.
func (t *Table) AsPlainText() string {
	var buf strings.Builder

	for _, row := range t.Rows {
		text := row.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// TableRow corresponds to trType.
type TableRow struct {
	Style string
	Align string
	Cells []TableCell
}

// AsPlainText extracts plain text from all cells in the row.
func (tr *TableRow) AsPlainText() string {
	var buf strings.Builder

	for _, cell := range tr.Cells {
		text := cell.AsPlainText()
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(text)
		}
	}

	return strings.TrimSpace(buf.String())
}

// TableCell models tdType/thType entries and stores inline content.
type TableCell struct {
	Header  bool
	ID      string
	Style   string
	ColSpan int
	RowSpan int
	Align   string
	VAlign  string
	Content []InlineSegment
}

// AsPlainText extracts plain text from the cell content.
func (tc *TableCell) AsPlainText() string {
	var buf strings.Builder

	for _, seg := range tc.Content {
		buf.WriteString(seg.AsText())
	}

	return strings.TrimSpace(buf.String())
}

// BinaryObject stores embedded binary data (images) encoded in base64.
type BinaryObject struct {
	ID          string
	ContentType string
	Data        []byte
}

func FormatIDToTOC(id string) string {
	if len(id) == 0 {
		return ""
	}
	return "~ " + id + " ~"
}
