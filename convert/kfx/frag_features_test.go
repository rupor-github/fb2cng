package kfx

import "testing"

func TestBuildFormatCapabilitiesUsesDefaultFeatures(t *testing.T) {
	frag := BuildFormatCapabilities(nil)
	if frag.FType != SymFormatCapab || frag.FID != SymFormatCapab || !frag.IsRoot() {
		t.Fatalf("BuildFormatCapabilities() returned fragment %#v, want root format_capabilities", frag)
	}

	entries, ok := frag.Value.(ListValue)
	if !ok {
		t.Fatalf("format capabilities value type = %T, want ListValue", frag.Value)
	}
	if len(entries) != 1 {
		t.Fatalf("format capabilities entries = %d, want 1", len(entries))
	}

	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("entry type = %T, want map[string]any", entries[0])
	}
	if got := entry["$492"]; got != "kfxgen.textBlock" {
		t.Fatalf("feature key = %v, want kfxgen.textBlock", got)
	}
	if got := entry["version"]; got != 1 {
		t.Fatalf("feature version = %v, want 1", got)
	}
}

func TestBuildFormatCapabilitiesCustomFeaturesOmitZeroVersion(t *testing.T) {
	frag := BuildFormatCapabilities([]FormatFeature{
		{Key: "feature.with.version", Version: 3},
		{Key: "feature.without.version"},
	})
	entries := frag.Value.(ListValue)
	if len(entries) != 2 {
		t.Fatalf("format capabilities entries = %d, want 2", len(entries))
	}

	withVersion := entries[0].(map[string]any)
	if got := withVersion["version"]; got != 3 {
		t.Fatalf("first feature version = %v, want 3", got)
	}

	withoutVersion := entries[1].(map[string]any)
	if _, ok := withoutVersion["version"]; ok {
		t.Fatalf("zero-version feature should not include version: %#v", withoutVersion)
	}
}

func TestFormatFeaturePresets(t *testing.T) {
	defaults := DefaultFormatFeatures()
	if len(defaults) != 1 || defaults[0] != (FormatFeature{Key: "kfxgen.textBlock", Version: 1}) {
		t.Fatalf("DefaultFormatFeatures() = %#v", defaults)
	}

	withOffset := FormatFeaturesWithPIDMapOffset()
	if len(withOffset) != 2 {
		t.Fatalf("FormatFeaturesWithPIDMapOffset() len = %d, want 2", len(withOffset))
	}
	if withOffset[1] != (FormatFeature{Key: "kfxgen.pidMapWithOffset", Version: 1}) {
		t.Fatalf("FormatFeaturesWithPIDMapOffset()[1] = %#v", withOffset[1])
	}
}

func TestBuildContentFeaturesMinimal(t *testing.T) {
	keys := contentFeatureKeys(t, BuildContentFeatures(&ContentFeatureInfo{}))
	want := []string{"reflow-style", "CanonicalFormat"}
	assertStringSliceEqual(t, keys, want)
}

func TestBuildContentFeaturesConditionalEntries(t *testing.T) {
	frag := BuildContentFeatures(&ContentFeatureInfo{
		ReflowSectionSize: 7,
		HasTables:         true,
		HasTableWithLinks: true,
		MaxImageWidth:     1921,
		Language:          "en",
	})

	keys := contentFeatureKeys(t, frag)
	want := []string{
		"reflow-language-expansion",
		"yj_table",
		"reflow-style",
		"yj_table_viewer",
		"CanonicalFormat",
		"yj_hdv",
		"reflow-section-size",
	}
	assertStringSliceEqual(t, keys, want)

	features := contentFeatureList(t, frag)
	sectionSize := features[len(features)-1].(map[KFXSymbol]any)
	versionInfo := sectionSize[SymVersionInfo].(map[string]any)
	version := versionInfo["version"].(map[KFXSymbol]any)
	if got := version[SymMajorVersion]; got != int64(7) {
		t.Fatalf("reflow-section-size major version = %v, want 7", got)
	}
}

func TestContentFeatureInfoHasHDVImages(t *testing.T) {
	tests := []struct {
		name string
		info ContentFeatureInfo
		want bool
	}{
		{name: "below threshold", info: ContentFeatureInfo{MaxImageWidth: 1920, MaxImageHeight: 1920}, want: false},
		{name: "wide", info: ContentFeatureInfo{MaxImageWidth: 1921}, want: true},
		{name: "tall", info: ContentFeatureInfo{MaxImageHeight: 1921}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.HasHDVImages(); got != tt.want {
				t.Fatalf("HasHDVImages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldAddReflowLanguageExpansion(t *testing.T) {
	tests := []struct {
		lang string
		want bool
	}{
		{lang: "", want: false},
		{lang: "en", want: true},
		{lang: "ru", want: true},
		{lang: "ja", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := shouldAddReflowLanguageExpansion(tt.lang); got != tt.want {
				t.Fatalf("shouldAddReflowLanguageExpansion(%q) = %v, want %v", tt.lang, got, tt.want)
			}
		})
	}
}

func TestBuildResourcePath(t *testing.T) {
	frag := BuildResourcePath()
	if frag.FType != SymResourcePath || !frag.IsRoot() {
		t.Fatalf("BuildResourcePath() returned fragment %#v, want root resource_path", frag)
	}
	value, ok := frag.Value.(StructValue)
	if !ok {
		t.Fatalf("resource_path value type = %T, want StructValue", frag.Value)
	}
	entries, ok := value.GetList(SymEntries)
	if !ok {
		t.Fatal("resource_path missing entries list")
	}
	if len(entries) != 0 {
		t.Fatalf("resource_path entries = %d, want 0", len(entries))
	}
}

func TestBuildAuxiliaryDataFragmentsSkipsEmptyIDs(t *testing.T) {
	fragments := BuildAuxiliaryDataFragments(sectionNameList{"c1", "", "c2"})
	if len(fragments) != 2 {
		t.Fatalf("BuildAuxiliaryDataFragments() len = %d, want 2", len(fragments))
	}

	for i, frag := range fragments {
		wantFID := []string{"c1-ad", "c2-ad"}[i]
		if frag.FType != SymAuxiliaryData || frag.FIDName != wantFID {
			t.Fatalf("fragment[%d] = %#v, want auxiliary fid %q", i, frag, wantFID)
		}
		value, ok := frag.Value.(StructValue)
		if !ok {
			t.Fatalf("fragment[%d] value type = %T, want StructValue", i, frag.Value)
		}
		if got, _ := value.GetString(SymKfxID); got != wantFID {
			t.Fatalf("fragment[%d] kfx_id = %q, want %q", i, got, wantFID)
		}
		metadata, ok := value.GetList(SymMetadata)
		if !ok || len(metadata) != 1 {
			t.Fatalf("fragment[%d] metadata = %#v, want one entry", i, metadata)
		}
	}
}

func contentFeatureList(t *testing.T, frag *Fragment) []any {
	t.Helper()
	value, ok := frag.Value.(map[KFXSymbol]any)
	if !ok {
		t.Fatalf("content_features value type = %T, want map[KFXSymbol]any", frag.Value)
	}
	features, ok := value[SymFeatures].([]any)
	if !ok {
		t.Fatalf("content_features features type = %T, want []any", value[SymFeatures])
	}
	return features
}

func contentFeatureKeys(t *testing.T, frag *Fragment) []string {
	t.Helper()
	features := contentFeatureList(t, frag)
	keys := make([]string, 0, len(features))
	for _, feature := range features {
		featureMap, ok := feature.(map[KFXSymbol]any)
		if !ok {
			t.Fatalf("feature type = %T, want map[KFXSymbol]any", feature)
		}
		key, ok := featureMap[SymKey].(string)
		if !ok {
			t.Fatalf("feature key type = %T, want string", featureMap[SymKey])
		}
		keys = append(keys, key)
	}
	return keys
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len = %d, want %d: got %#v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q (got %#v)", i, got[i], want[i], got)
		}
	}
}
