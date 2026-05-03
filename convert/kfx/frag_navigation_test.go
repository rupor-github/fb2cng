package kfx

import (
	"reflect"
	"testing"

	"fbc/common"
)

func TestBuildNavEntries_UntitledWrapperPromotesChildren(t *testing.T) {
	// Simulate the FB2 pattern where a top-level section has no title
	// but contains titled subsections:
	//   <section>          ← untitled wrapper (IncludeInTOC=false)
	//     <section><title>Chapter 1</title>...</section>
	//     <section><title>Chapter 2</title>...</section>
	//   </section>
	entries := []*TOCEntry{
		{
			ID:           "wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{ID: "ch1", Title: "Chapter 1", IncludeInTOC: true, FirstEID: 1001},
				{ID: "ch2", Title: "Chapter 2", IncludeInTOC: true, FirstEID: 1002},
			},
		},
	}

	result := buildNavEntries(entries, 1000, false, common.TOCTypeNormal)
	if len(result) != 2 {
		t.Fatalf("expected 2 promoted entries, got %d", len(result))
	}
}

func TestBuildNavEntries_MixedTitledAndWrapper(t *testing.T) {
	// Mix of titled top-level entries and untitled wrappers
	entries := []*TOCEntry{
		{ID: "intro", Title: "Introduction", IncludeInTOC: true, FirstEID: 1000},
		{
			ID:           "wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1010,
			Children: []*TOCEntry{
				{ID: "ch1", Title: "Part 1", IncludeInTOC: true, FirstEID: 1011},
				{ID: "ch2", Title: "Part 2", IncludeInTOC: true, FirstEID: 1012},
			},
		},
		{ID: "epilog", Title: "Epilogue", IncludeInTOC: true, FirstEID: 1020},
	}

	result := buildNavEntries(entries, 1000, false, common.TOCTypeNormal)
	// Introduction + Part 1 + Part 2 (promoted) + Epilogue = 4
	if len(result) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(result))
	}
}

func TestBuildNavEntries_NestedWrappers(t *testing.T) {
	// Edge case: wrapper inside a wrapper
	entries := []*TOCEntry{
		{
			ID:           "outer-wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{
					ID:           "inner-wrapper",
					Title:        "",
					IncludeInTOC: false,
					FirstEID:     1001,
					Children: []*TOCEntry{
						{ID: "ch1", Title: "Deep Chapter", IncludeInTOC: true, FirstEID: 1002},
					},
				},
			},
		},
	}

	result := buildNavEntries(entries, 1000, false, common.TOCTypeNormal)
	if len(result) != 1 {
		t.Fatalf("expected 1 promoted entry from nested wrappers, got %d", len(result))
	}
}

func TestBuildNavEntries_WrapperWithNoChildren(t *testing.T) {
	// An untitled wrapper with no children should produce no entries
	entries := []*TOCEntry{
		{
			ID:           "empty-wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
		},
	}

	result := buildNavEntries(entries, 1000, false, common.TOCTypeNormal)
	if len(result) != 0 {
		t.Fatalf("expected 0 entries for empty wrapper, got %d", len(result))
	}
}

func TestBuildNavEntries_TitledEntryWithChildren(t *testing.T) {
	// A titled entry with children should create a hierarchical navUnit
	// (children nested inside, not promoted)
	entries := []*TOCEntry{
		{
			ID:           "part1",
			Title:        "Part 1",
			IncludeInTOC: true,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{ID: "ch1", Title: "Chapter 1", IncludeInTOC: true, FirstEID: 1001},
				{ID: "ch2", Title: "Chapter 2", IncludeInTOC: true, FirstEID: 1002},
			},
		},
	}

	result := buildNavEntries(entries, 1000, false, common.TOCTypeNormal)
	// Should be 1 entry (Part 1) with children nested inside
	if len(result) != 1 {
		t.Fatalf("expected 1 top-level entry, got %d", len(result))
	}
}

func TestBuildNavEntries_TOCTypeNesting(t *testing.T) {
	entries := []*TOCEntry{{
		ID:           "a",
		Title:        "A",
		IncludeInTOC: true,
		FirstEID:     1000,
		Children: []*TOCEntry{{
			ID:           "b",
			Title:        "B",
			IncludeInTOC: true,
			FirstEID:     1001,
			Children: []*TOCEntry{{
				ID:           "c",
				Title:        "C",
				IncludeInTOC: true,
				FirstEID:     1002,
				Children: []*TOCEntry{{
					ID:           "d",
					Title:        "D",
					IncludeInTOC: true,
					FirstEID:     1003,
				}},
			}},
		}},
	}}

	tests := []struct {
		name    string
		tocType common.TOCType
		want    []navSnapshot
	}{
		{
			name:    "normal",
			tocType: common.TOCTypeNormal,
			want:    []navSnapshot{{Title: "A", Children: []navSnapshot{{Title: "B", Children: []navSnapshot{{Title: "C", Children: []navSnapshot{{Title: "D"}}}}}}}},
		},
		{
			name:    "old kindle",
			tocType: common.TOCTypeOldKindle,
			want:    []navSnapshot{{Title: "A", Children: []navSnapshot{{Title: "B"}, {Title: "C"}, {Title: "D"}}}},
		},
		{
			name:    "flat",
			tocType: common.TOCTypeFlat,
			want:    []navSnapshot{{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotNavUnits(buildNavEntries(entries, 1000, false, tt.tocType))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildNavEntries() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

type navSnapshot struct {
	Title    string
	Children []navSnapshot
}

func snapshotNavUnits(entries []any) []navSnapshot {
	out := make([]navSnapshot, 0, len(entries))
	for _, entry := range entries {
		unit, ok := entry.(StructValue)
		if !ok {
			continue
		}
		var title string
		if repr, ok := unit.GetStruct(SymRepresentation); ok {
			if label, ok := repr.GetString(SymLabel); ok {
				title = label
			}
		}
		var children []navSnapshot
		if nested, ok := unit.GetList(SymEntries); ok {
			children = snapshotNavUnits(nested)
		}
		out = append(out, navSnapshot{Title: title, Children: children})
	}
	return out
}

func TestBuildTOCEntryTree_UntitledWrapperPromotesChildren(t *testing.T) {
	entries := []*TOCEntry{
		{
			ID:           "wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{ID: "ch1", Title: "Chapter 1", IncludeInTOC: true, FirstEID: 1001},
				{ID: "ch2", Title: "Chapter 2", IncludeInTOC: true, FirstEID: 1002},
				{ID: "ch3", Title: "Chapter 3", IncludeInTOC: true, FirstEID: 1003},
			},
		},
	}

	result := buildTOCEntryTree(entries, false)
	if len(result) != 3 {
		t.Fatalf("expected 3 promoted entries, got %d", len(result))
	}
	for i, want := range []string{"Chapter 1", "Chapter 2", "Chapter 3"} {
		if result[i].Title != want {
			t.Errorf("entry[%d].Title = %q, want %q", i, result[i].Title, want)
		}
	}
}

func TestBuildTOCEntryTree_MixedTitledAndWrapper(t *testing.T) {
	entries := []*TOCEntry{
		{ID: "ann", Title: "Annotation", IncludeInTOC: true, FirstEID: 900},
		{
			ID:           "wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{ID: "ch1", Title: "Chapter 1", IncludeInTOC: true, FirstEID: 1001},
				{ID: "ch2", Title: "Chapter 2", IncludeInTOC: true, FirstEID: 1002},
			},
		},
		{ID: "notes", Title: "Notes", IncludeInTOC: true, FirstEID: 2000},
	}

	result := buildTOCEntryTree(entries, false)
	// Annotation + Chapter 1 + Chapter 2 (promoted) + Notes = 4
	if len(result) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(result))
	}

	expected := []string{"Annotation", "Chapter 1", "Chapter 2", "Notes"}
	for i, want := range expected {
		if result[i].Title != want {
			t.Errorf("entry[%d].Title = %q, want %q", i, result[i].Title, want)
		}
	}
}

func TestBuildTOCEntryTree_NestedWrappers(t *testing.T) {
	entries := []*TOCEntry{
		{
			ID:           "outer",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{
					ID:           "inner",
					Title:        "",
					IncludeInTOC: false,
					FirstEID:     1001,
					Children: []*TOCEntry{
						{ID: "ch1", Title: "Deep Chapter", IncludeInTOC: true, FirstEID: 1002},
					},
				},
			},
		},
	}

	result := buildTOCEntryTree(entries, false)
	if len(result) != 1 {
		t.Fatalf("expected 1 promoted entry from nested wrappers, got %d", len(result))
	}
	if result[0].Title != "Deep Chapter" {
		t.Errorf("entry[0].Title = %q, want %q", result[0].Title, "Deep Chapter")
	}
}

func TestBuildTOCEntryTree_PreservesAnchorIDs(t *testing.T) {
	entries := []*TOCEntry{
		{
			ID:           "wrapper",
			Title:        "",
			IncludeInTOC: false,
			FirstEID:     1000,
			Children: []*TOCEntry{
				{ID: "sec-001", Title: "Chapter 1", IncludeInTOC: true, FirstEID: 1001},
			},
		},
	}

	result := buildTOCEntryTree(entries, false)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].AnchorID != "sec-001" {
		t.Errorf("AnchorID = %q, want %q", result[0].AnchorID, "sec-001")
	}
	if result[0].FirstEID != 1001 {
		t.Errorf("FirstEID = %d, want %d", result[0].FirstEID, 1001)
	}
}
