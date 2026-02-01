package kfx

// collectReferencedResourceNames scans fragments and returns referenced resource names.
// Resource names are recorded from resource_name fields in content entries.
func collectReferencedResourceNames(fragments *FragmentList) map[string]bool {
	used := make(map[string]bool)
	if fragments == nil {
		return used
	}

	addName := func(v any) {
		switch sym := v.(type) {
		case SymbolByNameValue:
			if name := string(sym); name != "" {
				used[name] = true
			}
		case string:
			if sym != "" {
				used[sym] = true
			}
		case SymbolValue:
			if name := KFXSymbol(sym).Name(); name != "" {
				used[name] = true
			}
		case ReadSymbolValue:
			if name := string(sym); name != "" {
				used[name] = true
			}
		}
	}

	var scan func(v any)
	scan = func(v any) {
		switch val := v.(type) {
		case StructValue:
			if resName, ok := val[SymResourceName]; ok {
				addName(resName)
			}
			for _, child := range val {
				scan(child)
			}
		case map[KFXSymbol]any:
			if resName, ok := val[SymResourceName]; ok {
				addName(resName)
			}
			for _, child := range val {
				scan(child)
			}
		case map[string]any:
			for _, child := range val {
				scan(child)
			}
		case []any:
			for _, child := range val {
				scan(child)
			}
		}
	}

	for _, frag := range fragments.All() {
		scan(frag.Value)
	}

	return used
}

// filterImageResources removes external resources and raw media that are not referenced.
func filterImageResources(externalRes, rawMedia []*Fragment, imageResources imageResourceInfoByID, usedResourceNames map[string]bool) ([]*Fragment, []*Fragment, imageResourceInfoByID) {
	if len(usedResourceNames) == 0 {
		return nil, nil, nil
	}

	nameToLocation := make(map[string]string, len(externalRes))
	for _, frag := range externalRes {
		name := frag.FIDName
		if name == "" {
			continue
		}
		if v, ok := frag.Value.(StructValue); ok {
			if loc, ok := v.GetString(SymLocation); ok {
				nameToLocation[name] = loc
			}
		}
	}

	filteredExternal := make([]*Fragment, 0, len(externalRes))
	usedLocations := make(map[string]bool, len(usedResourceNames))
	for _, frag := range externalRes {
		if usedResourceNames[frag.FIDName] {
			filteredExternal = append(filteredExternal, frag)
			if loc := nameToLocation[frag.FIDName]; loc != "" {
				usedLocations[loc] = true
			}
		}
	}

	filteredRaw := make([]*Fragment, 0, len(rawMedia))
	for _, frag := range rawMedia {
		if usedLocations[frag.FIDName] {
			filteredRaw = append(filteredRaw, frag)
		}
	}

	filteredInfo := make(imageResourceInfoByID)
	for id, info := range imageResources {
		if usedResourceNames[info.ResourceName] {
			filteredInfo[id] = info
		}
	}

	return filteredExternal, filteredRaw, filteredInfo
}
