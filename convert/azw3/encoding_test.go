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
			if !bytes.Equal(bytes.Join(records, nil), []byte(text)) {
				t.Error("text segmentation lost bytes")
			}
			for _, record := range records {
				if len(record) > textRecordSize || len(record) == 0 || !utf8.Valid(record) {
					t.Errorf("invalid text record of %d bytes", len(record))
				}
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
		{"too little text", func(h *headerLayout) { h.TextLength = 1 }},
		{"index overlaps text", func(h *headerLayout) { h.FirstNonText = 1 }},
		{"missing nontext", func(h *headerLayout) { h.FirstNonText = missingRecord }},
		{"skeleton overlaps text", func(h *headerLayout) { h.Skeletons = 1 }},
		{"missing skeleton", func(h *headerLayout) { h.Skeletons = missingRecord }},
		{"no flows", func(h *headerLayout) { h.FlowCount = 0 }},
		{"missing FDST", func(h *headerLayout) { h.FlowCount = 2 }},
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
