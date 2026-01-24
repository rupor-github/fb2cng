package kfx

// This file provides helper functions for constructing common Ion value patterns
// used in KFX fragments. These builders create properly structured values for
// navigation, resources, and metadata.

// LandmarkInfo holds EIDs for landmark navigation entries.
// Zero values indicate the landmark is not present.
type LandmarkInfo struct {
	CoverEID int    // Page template EID of the cover section
	TOCEID   int    // First EID of the TOC page section
	StartEID int    // First EID of the start reading location (body intro)
	TOCLabel string // Display label for TOC landmark (e.g., "Table of Contents")
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

// NewLandmarkEntry creates a landmark entry with type, label and target position.
// landmarkType is the landmark type symbol (e.g., SymCoverPage, SymTOC, SymSRL).
// label is the display text for the landmark.
// eid is the target element ID as integer.
func NewLandmarkEntry(landmarkType KFXSymbol, label string, eid int) StructValue {
	// Target position: {$143: 0, $155: eid}
	targetPos := NewStruct().
		SetInt(SymOffset, 0).           // $143 = offset (always 0 for landmarks)
		SetInt(SymUniqueID, int64(eid)) // $155 = id

	entry := NewStruct().
		SetSymbol(SymLandmarkType, landmarkType) // $238 = landmark_type

	if label != "" {
		repr := NewStruct().SetString(SymLabel, label) // $244 = label
		entry.SetStruct(SymRepresentation, repr)       // $241 = representation
	}

	entry.SetStruct(SymTargetPosition, targetPos) // $246 = target_position
	return entry
}

// NewApproximatePageListContainer creates a page list container with APPROXIMATE_PAGE_LIST name.
// This is used for KFX-generated approximate page numbers.
func NewApproximatePageListContainer(entries []any) StructValue {
	return NewStruct().
		SetSymbol(SymNavType, SymPageList).                         // $235 = page_list
		Set(SymNavContName, SymbolByName("APPROXIMATE_PAGE_LIST")). // $239 = nav_container_name (local symbol)
		SetList(SymEntries, entries)                                // $247 = entries
}

// Resource builders - for $164 external resource descriptors

// NewExternalResource creates an external resource descriptor.
func NewExternalResource(location string, format KFXSymbol, mimeType string, width, height int64) StructValue {
	res := NewStruct().
		SetSymbol(SymFormat, format).    // $161 = format
		SetString(SymMIME, mimeType).    // $162 = mime type
		SetString(SymLocation, location) // $165 = location
	if width > 0 {
		res.SetInt(SymResourceWidth, width) // $422 = resource_width
	}
	if height > 0 {
		res.SetInt(SymResourceHeight, height) // $423 = resource_height
	}
	return res
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
