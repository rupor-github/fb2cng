package kfx

// buildAnchorFragments generates $266 anchor fragments for internal navigation.
// Fragment naming uses the actual ID from the source document (e.g., section IDs, note IDs).
// For TOC page links, anchor IDs may match section names (c0, c1, etc.) - this is allowed
// since anchor fragments ($266) use a different fragment type than section fragments ($260).
func buildAnchorFragments(idToEID eidByFB2ID, referenced map[string]bool) []*Fragment {
	var out []*Fragment
	if len(referenced) == 0 || len(idToEID) == 0 {
		return out
	}

	for id := range referenced {
		if id == "" {
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
