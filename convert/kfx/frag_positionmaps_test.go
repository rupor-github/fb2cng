package kfx

import "testing"

func TestCollectAllEIDs(t *testing.T) {
	all := CollectAllEIDs(map[string][]int{
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

func TestBuildPositionMapFragment(t *testing.T) {
	f := BuildPositionMapFragment([]string{"c0"}, map[string][]int{"c0": {1000, 1001, 1002}})
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

func TestBuildLocationMapFragment(t *testing.T) {
	f := BuildLocationMapFragment([]int{1000, 1001})
	if f.FType != SymLocationMap || f.FID != SymLocationMap {
		t.Fatalf("unexpected fragment key: %v/%v", f.FType, f.FID)
	}
	lst, ok := f.Value.(ListValue)
	if !ok || len(lst) != 1 {
		t.Fatalf("unexpected value type/len: %T %d", f.Value, len(lst))
	}
	_, ok = lst[0].(StructValue)
	if !ok {
		t.Fatalf("unexpected location_map[0] type: %T", lst[0])
	}
}

func TestBuildPositionIdMapFragmentSparse(t *testing.T) {
	items := []PositionItem{{EID: 1000, Length: 1}, {EID: 1001, Length: 1000}}
	f := BuildPositionIdMapFragment(nil, items)
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
