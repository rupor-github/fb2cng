// Package azw3 implements the native standalone KF8 compiler's binary structures.
// It is not yet connected to the conversion CLI.
package azw3

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	missingRecord  = uint32(math.MaxUint32)
	textRecordSize = 4096
)

// MOBI integers are big-endian base-128 with the high bit set on the FINAL byte,
// unlike encoding/binary's little-endian continuation-bit varints.
func appendVariable(dst []byte, value uint32) []byte {
	var encoded [5]byte
	i := len(encoded) - 1
	encoded[i] = byte(value&0x7f) | 0x80
	for value >>= 7; value != 0; value >>= 7 {
		i--
		encoded[i] = byte(value & 0x7f)
	}
	return append(dst, encoded[i:]...)
}

func align4(data []byte) []byte {
	return append(data, make([]byte, (-len(data))&3)...)
}

func splitText(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty KF8 text")
	}
	if uint64(len(data)) > math.MaxUint32 {
		return nil, errors.New("KF8 text exceeds 32-bit byte offsets")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("KF8 text must be UTF-8")
	}
	var records [][]byte
	for len(data) > 0 {
		end := min(len(data), textRecordSize)
		if end < len(data) {
			for !utf8.RuneStart(data[end]) {
				end--
			}
		}
		records = append(records, data[:end])
		data = data[end:]
		if len(records) > math.MaxUint16 {
			return nil, errors.New("too many KF8 text records")
		}
	}
	return records, nil
}

type exthRecord struct {
	ID   uint32
	Data []byte
}

func encodeEXTH(records []exthRecord) ([]byte, error) {
	size := uint64(12)
	for _, record := range records {
		size += 8 + uint64(len(record.Data))
	}
	if size > math.MaxUint32-3 || size > uint64(math.MaxInt-3) || uint64(len(records)) > math.MaxUint32 {
		return nil, errors.New("EXTH metadata exceeds 32-bit lengths")
	}
	data := make([]byte, 12, int(size)+3)
	copy(data, "EXTH")
	binary.BigEndian.PutUint32(data[4:], uint32(size))
	binary.BigEndian.PutUint32(data[8:], uint32(len(records)))
	for _, record := range records {
		data = binary.BigEndian.AppendUint32(data, record.ID)
		data = binary.BigEndian.AppendUint32(data, uint32(len(record.Data))+8)
		data = append(data, record.Data...)
	}
	return align4(data), nil
}

// headerLayout uses resolved record numbers. All absent record references must be
// missingRecord rather than zero, which names the book header itself.
type headerLayout struct {
	Title         string
	UID           uint32
	Language      uint32
	TextLength    uint32
	TextRecords   uint16
	FirstNonText  uint32
	FirstResource uint32
	FDST          uint32
	FlowCount     uint32
	Navigation    uint32
	Fragments     uint32
	Skeletons     uint32
	Guide         uint32
	Metadata      []exthRecord
	HasFonts      bool
}

func encodeHeader(layout headerLayout) ([]byte, error) {
	if !utf8.ValidString(layout.Title) || layout.Title == "" {
		return nil, errors.New("KF8 title must be nonempty UTF-8")
	}
	if layout.TextRecords == 0 || layout.TextLength < uint32(layout.TextRecords) ||
		uint64(layout.TextLength) > uint64(layout.TextRecords)*textRecordSize {
		return nil, errors.New("KF8 header requires text records")
	}
	if layout.FirstNonText == missingRecord || layout.FirstNonText <= uint32(layout.TextRecords) || layout.Skeletons == missingRecord {
		return nil, errors.New("invalid KF8 structural record layout")
	}
	for _, record := range []uint32{
		layout.FirstResource, layout.FDST, layout.Navigation, layout.Fragments, layout.Skeletons, layout.Guide,
	} {
		if record != missingRecord && record < layout.FirstNonText {
			return nil, errors.New("KF8 structural or resource record overlaps text")
		}
	}
	if layout.FlowCount == 0 || (layout.FlowCount > 1 && layout.FDST == missingRecord) {
		return nil, errors.New("KF8 flows require a matching FDST record")
	}
	exth, err := encodeEXTH(layout.Metadata)
	if err != nil {
		return nil, fmt.Errorf("encode KF8 metadata: %w", err)
	}
	const headerSize = 16 + 0x108
	titleOffset := uint64(headerSize) + uint64(len(exth))
	if titleOffset+uint64(len(layout.Title))+1 > math.MaxUint32 {
		return nil, errors.New("KF8 header exceeds 32-bit lengths")
	}
	data := make([]byte, headerSize)
	binary.BigEndian.PutUint16(data, 1) // uncompressed structural probes
	binary.BigEndian.PutUint32(data[4:], layout.TextLength)
	binary.BigEndian.PutUint16(data[8:], layout.TextRecords)
	binary.BigEndian.PutUint16(data[10:], textRecordSize)
	copy(data[0x10:], "MOBI")
	exthFlags := uint32(0x50) // Conventional standalone EXTH flags; no shared-resource bit.
	if layout.HasFonts {
		exthFlags |= 0x1000
	}
	words := map[int]uint32{
		0x14: 0x108, 0x18: 2, 0x1c: 65001, 0x20: layout.UID, 0x24: 8,
		0x28: missingRecord, 0x2c: missingRecord, 0x30: missingRecord, 0x34: missingRecord,
		0x38: missingRecord, 0x3c: missingRecord, 0x40: missingRecord, 0x44: missingRecord,
		0x48: missingRecord, 0x4c: missingRecord,
		0x50: layout.FirstNonText, 0x54: uint32(titleOffset), 0x58: uint32(len(layout.Title)),
		0x5c: layout.Language, 0x68: 8, 0x6c: layout.FirstResource,
		0x80: exthFlags,
		0xa4: missingRecord, 0xa8: missingRecord, // no DRM
		0xc0: layout.FDST, 0xc4: layout.FlowCount,
		0xc8: missingRecord, 0xd0: missingRecord, // no FCIS/FLIS in structural probes
		0xe0: missingRecord, 0xe8: missingRecord, 0xec: missingRecord,
		0xf4: layout.Navigation, 0xf8: layout.Fragments, 0xfc: layout.Skeletons,
		0x100: missingRecord, 0x104: layout.Guide, 0x108: missingRecord, 0x110: missingRecord,
	}
	for offset, value := range words {
		binary.BigEndian.PutUint32(data[offset:], value)
	}
	data = append(data, exth...)
	data = append(data, layout.Title...)
	return append(data, 0), nil
}
