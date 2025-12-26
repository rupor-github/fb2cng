package kfx

// BuildContainerEntityMapFragment creates the $419 container_entity_map fragment
// with entity dependencies ($253). Dependencies describe which fragments depend on which resources.
func BuildContainerEntityMapFragment(
	containerID string,
	fragments *FragmentList,
	dependencies []EntityDependency,
) *Fragment {
	// Build list of entity IDs
	entityIDs := make([]any, 0, fragments.Len())
	for _, frag := range fragments.All() {
		if CONTAINER_FRAGMENT_TYPES[frag.FType] {
			continue
		}
		// Use fragment ID as symbol - handle both numeric IDs and named IDs
		if frag.FIDName != "" {
			entityIDs = append(entityIDs, SymbolByName(frag.FIDName))
		} else {
			entityIDs = append(entityIDs, SymbolValue(frag.FID))
		}
	}

	// Build container entry
	containerEntry := NewContainerEntry(containerID, entityIDs)

	// Build container entity map
	entityMap := NewContainerEntityMap([]any{containerEntry})

	// Add dependencies if present
	if len(dependencies) > 0 {
		depList := make([]any, 0, len(dependencies))
		for _, dep := range dependencies {
			depEntry := NewStruct().
				SetSymbol(SymUniqueID, dep.FragmentID) // $155 = id

			if len(dep.MandatoryDeps) > 0 {
				mandList := make([]any, len(dep.MandatoryDeps))
				for i, id := range dep.MandatoryDeps {
					mandList[i] = SymbolValue(id)
				}
				depEntry.SetList(SymMandatoryDeps, mandList) // $254 = mandatory_dependencies
			}

			if len(dep.OptionalDeps) > 0 {
				optList := make([]any, len(dep.OptionalDeps))
				for i, id := range dep.OptionalDeps {
					optList[i] = SymbolValue(id)
				}
				depEntry.SetList(SymOptionalDeps, optList) // $255 = optional_dependencies
			}

			depList = append(depList, depEntry)
		}
		entityMap.SetList(SymEntityDeps, depList) // $253 = entity_dependencies
	}

	return NewRootFragment(SymContEntityMap, entityMap)
}

// EntityDependency describes dependencies of a fragment on other fragments.
type EntityDependency struct {
	FragmentID    int   // The fragment that has dependencies
	MandatoryDeps []int // Required dependencies (e.g., section -> resources)
	OptionalDeps  []int // Optional dependencies (e.g., resource -> raw media)
}

// ComputeEntityDependencies analyzes fragments and computes their dependencies.
// This is used for building the $253 entity_dependencies list.
func ComputeEntityDependencies(fragments *FragmentList) []EntityDependency {
	var deps []EntityDependency

	// Build resource location -> raw media mapping
	rawMediaByLocation := make(map[string]int)
	for _, frag := range fragments.GetByType(SymRawMedia) {
		// Raw media fragments use location ($165) as their key
		if v, ok := frag.Value.(StructValue); ok {
			if loc, ok := v.GetString(SymLocation); ok {
				rawMediaByLocation[loc] = frag.FID
			}
		}
	}

	// External resources ($164) depend on raw media ($417)
	for _, frag := range fragments.GetByType(SymExtResource) {
		v, ok := frag.Value.(StructValue)
		if !ok {
			continue
		}

		loc, ok := v.GetString(SymLocation)
		if !ok {
			continue
		}

		if rawID, exists := rawMediaByLocation[loc]; exists {
			deps = append(deps, EntityDependency{
				FragmentID:   frag.FID,
				OptionalDeps: []int{rawID},
			})
		}
	}

	return deps
}
