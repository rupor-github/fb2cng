package kfx

import (
	"bytes"
	"encoding/binary"
	"testing"

	"fbc/convert/kfx/envelope"
)

func TestReadFileEnvelope(t *testing.T) {
	c := NewContainer()
	c.ContainerID = "CR!OPAQUE"
	if err := c.Fragments.Add(NewRootFragment(SymMetadata, NewStruct())); err != nil {
		t.Fatal(err)
	}
	inner, err := c.WriteContainer()
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := ReadFile(t.Context(), inner)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []envelope.Mode{envelope.Plaintext, envelope.LZMA} {
		t.Run(string(mode), func(t *testing.T) {
			data, _, err := envelope.Encode(t.Context(), inner, mode)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadFile(t.Context(), data)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.InnerBytes, inner) || got.EnvelopeInfo.Mode != mode {
				t.Fatal("changed inner bytes/mode")
			}
			if got.Container.String() != baseline.Container.String() || got.Container.DumpFragments() != baseline.Container.DumpFragments() {
				t.Fatal("changed semantic dump")
			}
		})
	}
	for _, offset := range []int{6, 10, 14} {
		bad := bytes.Clone(inner)
		binary.LittleEndian.PutUint32(bad[offset:], ^uint32(0))
		if _, err := ReadFile(t.Context(), bad); err == nil {
			t.Fatalf("accepted invalid header offset %d", offset)
		}
	}
}
