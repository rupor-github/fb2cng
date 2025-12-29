package kfx

func isGeneratedSectionName(name string) bool {
	if len(name) < 2 || name[0] != 'c' {
		return false
	}
	for i := 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}

// buildAnchorFragments generates $266 anchor fragments for internal navigation.
// Fragment naming uses the actual ID from the source document (e.g., section IDs, note IDs).
// Generated section names (c0, c1, etc.) are filtered to avoid fragment id collisions.
func buildAnchorFragments(idToEID map[string]int, referenced map[string]bool) []*Fragment {
	var out []*Fragment
	if len(referenced) == 0 || len(idToEID) == 0 {
		return out
	}

	for id := range referenced {
		if id == "" || isGeneratedSectionName(id) {
			continue
		}
		eid, ok := idToEID[id]
		if !ok || eid == 0 {
			continue
		}
		pos := NewStruct().SetInt(SymUniqueID, int64(eid))
		out = append(out, &Fragment{
			FType:   SymAnchor,
			FIDName: id,
			Value: NewStruct().
				Set(SymAnchorName, SymbolByName(id)).
				SetStruct(SymPosition, pos),
		})
	}

	return out
}
