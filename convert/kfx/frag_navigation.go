package kfx

import "fmt"

// BuildNavigation creates a $389 book_navigation fragment from TOC entries.
// This creates a hierarchical TOC structure similar to epub's NCX/nav.
// The $389 fragment value is a list of reading order navigation entries.
// If posItems and pageSize are provided (pageSize > 0), an APPROXIMATE_PAGE_LIST
// is also included in the navigation.
// If landmarks has non-zero EIDs, a landmarks container is added.
func BuildNavigation(tocEntries []*TOCEntry, startEID int, posItems []PositionItem, pageSize int, landmarks LandmarkInfo) *Fragment {
	// Build TOC entries recursively
	entries := buildNavEntries(tocEntries, startEID)

	// Create TOC navigation container
	tocContainer := NewTOCContainer(entries)

	// Create nav_containers list starting with TOC
	navContainers := []any{tocContainer}

	// Add landmarks container if we have any landmark positions
	if landmarksContainer := buildLandmarksContainer(landmarks); landmarksContainer != nil {
		navContainers = append(navContainers, landmarksContainer)
	}

	// Add APPROXIMATE_PAGE_LIST if page mapping is enabled
	if pageSize > 0 && len(posItems) > 0 {
		pages := CalculateApproximatePages(posItems, pageSize)
		if len(pages) > 0 {
			pageEntries := buildPageListEntries(pages)
			pageListContainer := NewApproximatePageListContainer(pageEntries)
			navContainers = append(navContainers, pageListContainer)
		}
	}

	// Build reading order navigation entry
	// Structure: {$178: $351 (default), $392: [nav_containers]}
	readingOrderNav := NewStruct().
		SetSymbol(SymReadOrderName, SymDefault). // $178 = default reading order
		SetList(SymNavContainers, navContainers) // $392 = nav_containers

	// $389 is a list of reading order navigation entries
	bookNavList := []any{readingOrderNav}

	return &Fragment{
		FType: SymBookNavigation,
		FID:   SymBookNavigation, // Root fragment - FID == FType
		Value: bookNavList,
	}
}

// buildLandmarksContainer creates a landmarks navigation container from LandmarkInfo.
// Returns nil if no landmarks are configured.
func buildLandmarksContainer(landmarks LandmarkInfo) StructValue {
	landmarkEntries := make([]any, 0, 3)

	// Add cover landmark (if cover exists)
	if landmarks.CoverEID > 0 {
		landmarkEntries = append(landmarkEntries,
			NewLandmarkEntry(SymCoverPage, "cover-nav-unit", landmarks.CoverEID))
	}

	// Add TOC landmark (if TOC page exists)
	if landmarks.TOCEID > 0 {
		label := landmarks.TOCLabel
		if label == "" {
			label = "Table of Contents"
		}
		landmarkEntries = append(landmarkEntries,
			NewLandmarkEntry(SymTOC, label, landmarks.TOCEID))
	}

	// Add start reading location (if configured)
	if landmarks.StartEID > 0 {
		landmarkEntries = append(landmarkEntries,
			NewLandmarkEntry(SymSRL, "Start", landmarks.StartEID))
	}

	if len(landmarkEntries) == 0 {
		return nil
	}

	return NewLandmarksContainer(landmarkEntries)
}

// buildNavEntries recursively builds navigation unit entries from TOC entries.
func buildNavEntries(tocEntries []*TOCEntry, startEID int) []any {
	entries := make([]any, 0, len(tocEntries))

	for _, entry := range tocEntries {
		if !entry.IncludeInTOC {
			continue
		}

		// Create target position pointing to the first EID of the content
		targetPos := NewStruct().SetInt(SymUniqueID, int64(entry.FirstEID)) // $155 = id

		// Create nav unit with label and target
		navUnit := NewNavUnit(entry.Title, targetPos)

		// Add nested entries for children (hierarchical TOC)
		if len(entry.Children) > 0 {
			childEntries := buildNavEntries(entry.Children, startEID)
			if len(childEntries) > 0 {
				navUnit.SetList(SymEntries, childEntries) // $247 = nested entries
			}
		}

		entries = append(entries, navUnit)
	}

	return entries
}

// PageEntry represents an approximate page position in the book.
type PageEntry struct {
	PageNumber int   // 1-based page number
	EID        int   // Element ID where this page starts
	Offset     int64 // Offset within the element's text (in runes)
}

// CalculateApproximatePages computes page positions from position items.
// Each page is approximately pageSize runes. Returns page entries with
// EID and offset for navigation.
func CalculateApproximatePages(posItems []PositionItem, pageSize int) []PageEntry {
	if len(posItems) == 0 || pageSize <= 0 {
		return nil
	}

	var pages []PageEntry
	pageNumber := 1
	runesInPage := 0

	for _, item := range posItems {
		itemLen := item.Length
		if itemLen <= 0 {
			itemLen = 1
		}

		// Calculate how many runes we've consumed within this item
		runesConsumed := int64(0)

		for runesConsumed < int64(itemLen) {
			runesRemaining := int64(itemLen) - runesConsumed
			runesNeeded := int64(pageSize - runesInPage)

			if runesInPage == 0 {
				// Start of a new page - record position
				pages = append(pages, PageEntry{
					PageNumber: pageNumber,
					EID:        item.EID,
					Offset:     runesConsumed,
				})
			}

			if runesRemaining >= runesNeeded {
				// This item fills the page
				runesConsumed += runesNeeded
				runesInPage = 0
				pageNumber++
			} else {
				// This item doesn't fill the page
				runesConsumed += runesRemaining
				runesInPage += int(runesRemaining)
			}
		}
	}

	return pages
}

// buildPageListEntries creates navigation entries for APPROXIMATE_PAGE_LIST.
func buildPageListEntries(pages []PageEntry) []any {
	entries := make([]any, 0, len(pages))

	for _, page := range pages {
		// Create target position: {$143: offset, $155: eid}
		targetPos := NewStruct().
			SetInt(SymOffset, page.Offset).      // $143 = offset
			SetInt(SymUniqueID, int64(page.EID)) // $155 = id (EID as int)

		// Create nav unit with page number as label
		navUnit := NewNavUnit(fmt.Sprintf("%d", page.PageNumber), targetPos)

		entries = append(entries, navUnit)
	}

	return entries
}
