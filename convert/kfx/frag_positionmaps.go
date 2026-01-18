package kfx

import "sort"

// CollectAllEIDs returns a sorted unique list of all EIDs present in sectionEIDs.
func CollectAllEIDs(sectionEIDs sectionEIDsBySectionName) []int {
	seen := make(map[int]struct{})
	out := make([]int, 0)
	for _, eids := range sectionEIDs {
		for _, eid := range eids {
			if _, ok := seen[eid]; ok {
				continue
			}
			seen[eid] = struct{}{}
			out = append(out, eid)
		}
	}
	sort.Ints(out)
	return out
}

// compressEIDs returns a flat sorted list of EIDs for position_map.
// While [base, count] compression was attempted previously, KPV output shows
// that a flat list of all EIDs is expected.
func compressEIDs(eids []int) []any {
	if len(eids) == 0 {
		return nil
	}

	sorted := make([]int, len(eids))
	copy(sorted, eids)
	sort.Ints(sorted)

	out := make([]any, 0, len(sorted))
	for _, eid := range sorted {
		out = append(out, int64(eid))
	}

	return out
}

// BuildPositionMap creates the $264 position_map root fragment.
// Value is a list of structs, one per section: {$174: section_id, $181: [eid...]}.
func BuildPositionMap(sectionNames sectionNameList, sectionEIDs sectionEIDsBySectionName) *Fragment {
	entries := make([]any, 0, len(sectionNames))
	for _, sectionName := range sectionNames {
		eids := sectionEIDs[sectionName]
		entry := NewStruct().
			Set(SymSectionName, SymbolByName(sectionName)).
			SetList(SymContainsIds, compressEIDs(eids))
		entries = append(entries, entry)
	}
	return NewRootFragment(SymPositionMap, ListValue(entries))
}

// InlineImagePos tracks an inline image's position within mixed content.
type InlineImagePos struct {
	EID    int // Image element ID
	Offset int // Character offset in parent text where image appears
}

// PositionItem represents a content element for position tracking.
type PositionItem struct {
	EID    int
	Length int // approximate text length (runes)

	// For mixed content (text with inline images):
	// InlineImages contains the EIDs and offsets of embedded images.
	// When set, this entry will generate KP3-style granular position entries.
	InlineImages []InlineImagePos
}

// HasInlineImages returns true if any position item has inline images,
// indicating the book uses offset-based position entries.
func HasInlineImages(items []PositionItem) bool {
	for _, it := range items {
		if len(it.InlineImages) > 0 {
			return true
		}
	}
	return false
}

// CollectPositionItems extracts content entries (including page templates) in reading order.
func CollectPositionItems(fragments *FragmentList, sectionNames sectionNameList) []PositionItem {
	if fragments == nil {
		return nil
	}

	// Build content fragment lookup: contentName -> []string
	content := make(map[string][]string)
	for _, f := range fragments.GetByType(SymContent) {
		m, ok := f.Value.(map[string]any)
		if !ok {
			continue
		}
		nameVal, ok := m["name"].(SymbolByNameValue)
		if !ok {
			continue
		}
		lst, ok := m["$146"].([]any)
		if !ok {
			continue
		}
		items := make([]string, 0, len(lst))
		for _, it := range lst {
			s, _ := it.(string)
			items = append(items, s)
		}
		content[string(nameVal)] = items
	}

	out := make([]PositionItem, 0)

	// processEntry extracts position info from a storyline entry (handles nested children)
	var processEntry func(entry StructValue)
	processEntry = func(entry StructValue) {
		eid64, ok := entry[SymUniqueID].(int64)
		if !ok {
			return
		}
		eid := int(eid64)

		typeSym, ok := entry[SymType].(SymbolValue)
		if !ok {
			return
		}

		// Check for content_list - could be wrapper container OR mixed text+images
		if children, ok := entry[SymContentList].([]any); ok && len(children) > 0 {
			// Analyze content_list to determine handling:
			// - Mixed content: text entry with strings (possibly interleaved with inline images)
			// - Image-only text: text entry with only inline image(s), no strings
			// - Wrapper-style: container with only StructValue children (like title blocks)
			hasStrings := false
			hasInlineImages := false
			isTextType := KFXSymbol(typeSym) == SymText

			for _, child := range children {
				if _, isString := child.(string); isString {
					hasStrings = true
				} else if childSV, ok := child.(StructValue); ok {
					if childType, ok := childSV[SymType].(SymbolValue); ok && KFXSymbol(childType) == SymImage {
						if render, ok := childSV[SymRender].(SymbolValue); ok && KFXSymbol(render) == SymInline {
							hasInlineImages = true
						}
					}
				}
			}

			// Text entries with inline images (with or without text) use mixed content handling
			if isTextType && (hasStrings || hasInlineImages) {
				// Mixed content: collect text lengths and inline image positions
				// KP3 generates granular position entries for mixed content
				textLen := 0
				var inlineImages []InlineImagePos

				for _, child := range children {
					if s, ok := child.(string); ok {
						textLen += len([]rune(s))
					} else if childSV, ok := child.(StructValue); ok {
						// Check if this is an inline image
						if childEID, ok := childSV[SymUniqueID].(int64); ok {
							if childType, ok := childSV[SymType].(SymbolValue); ok && KFXSymbol(childType) == SymImage {
								inlineImages = append(inlineImages, InlineImagePos{
									EID:    int(childEID),
									Offset: textLen, // Current text offset where image appears
								})
								textLen++ // Image consumes 1 position in the stream
							}
						}
					}
				}

				if textLen < 1 {
					textLen = 1
				}
				out = append(out, PositionItem{
					EID:          eid,
					Length:       textLen,
					InlineImages: inlineImages,
				})
				return
			}

			// Wrapper-style container: emit wrapper EID first, then process children
			out = append(out, PositionItem{EID: eid, Length: 1})
			for _, child := range children {
				if childEntry, ok := child.(StructValue); ok {
					processEntry(childEntry)
				}
			}
			return
		}

		if KFXSymbol(typeSym) == SymImage {
			out = append(out, PositionItem{EID: eid, Length: 1})
			return
		}

		ref, ok := entry[SymContent].(map[string]any)
		if !ok {
			return
		}
		nameVal, ok := ref["name"].(SymbolByNameValue)
		if !ok {
			return
		}
		offAny, ok := ref["$403"]
		if !ok {
			return
		}
		off, ok := offAny.(int64)
		if !ok {
			offInt, ok2 := offAny.(int)
			if !ok2 {
				return
			}
			off = int64(offInt)
		}

		paras := content[string(nameVal)]
		if int(off) < 0 || int(off) >= len(paras) {
			return
		}
		out = append(out, PositionItem{EID: eid, Length: len([]rune(paras[off]))})
	}

	// Build section -> (pageTemplateEID, storyName)
	sections := make(map[string]struct {
		pageTemplateEID int
		storyName       string
	})
	for _, f := range fragments.GetByType(SymSection) {
		secName := f.FIDName
		sv, ok := f.Value.(StructValue)
		if !ok {
			continue
		}
		pts, ok := sv[SymPageTemplates].([]any)
		if !ok || len(pts) == 0 {
			continue
		}
		pt, ok := pts[0].(StructValue)
		if !ok {
			continue
		}
		eid64, ok := pt[SymUniqueID].(int64)
		if !ok {
			continue
		}
		story, _ := pt[SymStoryName].(SymbolByNameValue)
		sections[secName] = struct {
			pageTemplateEID int
			storyName       string
		}{pageTemplateEID: int(eid64), storyName: string(story)}
	}

	// Emit in section order: page template EID, then storyline content EIDs.
	for _, secName := range sectionNames {
		s, ok := sections[secName]
		if !ok {
			continue
		}
		out = append(out, PositionItem{EID: s.pageTemplateEID, Length: 1})

		var storyFrag *Fragment
		for _, f := range fragments.GetByType(SymStoryline) {
			if f.FIDName == s.storyName {
				storyFrag = f
				break
			}
		}
		if storyFrag == nil {
			continue
		}
		sv, ok := storyFrag.Value.(StructValue)
		if !ok {
			continue
		}
		cl, ok := sv[SymContentList].([]any)
		if !ok {
			continue
		}
		for _, ce := range cl {
			entry, ok := ce.(StructValue)
			if !ok {
				continue
			}
			processEntry(entry)
		}
	}

	return out
}

// BuildPositionIDMap creates the $265 position_id_map root fragment.
// Reference KFX uses a list of structs: { $184 pid, $185 eid }.
// We emit a sparse mapping (entry per EID start) with pid gaps based on text length.
// For mixed content (text with inline images), we emit KP3-style granular entries:
// - Parent entry at start
// - For each inline image: before/image/after entries with offset tracking
// For image-only text entries (no actual text, just inline images), we emit simpler entries:
// - One entry for wrapper EID at current PID
// - One entry for each image EID at the SAME PID
// - No offset entries
func BuildPositionIDMap(allEIDs []int, items []PositionItem) *Fragment {
	entries := make([]any, 0, len(allEIDs)+1)

	pid := int64(0)
	if len(items) > 0 {
		for _, it := range items {
			if len(it.InlineImages) > 0 {
				// Check if this is image-only (no actual text content)
				// Image-only: all images at offset 0 and length equals number of images
				isImageOnly := true
				for _, img := range it.InlineImages {
					if img.Offset != 0 {
						isImageOnly = false
						break
					}
				}
				// Also check that total length equals number of images (no text)
				if it.Length != len(it.InlineImages) {
					isImageOnly = false
				}

				if isImageOnly {
					// Image-only text entry: emit wrapper and images at same PID
					// KP3 emits: {pid, wrapper_eid}, {pid, image_eid}
					entries = append(entries, NewStruct().
						SetInt(SymPositionID, pid).
						SetInt(SymElementID, int64(it.EID)),
					)
					for _, img := range it.InlineImages {
						entries = append(entries, NewStruct().
							SetInt(SymPositionID, pid).
							SetInt(SymElementID, int64(img.EID)),
						)
					}
					// Advance PID by 1 for the image(s)
					pid++
				} else {
					// Mixed content: emit granular KP3-style entries
					startPID := pid

					// 1. Entry for parent element at start
					entries = append(entries, NewStruct().
						SetInt(SymPositionID, startPID).
						SetInt(SymElementID, int64(it.EID)),
					)

					// 2. For each inline image, emit before/image/after entries
					for _, img := range it.InlineImages {
						imgPID := startPID + int64(img.Offset)

						// Entry before image (with offset from parent start)
						entries = append(entries, NewStruct().
							SetInt(SymOffset, int64(img.Offset)).
							SetInt(SymPositionID, imgPID).
							SetInt(SymElementID, int64(it.EID)),
						)

						// Entry for inline image itself
						entries = append(entries, NewStruct().
							SetInt(SymPositionID, imgPID).
							SetInt(SymElementID, int64(img.EID)),
						)

						// Entry after image (offset+1, pid+1, parent eid)
						entries = append(entries, NewStruct().
							SetInt(SymOffset, int64(img.Offset+1)).
							SetInt(SymPositionID, imgPID+1).
							SetInt(SymElementID, int64(it.EID)),
						)
					}

					// Advance pid by total text length
					step := int64(it.Length)
					if step < 1 {
						step = 1
					}
					pid += step
				}
			} else {
				// Regular content: single entry
				entries = append(entries, NewStruct().
					SetInt(SymPositionID, pid).
					SetInt(SymElementID, int64(it.EID)),
				)
				step := int64(it.Length)
				if step < 1 {
					step = 1
				}
				pid += step
			}
		}
	} else {
		for _, eid := range allEIDs {
			entries = append(entries, NewStruct().
				SetInt(SymPositionID, pid).
				SetInt(SymElementID, int64(eid)),
			)
			pid++
		}
	}

	// Sentinel entry.
	entries = append(entries, NewStruct().
		SetInt(SymPositionID, pid).
		SetInt(SymElementID, 0),
	)

	return NewRootFragment(SymPositionIdMap, ListValue(entries))
}

// BuildLocationMap creates the $550 location_map root fragment.
// Value is a list of length 1 containing a struct with keys {$178, $182}.
func BuildLocationMap(allEIDs []int) *Fragment {
	locations := make([]any, 0)
	if len(allEIDs) > 0 {
		const positionsPerLocation = 110
		for i := 0; i < len(allEIDs); i += positionsPerLocation {
			locations = append(locations, NewStruct().SetInt(SymUniqueID, int64(allEIDs[i])))
		}
		if last := allEIDs[len(allEIDs)-1]; last != allEIDs[0] {
			// Ensure the final position is represented.
			lastPos := locations[len(locations)-1].(StructValue)
			if v, ok := lastPos.GetInt(SymUniqueID); ok && int(v) != last {
				locations = append(locations, NewStruct().SetInt(SymUniqueID, int64(last)))
			}
		}
	}

	locStruct := NewStruct().
		SetSymbol(SymReadOrderName, SymDefault).
		SetList(SymLocations, locations)

	return NewRootFragment(SymLocationMap, ListValue([]any{locStruct}))
}
