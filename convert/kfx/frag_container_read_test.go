package kfx

import "testing"

func TestReadContainerSynthesizesContainerFragment(t *testing.T) {
	c := NewContainer()
	c.ContainerID = "CR!TEST"
	c.GeneratorApp = "app"
	c.GeneratorPkg = "pkg"

	// Add one normal fragment so $270.$181 (contains) can be populated.
	if err := c.Fragments.Add(NewRootFragment(SymMetadata, NewStruct())); err != nil {
		t.Fatalf("add metadata fragment: %v", err)
	}

	data, err := c.WriteContainer()
	if err != nil {
		t.Fatalf("WriteContainer: %v", err)
	}

	parsed, err := ReadContainer(data)
	if err != nil {
		t.Fatalf("ReadContainer: %v", err)
	}

	frag := parsed.Fragments.GetRoot(SymContainer)
	if frag == nil {
		t.Fatalf("expected synthesized $270 fragment")
	}
	v, ok := frag.Value.(StructValue)
	if !ok {
		t.Fatalf("$270 fragment value type: %T", frag.Value)
	}

	if got, _ := v.GetString(SymContainerId); got != c.ContainerID {
		t.Fatalf("$270.$409 container_id: got %q want %q", got, c.ContainerID)
	}
	if got, _ := v.GetString(SymFormat); got != "KFX metadata" {
		t.Fatalf("$270.$161 format: got %q want %q", got, "KFX metadata")
	}

	contains, ok := v.GetList(SymContainsIds)
	if !ok || len(contains) != 1 {
		t.Fatalf("$270.$181 contains: got %v", contains)
	}
	pair, ok := contains[0].([]any)
	if !ok || len(pair) != 2 {
		t.Fatalf("$270.$181 entry: got %T %#v", contains[0], contains[0])
	}
	if pair[0] != int64(SymMetadata) || pair[1] != int64(SymMetadata) {
		t.Fatalf("$270.$181 entry value: got %#v", pair)
	}
}
