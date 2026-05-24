package pdf

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
)

const (
	ttfOffsetTableSize = 12
	ttfTableRecordSize = 16
)

type ttfTableRecord struct {
	Tag      string
	Checksum uint32
	Offset   uint32
	Length   uint32
	Data     []byte
}

type ttfSubsetTables struct {
	ScalerType [4]byte
	Records    map[string]ttfTableRecord
}

func subsetTrueTypeFont(data []byte, used map[uint16]shapedGlyph) ([]byte, bool, error) {
	tables, err := parseTTFTables(data)
	if err != nil {
		return nil, false, err
	}
	glyf, hasGlyf := tables.Records["glyf"]
	loca, hasLoca := tables.Records["loca"]
	head, hasHead := tables.Records["head"]
	maxp, hasMaxp := tables.Records["maxp"]
	if !hasGlyf {
		return nil, false, nil
	}
	if !hasLoca || !hasHead || !hasMaxp {
		return nil, false, fmt.Errorf("TrueType font has glyf table but is missing loca/head/maxp")
	}
	if len(head.Data) < 54 || len(maxp.Data) < 6 {
		return nil, false, fmt.Errorf("TrueType font has invalid head/maxp tables")
	}

	locFormat := int16(binary.BigEndian.Uint16(head.Data[50:52]))
	numGlyphs := int(binary.BigEndian.Uint16(maxp.Data[4:6]))
	locaOffsets, err := parseLocaOffsets(loca.Data, numGlyphs, locFormat)
	if err != nil {
		return nil, false, err
	}
	glyphs, err := collectSubsetGlyphs(glyf.Data, locaOffsets, numGlyphs, used)
	if err != nil {
		return nil, false, err
	}

	newGlyf, newLoca, err := buildSubsetGlyfLoca(glyf.Data, locaOffsets, glyphs, locFormat)
	if err != nil {
		return nil, false, err
	}
	records := make([]ttfTableRecord, 0, len(tables.Records))
	for tag, record := range tables.Records {
		updated := record
		updated.Data = slices.Clone(record.Data)
		switch tag {
		case "glyf":
			updated.Data = newGlyf
		case "loca":
			updated.Data = newLoca
		case "head":
			updated.Data = slices.Clone(record.Data)
			clear(updated.Data[8:12])
		}
		records = append(records, updated)
	}
	slices.SortFunc(records, func(a, b ttfTableRecord) int { return strings.Compare(a.Tag, b.Tag) })
	return writeTTFWithChecksums(tables.ScalerType, records)
}

func parseTTFTables(data []byte) (ttfSubsetTables, error) {
	if len(data) < ttfOffsetTableSize {
		return ttfSubsetTables{}, fmt.Errorf("font data too short")
	}
	var tables ttfSubsetTables
	copy(tables.ScalerType[:], data[:4])
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if ttfOffsetTableSize+numTables*ttfTableRecordSize > len(data) {
		return ttfSubsetTables{}, fmt.Errorf("font table directory exceeds data length")
	}
	tables.Records = make(map[string]ttfTableRecord, numTables)
	for i := range numTables {
		recordOffset := ttfOffsetTableSize + i*ttfTableRecordSize
		tag := string(data[recordOffset : recordOffset+4])
		checksum := binary.BigEndian.Uint32(data[recordOffset+4 : recordOffset+8])
		offset := binary.BigEndian.Uint32(data[recordOffset+8 : recordOffset+12])
		length := binary.BigEndian.Uint32(data[recordOffset+12 : recordOffset+16])
		end := uint64(offset) + uint64(length)
		if end > uint64(len(data)) {
			return ttfSubsetTables{}, fmt.Errorf("font table %s exceeds data length", tag)
		}
		tables.Records[tag] = ttfTableRecord{
			Tag:      tag,
			Checksum: checksum,
			Offset:   offset,
			Length:   length,
			Data:     slices.Clone(data[offset:end]),
		}
	}
	return tables, nil
}

func parseLocaOffsets(data []byte, numGlyphs int, locFormat int16) ([]uint32, error) {
	count := numGlyphs + 1
	offsets := make([]uint32, count)
	switch locFormat {
	case 0:
		if len(data) < count*2 {
			return nil, fmt.Errorf("short loca table too small")
		}
		for i := range count {
			offsets[i] = uint32(binary.BigEndian.Uint16(data[i*2:])) * 2
		}
	case 1:
		if len(data) < count*4 {
			return nil, fmt.Errorf("long loca table too small")
		}
		for i := range count {
			offsets[i] = binary.BigEndian.Uint32(data[i*4:])
		}
	default:
		return nil, fmt.Errorf("unsupported loca format %d", locFormat)
	}
	return offsets, nil
}

func collectSubsetGlyphs(glyf []byte, loca []uint32, numGlyphs int, used map[uint16]shapedGlyph) ([]bool, error) {
	glyphs := make([]bool, numGlyphs)
	if numGlyphs > 0 {
		glyphs[0] = true
	}
	var visit func(int) error
	visit = func(gid int) error {
		if gid < 0 || gid >= numGlyphs {
			return fmt.Errorf("composite glyph reference %d out of range", gid)
		}
		if glyphs[gid] {
			return nil
		}
		glyphs[gid] = true
		deps, err := compositeGlyphDependencies(glyf, loca, gid)
		if err != nil {
			return err
		}
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for gid := range used {
		if err := visit(int(gid)); err != nil {
			return nil, err
		}
	}
	return glyphs, nil
}

func compositeGlyphDependencies(glyf []byte, loca []uint32, gid int) ([]int, error) {
	start := int(loca[gid])
	end := int(loca[gid+1])
	if start == end {
		return nil, nil
	}
	if start < 0 || end < start || end > len(glyf) {
		return nil, fmt.Errorf("glyph %d has invalid loca offsets", gid)
	}
	glyph := glyf[start:end]
	if len(glyph) < 10 {
		return nil, fmt.Errorf("glyph %d data too short", gid)
	}
	contours := int16(binary.BigEndian.Uint16(glyph[0:2]))
	if contours >= 0 {
		return nil, nil
	}

	deps := make([]int, 0, 2)
	offset := 10
	for {
		if offset+4 > len(glyph) {
			return nil, fmt.Errorf("composite glyph %d component truncated", gid)
		}
		flags := binary.BigEndian.Uint16(glyph[offset : offset+2])
		componentGID := int(binary.BigEndian.Uint16(glyph[offset+2 : offset+4]))
		deps = append(deps, componentGID)
		offset += 4
		if flags&0x0001 != 0 {
			offset += 4
		} else {
			offset += 2
		}
		switch {
		case flags&0x0008 != 0:
			offset += 2
		case flags&0x0040 != 0:
			offset += 4
		case flags&0x0080 != 0:
			offset += 8
		}
		if offset > len(glyph) {
			return nil, fmt.Errorf("composite glyph %d component data truncated", gid)
		}
		if flags&0x0020 == 0 {
			break
		}
	}
	return deps, nil
}

func buildSubsetGlyfLoca(glyf []byte, loca []uint32, used []bool, locFormat int16) ([]byte, []byte, error) {
	newGlyf := make([]byte, 0, len(glyf)/4)
	newOffsets := make([]uint32, len(loca))
	for gid := range used {
		newOffsets[gid] = uint32(len(newGlyf))
		if !used[gid] {
			continue
		}
		start := int(loca[gid])
		end := int(loca[gid+1])
		if start < 0 || end < start || end > len(glyf) {
			return nil, nil, fmt.Errorf("glyph %d has invalid loca offsets", gid)
		}
		newGlyf = append(newGlyf, glyf[start:end]...)
		if locFormat == 0 && len(newGlyf)%2 != 0 {
			newGlyf = append(newGlyf, 0)
		}
	}
	newOffsets[len(newOffsets)-1] = uint32(len(newGlyf))
	newLoca, err := encodeLocaOffsets(newOffsets, locFormat)
	if err != nil {
		return nil, nil, err
	}
	return newGlyf, newLoca, nil
}

func encodeLocaOffsets(offsets []uint32, locFormat int16) ([]byte, error) {
	switch locFormat {
	case 0:
		out := make([]byte, len(offsets)*2)
		for i, offset := range offsets {
			if offset%2 != 0 || offset/2 > 0xFFFF {
				return nil, fmt.Errorf("subset glyf offsets do not fit short loca format")
			}
			binary.BigEndian.PutUint16(out[i*2:], uint16(offset/2))
		}
		return out, nil
	case 1:
		out := make([]byte, len(offsets)*4)
		for i, offset := range offsets {
			binary.BigEndian.PutUint32(out[i*4:], offset)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported loca format %d", locFormat)
	}
}

func writeTTFWithChecksums(scalerType [4]byte, records []ttfTableRecord) ([]byte, bool, error) {
	numTables := len(records)
	headerLen := ttfOffsetTableSize + numTables*ttfTableRecordSize
	tableOffset := align4(headerLen)
	out := make([]byte, tableOffset)
	copy(out[:4], scalerType[:])
	binary.BigEndian.PutUint16(out[4:], uint16(numTables))
	searchRange, entrySelector, rangeShift := ttfSearchParams(numTables)
	binary.BigEndian.PutUint16(out[6:], searchRange)
	binary.BigEndian.PutUint16(out[8:], entrySelector)
	binary.BigEndian.PutUint16(out[10:], rangeShift)

	headAdjustmentOffset := -1
	for i, record := range records {
		directoryOffset := ttfOffsetTableSize + i*ttfTableRecordSize
		copy(out[directoryOffset:directoryOffset+4], record.Tag)
		binary.BigEndian.PutUint32(out[directoryOffset+4:], tableChecksum(record.Data))
		binary.BigEndian.PutUint32(out[directoryOffset+8:], uint32(tableOffset))
		binary.BigEndian.PutUint32(out[directoryOffset+12:], uint32(len(record.Data)))
		if record.Tag == "head" {
			headAdjustmentOffset = tableOffset + 8
		}
		out = append(out, record.Data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		tableOffset = len(out)
	}
	if headAdjustmentOffset < 0 || headAdjustmentOffset+4 > len(out) {
		return nil, false, fmt.Errorf("subset font missing head checksum adjustment")
	}
	adjustment := uint32(0xB1B0AFBA - uint64(tableChecksum(out)))
	binary.BigEndian.PutUint32(out[headAdjustmentOffset:], adjustment)
	return out, true, nil
}

func tableChecksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		var word [4]byte
		copy(word[:], data[i:min(i+4, len(data))])
		sum += binary.BigEndian.Uint32(word[:])
	}
	return sum
}

func ttfSearchParams(numTables int) (uint16, uint16, uint16) {
	maxPower := 1
	entrySelector := 0
	for maxPower*2 <= numTables {
		maxPower *= 2
		entrySelector++
	}
	searchRange := maxPower * 16
	return uint16(searchRange), uint16(entrySelector), uint16(numTables*16 - searchRange)
}

func align4(value int) int {
	return (value + 3) &^ 3
}
