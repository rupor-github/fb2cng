package kfx

// BuildContentFeatures creates the $585 content_features root fragment.
// Reference KFX stores reflow-* and CanonicalFormat features here (not in $593).
func BuildContentFeatures(reflowSectionSize int) *Fragment {
	features := []any{
		map[int]any{
			SymKey:       "reflow-style",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[int]any{SymMajorVersion: int64(1), SymMinorVersion: int64(0)},
			},
		},
		map[int]any{
			SymKey:       "reflow-section-size",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[int]any{SymMajorVersion: int64(reflowSectionSize), SymMinorVersion: int64(0)},
			},
		},
		map[int]any{
			SymKey:       "reflow-language-expansion",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[int]any{SymMajorVersion: int64(1), SymMinorVersion: int64(0)},
			},
		},
		map[int]any{
			SymKey:       "CanonicalFormat",
			SymNamespace: "SDK.Marker",
			SymVersionInfo: map[string]any{
				"version": map[int]any{SymMajorVersion: int64(1), SymMinorVersion: int64(0)},
			},
		},
	}

	return NewRootFragment(SymContentFeatures, map[int]any{SymFeatures: features})
}
