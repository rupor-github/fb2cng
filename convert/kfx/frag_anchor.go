package kfx

func collectReferencedAnchorNames(fragments *FragmentList) map[string]bool {
	refs := make(map[string]bool)
	for _, frag := range fragments.All() {
		collectReferencedAnchorNamesFromValue(frag.Value, refs)
	}
	return refs
}

func collectReferencedAnchorNamesFromValue(v any, refs map[string]bool) {
	switch vv := v.(type) {
	case StructValue:
		for k, val := range vv {
			if k == SymLinkTo {
				if s, ok := val.(SymbolByNameValue); ok {
					refs[string(s)] = true
				} else if s, ok := val.(string); ok {
					refs[s] = true
				}
			}
			collectReferencedAnchorNamesFromValue(val, refs)
		}
	case map[int]any:
		for k, val := range vv {
			if k == SymLinkTo {
				if s, ok := val.(SymbolByNameValue); ok {
					refs[string(s)] = true
				} else if s, ok := val.(string); ok {
					refs[s] = true
				}
			}
			collectReferencedAnchorNamesFromValue(val, refs)
		}
	case ListValue:
		for _, item := range vv {
			collectReferencedAnchorNamesFromValue(item, refs)
		}
	case []any:
		for _, item := range vv {
			collectReferencedAnchorNamesFromValue(item, refs)
		}
	case map[string]any:
		for _, item := range vv {
			collectReferencedAnchorNamesFromValue(item, refs)
		}
	}
}

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

// buildAnchorFragments generates $266 anchors for internal navigation.
// KFXInput expects the entity ID name to match $180 (anchor_name).
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
				if !isGeneratedSectionName(e.ID) && referenced[e.ID] {
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
