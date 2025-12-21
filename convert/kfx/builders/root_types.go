package builders

// Root fragment types that are typically singleton in a KFX main container.
//
// NOTE: these are the fragment *types* (ftype). Their fragment IDs (fid) for
// root fragments must match the type (fid == ftype).
const (
	FragTypeContainerInfo      = "$270"
	FragTypeFormatCapabilities = "$593"
	FragTypeBookFeatures       = "$585"
	FragTypeDocumentData       = "$538"
	FragTypeMetadataRO         = "$258"
	FragTypeMetadata           = "$490"
	FragTypeEntityMap          = "$419"
	FragTypeNavigation         = "$389"
	FragTypePositionBuckets    = "$550"
	FragTypePidMap             = "$265"
	FragTypePidMapWithOffset   = "$264"
	FragTypeResourcePath       = "$395"
)
