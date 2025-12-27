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
// These IDs are preserved from the original FB2 to maintain link integrity, except for
// generated section names (c0, c1, etc.) which are filtered to avoid collisions.
// This is the only fragment type that uses source document IDs directly as FIDNames.
func buildAnchorFragments(tocEntries []*TOCEntry, referenced map[string]bool) []*Fragment {
	var out []*Fragment

	var walk func(entries []*TOCEntry)
	walk = func(entries []*TOCEntry) {
		for _, e := range entries {
			if e == nil {
				continue
			}
			if e.ID != "" && e.FirstEID != 0 {
				// Avoid collisions with our generated section IDs (c0,c1,...) which are
				// already used by $260 section entities.
				if !isGeneratedSectionName(e.ID) && (referenced == nil || referenced[e.ID]) {
					pos := NewStruct().SetInt(SymUniqueID, int64(e.FirstEID))
					out = append(out, &Fragment{
						FType:   SymAnchor,
						FIDName: e.ID,
						Value: NewStruct().
							Set(SymAnchorName, SymbolByName(e.ID)).
							SetStruct(SymPosition, pos),
					})
				}
			}
			if len(e.Children) > 0 {
				walk(e.Children)
			}
		}
	}
	walk(tocEntries)
	return out
}
