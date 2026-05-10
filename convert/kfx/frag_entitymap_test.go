package kfx

import "testing"

func TestBuildContainerEntityMapSkipsContainerAndMetadataRoots(t *testing.T) {
	fragments := NewFragmentList()
	mustAddFragment(t, fragments, NewRootFragment(SymContainer, NewStruct()))
	mustAddFragment(t, fragments, NewRootFragment(SymMetadata, NewStruct()))
	mustAddFragment(t, fragments, NewRootFragment(SymBookMetadata, NewStruct()))
	mustAddFragment(t, fragments, &Fragment{FType: SymSection, FIDName: "c1", Value: NewStruct()})
	mustAddFragment(t, fragments, NewFragment(SymStoryline, 1001, NewStruct()))

	frag := BuildContainerEntityMap("container", fragments, nil)
	if frag.FType != SymContEntityMap || !frag.IsRoot() {
		t.Fatalf("BuildContainerEntityMap() returned %#v, want root container_entity_map", frag)
	}

	value := frag.Value.(StructValue)
	containers, ok := value.GetList(SymContainerList)
	if !ok || len(containers) != 1 {
		t.Fatalf("container list = %#v, want one entry", containers)
	}
	container := containers[0].(StructValue)
	if got, _ := container.GetString(SymUniqueID); got != "container" {
		t.Fatalf("container id = %q, want container", got)
	}

	ids, ok := container.GetList(SymContainsIds)
	if !ok {
		t.Fatal("container entry missing contains list")
	}
	if len(ids) != 2 {
		t.Fatalf("contains ids = %#v, want named section and numeric storyline", ids)
	}
	if got := ids[0]; got != SymbolByNameValue("c1") {
		t.Fatalf("contains[0] = %#v, want SymbolByNameValue(c1)", got)
	}
	if got := ids[1]; got != SymbolValue(1001) {
		t.Fatalf("contains[1] = %#v, want SymbolValue(1001)", got)
	}
}

func TestBuildContainerEntityMapIncludesDependencies(t *testing.T) {
	fragments := NewFragmentList()
	mustAddFragment(t, fragments, &Fragment{FType: SymSection, FID: 10, Value: NewStruct()})

	frag := BuildContainerEntityMap("container", fragments, []EntityDependency{{
		FragmentID:    10,
		MandatoryDeps: []KFXSymbol{20, 21},
		OptionalDeps:  []KFXSymbol{30},
	}})

	value := frag.Value.(StructValue)
	deps, ok := value.GetList(SymEntityDeps)
	if !ok || len(deps) != 1 {
		t.Fatalf("entity deps = %#v, want one entry", deps)
	}

	dep := deps[0].(StructValue)
	if got, _ := dep.GetSymbol(SymUniqueID); got != SymbolValue(10) {
		t.Fatalf("dep id = %v, want 10", got)
	}
	mandatory, ok := dep.GetList(SymMandatoryDeps)
	if !ok {
		t.Fatal("dependency missing mandatory list")
	}
	if got := mandatory; len(got) != 2 || got[0] != SymbolValue(20) || got[1] != SymbolValue(21) {
		t.Fatalf("mandatory deps = %#v, want [20 21]", got)
	}
	optional, ok := dep.GetList(SymOptionalDeps)
	if !ok {
		t.Fatal("dependency missing optional list")
	}
	if got := optional; len(got) != 1 || got[0] != SymbolValue(30) {
		t.Fatalf("optional deps = %#v, want [30]", got)
	}
}

func TestComputeEntityDependenciesLinksExternalResourcesToMatchingRawMedia(t *testing.T) {
	fragments := NewFragmentList()
	mustAddFragment(t, fragments, NewFragment(SymRawMedia, 41, NewStruct().SetString(SymLocation, "resource/rsrc1")))
	mustAddFragment(t, fragments, NewFragment(SymRawMedia, 42, NewStruct().SetString(SymLocation, "resource/rsrc2")))
	mustAddFragment(t, fragments, NewFragment(SymExtResource, 51, NewStruct().SetString(SymLocation, "resource/rsrc2")))
	mustAddFragment(t, fragments, NewFragment(SymExtResource, 52, NewStruct().SetString(SymLocation, "resource/missing")))
	mustAddFragment(t, fragments, NewFragment(SymExtResource, 53, "not a struct"))

	deps := ComputeEntityDependencies(fragments)
	if len(deps) != 1 {
		t.Fatalf("ComputeEntityDependencies() len = %d, want 1: %#v", len(deps), deps)
	}
	dep := deps[0]
	if dep.FragmentID != 51 {
		t.Fatalf("dependency fragment id = %v, want 51", dep.FragmentID)
	}
	if len(dep.OptionalDeps) != 1 || dep.OptionalDeps[0] != 42 {
		t.Fatalf("optional deps = %#v, want [42]", dep.OptionalDeps)
	}
	if len(dep.MandatoryDeps) != 0 {
		t.Fatalf("mandatory deps = %#v, want empty", dep.MandatoryDeps)
	}
}

func mustAddFragment(t *testing.T, fragments *FragmentList, frag *Fragment) {
	t.Helper()
	if err := fragments.Add(frag); err != nil {
		t.Fatalf("Add(%v): %v", frag, err)
	}
}
