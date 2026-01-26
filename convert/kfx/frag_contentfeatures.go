package kfx

// ContentFeatureInfo holds information about content that determines which
// features should be included in content_features ($585).
// This matches KP3 behavior where features are conditionally added based on content.
type ContentFeatureInfo struct {
	// ReflowSectionSize is the computed version from max section PID count.
	// Only added when > 1 (sections exceed 65536 PIDs).
	ReflowSectionSize int

	// HasTables indicates whether the book contains any tables.
	// When true, yj_table feature is added.
	HasTables bool

	// HasTableWithLinks indicates whether any data table has links.
	// When true, yj_table_viewer feature is added.
	HasTableWithLinks bool

	// MaxImageWidth is the maximum image width in pixels.
	// Used to determine if yj_hdv feature should be added (>1920px).
	MaxImageWidth int

	// MaxImageHeight is the maximum image height in pixels.
	// Used to determine if yj_hdv feature should be added (>1920px).
	MaxImageHeight int

	// Language is the book's language code (e.g., "ru", "en").
	// Used to determine which reflow-language feature to add.
	Language string
}

// HasHDVImages returns true if any image exceeds 1920px in width or height.
func (c *ContentFeatureInfo) HasHDVImages() bool {
	return c.MaxImageWidth > 1920 || c.MaxImageHeight > 1920
}

// BuildContentFeatures creates the $585 content_features root fragment.
// Features are conditionally added based on content, matching KP3 behavior:
//   - reflow-style (v11): Always added for reflowable books
//   - reflow-section-size: Only when sections exceed 65536 PIDs
//   - reflow-language-expansion: Only for specific languages (Latin-based with expansion needs)
//   - yj_table (v6): Only when book has tables
//   - yj_table_viewer (v2): Only when data tables have links
//   - yj_hdv (v1): Only when images exceed 1920px
//   - CanonicalFormat (v1): Always added
func BuildContentFeatures(info *ContentFeatureInfo) *Fragment {
	features := []any{}

	// reflow-language-expansion - language expansion support
	// Added for Latin-based languages that may need character expansion.
	// KP3 checks for specific language families; we use a simplified check.
	if shouldAddReflowLanguageExpansion(info.Language) {
		features = append(features, map[KFXSymbol]any{
			SymKey:       "reflow-language-expansion",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[KFXSymbol]any{SymMajorVersion: int64(1), SymMinorVersion: int64(0)},
			},
		})
	}

	// yj_table - reflowable table support (version 6)
	// Only added when book contains tables.
	if info.HasTables {
		features = append(features, map[KFXSymbol]any{
			SymKey:       "yj_table",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[KFXSymbol]any{SymMajorVersion: int64(6), SymMinorVersion: int64(0)},
			},
		})
	}

	// reflow-style - reflowable layout version (version 11 matches KP3)
	// Always added for reflowable books.
	features = append(features, map[KFXSymbol]any{
		SymKey:       "reflow-style",
		SymNamespace: "com.amazon.yjconversion",
		SymVersionInfo: map[string]any{
			"version": map[KFXSymbol]any{SymMajorVersion: int64(11), SymMinorVersion: int64(0)},
		},
	})

	// yj_table_viewer - table viewer support (version 2)
	// Only added when data tables have links.
	if info.HasTableWithLinks {
		features = append(features, map[KFXSymbol]any{
			SymKey:       "yj_table_viewer",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[KFXSymbol]any{SymMajorVersion: int64(2), SymMinorVersion: int64(0)},
			},
		})
	}

	// CanonicalFormat - SDK marker for canonical KFX format
	// Always added.
	features = append(features, map[KFXSymbol]any{
		SymKey:       "CanonicalFormat",
		SymNamespace: "SDK.Marker",
		SymVersionInfo: map[string]any{
			"version": map[KFXSymbol]any{SymMajorVersion: int64(1), SymMinorVersion: int64(0)},
		},
	})

	// yj_hdv - HDV (high-definition) image support
	// Only added when images exceed 1920px in width or height.
	if info.HasHDVImages() {
		features = append(features, map[KFXSymbol]any{
			SymKey:       "yj_hdv",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[KFXSymbol]any{SymMajorVersion: int64(1), SymMinorVersion: int64(0)},
			},
		})
	}

	// reflow-section-size - computed from maximum per-section PID count
	// Only added when sections exceed 65536 PIDs (per KP3 behavior).
	if info.ReflowSectionSize > 1 {
		features = append(features, map[KFXSymbol]any{
			SymKey:       "reflow-section-size",
			SymNamespace: "com.amazon.yjconversion",
			SymVersionInfo: map[string]any{
				"version": map[KFXSymbol]any{SymMajorVersion: int64(info.ReflowSectionSize), SymMinorVersion: int64(0)},
			},
		})
	}

	return NewRootFragment(SymContentFeatures, map[KFXSymbol]any{SymFeatures: features})
}

// shouldAddReflowLanguageExpansion determines if the reflow-language-expansion
// feature should be added based on the book's language.
//
// KP3 adds different language features based on language family:
//   - ar-reflow-language: Arabic
//   - cn-reflow-language: Chinese
//   - tcn-reflow-language: Traditional Chinese
//   - jp-reflow-language: Japanese
//   - he-reflow-language: Hebrew
//   - fa-reflow-language: Farsi/Persian
//   - indic-reflow-language: Indic languages
//   - reflow-language-expansion: Latin-based languages with expansion needs
//
// For simplicity, we add reflow-language-expansion for common Latin-based languages
// that may benefit from character expansion (hyphenation, ligatures, etc.).
func shouldAddReflowLanguageExpansion(lang string) bool {
	if lang == "" {
		return false
	}

	// Languages that typically use Latin script and benefit from language expansion.
	// This includes most European languages.
	// KP3 adds this for specific language conditions (var14/var15 in m.java:82-89).
	switch lang {
	case "en", "de", "fr", "es", "it", "pt", "nl", "pl", "cs", "sk", "hu",
		"ro", "bg", "hr", "sl", "sr", "uk", "be", "lt", "lv", "et",
		"fi", "sv", "no", "da", "is", "ga", "cy", "eu", "ca", "gl",
		"ru", "el", "tr", "vi", "id", "ms", "tl", "sw":
		return true
	default:
		return false
	}
}
