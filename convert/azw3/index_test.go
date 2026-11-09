package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestEncodeIndex(t *testing.T) {
	tags := []indexTag{{ID: 1, Values: 1}, {ID: 6, Values: 2}}
	entries := []indexEntry{
		{Key: "SKEL0000000000", Values: map[byte][]uint32{1: {1}, 6: {0, 300}}},
		{Key: "SKEL0000000001", Values: map[byte][]uint32{1: {0}, 6: {400, 128}}},
	}
	records, err := encodeIndex(tags, entries, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	wantTAGX := []byte{'T', 'A', 'G', 'X', 0, 0, 0, 24, 0, 0, 0, 1, 1, 1, 1, 0, 6, 2, 2, 0, 0, 0, 0, 1}
	if !bytes.Equal(records[0][indexHeaderSize:indexHeaderSize+len(wantTAGX)], wantTAGX) {
		t.Error("incorrect fixed-arity TAGX schema")
	}
	assertIndexEntries(t, records, entries)
	again, err := encodeIndex(tags, entries, 0)
	if err != nil || !reflect.DeepEqual(records, again) {
		t.Error("index encoding is not deterministic")
	}
}

func TestEncodeIndexMultiRecord(t *testing.T) {
	// More than eight tags exercises control-byte transitions. Optional values
	// exercise absent tags, including tags in later control bytes.
	var tags []indexTag
	for id := byte(1); id <= 10; id++ {
		tags = append(tags, indexTag{ID: id, Values: 1})
	}
	entries := make([]indexEntry, 12000)
	for i := range entries {
		entries[i] = indexEntry{
			Key: fmt.Sprintf("%08d", i),
			Values: map[byte][]uint32{
				1: {uint32(i)}, 6: {math.MaxUint32}, 9: {128},
			},
		}
		if i%2 == 0 {
			entries[i].Values[10] = []uint32{0}
		}
	}
	records, err := encodeIndex(tags, entries, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 4 {
		t.Fatalf("expected several INDX data records, got %d", len(records)-1)
	}
	assertIndexEntries(t, records, entries)
	var database bytes.Buffer
	if err := writePalmDB(context.Background(), &database, "Multi_record_index", records); err != nil {
		t.Fatal(err)
	}
	writeProbeArtifacts(t, map[string][]byte{"multi-index.pdb": database.Bytes()})
}

func TestEncodeIndexRejectsInvalidInput(t *testing.T) {
	validTags := []indexTag{{ID: 1, Values: 1}}
	for _, tt := range []struct {
		name    string
		tags    []indexTag
		entries []indexEntry
	}{
		{name: "no tags"},
		{name: "zero ID", tags: []indexTag{{Values: 1}}},
		{name: "zero arity", tags: []indexTag{{ID: 1}}},
		{name: "duplicate tag", tags: []indexTag{{ID: 1, Values: 1}, {ID: 1, Values: 2}}},
		{name: "empty key", tags: validTags, entries: []indexEntry{{}}},
		{name: "long key", tags: validTags, entries: []indexEntry{{Key: strings.Repeat("x", 256)}}},
		{name: "unsorted", tags: validTags, entries: []indexEntry{{Key: "b"}, {Key: "a"}}},
		{name: "duplicate key", tags: validTags, entries: []indexEntry{{Key: "a"}, {Key: "a"}}},
		{name: "unknown tag", tags: validTags, entries: []indexEntry{{Key: "a", Values: map[byte][]uint32{2: {0}}}}},
		{name: "wrong arity", tags: validTags, entries: []indexEntry{{Key: "a", Values: map[byte][]uint32{1: {0, 1}}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := encodeIndex(tt.tags, tt.entries, 0); err == nil {
				t.Error("invalid index accepted")
			}
		})
	}
	t.Run("oversized entry", func(t *testing.T) {
		var tags []indexTag
		values := make(map[byte][]uint32)
		for id := 1; id <= 255; id++ {
			tags = append(tags, indexTag{ID: byte(id), Values: 255})
			values[byte(id)] = make([]uint32, 255)
			for i := range values[byte(id)] {
				values[byte(id)][i] = math.MaxUint32
			}
		}
		if _, err := encodeIndex(tags, []indexEntry{{Key: "a", Values: values}}, 0); err == nil {
			t.Error("entry exceeding IDXT limits accepted")
		}
	})
}

func assertIndexEntries(t *testing.T, records [][]byte, expected []indexEntry) {
	t.Helper()
	root := records[0]
	if int(binary.BigEndian.Uint32(root[24:])) != len(records)-1 || int(binary.BigEndian.Uint32(root[36:])) != len(expected) {
		t.Fatal("main INDX counts do not match its data records")
	}
	tagx := int(binary.BigEndian.Uint32(root[4:]))
	tagEnd := tagx + int(binary.BigEndian.Uint32(root[tagx+4:]))
	controls := int(binary.BigEndian.Uint32(root[tagx+8:]))
	var decoded []indexEntry
	for _, leaf := range records[1:] {
		if len(leaf) > math.MaxUint16 || len(leaf)%4 != 0 || string(leaf[:4]) != "INDX" {
			t.Fatal("invalid INDX record size, alignment or signature")
		}
		idxt, count := int(binary.BigEndian.Uint32(leaf[20:])), int(binary.BigEndian.Uint32(leaf[24:]))
		if string(leaf[idxt:idxt+4]) != "IDXT" || idxt+4+2*count > len(leaf) {
			t.Fatal("invalid IDXT array")
		}
		previous := indexHeaderSize - 1
		for i := range count {
			pos := int(binary.BigEndian.Uint16(leaf[idxt+4+2*i:]))
			if pos <= previous || pos >= idxt {
				t.Fatal("IDXT entry offsets are not strictly ordered inside the record")
			}
			previous = pos
			length := int(leaf[pos])
			entry := indexEntry{Key: string(leaf[pos+1 : pos+1+length]), Values: make(map[byte][]uint32)}
			controlStart := pos + 1 + length
			valueStart, control := controlStart+controls, 0
			for p := tagx + 12; p < tagEnd; p += 4 {
				id, arity, mask, end := root[p], root[p+1], root[p+2], root[p+3]
				if end == 1 {
					control++
					continue
				}
				if leaf[controlStart+control]&mask == 0 {
					continue
				}
				for range arity {
					var value uint32
					for {
						b := leaf[valueStart]
						valueStart++
						value = value<<7 | uint32(b&0x7f)
						if b&0x80 != 0 {
							break
						}
					}
					entry.Values[id] = append(entry.Values[id], value)
				}
			}
			decoded = append(decoded, entry)
		}
	}
	if !reflect.DeepEqual(decoded, expected) {
		t.Fatal("decoded index entries do not match the input")
	}
}
