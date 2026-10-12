package envelope

const (
	// PageSize is the minimum and legacy decoded page size.
	PageSize = 4096
	// MaxPageSize bounds decoded pages independently of untrusted metadata.
	MaxPageSize     = 64 * 1024
	targetPageCount = 256
)

// adaptivePageSize aims for at most 256 pages, subject to the 64-KiB cap.
// Use only page sizes verified with the native SDK and larger-page KPV controls.
func adaptivePageSize(size int) int {
	for _, pageSize := range [...]int{PageSize, 16 * 1024, 32 * 1024, MaxPageSize} {
		if size <= pageSize*targetPageCount {
			return pageSize
		}
	}
	return MaxPageSize
}

func supportedPageSize(size uint64) bool {
	switch size {
	case PageSize, 16 * 1024, 32 * 1024, MaxPageSize:
		return true
	default:
		return false
	}
}
