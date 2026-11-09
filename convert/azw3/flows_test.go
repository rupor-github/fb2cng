package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestEncodeStrings(t *testing.T) {
	values := []string{"", "P-//*[@aid='0000']", "Кириллица 🙂", "", strings.Repeat("Ж", 20000), strings.Repeat("x", 30000), "last"}
	records, offsets, err := encodeStrings(values)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || offsets[0] != offsets[3] || offsets[5] != stringRecordSize {
		t.Fatalf("incorrect string deduplication or record stride: %v", offsets)
	}
	for i, offset := range offsets {
		record, pos := records[offset/stringRecordSize], int(offset%stringRecordSize)
		var length uint32
		for {
			b := record[pos]
			pos++
			length = length<<7 | uint32(b&0x7f)
			if b&0x80 != 0 {
				break
			}
		}
		if string(record[pos:pos+int(length)]) != values[i] {
			t.Errorf("CTOC string %d changed", i)
		}
	}
	for _, record := range records {
		if len(record)%4 != 0 || len(record) > stringRecordSize {
			t.Error("invalid CTOC record size or padding")
		}
	}
	for _, tt := range []struct {
		name  string
		value string
	}{
		{"invalid UTF-8", "\xff"}, {"oversized", strings.Repeat("a", stringRecordSize)},
		{"oversized including length", strings.Repeat("a", stringRecordSize-1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := encodeStrings([]string{tt.value}); err == nil {
				t.Error("invalid string accepted")
			}
		})
	}
	if records, offsets, err := encodeStrings(nil); err != nil || len(records) != 0 || len(offsets) != 0 {
		t.Error("empty input should not produce a CTOC record")
	}
	t.Run("exact record boundary", func(t *testing.T) {
		maximum := strings.Repeat("a", stringRecordSize-3)
		records, offsets, err := encodeStrings([]string{maximum, "next", maximum})
		if err != nil || len(records) != 2 || len(records[0]) != stringRecordSize ||
			offsets[1] != stringRecordSize || offsets[0] != offsets[2] {
			t.Error("exact CTOC record boundary or cross-record deduplication changed")
		}
	})
}

func TestEncodeFDST(t *testing.T) {
	data, err := encodeFDST([]uint32{100, 30, 20})
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "FDST" || binary.BigEndian.Uint32(data[4:]) != 12 || binary.BigEndian.Uint32(data[8:]) != 3 {
		t.Fatal("incorrect FDST header")
	}
	var got []uint32
	for offset := 12; offset < len(data); offset += 4 {
		got = append(got, binary.BigEndian.Uint32(data[offset:]))
	}
	if !slices.Equal(got, []uint32{0, 100, 100, 130, 130, 150}) {
		t.Errorf("flow boundaries = %v", got)
	}
	for _, tt := range []struct {
		name    string
		lengths []uint32
	}{
		{"no flows", nil}, {"empty flow", []uint32{1, 0}}, {"overflow", []uint32{math.MaxUint32, 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := encodeFDST(tt.lengths); err == nil {
				t.Error("invalid flow layout accepted")
			}
		})
	}
	maximum, err := encodeFDST([]uint32{math.MaxUint32})
	if err != nil || !bytes.Equal(maximum[len(maximum)-4:], []byte{0xff, 0xff, 0xff, 0xff}) {
		t.Error("32-bit boundary was not preserved")
	}
}

func TestCTOCProbe(t *testing.T) {
	values := []string{"", strings.Repeat("a", 30000), strings.Repeat("Ж", 15000), strings.Repeat("b", 30000), "", "é🙂"}
	stringsData, offsets, err := encodeStrings(values)
	if err != nil {
		t.Fatal(err)
	}
	index, err := encodeIndex([]indexTag{{ID: 1, Values: 1}}, nil, uint32(len(stringsData)))
	if err != nil {
		t.Fatal(err)
	}
	var database bytes.Buffer
	if err := writePalmDB(context.Background(), &database, "CTOC_probe", append(index, stringsData...)); err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(struct {
		Values  []string
		Offsets []uint32
	}{values, offsets})
	if err != nil {
		t.Fatal(err)
	}
	writeProbeArtifacts(t, map[string][]byte{"ctoc.pdb": database.Bytes(), "ctoc.json": expected})
}
