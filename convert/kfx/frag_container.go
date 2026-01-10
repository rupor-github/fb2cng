package kfx

// BuildContainerFragment creates a $270 fragment from Container state. This is
// used when the container already has its configuration set.
// NOTE: $270 (container) fragment is not stored as an ENTY; synthesize it for
// tooling/debug.
func BuildContainerFragment(c *Container) *Fragment {
	container := NewStruct()

	// $409 container id
	if c.ContainerID != "" {
		container.SetString(SymContainerId, c.ContainerID)
	}

	// $412 chunk size
	container.SetInt(SymChunkSize, int64(c.ChunkSize))

	// $410 compression type
	container.SetInt(SymComprType, int64(c.CompressionType))

	// $411 DRM scheme
	container.SetInt(SymDRMScheme, int64(c.DRMScheme))

	// $587 major_version - generator application
	if c.GeneratorApp != "" {
		container.SetString(SymMajorVersion, c.GeneratorApp)
	}

	// $588 minor_version - generator package
	if c.GeneratorPkg != "" {
		container.SetString(SymMinorVersion, c.GeneratorPkg)
	}

	// $161 format - container format label
	if c.ContainerFormat != "" {
		container.SetString(SymFormat, c.ContainerFormat)
	}

	// $181 contains - build from fragments
	if c.Fragments != nil && c.Fragments.Len() > 0 {
		entities := make([]any, 0, c.Fragments.Len())
		for _, frag := range c.Fragments.All() {
			// Skip container-level fragments (they don't go in entity list)
			if CONTAINER_FRAGMENT_TYPES[frag.FType] {
				continue
			}
			entities = append(entities, []any{
				int64(frag.FType),
				int64(frag.FID),
			})
		}
		if len(entities) > 0 {
			container.SetList(SymContainsIds, entities)
		}
	}

	return NewRootFragment(SymContainer, container)
}
