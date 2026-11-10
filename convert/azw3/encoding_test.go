package azw3

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAppendVariable(t *testing.T) {
	for _, tt := range []struct {
		value uint32
		want  []byte
	}{
		{0, []byte{0x80}}, {1, []byte{0x81}}, {127, []byte{0xff}},
		{128, []byte{0x01, 0x80}}, {16383, []byte{0x7f, 0xff}},
		{16384, []byte{0x01, 0x00, 0x80}}, {0xffffffff, []byte{0x0f, 0x7f, 0x7f, 0x7f, 0xff}},
	} {
		t.Run(stringName(tt.value), func(t *testing.T) {
			got := appendVariable([]byte{0x42}, tt.value)
			if !bytes.Equal(got, append([]byte{0x42}, tt.want...)) {
				t.Errorf("encoded %d = %x, want prefix and %x", tt.value, got, tt.want)
			}
		})
	}
}

func TestSplitText(t *testing.T) {
	for _, text := range []string{
		"short", strings.Repeat("a", textRecordSize), strings.Repeat("a", textRecordSize+1),
		strings.Repeat("Ж🙂", textRecordSize), strings.Repeat("a", textRecordSize-1) + "🙂" + strings.Repeat("Ж", textRecordSize),
	} {
		t.Run(stringName(uint32(len(text))), func(t *testing.T) {
			records, err := splitText([]byte(text))
			if err != nil {
				t.Fatal(err)
			}
			var decoded []byte
			for i, record := range records {
				if len(record.Data) == 0 || len(record.Data) > textRecordSize || i < len(records)-1 && len(record.Data) != textRecordSize {
					t.Errorf("invalid fixed-size text record of %d bytes", len(record.Data))
				}
				if len(record.Overlap) > 3 {
					t.Error("UTF-8 overlap exceeds three continuation bytes")
				}
				if i < len(records)-1 && !bytes.HasPrefix(records[i+1].Data, record.Overlap) {
					t.Error("overlap bytes were not repeated at the next record's start")
				}
				start := 0
				for start < len(record.Data) && !utf8.RuneStart(record.Data[start]) {
					start++
				}
				if !utf8.Valid(append(bytes.Clone(record.Data[start:]), record.Overlap...)) {
					t.Error("overlap did not complete the record's final UTF-8 character")
				}
				decoded = append(decoded, record.Data...)
			}
			if !bytes.Equal(decoded, []byte(text)) {
				t.Error("text segmentation lost or duplicated bytes")
			}
		})
	}
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "empty"}, {name: "invalid UTF-8", data: []byte{0xff}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := splitText(tt.data); err == nil {
				t.Error("invalid text accepted")
			}
		})
	}
}

func TestSplitTextUTF8Overlap(t *testing.T) {
	for _, tt := range []struct {
		name    string
		prefix  int
		char    string
		overlap int
	}{
		{"two-byte rune", textRecordSize - 1, "é", 1},
		{"three-byte rune", textRecordSize - 1, "中", 2},
		{"four-byte rune", textRecordSize - 1, "🙂", 3},
		{"four-byte rune half", textRecordSize - 2, "🙂", 2},
		{"four-byte rune final byte", textRecordSize - 3, "🙂", 1},
		{"complete rune", textRecordSize - 4, "🙂", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(strings.Repeat("a", tt.prefix) + tt.char + "\n")
			records, err := splitText(input)
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != 2 || len(records[0].Data) != textRecordSize || len(records[0].Overlap) != tt.overlap {
				t.Fatalf("unexpected overlap at fixed 4096-byte record boundary: %+v", records)
			}
			if !bytes.Equal(records[0].Overlap, input[textRecordSize:textRecordSize+tt.overlap]) || len(records[1].Overlap) != 0 {
				t.Error("overlap bytes or final empty overlap are incorrect")
			}
		})
	}
}

func TestEncodeEXTH(t *testing.T) {
	input := []exthRecord{{ID: 100, Data: []byte("One")}, {ID: 100, Data: []byte("Второй")}, {ID: 501, Data: []byte("PDOC")}}
	data, err := encodeEXTH(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "EXTH" || len(data)%4 != 0 || binary.BigEndian.Uint32(data[8:]) != uint32(len(input)) {
		t.Fatal("invalid EXTH header or alignment")
	}
	pos := 12
	for _, want := range input {
		id, size := binary.BigEndian.Uint32(data[pos:]), int(binary.BigEndian.Uint32(data[pos+4:]))
		if id != want.ID || !bytes.Equal(data[pos+8:pos+size], want.Data) {
			t.Error("metadata record or repeated-author order changed")
		}
		pos += size
	}
	if binary.BigEndian.Uint32(data[4:]) != uint32(pos) || !slices.Equal(data[pos:], make([]byte, len(data)-pos)) {
		t.Error("EXTH length or trailing padding is incorrect")
	}
}

func TestEncodeHeader(t *testing.T) {
	layout := probeLayout()
	layout.Title = "Тест 🙂"
	layout.TextLength, layout.TextRecords = 5000, 2
	layout.FirstNonText, layout.Skeletons = 3, 3
	data, err := encodeHeader(layout)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		offset int
		want   uint32
	}{
		{0x14, 0x108}, {0x1c, 65001}, {0x24, 8}, {0x50, 3}, {0xf4, missingRecord},
		{0xf8, missingRecord}, {0xfc, 3}, {0x104, missingRecord}, {0xa8, missingRecord},
	} {
		if got := binary.BigEndian.Uint32(data[tt.offset:]); got != tt.want {
			t.Errorf("header word %#x = %#x, want %#x", tt.offset, got, tt.want)
		}
	}
	start, length := binary.BigEndian.Uint32(data[0x54:]), binary.BigEndian.Uint32(data[0x58:])
	if string(data[start:start+length]) != layout.Title || length != uint32(len(layout.Title)) {
		t.Error("title offsets must use UTF-8 bytes")
	}
	if string(data[0x118:0x11c]) != "EXTH" || binary.BigEndian.Uint16(data[8:]) != 2 {
		t.Error("EXTH or PalmDOC header is misplaced")
	}
	for _, tt := range []struct {
		name   string
		change func(*headerLayout)
	}{
		{"empty title", func(h *headerLayout) { h.Title = "" }},
		{"invalid title", func(h *headerLayout) { h.Title = "\xff" }},
		{"no text", func(h *headerLayout) { h.TextRecords = 0 }},
		{"too much text", func(h *headerLayout) { h.TextLength = 9000 }},
		{"no text bytes", func(h *headerLayout) { h.TextLength = 0 }},
		{"unsupported compression", func(h *headerLayout) { h.Compression = 3 }},
		{"HUFF without dictionary", func(h *headerLayout) { h.Compression = compressionHuffdic }},
		{"HUFF missing CDIC", func(h *headerLayout) { h.Compression = compressionHuffdic; h.Huffman = 3; h.HuffmanCount = 1 }},
		{"HUFF overlaps text", func(h *headerLayout) { h.Compression = compressionHuffdic; h.Huffman = 1; h.HuffmanCount = 2 }},
		{"HUFF exceeds record limit", func(h *headerLayout) {
			h.Compression, h.Huffman, h.HuffmanCount = compressionHuffdic, 65534, 2
		}},
		{"non-HUFF dictionary", func(h *headerLayout) { h.Huffman = 3; h.HuffmanCount = 2 }},
		{"unsupported trailers", func(h *headerLayout) { h.TrailingFlags = 2 }},
		{"index overlaps text", func(h *headerLayout) { h.FirstNonText = 1 }},
		{"missing nontext", func(h *headerLayout) { h.FirstNonText = missingRecord }},
		{"skeleton overlaps text", func(h *headerLayout) { h.Skeletons = 1 }},
		{"missing skeleton", func(h *headerLayout) { h.Skeletons = missingRecord }},
		{"no flows", func(h *headerLayout) { h.FlowCount = 0 }},
		{"missing FDST", func(h *headerLayout) { h.FlowCount = 2 }},
		{"FCIS overlaps text", func(h *headerLayout) { h.FCIS = 1 }},
		{"FLIS overlaps text", func(h *headerLayout) { h.FLIS = 1 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bad := layout
			tt.change(&bad)
			if _, err := encodeHeader(bad); err == nil {
				t.Error("invalid layout accepted")
			}
		})
	}
}
