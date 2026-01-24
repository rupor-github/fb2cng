package kfx

import "testing"

func TestCollectAllEIDs(t *testing.T) {
	all := CollectAllEIDs(sectionEIDsBySectionName{
		"c0": {1002, 1001, 1001},
		"c1": {1004, 1003},
	})
	want := []int{1001, 1002, 1003, 1004}
	if len(all) != len(want) {
		t.Fatalf("len=%d want=%d", len(all), len(want))
	}
	for i := range want {
		if all[i] != want[i] {
			t.Fatalf("all[%d]=%d want=%d", i, all[i], want[i])
		}
	}
}

func TestBuildPositionMap(t *testing.T) {
	f := BuildPositionMap(sectionNameList{"c0"}, sectionEIDsBySectionName{"c0": {1000, 1001, 1002}})
	if f.FType != SymPositionMap || f.FID != SymPositionMap {
		t.Fatalf("unexpected fragment key: %v/%v", f.FType, f.FID)
	}
	lst, ok := f.Value.(ListValue)
	if !ok || len(lst) != 1 {
		t.Fatalf("unexpected value type/len: %T %d", f.Value, len(lst))
	}
	entry, ok := lst[0].(StructValue)
	if !ok {
		t.Fatalf("unexpected entry type: %T", lst[0])
	}
	if _, ok := entry[SymSectionName]; !ok {
		t.Fatalf("missing section_name")
	}
	if _, ok := entry[SymContainsIds]; !ok {
		t.Fatalf("missing contains")
	}
}

func TestBuildLocationMap(t *testing.T) {
	// Test with position items that have enough text to generate multiple locations
	// 40 positions per location, so 250 chars should give us 7 locations (at 0,40,80,120,160,200,240)
	items := []PositionItem{
		{EID: 1000, Length: 150}, // covers PIDs 0-149, crosses location boundaries at 0, 40, 80, 120
		{EID: 1001, Length: 100}, // covers PIDs 150-249, crosses location boundaries at 160, 200, 240
	}
	f := BuildLocationMap(items)
	if f.FType != SymLocationMap || f.FID != SymLocationMap {
		t.Fatalf("unexpected fragment key: %v/%v", f.FType, f.FID)
	}
	lst, ok := f.Value.(ListValue)
	if !ok || len(lst) != 1 {
		t.Fatalf("unexpected value type/len: %T %d", f.Value, len(lst))
	}
	locStruct, ok := lst[0].(StructValue)
	if !ok {
		t.Fatalf("unexpected location_map[0] type: %T", lst[0])
	}
	// Verify locations list exists and has expected entries
	locs, ok := locStruct.GetList(SymLocations)
	if !ok {
		t.Fatalf("missing locations list")
	}
	// With 250 total positions (150+100), we expect 7 locations:
	// - Location 0 at PID 0 (EID 1000, offset 0)
	// - Location 1 at PID 40 (EID 1000, offset 40)
	// - Location 2 at PID 80 (EID 1000, offset 80)
	// - Location 3 at PID 120 (EID 1000, offset 120)
	// - Location 4 at PID 160 (EID 1001, offset 10 since 160-150=10)
	// - Location 5 at PID 200 (EID 1001, offset 50 since 200-150=50)
	// - Location 6 at PID 240 (EID 1001, offset 90 since 240-150=90)
	if len(locs) != 7 {
		t.Fatalf("expected 7 location entries, got %d", len(locs))
	}

	// Verify first location has EID 1000 with no offset (or offset 0)
	loc0, ok := locs[0].(StructValue)
	if !ok {
		t.Fatalf("expected StructValue for loc[0], got %T", locs[0])
	}
	if eid, ok := loc0.GetInt(SymUniqueID); !ok || eid != 1000 {
		t.Fatalf("expected loc[0] EID=1000, got %v", eid)
	}

	// Verify second location has EID 1000 with offset 40
	loc1, ok := locs[1].(StructValue)
	if !ok {
		t.Fatalf("expected StructValue for loc[1], got %T", locs[1])
	}
	if eid, ok := loc1.GetInt(SymUniqueID); !ok || eid != 1000 {
		t.Fatalf("expected loc[1] EID=1000, got %v", eid)
	}
	if offset, ok := loc1.GetInt(SymOffset); !ok || offset != 40 {
		t.Fatalf("expected loc[1] offset=40, got %v", offset)
	}

	// Verify location 4 has EID 1001 with offset 10 (PID 160 - 150 = 10)
	loc4, ok := locs[4].(StructValue)
	if !ok {
		t.Fatalf("expected StructValue for loc[4], got %T", locs[4])
	}
	if eid, ok := loc4.GetInt(SymUniqueID); !ok || eid != 1001 {
		t.Fatalf("expected loc[4] EID=1001, got %v", eid)
	}
	if offset, ok := loc4.GetInt(SymOffset); !ok || offset != 10 {
		t.Fatalf("expected loc[4] offset=10, got %v", offset)
	}
}

func TestBuildPositionIdMapFragmentSparse(t *testing.T) {
	items := []PositionItem{{EID: 1000, Length: 1}, {EID: 1001, Length: 1000}}
	f := BuildPositionIDMap(nil, items)
	lst, ok := f.Value.(ListValue)
	if !ok || len(lst) < 3 {
		t.Fatalf("unexpected value type/len: %T %d", f.Value, len(lst))
	}
	first, _ := lst[0].(StructValue)
	if first == nil {
		t.Fatalf("unexpected first entry type: %T", lst[0])
	}
	if _, ok := first[SymPositionID]; !ok {
		t.Fatalf("missing pid")
	}
	if _, ok := first[SymElementID]; !ok {
		t.Fatalf("missing eid")
	}
}
