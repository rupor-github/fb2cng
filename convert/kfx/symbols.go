package kfx

import (
	"fmt"
	"strconv"
	"strings"
)

// LargestKnownSymbol is the highest symbol ID in YJ_symbols.
// This should be updated when new symbols are discovered.
// As of Kindle Previewer 3.101.0, the highest known symbol is 851.
const LargestKnownSymbol = 851

// Symbol IDs from the YJ_symbols shared symbol table.
// These correspond to the enum values from Kindle Previewer's EpubToKFXConverter.
// See docs/symdict.md for the full mapping.
const (
	// Ion system symbols (1-9 reserved by Ion)
	SymIonSymbolTable = 3 // $ion_symbol_table

	// Style/formatting properties ($10-$155)
	SymLanguage      = 10  // language
	SymFontFamily    = 11  // font_family
	SymFontStyle     = 12  // font_style
	SymFontWeight    = 13  // font_weight
	SymFontSize      = 16  // font_size
	SymTextColor     = 19  // text_color
	SymUnderline     = 23  // underline
	SymStrikethrough = 27  // strikethrough
	SymBaselineShift = 31  // baseline_shift
	SymLetterspacing = 32  // letterspacing
	SymTextAlignment = 34  // text_alignment
	SymTextIndent    = 36  // text_indent
	SymLeftIndent    = 37  // left_indent
	SymRightIndent   = 38  // right_indent
	SymSpaceBefore   = 39  // space_before
	SymSpaceAfter    = 40  // space_after
	SymLineHeight    = 42  // line_height
	SymMargin        = 46  // margin
	SymMarginTop     = 47  // margin_top
	SymMarginLeft    = 48  // margin_left
	SymMarginBottom  = 49  // margin_bottom
	SymMarginRight   = 50  // margin_right
	SymPadding       = 51  // padding
	SymWidth         = 56  // width
	SymHeight        = 57  // height
	SymTop           = 58  // top
	SymLeft          = 59  // left
	SymBottom        = 60  // bottom
	SymRight         = 61  // right
	SymFillColor     = 70  // fill_color
	SymFillOpacity   = 72  // fill_opacity
	SymBorderColor   = 83  // border_color
	SymBorderStyle   = 88  // border_style
	SymBorderWeight  = 93  // border_weight
	SymTransform     = 98  // transform
	SymColumnCount   = 112 // column_count
	SymDropcapLines  = 125 // dropcap_lines
	SymDropcapChars  = 126 // dropcap_chars
	SymHyphens       = 127 // hyphens
	SymKeepFirst     = 131 // first (keep_together)
	SymKeepLast      = 132 // last (keep_together)
	SymFloat         = 140 // float
	SymPageTemplates = 141 // page_templates
	SymStyleEvents   = 142 // style_events
	SymOffset        = 143 // offset
	SymLength        = 144 // length
	SymContent       = 145 // content
	SymContentList   = 146 // content_list
	SymTableColSpan  = 148 // table_column_span
	SymTableRowSpan  = 149 // table_row_span
	SymHeader        = 151 // header
	SymTitle         = 153 // title
	SymDescription   = 154 // description
	SymUniqueID      = 155 // id

	// Content/document structure ($156-$200)
	SymLayout        = 156 // layout
	SymStyle         = 157 // style
	SymParentStyle   = 158 // parent_style
	SymType          = 159 // type
	SymFormat        = 161 // format
	SymMIME          = 162 // mime
	SymTarget        = 163 // target
	SymExtResource   = 164 // external_resource
	SymLocation      = 165 // location
	SymReadingOrders = 169 // reading_orders
	SymSections      = 170 // sections
	SymStyleName     = 173 // style_name
	SymSectionName   = 174 // section_name
	SymResourceName  = 175 // resource_name
	SymStoryName     = 176 // story_name
	SymReadOrderName = 178 // reading_order_name
	SymLinkTo        = 179 // link_to
	SymAnchorName    = 180 // anchor_name
	SymContainsIds   = 181 // contains
	SymLocations     = 182 // locations
	SymPosition      = 183 // position
	SymPositionID    = 184 // pid
	SymElementID     = 185 // eid
	SymURI           = 186 // uri

	// Navigation, metadata ($212-$260)
	SymTOC            = 212 // toc
	SymOrientation    = 215 // orientation
	SymBindDirection  = 216 // binding_direction
	SymIssueDate      = 219 // issue_date
	SymAuthor         = 222 // author
	SymISBN           = 223 // ISBN
	SymASIN           = 224 // ASIN
	SymPublisher      = 232 // publisher
	SymCoverPage      = 233 // cover_page
	SymNavType        = 235 // nav_type
	SymLandmarks      = 236 // landmarks
	SymPageList       = 237 // page_list
	SymLandmarkType   = 238 // landmark_type
	SymNavContName    = 239 // nav_container_name
	SymNavUnitName    = 240 // nav_unit_name
	SymRepresentation = 241 // representation
	SymLabel          = 244 // label
	SymIcon           = 245 // icon
	SymTargetPosition = 246 // target_position
	SymEntries        = 247 // entries
	SymEntrySet       = 248 // entry_set
	SymCDEContentType = 251 // cde_content_type
	SymContainerList  = 252 // container_list
	SymEntityDeps     = 253 // entity_dependencies
	SymMandatoryDeps  = 254 // mandatory_dependencies
	SymOptionalDeps   = 255 // optional_dependencies
	SymInherit        = 257 // inherit
	SymMetadata       = 258 // metadata
	SymStoryline      = 259 // storyline
	SymSection        = 260 // section

	// Content types ($269-$282)
	SymText      = 269 // text
	SymContainer = 270 // container
	SymImage     = 271 // image
	SymKVG       = 272 // kvg
	SymShape     = 273 // shape
	SymPlugin    = 274 // plugin
	SymKnockout  = 275 // knockout
	SymList      = 276 // list
	SymListItem  = 277 // listitem
	SymTable     = 278 // table
	SymTableRow  = 279 // table_row
	SymSidebar   = 280 // sidebar
	SymFootnote  = 281 // footnote
	SymFigure    = 282 // figure
	SymInline    = 283 // inline

	// Format types ($284-$287)
	SymFormatPNG    = 284 // png
	SymFormatJPG    = 285 // jpg
	SymFormatGIF    = 286 // gif
	SymFormatPlugin = 287 // pobject

	// Units and values ($306-$330)
	SymUnit        = 306 // unit
	SymValue       = 307 // value
	SymUnitEm      = 308 // em
	SymUnitEx      = 309 // ex
	SymUnitRatio   = 310 // ratio
	SymUnitPercent = 314 // percent
	SymUnitCm      = 315 // cm
	SymUnitMm      = 316 // mm
	SymUnitIn      = 317 // in
	SymUnitPt      = 318 // pt
	SymUnitPx      = 319 // px
	SymCenter      = 320 // center
	SymJustify     = 321 // justify
	SymHorizontal  = 322 // horizontal
	SymVertical    = 323 // vertical
	SymFixedFit    = 324 // fixed
	SymOverflow    = 325 // overflow
	SymScaleFit    = 326 // scale_fit
	SymRadial      = 327 // radial
	SymSolid       = 328 // solid

	// More values ($348-$386)
	SymNull      = 348 // null (placeholder for root fragments)
	SymNone      = 349 // none
	SymNormal    = 350 // normal
	SymDefault   = 351 // default
	SymAlways    = 352 // always
	SymAvoid     = 353 // avoid
	SymBold      = 361 // bold
	SymSemibold  = 362 // semibold
	SymLight     = 363 // light
	SymMedium    = 364 // medium
	SymItalic    = 382 // italic
	SymAuto      = 383 // auto
	SymPortrait  = 385 // portrait
	SymLandscape = 386 // landscape

	// Navigation ($389-$394)
	SymBookNavigation = 389 // book_navigation
	SymSectionNav     = 390 // section_navigation (magazine - skip)
	SymNavContainer   = 391 // nav_container
	SymNavContainers  = 392 // nav_containers
	SymNavUnit        = 393 // nav_unit
	SymCondNavUnit    = 394 // conditional_nav_group_unit (skip)

	// Landmarks ($397-$408)
	SymTitlePage        = 397 // titlepage
	SymAcknowledgements = 398 // acknowledgements
	SymPreface          = 399 // preface
	SymFrontmatter      = 405 // frontmatter
	SymBodymatter       = 406 // bodymatter
	SymBackmatter       = 407 // backmatter

	// Container-specific ($409-$419)
	SymContainerId    = 409 // bcContId
	SymComprType      = 410 // bcComprType
	SymDRMScheme      = 411 // bcDRMScheme
	SymChunkSize      = 412 // bcChunkSize
	SymIndexTabOffset = 413 // bcIndexTabOffset
	SymIndexTabLength = 414 // bcIndexTabLength
	SymDocSymOffset   = 415 // bcDocSymbolOffset
	SymDocSymLength   = 416 // bcDocSymbolLength
	SymRawMedia       = 417 // bcRawMedia
	SymRawFont        = 418 // bcRawFont
	SymContEntityMap  = 419 // container_entity_map

	// Resource properties ($422-$425)
	SymResourceWidth  = 422 // resource_width
	SymResourceHeight = 423 // resource_height
	SymCoverImage     = 424 // cover_image
	SymPageProgDir    = 425 // page_progression_direction

	// More metadata ($464-$467)
	SymVolumeLabel = 464 // volume_label
	SymParentAsin  = 465 // parent_asin
	SymAssetId     = 466 // asset_id
	SymRevisionId  = 467 // revision_id

	// Book metadata ($490-$495)
	SymBookMetadata = 490 // book_metadata
	SymCatMetadata  = 491 // categorised_metadata
	SymKey          = 492 // key
	SymPriority     = 493 // priority
	SymRefines      = 494 // refines
	SymCategory     = 495 // category

	// Document data ($538)
	SymDocumentData = 538 // document_data

	// Location map ($550)
	SymLocationMap = 550 // location_map

	// Position maps ($264-$265)
	SymPositionMap   = 264 // position_map
	SymPositionIdMap = 265 // position_id_map

	// Anchors ($266)
	SymAnchor = 266 // anchor

	// Version info ($587-$593)
	SymMajorVersion    = 587 // major_version
	SymMinorVersion    = 588 // minor_version
	SymNamespace       = 586 // namespace
	SymVersionInfo     = 589 // version_info
	SymFeatures        = 590 // features
	SymContentFeatures = 585 // content_features
	SymFormatCapab     = 593 // format_capabilities
	SymFCapabOffset    = 594 // bcFCapabilitiesOffset
	SymFCapabLength    = 595 // bcFCapabilitiesLength

	// Auxiliary ($597-$598)
	SymAuxiliaryData = 597 // auxiliary_data
	SymKfxID         = 598 // kfx_id

	// Render mode ($601-$602)
	SymRender = 601 // render
	SymBlock  = 602 // block

	// Alt text ($584)
	SymAltText = 584 // alt_text

	// Start/End ($680-$681)
	SymStart = 680 // start
	SymEnd   = 681 // end
)

// yjSymbolNames maps symbol IDs to their string names.
// This is used for creating the YJ_symbols shared symbol table.
var yjSymbolNames = map[int]string{
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
	392: "nav_containers", 393: "nav_unit", 394: "conditional_nav_group_unit",
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
	587: "major_version", 588: "minor_version", 589: "version_info", 590: "features",
	593: "format_capabilities", 594: "bcFCapabilitiesOffset", 595: "bcFCapabilitiesLength",
	597: "auxiliary_data", 598: "kfx_id", 601: "render", 602: "block",
	680: "start", 681: "end",
}

// yjSymbolIDs maps symbol names to their IDs (reverse of yjSymbolNames).
var yjSymbolIDs map[string]int

func init() {
	yjSymbolIDs = make(map[string]int, len(yjSymbolNames))
	for id, name := range yjSymbolNames {
		yjSymbolIDs[name] = id
	}
}

// SymbolName returns the string name for a symbol ID.
// Returns "$NNN" format if the symbol is not in the known table.
func SymbolName(id int) string {
	if name, ok := yjSymbolNames[id]; ok {
		return name
	}
	return "$" + strconv.Itoa(id)
}

// SymbolID returns the ID for a symbol name.
// Returns -1 if the symbol is not in the known table.
func SymbolID(name string) int {
	// Handle $NNN format
	if strings.HasPrefix(name, "$") {
		if id, err := strconv.Atoi(name[1:]); err == nil {
			return id
		}
	}
	if id, ok := yjSymbolIDs[name]; ok {
		return id
	}
	return -1
}

// FormatSymbol returns a display string for a symbol ID: "name ($id)" or just "$id".
func FormatSymbol[T int | string](id T) string {
	switch v := any(id).(type) {
	case int:
		if name, ok := yjSymbolNames[v]; ok {
			return fmt.Sprintf("%s ($%d)", name, v)
		}
		return fmt.Sprintf("$%d", v)
	case string:
		// If it's a $NNN string, decode and format it
		if strings.HasPrefix(v, "$") {
			if num, err := strconv.Atoi(v[1:]); err == nil {
				if name, ok := yjSymbolNames[num]; ok {
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
var RAW_FRAGMENT_TYPES = map[int]bool{
	SymRawMedia: true, // $417
	SymRawFont:  true, // $418
}

// ROOT_FRAGMENT_TYPES are fragment types that use fid == ftype (singleton pattern).
var ROOT_FRAGMENT_TYPES = map[int]bool{
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

// CONTAINER_FRAGMENT_TYPES are fragment types that live in the container header,
// not as ENTY records (except $419 which is special).
var CONTAINER_FRAGMENT_TYPES = map[int]bool{
	SymContainer:   true, // $270
	SymFormatCapab: true, // $593
	// $ion_symbol_table is also a container fragment but handled separately
}

// SINGLETON_FRAGMENT_TYPES are root fragment types where only one fragment is expected per book.
var SINGLETON_FRAGMENT_TYPES = map[int]bool{
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
var REQUIRED_BOOK_FRAGMENT_TYPES = map[int]bool{
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
var ALLOWED_BOOK_FRAGMENT_TYPES = map[int]bool{
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
	SymSectionMeta   = 267 // section_metadata
	SymGradient      = 263 // gradient
	SymSectionPosMap = 609 // section_position_id_map
)
