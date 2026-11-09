package azw3

import (
	"encoding/binary"
	"errors"
	"math"
	"unicode/utf8"
)

const stringRecordSize = 0x10000

// encodeStrings builds CTOC records in first-use order. Offsets use a 64 KiB
// record stride, even when an earlier record ends before that boundary.
func encodeStrings(values []string) ([][]byte, []uint32, error) {
	var records [][]byte
	var current []byte
	offsets := make([]uint32, len(values))
	seen := make(map[string]uint32)
	for i, value := range values {
		if offset, exists := seen[value]; exists {
			offsets[i] = offset
			continue
		}
		if !utf8.ValidString(value) || len(value) > stringRecordSize {
			return nil, nil, errors.New("invalid or oversized KF8 index string")
		}
		encoded := appendVariable(nil, uint32(len(value)))
		encoded = append(encoded, value...)
		if (len(encoded)+3)&^3 > stringRecordSize {
			return nil, nil, errors.New("KF8 index string exceeds a CTOC record")
		}
		if (len(current)+len(encoded)+3)&^3 > stringRecordSize {
			records = append(records, align4(current))
			current = nil
		}
		offset := uint64(len(records))*stringRecordSize + uint64(len(current))
		if offset > math.MaxUint32 {
			return nil, nil, errors.New("KF8 CTOC offset exceeds 32 bits")
		}
		offsets[i] = uint32(offset)
		seen[value] = uint32(offset)
		current = append(current, encoded...)
	}
	if len(current) > 0 {
		records = append(records, align4(current))
	}
	return records, offsets, nil
}

func encodeFDST(lengths []uint32) ([]byte, error) {
	if len(lengths) == 0 || uint64(len(lengths)) > (math.MaxUint32-12)/8 || uint64(len(lengths)) > uint64(math.MaxInt-12)/8 {
		return nil, errors.New("invalid KF8 flow count")
	}
	var end uint64
	for _, length := range lengths {
		end += uint64(length)
		if length == 0 || end > math.MaxUint32 {
			return nil, errors.New("empty KF8 flow or flow offsets exceed 32 bits")
		}
	}
	data := make([]byte, 12, 12+8*len(lengths))
	copy(data, "FDST")
	binary.BigEndian.PutUint32(data[4:], 12)
	binary.BigEndian.PutUint32(data[8:], uint32(len(lengths)))
	var start uint32
	for _, length := range lengths {
		data = binary.BigEndian.AppendUint32(data, start)
		start += length
		data = binary.BigEndian.AppendUint32(data, start)
	}
	return data, nil
}
