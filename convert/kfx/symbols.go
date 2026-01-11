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
	SymLanguage            KFXSymbol = 10  // language
	SymFontFamily          KFXSymbol = 11  // font_family
	SymFontStyle           KFXSymbol = 12  // font_style
	SymFontWeight          KFXSymbol = 13  // font_weight
	SymFontSize            KFXSymbol = 16  // font_size
	SymTextColor           KFXSymbol = 19  // text_color
	SymUnderline           KFXSymbol = 23  // underline
	SymStrikethrough       KFXSymbol = 27  // strikethrough
	SymBaselineShift       KFXSymbol = 31  // baseline_shift
	SymLetterspacing       KFXSymbol = 32  // letterspacing
	SymTextAlignment       KFXSymbol = 34  // text_alignment
	SymTextIndent          KFXSymbol = 36  // text_indent
	SymLeftIndent          KFXSymbol = 37  // left_indent
	SymRightIndent         KFXSymbol = 38  // right_indent
	SymSpaceBefore         KFXSymbol = 39  // space_before
	SymSpaceAfter          KFXSymbol = 40  // space_after
	SymLineHeight          KFXSymbol = 42  // line_height
	SymBaselineStyle       KFXSymbol = 44  // baseline_style
	SymMargin              KFXSymbol = 46  // margin
	SymMarginTop           KFXSymbol = 47  // margin_top
	SymMarginLeft          KFXSymbol = 48  // margin_left
	SymMarginBottom        KFXSymbol = 49  // margin_bottom
	SymMarginRight         KFXSymbol = 50  // margin_right
	SymPadding             KFXSymbol = 51  // padding
	SymPaddingTop          KFXSymbol = 52  // padding_top
	SymPaddingLeft         KFXSymbol = 53  // padding_left
	SymPaddingBottom       KFXSymbol = 54  // padding_bottom
	SymPaddingRight        KFXSymbol = 55  // padding_right
	SymWidth               KFXSymbol = 56  // width
	SymHeight              KFXSymbol = 57  // height
	SymTop                 KFXSymbol = 58  // top
	SymLeft                KFXSymbol = 59  // left
	SymBottom              KFXSymbol = 60  // bottom
	SymRight               KFXSymbol = 61  // right
	SymContainerWidth      KFXSymbol = 66  // container width (for page templates)
	SymContainerHeight     KFXSymbol = 67  // container height (for page templates)
	SymFillColor           KFXSymbol = 70  // fill_color
	SymFillOpacity         KFXSymbol = 72  // fill_opacity
	SymBorderColor         KFXSymbol = 83  // border_color
	SymBorderStyle         KFXSymbol = 88  // border_style
	SymBorderWeight        KFXSymbol = 93  // border_weight
	SymTransform           KFXSymbol = 98  // transform
	SymListStyle           KFXSymbol = 100 // list_style
	SymColumnCount         KFXSymbol = 112 // column_count
	SymDropcapLines        KFXSymbol = 125 // dropcap_lines
	SymDropcapChars        KFXSymbol = 126 // dropcap_chars
	SymHyphens             KFXSymbol = 127 // hyphens
	SymKeepFirst           KFXSymbol = 131 // first (keep_together)
	SymKeepLast            KFXSymbol = 132 // last (keep_together)
	SymBreakInside         KFXSymbol = 135 // break_inside
	SymFloat               KFXSymbol = 140 // float
	SymPageTemplates       KFXSymbol = 141 // page_templates
	SymStyleEvents         KFXSymbol = 142 // style_events
	SymOffset              KFXSymbol = 143 // offset
	SymLength              KFXSymbol = 144 // length
	SymContent             KFXSymbol = 145 // content
	SymContentList         KFXSymbol = 146 // content_list
	SymSizingBounds        KFXSymbol = 546 // sizing_bounds
	SymContentBounds       KFXSymbol = 377 // content_bounds (value for sizing_bounds)
	SymBoxAlign            KFXSymbol = 580 // box_align
	SymYjDisplay           KFXSymbol = 616 // yj.display
	SymYjNote              KFXSymbol = 617 // yj.note
	SymTreatAsTitle        KFXSymbol = 760 // treat_as_title (layout hint value)
	SymLayoutHints         KFXSymbol = 761 // layout_hints
	SymYjBreakAfter        KFXSymbol = 788 // yj_break_after
	SymYjBreakBefore       KFXSymbol = 789 // yj_break_before
	SymYjHeadingLevel      KFXSymbol = 790 // yj.semantics.heading_level
	SymTableColSpan        KFXSymbol = 148 // table_column_span
	SymTableRowSpan        KFXSymbol = 149 // table_row_span
	SymTableBorderCollapse KFXSymbol = 150 // table_border_collapse
	SymHeader              KFXSymbol = 151 // header
	SymTitle               KFXSymbol = 153 // title
	SymDescription         KFXSymbol = 154 // description
	SymUniqueID            KFXSymbol = 155 // id

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
	SymTableBody KFXSymbol = 454 // body (table body section)
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
	SymUnit        KFXSymbol = 306       // unit
	SymValue       KFXSymbol = 307       // value
	SymUnitEm      KFXSymbol = 308       // em
	SymUnitEx      KFXSymbol = 309       // ex
	SymUnitLh      KFXSymbol = 310       // lh (line-height unit)
	SymUnitRatio             = SymUnitLh // alias for unitless ratio values
	SymUnitPercent KFXSymbol = 314       // percent
	SymUnitCm      KFXSymbol = 315       // cm
	SymUnitMm      KFXSymbol = 316       // mm
	SymUnitIn      KFXSymbol = 317       // in
	SymUnitPt      KFXSymbol = 318       // pt
	SymUnitPx      KFXSymbol = 319       // px
	SymCenter      KFXSymbol = 320       // center
	SymJustify     KFXSymbol = 321       // justify
	SymHorizontal  KFXSymbol = 322       // horizontal
	SymVertical    KFXSymbol = 323       // vertical
	SymFixedFit    KFXSymbol = 324       // fixed
	SymOverflow    KFXSymbol = 325       // overflow
	SymScaleFit    KFXSymbol = 326       // scale_fit
	SymRadial      KFXSymbol = 327       // radial
	SymSolid       KFXSymbol = 328       // solid
	SymDashed      KFXSymbol = 330       // dashed
	SymDotted      KFXSymbol = 331       // dotted

	// List style values ($340-$347)
	SymListStyleDisc   KFXSymbol = 340 // disc (bullet)
	SymListStyleSquare KFXSymbol = 341 // square
	SymListStyleCircle KFXSymbol = 342 // circle
	SymListStyleNumber KFXSymbol = 343 // numeric

	// More units ($505-$507)
	SymUnitRem KFXSymbol = 505 // rem (root em)

	// More values ($348-$386)
	SymNull        KFXSymbol = 348 // null (placeholder for root fragments)
	SymNone        KFXSymbol = 349 // none
	SymNormal      KFXSymbol = 350 // normal
	SymDefault     KFXSymbol = 351 // default
	SymAlways      KFXSymbol = 352 // always
	SymAvoid       KFXSymbol = 353 // avoid
	SymBold        KFXSymbol = 361 // bold
	SymSemibold    KFXSymbol = 362 // semibold
	SymLight       KFXSymbol = 363 // light
	SymMedium      KFXSymbol = 364 // medium
	SymSuperscript KFXSymbol = 370 // superscript
	SymSubscript   KFXSymbol = 371 // subscript
	SymItalic      KFXSymbol = 382 // italic
	SymAuto        KFXSymbol = 383 // auto
	SymPortrait    KFXSymbol = 385 // portrait
	SymLandscape   KFXSymbol = 386 // landscape

	// Navigation ($389-$395)
	SymBookNavigation KFXSymbol = 389 // book_navigation
	SymSectionNav     KFXSymbol = 390 // section_navigation (magazine - skip)
	SymNavContainer   KFXSymbol = 391 // nav_container
	SymNavContainers  KFXSymbol = 392 // nav_containers
	SymNavUnit        KFXSymbol = 393 // nav_unit
	SymCondNavUnit    KFXSymbol = 394 // conditional_nav_group_unit (skip)
	SymResourcePath   KFXSymbol = 395 // resource_path
	SymSRL            KFXSymbol = 396 // srl (start reading location)

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
// This is the complete YJ_symbols shared symbol table from Kindle Previewer 3.
// Derived from: EpubToKFXConverter-4.0.jar, enum class com.amazon.kaf.c/B.class
var yjSymbolNames = map[KFXSymbol]string{
	// Style/formatting properties ($10-$99)
	10: "language", 11: "font_family", 12: "font_style", 13: "font_weight", 14: "font_variant",
	15: "font_stretch", 16: "font_size", 17: "font_scale", 18: "ot_features", 19: "text_color",
	20: "text_opacity", 21: "text_background_color", 22: "text_background_opacity",
	23: "underline", 24: "underline_color", 25: "underline_opacity", 26: "underline_weight",
	27: "strikethrough", 28: "strikethrough_color", 29: "strikethrough_opacity",
	30: "strikethrough_weight", 31: "baseline_shift", 32: "letterspacing", 33: "wordspacing",
	34: "text_alignment", 35: "text_alignment_last", 36: "text_indent", 37: "left_indent",
	38: "right_indent", 39: "space_before", 40: "space_after", 41: "text_transform",
	42: "line_height", 43: "line_height_fit", 44: "baseline_style", 45: "nobreak", 46: "margin",
	47: "margin_top", 48: "margin_left", 49: "margin_bottom", 50: "margin_right", 51: "padding",
	52: "padding_top", 53: "padding_left", 54: "padding_bottom", 55: "padding_right", 56: "width",
	57: "height", 58: "top", 59: "left", 60: "bottom", 61: "right", 62: "min_height",
	63: "min_width", 64: "max_height", 65: "max_width", 66: "fixed_width", 67: "fixed_height",
	68: "visibility", 69: "ignore", 70: "fill_color", 71: "fill_gradient", 72: "fill_opacity",
	73: "fill_bounds", 74: "fill_rule", 75: "stroke_color", 76: "stroke_width",
	77: "stroke_linecap", 78: "border_opacity", 79: "border_opacity_top",
	80: "border_opacity_left", 81: "border_opacity_bottom", 82: "border_opacity_right",
	83: "border_color", 84: "border_color_top", 85: "border_color_left", 86: "border_color_bottom",
	87: "border_color_right", 88: "border_style", 89: "border_style_top", 90: "border_style_left",
	91: "border_style_bottom", 92: "border_style_right", 93: "border_weight",
	94: "border_weight_top", 95: "border_weight_left", 96: "border_weight_bottom",
	97: "border_weight_right", 98: "transform", 99: "draw_spanning_borders",

	// More formatting properties ($100-$155)
	100: "list_style", 101: "list_indent_style", 102: "list_indent", 103: "list_replacer",
	104: "list_start_offset", 105: "outline_color", 106: "outline_offset", 107: "outline_style",
	108: "outline_weight", 109: "gradient_type", 110: "gradient_stops", 111: "gradient_stop",
	112: "column_count", 113: "column_gap", 114: "column_min_width", 115: "column_rule_style",
	116: "column_rule_color", 117: "column_rule_weight", 118: "column_span", 119: "column_balance",
	120: "footnote_line_style", 121: "footnote_line_color", 122: "footnote_line_weight",
	123: "footnote_line_length", 124: "footnote_spacing", 125: "dropcap_lines",
	126: "dropcap_chars", 127: "hyphens", 128: "min_hyphen_word_length", 129: "min_chars_per_line",
	130: "keep_together", 131: "first", 132: "last", 133: "break_after", 134: "break_before",
	135: "break_inside", 136: "max_auto_grow", 137: "min_auto_shrink", 138: "scale_with_image",
	139: "wrap_rule", 140: "float", 141: "page_templates", 142: "style_events", 143: "offset",
	144: "length", 145: "content", 146: "content_list", 147: "knockout_region",
	148: "table_column_span", 149: "table_row_span", 150: "table_border_collapse", 151: "header",
	152: "column_format", 153: "title", 154: "description", 155: "id",

	// Content/document structure ($156-$200)
	156: "layout", 157: "style", 158: "parent_style", 159: "type", 160: "embed", 161: "format",
	162: "mime", 163: "target", 164: "external_resource", 165: "location", 166: "search_path",
	167: "referred_resources", 168: "manifest", 169: "reading_orders", 170: "sections",
	171: "condition", 172: "conditional_styling", 173: "style_name", 174: "section_name",
	175: "resource_name", 176: "story_name", 177: "gradient_name", 178: "reading_order_name",
	179: "link_to", 180: "anchor_name", 181: "contains", 182: "locations", 183: "position",
	184: "pid", 185: "eid", 186: "uri", 187: "link_confirm", 188: "link_use_external_app",
	189: "up_image", 190: "down_image", 191: "paragraph_mark", 192: "direction",
	193: "PRIVATE_parent_image_scale", 194: "PRIVATE_view_width", 195: "PRIVATE_view_height",
	196: "PRIVATE_is_storyline_content", 197: "PRIVATE_paper_color", 198: "PRIVATE_ink_color",
	199: "section_title", 200: "section_kicker",

	// Navigation, metadata ($201-$268)
	201: "section_description", 202: "section_author", 203: "section_tags",
	204: "section_date_created", 205: "is_advertisement", 206: "smooth_scrolling",
	207: "hide_from_toc", 208: "section_layout", 209: "has_audio", 210: "has_video",
	211: "has_slideshow", 212: "toc", 213: "scrubbers", 214: "thumbnails", 215: "orientation",
	216: "binding_direction", 217: "support_portrait", 218: "support_landscape", 219: "issue_date",
	220: "binding_direction_left", 221: "binding_direction_right", 222: "author", 223: "ISBN",
	224: "ASIN", 225: "is_TTS_enabled", 226: "date_created", 227: "ISBN-10", 228: "ISBN-13",
	229: "MHID", 230: "target_WideDimension", 231: "target_NarrowDimension", 232: "publisher",
	233: "cover_page", 234: "illustrator", 235: "nav_type", 236: "landmarks", 237: "page_list",
	238: "landmark_type", 239: "nav_container_name", 240: "nav_unit_name", 241: "representation",
	242: "designation", 243: "enumeration", 244: "label", 245: "icon", 246: "target_position",
	247: "entries", 248: "entry_set", 249: "path", 250: "shape_list", 251: "cde_content_type",
	252: "container_list", 253: "entity_dependencies", 254: "mandatory_dependencies",
	255: "optional_dependencies", 256: "AmazonDigitalBook", 257: "inherit", 258: "metadata",
	259: "storyline", 260: "section", 261: "style_group", 262: "font", 263: "gradient",
	264: "position_map", 265: "position_id_map", 266: "anchor", 267: "section_metadata",
	268: "hyphen_dictionary",

	// Content types and formats ($269-$305)
	269: "text", 270: "container", 271: "image", 272: "kvg", 273: "shape", 274: "plugin",
	275: "knockout", 276: "list", 277: "listitem", 278: "table", 279: "table_row", 280: "sidebar",
	281: "footnote", 282: "figure", 283: "inline", 284: "png", 285: "jpg", 286: "gif",
	287: "pobject", 288: "localPage", 289: "hasContent", 290: "paragraphMark", 291: "or",
	292: "and", 293: "not", 294: "==", 295: "!=", 296: ">", 297: ">=", 298: "<", 299: "<=",
	300: "hasColor", 301: "hasVideo", 302: "screenPixelWidth", 303: "screenPixelHeight",
	304: "screenActualWidth", 305: "screenActualHeight",

	// Units and values ($306-$347)
	306: "unit", 307: "value", 308: "em", 309: "ex", 310: "lh", 311: "vw", 312: "vh",
	313: "vmin", 314: "percent", 315: "cm", 316: "mm", 317: "in", 318: "pt", 319: "px",
	320: "center", 321: "justify", 322: "horizontal", 323: "vertical", 324: "fixed",
	325: "overflow", 326: "scale_fit", 327: "radial", 328: "solid", 329: "double", 330: "dashed",
	331: "dotted", 332: "thick_thin", 333: "thin_thick", 334: "groove", 335: "ridge", 336: "inset",
	337: "outset", 338: "non_zero", 339: "even_odd", 340: "disc", 341: "square", 342: "circle",
	343: "numeric", 344: "roman_lower", 345: "roman_upper", 346: "alpha_lower", 347: "alpha_upper",

	// More values ($348-$388)
	348: "null", 349: "none", 350: "normal", 351: "default", 352: "always", 353: "avoid",
	354: "column", 355: "thin", 356: "ultra_light", 357: "light", 358: "book", 359: "medium",
	360: "semi_bold", 361: "bold", 362: "ultra_bold", 363: "heavy", 364: "ultra_heavy",
	365: "condensed", 366: "semi_condensed", 367: "semi_expanded", 368: "expanded",
	369: "small_caps", 370: "superscript", 371: "subscript", 372: "uppercase", 373: "lowercase",
	374: "titlecase", 375: "rtl", 376: "ltr", 377: "content_bounds", 378: "border_bounds",
	379: "padding_bounds", 380: "margin_bounds", 381: "oblique", 382: "italic", 383: "auto",
	384: "manual", 385: "portrait", 386: "landscape", 387: "preview_images",
	388: "overlay_resource",

	// Navigation and landmarks ($389-$408)
	389: "book_navigation", 390: "section_navigation", 391: "nav_container",
	392: "nav_containers", 393: "nav_unit", 394: "conditional_nav_group_unit",
	395: "resource_path", 396: "srl", 397: "titlepage", 398: "acknowledgements", 399: "preface",
	400: "loi", 401: "lot", 402: "bibliography", 403: "index", 404: "glossary", 405: "frontmatter",
	406: "bodymatter", 407: "backmatter", 408: "erl",

	// Container-specific ($409-$420)
	409: "bcContId", 410: "bcComprType", 411: "bcDRMScheme", 412: "bcChunkSize",
	413: "bcIndexTabOffset", 414: "bcIndexTabLength", 415: "bcDocSymbolOffset",
	416: "bcDocSymbolLength", 417: "bcRawMedia", 418: "bcRawFont", 419: "container_entity_map",
	420: "pbm",

	// Resource and metadata properties ($421-$499)
	421: "both", 422: "resource_width", 423: "resource_height", 424: "cover_image",
	425: "page_progression_direction", 426: "activate", 427: "ordinal", 428: "action",
	429: "backdrop_style", 430: "hide", 431: "show", 432: "blank", 433: "orientation_lock",
	434: "virtual_panel", 435: "auto_crop", 436: "selection", 437: "page_spread",
	438: "facing_page", 439: "zoom_target", 440: "popup", 441: "enabled", 442: "disabled",
	443: "zoom_panel", 444: "popup_text", 445: "text_vert_anchor", 446: "text_hori_anchor",
	447: "text_top", 448: "text_baseline", 449: "text_bottom", 450: "text_start",
	451: "text_middle", 452: "text_end", 453: "caption", 454: "body", 455: "footer",
	456: "border_spacing_vertical", 457: "border_spacing_horizontal", 458: "hide_empty_cells",
	459: "border_radius_top_left", 460: "border_radius_top_right",
	461: "border_radius_bottom_left", 462: "border_radius_bottom_right", 463: "PRIVATE_doc_fonts",
	464: "volume_label", 465: "parent_asin", 466: "asset_id", 467: "revision_id", 468: "zoom_in",
	469: "zoom_out", 470: "btt", 471: "ttb", 472: "force", 473: "scale", 474: "source",
	475: "fit_text", 476: "clip", 477: "spacing_percent_base", 478: "fit_width",
	479: "background_image", 480: "background_positionx", 481: "background_positiony",
	482: "background_sizex", 483: "background_sizey", 484: "background_repeat", 485: "repeat_x",
	486: "repeat_y", 487: "no_repeat", 488: "relative", 489: "viewport", 490: "book_metadata",
	491: "categorised_metadata", 492: "key", 493: "priority", 494: "refines", 495: "category",
	496: "shadows", 497: "text_shadows", 498: "color", 499: "horizontal_offset",

	// Advanced units and features ($500-$537)
	500: "vertical_offset", 501: "blur", 502: "spread", 503: "list_style_image",
	504: "custom_viewer", 505: "rem", 506: "ch", 507: "vmax", 508: "gridlines",
	509: "parameter_list", 510: "set_parameters", 511: "hang_punctuation", 512: "layouts",
	513: "layout_name", 514: "grid_system", 515: "component_layout", 516: "+", 517: "-", 518: "*",
	519: "/", 520: "asSymbol", 521: "asString", 522: "asNumber", 523: "asList", 524: "asStructure",
	525: "isLandscape", 526: "isPortrait", 527: "isFirstPage", 528: "text_background_image",
	529: "stroke_linejoin", 530: "stroke_miterlimit", 531: "stroke_dasharray",
	532: "stroke_dashoffset", 533: "round", 534: "butt", 535: "miter", 536: "bevel",
	537: "component",

	// Document data and version info ($538-$599)
	538: "document_data", 539: "component_name", 540: "salience", 541: "border_radius",
	542: "clip_path_list", 543: "clip_path", 544: "clip_rule", 545: "clip_path_index",
	546: "sizing_bounds", 547: "background_origin", 548: "jxr", 549: "transform_origin",
	550: "location_map", 551: "list_style_position", 552: "inside", 553: "outside",
	554: "overline", 555: "overline_color", 556: "overline_weight", 557: "horizontal_tb",
	558: "vertical_lr", 559: "vertical_rl", 560: "writing_mode", 561: "all_small_caps",
	562: "ligatures", 563: "kerning", 564: "page_index", 565: "pdf", 566: "text_overflow",
	567: "ellipsis", 568: "text_clip", 569: "word_break", 570: "break_all", 571: "kicker",
	572: "article_id", 573: "all", 574: "browse", 575: "nav_visibility", 576: "link_visited_style",
	577: "link_unvisited_style", 578: "nbsp_mode", 579: "space", 580: "box_align", 581: "pan_zoom",
	582: "letterspacing_left", 583: "glyph_transform", 584: "alt_text", 585: "content_features",
	586: "namespace", 587: "major_version", 588: "minor_version", 589: "version_info",
	590: "features", 591: "exclude", 592: "include", 593: "format_capabilities",
	594: "bcFCapabilitiesOffset", 595: "bcFCapabilitiesLength", 596: "horizontal_rule",
	597: "auxiliary_data", 598: "kfx_id", 599: "bmp",

	// Advanced features ($600-$699)
	600: "tiff", 601: "render", 602: "block", 603: "layout_type", 604: "model",
	605: "word_iteration_type", 606: "word", 607: "icu", 608: "structure",
	609: "section_position_id_map", 610: "yj.eidhash_eid_section_map",
	611: "yj.section_pid_count_map", 612: "yj.bpg", 613: "yj.authoring", 614: "yj.conversion",
	615: "yj.classification", 616: "yj.display", 617: "yj.note", 618: "yj.chapternote",
	619: "yj.endnote", 620: "yj.sidenote", 621: "yj.location_pid_map", 622: "yj.first_line_style",
	623: "yj.number_of_lines", 624: "yj.percentage", 625: "yj.first_line_style_type",
	626: "yj.kfxid_eid_map", 627: "yj.interactive_element_list", 628: "yj.float_clear",
	629: "yj.table_features", 630: "yj.table_selection_mode", 631: "yj.rowwise",
	632: "yj.regional", 633: "yj.vertical_align", 634: "yj.sorting", 635: "yj.variants",
	636: "yj.tiles", 637: "yj.tile_width", 638: "yj.tile_height",
	639: "yj.user_margin_top_percentage", 640: "yj.user_margin_bottom_percentage",
	641: "yj.user_margin_left_percentage", 642: "yj.user_margin_right_percentage",
	643: "yj.header_overlay", 644: "yj.footer_overlay", 645: "yj.max_crop", 646: "yj.collision",
	647: "yj.min_aspect_ratio", 648: "yj.max_aspect_ratio", 649: "yj.viewer",
	650: "yj.border_path", 651: "yj.majority", 652: "yj.queue", 653: "yj.connected_page_spread",
	654: "yj.connected_panels", 655: "yj.connected_pagination", 656: "yj.enable_connected_dps",
	657: "yj.disable_stacking", 658: "yj.float_align", 659: "yj.supports",
	660: "yj.illustrated_layout", 661: "yj.disable_adaptive_layout",
	662: "yj.disable_repeated_headers", 663: "yj.conditional_properties", 664: "yj.sdl_version",
	665: "yj.comic_panel_view_mode", 666: "yj.guided_view", 667: "yj.content_defined",
	668: "yj.auto_contrast", 669: "yj.before", 670: "yj.after", 671: "yj.at", 672: "yj.float_bias",
	673: "yj.float_to_block", 674: "bidi_unicode", 675: "bidi_embed", 676: "isolate",
	677: "override", 678: "isolate_override", 679: "plaintext", 680: "start", 681: "end",
	682: "bidi_direction", 683: "annotations", 684: "pan_zoom_viewer", 685: "select_as_group",
	686: "kvg_content_type", 687: "annotation_type", 688: "math", 689: "mathsegment",
	690: "mathml", 691: "nontext", 692: "path_bundle", 693: "path_list", 694: "arabic_indic",
	695: "persian", 696: "word_boundary_list", 697: "yj.dictionary", 698: "is_empty",
	699: "fallback_width",

	// Ruby, layout hints, and other advanced features ($700-$851)
	700: "important_cells", 701: "default_fixed_reading_order", 702: "reading_order_switch_map",
	703: "switch_map", 704: "target_reading_order", 705: "source_position",
	706: "text_orientation", 707: "text_combine", 708: "character_width", 709: "fullwidth",
	710: "halfwidth", 711: "quarterwidth", 712: "thirdwidth", 713: "proportional", 714: "yj",
	715: "nowrap", 716: "white_space", 717: "text_emphasis_style", 718: "text_emphasis_color",
	719: "text_emphasis_position_horizontal", 720: "text_emphasis_position_vertical",
	721: "text_emphasis_spacing", 722: "text_emphasis_size", 723: "text_emphasis_align",
	724: "filled", 725: "open", 726: "filled_dot", 727: "open_dot", 728: "filled_circle",
	729: "open_circle", 730: "filled_double_circle", 731: "open_double_circle",
	732: "filled_triangle", 733: "open_triangle", 734: "filled_sesame", 735: "open_sesame",
	736: "cjk_ideographic", 737: "cjk_earthly_branch", 738: "cjk_heavenly_stem", 739: "hiragana",
	740: "hiragana_iroha", 741: "katakana", 742: "katakana_iroha", 743: "japanese_formal",
	744: "japanese_informal", 745: "simp_chinese_informal", 746: "simp_chinese_formal",
	747: "trad_chinese_informal", 748: "trad_chinese_formal", 749: "alt_content",
	750: "yj.layout_type", 751: "yj.large_tables", 752: "yj.in_page", 753: "yj.table_viewer",
	754: "main_content_id", 755: "truncated_bounds", 756: "ruby_content", 757: "ruby_name",
	758: "ruby_id", 759: "ruby_id_list", 760: "treat_as_title", 761: "layout_hints",
	762: "ruby_position_horizontal", 763: "ruby_position_vertical", 764: "ruby_merge",
	765: "ruby_text_align", 766: "ruby_base_align", 767: "ruby_overhang_chars",
	768: "ruby_overhang_amount", 769: "ruby_text_gap", 770: "ruby_base_edge_align",
	771: "separate", 772: "collapse", 773: "space_around", 774: "space_between", 775: "any",
	776: "JLREQ", 777: "JIS_X_4051", 778: "sideways", 779: "upright", 780: "line_break",
	781: "loose", 782: "strict", 783: "anywhere", 784: "fit_tight", 785: "keep_lines_together",
	786: "snap_block", 787: "recaps_reading_order", 788: "yj_break_after", 789: "yj_break_before",
	790: "yj.semantics.heading_level", 791: "lower_greek", 792: "upper_greek",
	793: "lower_armenian", 794: "upper_armenian", 795: "georgian", 796: "decimal_leading_zero",
	797: "yj.tile_padding", 798: "headings", 799: "h1", 800: "h2", 801: "h3", 802: "h4", 803: "h5",
	804: "h6", 805: "gradient_angle", 806: "gradient_direction", 807: "to_right", 808: "to_left",
	809: "to_top", 810: "to_bottom", 811: "to_top_right", 812: "to_top_left",
	813: "to_bottom_right", 814: "to_bottom_left", 815: "deg", 816: "grad", 817: "rad",
	818: "turn", 819: "conic", 820: "linear", 821: "table_metadata", 822: "table_row_count",
	823: "table_column_count", 824: "table_cell_count", 825: "table_character_count", 826: "audio",
	827: "video", 828: "rendition_flow", 829: "continue_rendition_flow", 830: "scrollable",
	831: "paginated", 832: "standalone_entities", 833: "document_regions",
	834: "yj.user_margin_bounds", 835: "ellipse", 836: "rectangle", 837: "line", 838: "polygon",
	839: "polyline", 840: "shape_dimensions", 841: "x", 842: "y", 843: "cx", 844: "cy",
	845: "radius_x", 846: "radius_y", 847: "start_x", 848: "start_y", 849: "end_x", 850: "end_y",
	851: "vertex_list",
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
