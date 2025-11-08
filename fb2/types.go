package fb2

import (
	"time"

	"golang.org/x/text/language"
)

// Type definitions for FictionBook2 format structures.

// FictionBook mirrors the root element defined in FictionBook.xsd.
type FictionBook struct {
	Stylesheets []Stylesheet
	Description Description
	Bodies      []Body
	Binaries    []BinaryObject
}

type FootnoteRef struct {
	BodyIdx    int
	SectionIdx int
}

type FootnoteRefs map[string]FootnoteRef

type BookImage struct {
	MimeType string
	Data     []byte
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

// Title aggregates one or more title paragraphs or empty lines.
type Title struct {
	Lang  string
	Items []TitleItem
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

// Paragraph corresponds to pType in the schema.
type Paragraph struct {
	ID    string
	Lang  string
	Style string
	Text  []InlineSegment
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

// Stanza groups verses that form a single poem stanza.
type Stanza struct {
	Lang     string
	Title    *Title
	Subtitle *Paragraph
	Verses   []Paragraph
}

// Cite corresponds to citeType in the schema.
type Cite struct {
	ID          string
	Lang        string
	Items       []FlowItem
	TextAuthors []Paragraph
}

// Table corresponds to tableType in the schema.
type Table struct {
	ID    string
	Style string
	Rows  []TableRow
}

// TableRow corresponds to trType.
type TableRow struct {
	Align string
	Cells []TableCell
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

// BinaryObject stores embedded binary data (images) encoded in base64.
type BinaryObject struct {
	ID          string
	ContentType string
	Data        []byte
}
