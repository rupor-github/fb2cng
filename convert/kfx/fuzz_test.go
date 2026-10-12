package kfx

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const maxFuzzPayload = 8 * 1024

type fuzzIonDocument struct {
	Name    *string `ion:"name"`
	Version *int64  `ion:"version"`
	Data    []byte  `ion:"data"`
	Values  []int64 `ion:"values"`
	Active  bool    `ion:"active"`
}

func fuzzDocument(data []byte) fuzzIonDocument {
	var number [8]byte
	copy(number[:], data)
	version := int64(binary.LittleEndian.Uint64(number[:]))
	name := strings.ToValidUTF8(string(data), "\ufffd")
	doc := fuzzIonDocument{Data: bytes.Clone(data), Active: number[0]&1 != 0}
	if number[0]&2 == 0 {
		doc.Name = &name
	}
	if number[0]&4 == 0 {
		doc.Version = &version
	}
	for _, value := range data[:min(len(data), 16)] {
		doc.Values = append(doc.Values, int64(value)-128)
	}
	return doc
}

func checkFuzzDocument(t *testing.T, got, want fuzzIonDocument) {
	t.Helper()
	if !reflect.DeepEqual(got.Name, want.Name) || !reflect.DeepEqual(got.Version, want.Version) ||
		!bytes.Equal(got.Data, want.Data) || !slices.Equal(got.Values, want.Values) || got.Active != want.Active {
		t.Fatal("Ion round-trip changed document values or null handling")
	}
}

func FuzzIonRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		{}, {0}, {2}, {4}, {6},
		[]byte("Unicode 世界\x00\xff"),
		bytes.Repeat([]byte{0xff}, 16),
		bytes.Repeat([]byte("nested values"), 128),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFuzzPayload {
			t.Skip()
		}
		// Encoder-generated input bounds declared lengths and nesting before ion-go sees it.
		want := fuzzDocument(data)
		encoded, err := EncodeIon(want)
		if err != nil {
			t.Fatalf("encode document: %v", err)
		}
		var got fuzzIonDocument
		if err := DecodeIon(GetIonProlog(), encoded, &got); err != nil {
			t.Fatalf("decode document: %v", err)
		}
		checkFuzzDocument(t, got, want)
		r := NewIonReaderBytes(encoded)
		if !r.Next() {
			t.Fatalf("missing document value: %v", r.Err())
		}
		value, err := r.ReadValue()
		if err != nil {
			t.Fatalf("read document value: %v", err)
		}
		reencoded, err := EncodeIon(value)
		if err != nil {
			t.Fatalf("re-encode reader value: %v", err)
		}
		var roundTrip fuzzIonDocument
		if err := DecodeIon(GetIonProlog(), reencoded, &roundTrip); err != nil {
			t.Fatalf("decode reader round-trip: %v", err)
		}
		checkFuzzDocument(t, roundTrip, want)
		for _, field := range []string{"name", "version", "imports"} {
			bad := nullImportTable(t, field, len(data) > 0 && data[0]&8 != 0)
			if _, err := DecodeSymbolTable(bad); err == nil {
				t.Fatalf("accepted null %s import", field)
			}
		}
	})
}

func FuzzContainerRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		{}, {0}, {1}, {2}, {3},
		[]byte("CONT\x00opaque resource\xff"),
		[]byte("Привет, 世界!"),
		bytes.Repeat([]byte{0, 0xff, 0x80, 0x7f}, 1024),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFuzzPayload {
			t.Skip()
		}
		doc := fuzzDocument(data)
		metadata := map[string]any{"name": nil, "version": nil, "active": doc.Active}
		if doc.Name != nil {
			metadata["name"] = *doc.Name
		}
		if doc.Version != nil {
			metadata["version"] = *doc.Version
		}
		c := NewContainer()
		c.ContainerID = "CR!FUZZ"
		c.FormatCapabilities = []any{map[string]any{"version": int64(1)}}
		if err := c.Fragments.Add(NewRootFragment(SymMetadata, metadata)); err != nil {
			t.Fatal(err)
		}
		if err := c.Fragments.Add(&Fragment{
			FType: SymRawMedia, FIDName: "resource/fuzz", Value: RawValue(bytes.Clone(data)),
		}); err != nil {
			t.Fatal(err)
		}
		encoded, err := c.WriteContainer()
		if err != nil {
			t.Fatalf("write container: %v", err)
		}
		if err := ValidateContainer(encoded); err != nil {
			t.Fatalf("validate written container: %v", err)
		}
		decoded, err := ReadContainer(encoded)
		if err != nil {
			t.Fatalf("read container: %v", err)
		}
		checkFuzzContainer(t, decoded, metadata, data)
		reencoded, err := decoded.WriteContainer()
		if err != nil {
			t.Fatalf("rewrite container: %v", err)
		}
		roundTrip, err := ReadContainer(reencoded)
		if err != nil {
			t.Fatalf("read rewritten container: %v", err)
		}
		checkFuzzContainer(t, roundTrip, metadata, data)
		// Corrupt ranges to values outside the buffer; these must fail before Ion allocation.
		for _, offset := range []int{6, 10, 14} {
			bad := bytes.Clone(encoded)
			binary.LittleEndian.PutUint32(bad[offset:], ^uint32(0))
			if err := ValidateContainer(bad); err == nil {
				t.Fatalf("accepted invalid CONT header field at %d", offset)
			}
			if _, err := ReadContainer(bad); err == nil {
				t.Fatalf("read invalid CONT header field at %d", offset)
			}
		}
	})
}

func checkFuzzContainer(t *testing.T, c *Container, metadata map[string]any, data []byte) {
	t.Helper()
	if c.ContainerID != "CR!FUZZ" || c.CompressionType != 0 || c.DRMScheme != 0 {
		t.Fatal("container identity or encoding changed")
	}
	frag := c.Fragments.GetRoot(SymMetadata)
	if frag == nil || !reflect.DeepEqual(frag.Value, metadata) {
		t.Fatal("container metadata or null handling changed")
	}
	resources := c.Fragments.GetByType(SymRawMedia)
	if len(resources) != 1 || resources[0].FIDName != "resource/fuzz" {
		t.Fatal("resource identity changed")
	}
	blob, ok := resources[0].Value.(RawValue)
	if !ok || !bytes.Equal(blob, data) {
		t.Fatal("raw resource bytes changed")
	}
	if want := []any{map[string]any{"version": int64(1)}}; !reflect.DeepEqual(c.FormatCapabilities, want) {
		t.Fatal("format capabilities changed")
	}
}
