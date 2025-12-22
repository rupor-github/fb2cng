package builders

// BuildConversionFeatures builds fragment $585 (conversion features).
// Only the minimal subset needed to satisfy kfxlib validations is emitted.
func BuildConversionFeatures(reflowSectionSize int64) any {
	if reflowSectionSize <= 0 {
		reflowSectionSize = 1
	}

	type versionMajorMinor struct {
		Major int64 `ion:"$587"`
		Minor int64 `ion:"$588,omitempty"`
	}
	type versionInfo struct {
		Version versionMajorMinor `ion:"version"`
	}
	type conversionFeature struct {
		Namespace string      `ion:"$586"`
		Feature   string      `ion:"$492"`
		VInfo     versionInfo `ion:"$589"`
	}
	type conversionFeatures struct {
		Features []any `ion:"$590"`
	}

	return conversionFeatures{Features: []any{
		conversionFeature{Namespace: "com.amazon.yjconversion", Feature: "reflow-style", VInfo: versionInfo{Version: versionMajorMinor{Major: 11, Minor: 0}}},
		conversionFeature{Namespace: "com.amazon.yjconversion", Feature: "reflow-language-expansion", VInfo: versionInfo{Version: versionMajorMinor{Major: 1, Minor: 0}}},
		conversionFeature{Namespace: "SDK.Marker", Feature: "CanonicalFormat", VInfo: versionInfo{Version: versionMajorMinor{Major: 1}}},
		conversionFeature{Namespace: "com.amazon.yjconversion", Feature: "reflow-section-size", VInfo: versionInfo{Version: versionMajorMinor{Major: reflowSectionSize}}},
	}}
}
