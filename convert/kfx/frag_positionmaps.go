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

type PositionItem struct {
	EID    int
	Length int // approximate text length (runes)
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
			eid64, ok := entry[SymUniqueID].(int64)
			if !ok {
				continue
			}
			eid := int(eid64)

			typeSym, ok := entry[SymType].(SymbolValue)
			if !ok {
				continue
			}

			if KFXSymbol(typeSym) == SymImage {
				out = append(out, PositionItem{EID: eid, Length: 1})
				continue
			}

			ref, ok := entry[SymContent].(map[string]any)
			if !ok {
				continue
			}
			nameVal, ok := ref["name"].(SymbolByNameValue)
			if !ok {
				continue
			}
			offAny, ok := ref["$403"]
			if !ok {
				continue
			}
			off, ok := offAny.(int64)
			if !ok {
				offInt, ok2 := offAny.(int)
				if !ok2 {
					continue
				}
				off = int64(offInt)
			}

			paras := content[string(nameVal)]
			if int(off) < 0 || int(off) >= len(paras) {
				continue
			}
			out = append(out, PositionItem{EID: eid, Length: len([]rune(paras[off]))})
		}
	}

	return out
}

// BuildPositionIDMap creates the $265 position_id_map root fragment.
// Reference KFX uses a list of structs: { $184 pid, $185 eid }.
// We emit a sparse mapping (entry per EID start) with pid gaps based on text length.
func BuildPositionIDMap(allEIDs []int, items []PositionItem) *Fragment {
	entries := make([]any, 0, len(allEIDs)+1)

	pid := int64(0)
	if len(items) > 0 {
		for _, it := range items {
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
