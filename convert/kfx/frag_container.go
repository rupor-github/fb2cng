package kfx

import (
	"fbc/misc"
)

// BuildContainerFragment creates the $270 container fragment.
// This is a root fragment that contains container-level metadata.
func BuildContainerFragment(containerID string, entityList []FragmentKey) *Fragment {
	container := NewStruct()

	// $409 container id
	container.SetString(SymContainerId, containerID)

	// $412 chunk size
	container.SetInt(SymChunkSize, DefaultChunkSize)

	// $410 compression type (always 0 - no compression)
	container.SetInt(SymComprType, 0)

	// $411 DRM scheme (always 0 - no DRM)
	container.SetInt(SymDRMScheme, 0)

	// $587 major_version - generator application version
	container.SetString(SymMajorVersion, misc.GetAppName())

	// $588 minor_version - generator package version
	container.SetString(SymMinorVersion, misc.GetVersion())

	// $161 format - container format label
	container.SetString(SymFormat, "KFX main")

	// $181 contains - list of entity [type, id] pairs
	if len(entityList) > 0 {
		entities := make([]any, 0, len(entityList))
		for _, key := range entityList {
			// Each entity is represented as [type_idnum, id_idnum]
			entities = append(entities, []any{
				int64(key.FType),
				int64(key.FID),
			})
		}
		container.SetList(SymContainsIds, entities)
	}

	return NewRootFragment(SymContainer, container)
}

// BuildContainerFragmentFromContainer creates a $270 fragment from Container state.
// This is used when the container already has its configuration set.
func BuildContainerFragmentFromContainer(c *Container) *Fragment {
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
