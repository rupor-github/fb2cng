package kfx

// buildAnchorFragments is intentionally unused for now: $266 anchors are implemented
// but generation is disabled until Phase 6+ (position maps) and correct id scheme are in place.
//
//lint:ignore U1000 kept for planned feature
func buildAnchorFragments(tocEntries []*TOCEntry) []*Fragment {
	var out []*Fragment
	var walk func(entries []*TOCEntry)
	walk = func(entries []*TOCEntry) {
		for _, e := range entries {
			if e == nil {
				continue
			}
			if e.ID != "" && e.FirstEID != 0 {
				pos := NewStruct().SetInt(SymUniqueID, int64(e.FirstEID))
				out = append(out, &Fragment{
					FType:   SymAnchor,
					FIDName: e.ID,
					Value:   NewStruct().SetStruct(SymPosition, pos),
				})
			}
			if len(e.Children) > 0 {
				walk(e.Children)
			}
		}
	}
	walk(tocEntries)
	return out
}
