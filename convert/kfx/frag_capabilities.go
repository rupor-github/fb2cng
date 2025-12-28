package kfx

// BuildFormatCapabilities creates the $593 format_capabilities fragment.
// This declares the KFX format features used in the book.
// The value is a list of feature entries, each with $492 (key) and version.
func BuildFormatCapabilities(features []FormatFeature) *Fragment {
	if len(features) == 0 {
		// Default minimal features
		features = DefaultFormatFeatures()
	}

	// Build list of feature entries
	// Each entry is a map[string]any with "$492" and "version" keys
	entries := make([]any, 0, len(features))
	for _, f := range features {
		// Use string-keyed map for mixed symbol/string keys
		entry := map[string]any{
			"$492": f.Key, // key field (symbol $492 encoded as string)
		}
		if f.Version != 0 {
			entry["version"] = f.Version
		}
		entries = append(entries, entry)
	}

	return NewRootFragment(SymFormatCapab, ListValue(entries))
}

// FormatFeature represents a single feature entry in format_capabilities.
type FormatFeature struct {
	Key     string // Feature key (e.g., "kfxgen.textBlock", "kfxgen.positionMaps")
	Version int    // Feature version number
}

// DefaultFormatFeatures returns the default format features for a basic KFX book.
func DefaultFormatFeatures() []FormatFeature {
	return []FormatFeature{
		{Key: "kfxgen.textBlock", Version: 1},
	}
}
