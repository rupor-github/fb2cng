package kfx

// This file provides helper functions for constructing common Ion value patterns
// used in KFX fragments. These builders create properly structured values for
// positions, lengths, style properties, content nodes, and navigation.

// Position builders - for anchors and navigation targets
// A position is a struct with EID ($155 or $598) and optional offset ($143).

// NewPosition creates a position struct for anchors/navigation.
// Position is {$155: eid} or {$598: eid} with optional {$143: offset}.
func NewPosition(eid string, offset int64) StructValue {
	pos := NewStruct().SetString(SymUniqueID, eid) // $155 = id
	if offset != 0 {
		pos.SetInt(SymOffset, offset) // $143 = offset
	}
	return pos
}

// NewPositionKFXID creates a position using kfx_id ($598) instead of id ($155).
func NewPositionKFXID(kfxID string, offset int64) StructValue {
	pos := NewStruct().SetString(SymKfxID, kfxID) // $598 = kfx_id
	if offset != 0 {
		pos.SetInt(SymOffset, offset) // $143 = offset
	}
	return pos
}

// Length/unit builders - for dimensions and spacing
// A length with unit is {$307: value, $306: unit_symbol}.

// NewLength creates a length value with unit: {value: v, unit: u}.
func NewLength(value float64, unit KFXSymbol) StructValue {
	return NewStruct().
		Set(SymValue, value).    // $307 = value
		SetSymbol(SymUnit, unit) // $306 = unit
}

// NewLengthEm creates a length in em units.
func NewLengthEm(value float64) StructValue {
	return NewLength(value, SymUnitEm) // $308 = em
}

// NewLengthPt creates a length in point units.
func NewLengthPt(value float64) StructValue {
	return NewLength(value, SymUnitPt) // $318 = pt
}

// NewLengthPx creates a length in pixel units.
func NewLengthPx(value float64) StructValue {
	return NewLength(value, SymUnitPx) // $319 = px
}

// NewLengthPercent creates a length in percent units.
func NewLengthPercent(value float64) StructValue {
	return NewLength(value, SymUnitPercent) // $314 = percent
}

// NewLengthCm creates a length in centimeter units.
func NewLengthCm(value float64) StructValue {
	return NewLength(value, SymUnitCm) // $315 = cm
}

// NewLengthMm creates a length in millimeter units.
func NewLengthMm(value float64) StructValue {
	return NewLength(value, SymUnitMm) // $316 = mm
}

// NewLengthIn creates a length in inch units.
func NewLengthIn(value float64) StructValue {
	return NewLength(value, SymUnitIn) // $317 = in
}

// Style property builders - for $157 style fragments and inline styles

// NewStyleEvent creates a style event for text styling.
// Style events are used in $142 (style_events) to apply formatting to text ranges.
func NewStyleEvent(offset, length int64) StructValue {
	return NewStruct().
		SetInt(SymOffset, offset). // $143 = offset
		SetInt(SymLength, length)  // $144 = length
}

// SetStyleRef sets the style reference on a style event or content.
func SetStyleRef(s StructValue, styleName string) StructValue {
	return s.SetSymbol(SymStyle, SymbolID(styleName)) // $157 = style
}

// Content node builders - for section content

// NewTextContent creates a text content node.
// Text content is {$145: "text string"} optionally with style_events.
func NewTextContent(text string) StructValue {
	return NewStruct().SetString(SymContent, text) // $145 = content
}

// NewTextContentWithID creates a text content node with an ID for position mapping.
func NewTextContentWithID(text, id string) StructValue {
	return NewTextContent(text).SetString(SymUniqueID, id) // $155 = id
}

// NewImageContent creates an image content node.
// Image content references an external resource.
func NewImageContent(resourceName string) StructValue {
	return NewStruct().SetSymbol(SymResourceName, SymbolID(resourceName)) // $175 = resource_name
}

// NewContainerContent creates a container content node.
// Container wraps other content elements.
func NewContainerContent() StructValue {
	return NewStruct()
}

// Navigation builders - for $389, $391, $393 fragments

// NewNavUnit creates a navigation unit (TOC entry, page list entry, etc.).
func NewNavUnit(label string, targetPos StructValue) StructValue {
	nav := NewStruct()
	if label != "" {
		// Representation struct contains the label
		repr := NewStruct().SetString(SymLabel, label) // $244 = label
		nav.SetStruct(SymRepresentation, repr)         // $241 = representation
	}
	if targetPos != nil {
		nav.SetStruct(SymTargetPosition, targetPos) // $246 = target_position
	}
	return nav
}

// NewNavContainer creates a navigation container (TOC, landmarks, page list).
func NewNavContainer(navType KFXSymbol, entries []any) StructValue {
	return NewStruct().
		SetSymbol(SymNavType, navType). // $235 = nav_type
		SetList(SymEntries, entries)    // $247 = entries
}

// NewTOCContainer creates a TOC navigation container.
func NewTOCContainer(entries []any) StructValue {
	return NewNavContainer(SymTOC, entries) // $212 = toc
}

// NewLandmarksContainer creates a landmarks navigation container.
func NewLandmarksContainer(entries []any) StructValue {
	return NewNavContainer(SymLandmarks, entries) // $236 = landmarks
}

// NewPageListContainer creates a page list navigation container.
func NewPageListContainer(entries []any) StructValue {
	return NewNavContainer(SymPageList, entries) // $237 = page_list
}

// Resource builders - for $164 external resource descriptors

// NewExternalResource creates an external resource descriptor.
func NewExternalResource(location string, format KFXSymbol, width, height int64) StructValue {
	res := NewStruct().
		SetString(SymLocation, location). // $165 = location
		SetSymbol(SymFormat, format)      // $161 = format
	if width > 0 {
		res.SetInt(SymResourceWidth, width) // $422 = resource_width
	}
	if height > 0 {
		res.SetInt(SymResourceHeight, height) // $423 = resource_height
	}
	return res
}

// NewImageResourcePNG creates an external resource for a PNG image.
func NewImageResourcePNG(location string, width, height int64) StructValue {
	return NewExternalResource(location, SymFormatPNG, width, height) // $284 = png
}

// NewImageResourceJPG creates an external resource for a JPEG image.
func NewImageResourceJPG(location string, width, height int64) StructValue {
	return NewExternalResource(location, SymFormatJPG, width, height) // $285 = jpg
}

// NewImageResourceGIF creates an external resource for a GIF image.
func NewImageResourceGIF(location string, width, height int64) StructValue {
	return NewExternalResource(location, SymFormatGIF, width, height) // $286 = gif
}

// Anchor builders - for $266 anchor fragments

// NewPositionAnchor creates an anchor with a position reference.
func NewPositionAnchor(position StructValue) StructValue {
	return NewStruct().SetStruct(SymPosition, position) // $183 = position
}

// NewURIAnchor creates an anchor with an external URI.
func NewURIAnchor(uri string) StructValue {
	return NewStruct().SetString(SymURI, uri) // $186 = uri
}

// Metadata builders - for $258 metadata and $490 book_metadata

// NewMetadataEntry creates a metadata key-value entry for $490 categorised metadata.
// Value is intentionally `any` because reference KFX often uses non-string values (e.g. bool)
// under $307.
func NewMetadataEntry(key string, value any) StructValue {
	return NewStruct().
		SetString(SymKey, key). // $492 = key
		Set(SymValue, value)    // $307 = value
}

// NewCategorisedMetadata creates a categorised metadata entry for $490.
func NewCategorisedMetadata(category string, entries []any) StructValue {
	return NewStruct().
		SetString(SymCategory, category). // $495 = category
		SetList(SymMetadata, entries)     // $258 = metadata (list of entries)
}

// Section and storyline builders

// NewSection creates a section fragment value.
func NewSection(sectionName string, content []any) StructValue {
	return NewStruct().
		SetString(SymSectionName, sectionName). // $174 = section_name
		SetList(SymContentList, content)        // $146 = content_list
}

// NewStoryline creates a storyline fragment value.
func NewStoryline(storyName string, sections []any) StructValue {
	return NewStruct().
		SetString(SymStoryName, storyName). // $176 = story_name
		SetList(SymSections, sections)      // $170 = sections
}

// Reading order builders

// NewReadingOrder creates a reading order entry for $538 document_data.
// The name should be a symbol like SymDefault ($351 = "default").
func NewReadingOrder(name KFXSymbol, sections []any) StructValue {
	ro := NewStruct().SetSymbol(SymReadOrderName, name) // $178 = reading_order_name
	if len(sections) > 0 {
		ro.SetList(SymSections, sections) // $170 = sections
	}
	return ro
}

// Format capabilities builders - for $593

// NewFormatCapabilities creates a format capabilities fragment value.
func NewFormatCapabilities(majorVersion, minorVersion int64) StructValue {
	return NewStruct().
		SetInt(SymMajorVersion, majorVersion). // $587 = major_version
		SetInt(SymMinorVersion, minorVersion)  // $588 = minor_version
}

// AddFeature adds a feature to format capabilities.
func AddFeature(fc StructValue, featureName string, value any) StructValue {
	features, ok := fc.GetStruct(SymFeatures) // $590 = features
	if !ok {
		features = NewStruct()
	}
	features.Set(SymbolID(featureName), value)
	return fc.SetStruct(SymFeatures, features)
}

// Container entity map builders - for $419

// NewContainerEntry creates a container list entry for $419.
func NewContainerEntry(containerID string, entityIDs []any) StructValue {
	return NewStruct().
		SetString(SymUniqueID, containerID). // $155 = id
		SetList(SymContainsIds, entityIDs)   // $181 = contains
}

// NewContainerEntityMap creates a container entity map fragment value.
func NewContainerEntityMap(containers []any) StructValue {
	return NewStruct().SetList(SymContainerList, containers) // $252 = container_list
}

// Sym converts symbol ID to SymbolValue for use in lists.
// Deprecated: Use KFXSymbol.Value() method instead.
func Sym(id KFXSymbol) SymbolValue {
	return id.Value()
}

// NewSymbolList creates a list of symbol values from IDs.
func NewSymbolList(ids ...KFXSymbol) ListValue {
	list := NewList()
	for _, id := range ids {
		list.AddSymbol(id)
	}
	return list
}

// StrList creates a list of string values.
func StrList(strs ...string) ListValue {
	list := NewList()
	for _, s := range strs {
		list.AddString(s)
	}
	return list
}
