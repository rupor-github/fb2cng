package azw3

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const indexHeaderSize = 192

// indexTag describes one optional fixed-arity tag. Single-bit presence masks avoid
// the variable-count TAGX convention; each present tag has exactly Values integers.
type indexTag struct {
	ID     byte
	Values byte
}

type indexEntry struct {
	Key    string
	Values map[byte][]uint32
}

// encodeIndex emits a main INDX/TAGX record followed by data INDX records. Entries
// must be strictly key-sorted. CTOC records, when needed, are appended by the caller.
// Data records are split before either their size or an IDXT offset exceeds 16 bits.
func encodeIndex(tags []indexTag, entries []indexEntry, stringRecords uint32) ([][]byte, error) {
	if len(tags) == 0 || len(tags) > math.MaxUint8 || uint64(len(entries)) > math.MaxUint32 {
		return nil, errors.New("invalid KF8 index dimensions")
	}
	known := make(map[byte]indexTag, len(tags))
	for _, tag := range tags {
		if tag.Values == 0 || tag.ID == 0 {
			return nil, errors.New("KF8 index tags require nonzero IDs and arity")
		}
		if _, exists := known[tag.ID]; exists {
			return nil, fmt.Errorf("duplicate KF8 index tag %d", tag.ID)
		}
		known[tag.ID] = tag
	}
	controls := (len(tags) + 7) / 8
	tagx := make([]byte, 12)
	copy(tagx, "TAGX")
	binary.BigEndian.PutUint32(tagx[8:], uint32(controls))
	for i, tag := range tags {
		tagx = append(tagx, tag.ID, tag.Values, 1<<uint(i%8), 0)
		if i%8 == 7 || i == len(tags)-1 {
			tagx = append(tagx, 0, 0, 0, 1)
		}
	}
	binary.BigEndian.PutUint32(tagx[4:], uint32(len(tagx)))

	var leaves [][]byte
	var summaries [][]byte
	leaf := make([]byte, indexHeaderSize)
	var offsets []uint16
	var lastKey string
	flush := func() {
		if len(offsets) == 0 {
			return
		}
		leaf = align4(leaf)
		idxt := len(leaf)
		leaf = append(leaf, "IDXT"...)
		for _, offset := range offsets {
			leaf = binary.BigEndian.AppendUint16(leaf, offset)
		}
		setIndexHeader(leaf, 1, uint32(idxt), uint32(len(offsets)), missingRecord, 0, 0)
		leaves = append(leaves, align4(leaf))
		summary := append([]byte{byte(len(lastKey))}, lastKey...)
		summaries = append(summaries, binary.BigEndian.AppendUint16(summary, uint16(len(offsets))))
		leaf = make([]byte, indexHeaderSize)
		offsets = nil
	}
	for i, entry := range entries {
		if len(entry.Key) == 0 || len(entry.Key) > math.MaxUint8 || (i > 0 && entry.Key <= entries[i-1].Key) {
			return nil, fmt.Errorf("invalid or unsorted KF8 index key at entry %d", i)
		}
		for id := range entry.Values {
			if _, exists := known[id]; !exists {
				return nil, fmt.Errorf("unknown tag %d in KF8 index entry %d", id, i)
			}
		}
		encoded := make([]byte, 1+len(entry.Key)+controls)
		encoded[0] = byte(len(entry.Key))
		copy(encoded[1:], entry.Key)
		for j, tag := range tags {
			values, exists := entry.Values[tag.ID]
			if !exists {
				continue
			}
			if len(values) != int(tag.Values) {
				return nil, fmt.Errorf("tag %d in KF8 index entry %d requires %d values", tag.ID, i, tag.Values)
			}
			encoded[1+len(entry.Key)+j/8] |= 1 << uint(j%8)
			for _, value := range values {
				encoded = appendVariable(encoded, value)
			}
		}
		// Include worst-case alignment and the IDXT array in the record limit.
		if indexHeaderSize+len(encoded)+3+4+2+3 > math.MaxUint16 {
			return nil, fmt.Errorf("KF8 index entry %d exceeds a data record", i)
		}
		if len(leaf)+len(encoded)+3+4+2*(len(offsets)+1)+3 > math.MaxUint16 {
			flush()
		}
		offsets = append(offsets, uint16(len(leaf)))
		leaf = append(leaf, encoded...)
		lastKey = entry.Key
	}
	flush()
	root := append(make([]byte, indexHeaderSize), tagx...)
	var rootOffsets []uint16
	for _, summary := range summaries {
		if len(root)+len(summary)+3+4+2*(len(rootOffsets)+1)+3 > math.MaxUint16 {
			return nil, errors.New("KF8 main index exceeds 16-bit IDXT offsets")
		}
		rootOffsets = append(rootOffsets, uint16(len(root)))
		root = append(root, summary...)
	}
	root = align4(root)
	idxt := len(root)
	root = append(root, "IDXT"...)
	for _, offset := range rootOffsets {
		root = binary.BigEndian.AppendUint16(root, offset)
	}
	setIndexHeader(root, 0, uint32(idxt), uint32(len(leaves)), 65001, uint32(len(entries)), stringRecords)
	binary.BigEndian.PutUint32(root[0x10:], 2)
	binary.BigEndian.PutUint32(root[0xb4:], indexHeaderSize)
	return append([][]byte{align4(root)}, leaves...), nil
}

func setIndexHeader(data []byte, kind, idxt, count, codepage, total, stringRecords uint32) {
	copy(data, "INDX")
	for offset, value := range map[int]uint32{
		4: indexHeaderSize, 12: kind, 20: idxt, 24: count, 28: codepage,
		32: missingRecord, 36: total, 52: stringRecords,
	} {
		binary.BigEndian.PutUint32(data[offset:], value)
	}
}
