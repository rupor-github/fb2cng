package kfx

import (
	"fmt"
	"strconv"
	"strings"
)

// KFXSymbol represents a symbol ID from the YJ_symbols shared symbol table.
// These are property/type identifiers like $16 (font_size), $157 (style), etc.
// Not to be confused with EID (element ID) which identifies content entries.
type KFXSymbol int

// LargestKnownSymbol is the highest symbol ID in YJ_symbols.
// This should be updated when new symbols are discovered.
// As of Kindle Previewer 3.101.0, the highest known symbol is 851.
const LargestKnownSymbol KFXSymbol = 851

// Symbol IDs from the YJ_symbols shared symbol table.
// These correspond to the enum values from Kindle Previewer's EpubToKFXConverter.
// See docs/symdict.md for the full mapping.
const (
	// Ion system symbols (1-9 reserved by Ion)
	SymIonSymbolTable KFXSymbol = 3 // $ion_symbol_table

	// Style/formatting properties ($10-$155)
	SymLanguage      KFXSymbol = 10  // language
	SymFontFamily    KFXSymbol = 11  // font_family
	SymFontStyle     KFXSymbol = 12  // font_style
	SymFontWeight    KFXSymbol = 13  // font_weight
	SymFontSize      KFXSymbol = 16  // font_size
	SymTextColor     KFXSymbol = 19  // text_color
	SymUnderline     KFXSymbol = 23  // underline
	SymStrikethrough KFXSymbol = 27  // strikethrough
	SymBaselineShift KFXSymbol = 31  // baseline_shift
	SymLetterspacing KFXSymbol = 32  // letterspacing
	SymTextAlignment KFXSymbol = 34  // text_alignment
	SymTextIndent    KFXSymbol = 36  // text_indent
	SymLeftIndent    KFXSymbol = 37  // left_indent
	SymRightIndent   KFXSymbol = 38  // right_indent
	SymSpaceBefore   KFXSymbol = 39  // space_before
	SymSpaceAfter    KFXSymbol = 40  // space_after
	SymLineHeight    KFXSymbol = 42  // line_height
	SymMargin        KFXSymbol = 46  // margin
	SymMarginTop     KFXSymbol = 47  // margin_top
	SymMarginLeft    KFXSymbol = 48  // margin_left
	SymMarginBottom  KFXSymbol = 49  // margin_bottom
	SymMarginRight   KFXSymbol = 50  // margin_right
	SymPadding       KFXSymbol = 51  // padding
	SymWidth         KFXSymbol = 56  // width
	SymHeight        KFXSymbol = 57  // height
	SymTop           KFXSymbol = 58  // top
	SymLeft          KFXSymbol = 59  // left
	SymBottom        KFXSymbol = 60  // bottom
	SymRight         KFXSymbol = 61  // right
	SymFillColor     KFXSymbol = 70  // fill_color
	SymFillOpacity   KFXSymbol = 72  // fill_opacity
	SymBorderColor   KFXSymbol = 83  // border_color
	SymBorderStyle   KFXSymbol = 88  // border_style
	SymBorderWeight  KFXSymbol = 93  // border_weight
	SymTransform     KFXSymbol = 98  // transform
	SymColumnCount   KFXSymbol = 112 // column_count
	SymDropcapLines  KFXSymbol = 125 // dropcap_lines
	SymDropcapChars  KFXSymbol = 126 // dropcap_chars
	SymHyphens       KFXSymbol = 127 // hyphens
	SymKeepFirst     KFXSymbol = 131 // first (keep_together)
	SymKeepLast      KFXSymbol = 132 // last (keep_together)
	SymFloat         KFXSymbol = 140 // float
	SymPageTemplates KFXSymbol = 141 // page_templates
	SymStyleEvents   KFXSymbol = 142 // style_events
	SymOffset        KFXSymbol = 143 // offset
	SymLength        KFXSymbol = 144 // length
	SymContent       KFXSymbol = 145 // content
	SymContentList   KFXSymbol = 146 // content_list
	SymTableColSpan  KFXSymbol = 148 // table_column_span
	SymTableRowSpan  KFXSymbol = 149 // table_row_span
	SymHeader        KFXSymbol = 151 // header
	SymTitle         KFXSymbol = 153 // title
	SymDescription   KFXSymbol = 154 // description
	SymUniqueID      KFXSymbol = 155 // id

	// Content/document structure ($156-$200)
	SymLayout        KFXSymbol = 156 // layout
	SymStyle         KFXSymbol = 157 // style
	SymParentStyle   KFXSymbol = 158 // parent_style
	SymType          KFXSymbol = 159 // type
	SymFormat        KFXSymbol = 161 // format
	SymMIME          KFXSymbol = 162 // mime
	SymTarget        KFXSymbol = 163 // target
	SymExtResource   KFXSymbol = 164 // external_resource
	SymLocation      KFXSymbol = 165 // location
	SymReadingOrders KFXSymbol = 169 // reading_orders
	SymSections      KFXSymbol = 170 // sections
	SymStyleName     KFXSymbol = 173 // style_name
	SymSectionName   KFXSymbol = 174 // section_name
	SymResourceName  KFXSymbol = 175 // resource_name
	SymStoryName     KFXSymbol = 176 // story_name
	SymReadOrderName KFXSymbol = 178 // reading_order_name
	SymLinkTo        KFXSymbol = 179 // link_to
	SymAnchorName    KFXSymbol = 180 // anchor_name
	SymContainsIds   KFXSymbol = 181 // contains
	SymLocations     KFXSymbol = 182 // locations
	SymPosition      KFXSymbol = 183 // position
	SymPositionID    KFXSymbol = 184 // pid
	SymElementID     KFXSymbol = 185 // eid
	SymURI           KFXSymbol = 186 // uri

	// Navigation, metadata ($212-$260)
	SymTOC            KFXSymbol = 212 // toc
	SymOrientation    KFXSymbol = 215 // orientation
	SymBindDirection  KFXSymbol = 216 // binding_direction
	SymIssueDate      KFXSymbol = 219 // issue_date
	SymAuthor         KFXSymbol = 222 // author
	SymISBN           KFXSymbol = 223 // ISBN
	SymASIN           KFXSymbol = 224 // ASIN
	SymPublisher      KFXSymbol = 232 // publisher
	SymCoverPage      KFXSymbol = 233 // cover_page
	SymNavType        KFXSymbol = 235 // nav_type
	SymLandmarks      KFXSymbol = 236 // landmarks
	SymPageList       KFXSymbol = 237 // page_list
	SymLandmarkType   KFXSymbol = 238 // landmark_type
	SymNavContName    KFXSymbol = 239 // nav_container_name
	SymNavUnitName    KFXSymbol = 240 // nav_unit_name
	SymRepresentation KFXSymbol = 241 // representation
	SymLabel          KFXSymbol = 244 // label
	SymIcon           KFXSymbol = 245 // icon
	SymTargetPosition KFXSymbol = 246 // target_position
	SymEntries        KFXSymbol = 247 // entries
	SymEntrySet       KFXSymbol = 248 // entry_set
	SymCDEContentType KFXSymbol = 251 // cde_content_type
	SymContainerList  KFXSymbol = 252 // container_list
	SymEntityDeps     KFXSymbol = 253 // entity_dependencies
	SymMandatoryDeps  KFXSymbol = 254 // mandatory_dependencies
	SymOptionalDeps   KFXSymbol = 255 // optional_dependencies
	SymInherit        KFXSymbol = 257 // inherit
	SymMetadata       KFXSymbol = 258 // metadata
	SymStoryline      KFXSymbol = 259 // storyline
	SymSection        KFXSymbol = 260 // section

	// Content types ($269-$282)
	SymText      KFXSymbol = 269 // text
	SymContainer KFXSymbol = 270 // container
	SymImage     KFXSymbol = 271 // image
	SymKVG       KFXSymbol = 272 // kvg
	SymShape     KFXSymbol = 273 // shape
	SymPlugin    KFXSymbol = 274 // plugin
	SymKnockout  KFXSymbol = 275 // knockout
	SymList      KFXSymbol = 276 // list
	SymListItem  KFXSymbol = 277 // listitem
	SymTable     KFXSymbol = 278 // table
	SymTableRow  KFXSymbol = 279 // table_row
	SymSidebar   KFXSymbol = 280 // sidebar
	SymFootnote  KFXSymbol = 281 // footnote
	SymFigure    KFXSymbol = 282 // figure
	SymInline    KFXSymbol = 283 // inline

	// Format types ($284-$287)
	SymFormatPNG    KFXSymbol = 284 // png
	SymFormatJPG    KFXSymbol = 285 // jpg
	SymFormatGIF    KFXSymbol = 286 // gif
	SymFormatPlugin KFXSymbol = 287 // pobject

	// Units and values ($306-$330)
	SymUnit        KFXSymbol = 306 // unit
	SymValue       KFXSymbol = 307 // value
	SymUnitEm      KFXSymbol = 308 // em
	SymUnitEx      KFXSymbol = 309 // ex
	SymUnitRatio   KFXSymbol = 310 // ratio
	SymUnitPercent KFXSymbol = 314 // percent
	SymUnitCm      KFXSymbol = 315 // cm
	SymUnitMm      KFXSymbol = 316 // mm
	SymUnitIn      KFXSymbol = 317 // in
	SymUnitPt      KFXSymbol = 318 // pt
	SymUnitPx      KFXSymbol = 319 // px
	SymCenter      KFXSymbol = 320 // center
	SymJustify     KFXSymbol = 321 // justify
	SymHorizontal  KFXSymbol = 322 // horizontal
	SymVertical    KFXSymbol = 323 // vertical
	SymFixedFit    KFXSymbol = 324 // fixed
	SymOverflow    KFXSymbol = 325 // overflow
	SymScaleFit    KFXSymbol = 326 // scale_fit
	SymRadial      KFXSymbol = 327 // radial
	SymSolid       KFXSymbol = 328 // solid

	// More values ($348-$386)
	SymNull      KFXSymbol = 348 // null (placeholder for root fragments)
	SymNone      KFXSymbol = 349 // none
	SymNormal    KFXSymbol = 350 // normal
	SymDefault   KFXSymbol = 351 // default
	SymAlways    KFXSymbol = 352 // always
	SymAvoid     KFXSymbol = 353 // avoid
	SymBold      KFXSymbol = 361 // bold
	SymSemibold  KFXSymbol = 362 // semibold
	SymLight     KFXSymbol = 363 // light
	SymMedium    KFXSymbol = 364 // medium
	SymItalic    KFXSymbol = 382 // italic
	SymAuto      KFXSymbol = 383 // auto
	SymPortrait  KFXSymbol = 385 // portrait
	SymLandscape KFXSymbol = 386 // landscape

	// Navigation ($389-$395)
	SymBookNavigation KFXSymbol = 389 // book_navigation
	SymSectionNav     KFXSymbol = 390 // section_navigation (magazine - skip)
	SymNavContainer   KFXSymbol = 391 // nav_container
	SymNavContainers  KFXSymbol = 392 // nav_containers
	SymNavUnit        KFXSymbol = 393 // nav_unit
	SymCondNavUnit    KFXSymbol = 394 // conditional_nav_group_unit (skip)
	SymResourcePath   KFXSymbol = 395 // resource_path

	// Landmarks ($397-$408)
	SymTitlePage        KFXSymbol = 397 // titlepage
	SymAcknowledgements KFXSymbol = 398 // acknowledgements
	SymPreface          KFXSymbol = 399 // preface
	SymFrontmatter      KFXSymbol = 405 // frontmatter
	SymBodymatter       KFXSymbol = 406 // bodymatter
	SymBackmatter       KFXSymbol = 407 // backmatter

	// Container-specific ($409-$419)
	SymContainerId    KFXSymbol = 409 // bcContId
	SymComprType      KFXSymbol = 410 // bcComprType
	SymDRMScheme      KFXSymbol = 411 // bcDRMScheme
	SymChunkSize      KFXSymbol = 412 // bcChunkSize
	SymIndexTabOffset KFXSymbol = 413 // bcIndexTabOffset
	SymIndexTabLength KFXSymbol = 414 // bcIndexTabLength
	SymDocSymOffset   KFXSymbol = 415 // bcDocSymbolOffset
	SymDocSymLength   KFXSymbol = 416 // bcDocSymbolLength
	SymRawMedia       KFXSymbol = 417 // bcRawMedia
	SymRawFont        KFXSymbol = 418 // bcRawFont
	SymContEntityMap  KFXSymbol = 419 // container_entity_map

	// Resource properties ($422-$425)
	SymResourceWidth  KFXSymbol = 422 // resource_width
	SymResourceHeight KFXSymbol = 423 // resource_height
	SymCoverImage     KFXSymbol = 424 // cover_image
	SymPageProgDir    KFXSymbol = 425 // page_progression_direction

	// More metadata ($464-$467)
	SymVolumeLabel KFXSymbol = 464 // volume_label
	SymParentAsin  KFXSymbol = 465 // parent_asin
	SymAssetId     KFXSymbol = 466 // asset_id
	SymRevisionId  KFXSymbol = 467 // revision_id

	// Book metadata ($490-$495)
	SymBookMetadata KFXSymbol = 490 // book_metadata
	SymCatMetadata  KFXSymbol = 491 // categorised_metadata
	SymKey          KFXSymbol = 492 // key
	SymPriority     KFXSymbol = 493 // priority
	SymRefines      KFXSymbol = 494 // refines
	SymCategory     KFXSymbol = 495 // category

	// Document data ($538)
	SymDocumentData KFXSymbol = 538 // document_data

	// Location map ($550)
	SymLocationMap KFXSymbol = 550 // location_map

	// Position maps ($264-$265)
	SymPositionMap   KFXSymbol = 264 // position_map
	SymPositionIdMap KFXSymbol = 265 // position_id_map

	// Anchors ($266)
	SymAnchor KFXSymbol = 266 // anchor

	// Version info ($587-$593)
	SymMajorVersion    KFXSymbol = 587 // major_version
	SymMinorVersion    KFXSymbol = 588 // minor_version
	SymNamespace       KFXSymbol = 586 // namespace
	SymVersionInfo     KFXSymbol = 589 // version_info
	SymFeatures        KFXSymbol = 590 // features
	SymContentFeatures KFXSymbol = 585 // content_features
	SymFormatCapab     KFXSymbol = 593 // format_capabilities
	SymFCapabOffset    KFXSymbol = 594 // bcFCapabilitiesOffset
	SymFCapabLength    KFXSymbol = 595 // bcFCapabilitiesLength

	// Auxiliary ($597-$598)
	SymAuxiliaryData KFXSymbol = 597 // auxiliary_data
	SymKfxID         KFXSymbol = 598 // kfx_id

	// Render mode ($601-$602)
	SymRender KFXSymbol = 601 // render
	SymBlock  KFXSymbol = 602 // block

	// Alt text ($584)
	SymAltText KFXSymbol = 584 // alt_text

	// Start/End ($680-$681)
	SymStart KFXSymbol = 680 // start
	SymEnd   KFXSymbol = 681 // end
)

// yjSymbolNames maps symbol IDs to their string names.
// This is used for creating the YJ_symbols shared symbol table.
var yjSymbolNames = map[KFXSymbol]string{
	10: "language", 11: "font_family", 12: "font_style", 13: "font_weight",
	16: "font_size", 19: "text_color", 23: "underline", 27: "strikethrough",
	31: "baseline_shift", 32: "letterspacing", 34: "text_alignment",
	36: "text_indent", 37: "left_indent", 38: "right_indent",
	39: "space_before", 40: "space_after", 42: "line_height",
	46: "margin", 47: "margin_top", 48: "margin_left", 49: "margin_bottom", 50: "margin_right",
	51: "padding", 56: "width", 57: "height", 58: "top", 59: "left", 60: "bottom", 61: "right",
	70: "fill_color", 72: "fill_opacity", 83: "border_color", 88: "border_style", 93: "border_weight",
	98: "transform", 112: "column_count", 125: "dropcap_lines", 126: "dropcap_chars",
	127: "hyphens", 131: "first", 132: "last", 140: "float", 141: "page_templates",
	142: "style_events", 143: "offset", 144: "length", 145: "content", 146: "content_list",
	148: "table_column_span", 149: "table_row_span", 151: "header",
	153: "title", 154: "description", 155: "id",
	156: "layout", 157: "style", 158: "parent_style", 159: "type",
	161: "format", 162: "mime", 163: "target", 164: "external_resource", 165: "location",
	169: "reading_orders", 170: "sections", 173: "style_name", 174: "section_name",
	175: "resource_name", 176: "story_name", 178: "reading_order_name",
	179: "link_to", 180: "anchor_name", 181: "contains", 182: "locations",
	183: "position", 184: "pid", 185: "eid", 186: "uri",
	212: "toc", 215: "orientation", 216: "binding_direction", 219: "issue_date",
	222: "author", 223: "ISBN", 224: "ASIN", 232: "publisher", 233: "cover_page",
	235: "nav_type", 236: "landmarks", 237: "page_list", 238: "landmark_type",
	239: "nav_container_name", 240: "nav_unit_name", 241: "representation",
	244: "label", 245: "icon", 246: "target_position", 247: "entries", 248: "entry_set",
	251: "cde_content_type", 252: "container_list", 253: "entity_dependencies",
	254: "mandatory_dependencies", 255: "optional_dependencies",
	257: "inherit", 258: "metadata", 259: "storyline", 260: "section",
	264: "position_map", 265: "position_id_map", 266: "anchor",
	269: "text", 270: "container", 271: "image", 272: "kvg", 273: "shape",
	274: "plugin", 275: "knockout", 276: "list", 277: "listitem",
	278: "table", 279: "table_row", 280: "sidebar", 281: "footnote", 282: "figure", 283: "inline",
	284: "png", 285: "jpg", 286: "gif", 287: "pobject",
	306: "unit", 307: "value", 308: "em", 309: "ex", 310: "ratio", 314: "percent",
	315: "cm", 316: "mm", 317: "in", 318: "pt", 319: "px",
	320: "center", 321: "justify", 322: "horizontal", 323: "vertical",
	324: "fixed", 325: "overflow", 326: "scale_fit", 327: "radial", 328: "solid",
	348: "null", 349: "none", 350: "normal", 351: "default", 352: "always", 353: "avoid",
	361: "bold", 362: "semibold", 363: "light", 364: "medium", 382: "italic", 383: "auto", 385: "portrait", 386: "landscape",
	389: "book_navigation", 390: "section_navigation", 391: "nav_container",
	392: "nav_containers", 393: "nav_unit", 394: "conditional_nav_group_unit", 395: "resource_path",
	397: "titlepage", 398: "acknowledgements", 399: "preface",
	405: "frontmatter", 406: "bodymatter", 407: "backmatter",
	409: "bcContId", 410: "bcComprType", 411: "bcDRMScheme", 412: "bcChunkSize",
	413: "bcIndexTabOffset", 414: "bcIndexTabLength",
	415: "bcDocSymbolOffset", 416: "bcDocSymbolLength",
	417: "bcRawMedia", 418: "bcRawFont", 419: "container_entity_map",
	422: "resource_width", 423: "resource_height", 424: "cover_image", 425: "page_progression_direction",
	464: "volume_label", 465: "parent_asin", 466: "asset_id", 467: "revision_id",
	490: "book_metadata", 491: "categorised_metadata", 492: "key", 493: "priority",
	494: "refines", 495: "category",
	538: "document_data", 550: "location_map",
	584: "alt_text", 585: "content_features", 586: "namespace",
	587: "major_version", 588: "minor_version", 589: "version_info", 590: "features",
	593: "format_capabilities", 594: "bcFCapabilitiesOffset", 595: "bcFCapabilitiesLength",
	597: "auxiliary_data", 598: "kfx_id", 601: "render", 602: "block",
	680: "start", 681: "end",
}

// yjSymbolIDs maps symbol names to their IDs (reverse of yjSymbolNames).
var yjSymbolIDs map[string]KFXSymbol

func init() {
	yjSymbolIDs = make(map[string]KFXSymbol, len(yjSymbolNames))
	for id, name := range yjSymbolNames {
		yjSymbolIDs[name] = id
	}
}

// Name returns the string name for a symbol ID.
// Returns "$NNN" format if the symbol is not in the known table.
func (s KFXSymbol) Name() string {
	if name, ok := yjSymbolNames[s]; ok {
		return name
	}
	return "$" + strconv.Itoa(int(s))
}

// String returns a display string for a symbol ID: "name ($id)" or just "$id".
// Implements fmt.Stringer interface.
func (s KFXSymbol) String() string {
	if name, ok := yjSymbolNames[s]; ok {
		return fmt.Sprintf("%s ($%d)", name, s)
	}
	return fmt.Sprintf("$%d", s)
}

// Value converts KFXSymbol to SymbolValue for use in struct values.
func (s KFXSymbol) Value() SymbolValue {
	return SymbolValue(s)
}

// SymbolID returns the ID for a symbol name.
// Returns -1 if the symbol is not in the known table.
func SymbolID(name string) KFXSymbol {
	// Handle $NNN format
	if strings.HasPrefix(name, "$") {
		if id, err := strconv.Atoi(name[1:]); err == nil {
			return KFXSymbol(id)
		}
	}
	if id, ok := yjSymbolIDs[name]; ok {
		return id
	}
	return -1
}

// FormatSymbol returns a display string for a symbol ID: "name ($id)" or just "$id".
// Deprecated: Use KFXSymbol.String() method instead for KFXSymbol values.
func FormatSymbol[T KFXSymbol | string](id T) string {
	switch v := any(id).(type) {
	case KFXSymbol:
		return v.String()
	case string:
		// If it's a $NNN string, decode and format it
		if strings.HasPrefix(v, "$") {
			if num, err := strconv.Atoi(v[1:]); err == nil {
				if name, ok := yjSymbolNames[KFXSymbol(num)]; ok {
					return fmt.Sprintf("%s ($%d)", name, num)
				}
				return v
			}
		}
		// Return the string as-is (it's a local symbol name)
		return v
	default:
		return fmt.Sprintf("%v", id)
	}
}

// RAW_FRAGMENT_TYPES are fragment types whose payloads are stored as raw bytes.
var RAW_FRAGMENT_TYPES = map[KFXSymbol]bool{
	SymRawMedia: true, // $417
	SymRawFont:  true, // $418
}

// ROOT_FRAGMENT_TYPES are fragment types that use fid == ftype (singleton pattern).
var ROOT_FRAGMENT_TYPES = map[KFXSymbol]bool{
	SymMetadata:        true, // $258
	SymPositionMap:     true, // $264
	SymPositionIdMap:   true, // $265
	SymContainer:       true, // $270
	SymBookNavigation:  true, // $389
	SymResourcePath:    true, // $395
	SymContEntityMap:   true, // $419
	SymBookMetadata:    true, // $490
	SymDocumentData:    true, // $538
	SymLocationMap:     true, // $550
	SymContentFeatures: true, // $585
	SymFormatCapab:     true, // $593
}

// CONTAINER_FRAGMENT_TYPES are fragment types that live in the container header,
// not as ENTY records (except $419 which is special).
var CONTAINER_FRAGMENT_TYPES = map[KFXSymbol]bool{
	SymContainer:   true, // $270
	SymFormatCapab: true, // $593
	// $ion_symbol_table is also a container fragment but handled separately
}

// SINGLETON_FRAGMENT_TYPES are root fragment types where only one fragment is expected per book.
var SINGLETON_FRAGMENT_TYPES = map[KFXSymbol]bool{
	SymMetadata:        true, // $258
	SymPositionMap:     true, // $264
	SymPositionIdMap:   true, // $265
	SymContainer:       true, // $270
	SymBookNavigation:  true, // $389
	SymContEntityMap:   true, // $419
	SymBookMetadata:    true, // $490
	SymDocumentData:    true, // $538
	SymLocationMap:     true, // $550
	SymContentFeatures: true, // $585
	SymFormatCapab:     true, // $593
}

// REQUIRED_BOOK_FRAGMENT_TYPES are fragment types that must be present for a normal book.
// Note: Some types may be conditionally required based on book type (dictionary, KPF, etc.).
var REQUIRED_BOOK_FRAGMENT_TYPES = map[KFXSymbol]bool{
	SymMetadata:      true, // $258 - basic metadata (or $490)
	SymStoryline:     true, // $259 - root content container
	SymSection:       true, // $260 - at least one section
	SymPositionMap:   true, // $264 - EID to section mapping
	SymPositionIdMap: true, // $265 - PID to EID/offset mapping
	SymContEntityMap: true, // $419 - container entity map
	SymDocumentData:  true, // $538 - reading orders (KFX v2+)
	SymLocationMap:   true, // $550 - location map
}

// ALLOWED_BOOK_FRAGMENT_TYPES are optional, known fragment types for books.
var ALLOWED_BOOK_FRAGMENT_TYPES = map[KFXSymbol]bool{
	SymStyle:           true, // $157 - styles
	SymExtResource:     true, // $164 - external resource descriptors
	SymGradient:        true, // $263 - gradients
	SymAnchor:          true, // $266 - anchors for navigation
	SymSectionMeta:     true, // $267 - section metadata (magazine)
	SymText:            true, // $269 - text content (inline)
	SymRawMedia:        true, // $417 - raw media bytes
	SymRawFont:         true, // $418 - raw font bytes
	SymBookMetadata:    true, // $490 - categorised metadata
	SymBookNavigation:  true, // $389 - navigation per reading order
	SymSectionNav:      true, // $390 - section navigation (magazine)
	SymNavContainer:    true, // $391 - navigation container
	SymNavUnit:         true, // $393 - navigation unit
	SymFormatCapab:     true, // $593 - format capabilities
	SymAuxiliaryData:   true, // $597 - auxiliary data
	SymContentFeatures: true, // $585 - content_features
	SymSectionPosMap:   true, // $609 - section position ID map
}

// Additional symbols for fragment types not in the main constants
const (
	SymSectionMeta   KFXSymbol = 267 // section_metadata
	SymGradient      KFXSymbol = 263 // gradient
	SymSectionPosMap KFXSymbol = 609 // section_position_id_map
)
