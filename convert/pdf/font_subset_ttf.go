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

type trueTypeFontSubset struct {
	Data     []byte
	GlyphMap map[uint16]uint16
}

func subsetTrueTypeFont(data []byte, used map[uint16]shapedGlyph) (trueTypeFontSubset, bool, error) {
	tables, err := parseTTFTables(data)
	if err != nil {
		return trueTypeFontSubset{}, false, err
	}
	glyf, hasGlyf := tables.Records["glyf"]
	loca, hasLoca := tables.Records["loca"]
	head, hasHead := tables.Records["head"]
	maxp, hasMaxp := tables.Records["maxp"]
	hhea, hasHhea := tables.Records["hhea"]
	hmtx, hasHmtx := tables.Records["hmtx"]
	if !hasGlyf {
		return trueTypeFontSubset{}, false, nil
	}
	if !hasLoca || !hasHead || !hasMaxp || !hasHhea || !hasHmtx {
		return trueTypeFontSubset{}, false, fmt.Errorf("TrueType font has glyf table but is missing loca/head/maxp/hhea/hmtx")
	}
	if len(head.Data) < 54 || len(maxp.Data) < 6 || len(hhea.Data) < 36 {
		return trueTypeFontSubset{}, false, fmt.Errorf("TrueType font has invalid head/maxp/hhea tables")
	}

	locFormat := int16(binary.BigEndian.Uint16(head.Data[50:52]))
	numGlyphs := int(binary.BigEndian.Uint16(maxp.Data[4:6]))
	numberOfHMetrics := int(binary.BigEndian.Uint16(hhea.Data[34:36]))
	locaOffsets, err := parseLocaOffsets(loca.Data, numGlyphs, locFormat)
	if err != nil {
		return trueTypeFontSubset{}, false, err
	}
	glyphOrder, err := collectSubsetGlyphOrder(glyf.Data, locaOffsets, numGlyphs, used)
	if err != nil {
		return trueTypeFontSubset{}, false, err
	}
	if len(glyphOrder) > 0xFFFF {
		return trueTypeFontSubset{}, false, fmt.Errorf("TrueType subset has too many glyphs: %d", len(glyphOrder))
	}
	glyphMap := make(map[uint16]uint16, len(glyphOrder))
	for subsetGID, originalGID := range glyphOrder {
		glyphMap[uint16(originalGID)] = uint16(subsetGID)
	}

	newGlyf, newLoca, err := buildCompactSubsetGlyfLoca(glyf.Data, locaOffsets, glyphOrder, glyphMap, locFormat)
	if err != nil {
		return trueTypeFontSubset{}, false, err
	}
	newHmtx, err := buildCompactSubsetHmtx(hmtx.Data, glyphOrder, numberOfHMetrics)
	if err != nil {
		return trueTypeFontSubset{}, false, err
	}
	newCmap := buildCompactSubsetCmap(used, glyphMap)

	records := make([]ttfTableRecord, 0, len(tables.Records))
	for tag, record := range tables.Records {
		if dropTrueTypeSubsetTable(tag) {
			continue
		}
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
		case "maxp":
			updated.Data = slices.Clone(record.Data)
			binary.BigEndian.PutUint16(updated.Data[4:6], uint16(len(glyphOrder)))
		case "hhea":
			updated.Data = slices.Clone(record.Data)
			binary.BigEndian.PutUint16(updated.Data[34:36], uint16(len(glyphOrder)))
		case "hmtx":
			updated.Data = newHmtx
		case "cmap":
			updated.Data = newCmap
		}
		records = append(records, updated)
	}
	slices.SortFunc(records, func(a, b ttfTableRecord) int { return strings.Compare(a.Tag, b.Tag) })
	fontData, ok, err := writeTTFWithChecksums(tables.ScalerType, records)
	if err != nil || !ok {
		return trueTypeFontSubset{}, ok, err
	}
	return trueTypeFontSubset{Data: fontData, GlyphMap: glyphMap}, true, nil
}

func dropTrueTypeSubsetTable(tag string) bool {
	switch tag {
	case "BASE", "DSIG", "FFTM", "GDEF", "GPOS", "GSUB", "JSTF", "MATH", "VORG", "kern", "meta", "vhea", "vmtx":
		return true
	default:
		return false
	}
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

func collectSubsetGlyphOrder(glyf []byte, loca []uint32, numGlyphs int, used map[uint16]shapedGlyph) ([]int, error) {
	order := make([]int, 0, len(used)+1)
	seen := make([]bool, numGlyphs)
	var visit func(int) error
	visit = func(gid int) error {
		if gid < 0 || gid >= numGlyphs {
			return fmt.Errorf("composite glyph reference %d out of range", gid)
		}
		if seen[gid] {
			return nil
		}
		seen[gid] = true
		order = append(order, gid)
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
	if numGlyphs > 0 {
		if err := visit(0); err != nil {
			return nil, err
		}
	}
	usedIDs := make([]int, 0, len(used))
	for gid := range used {
		usedIDs = append(usedIDs, int(gid))
	}
	slices.Sort(usedIDs)
	for _, gid := range usedIDs {
		if err := visit(gid); err != nil {
			return nil, err
		}
	}
	return order, nil
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

func buildCompactSubsetGlyfLoca(glyf []byte, loca []uint32, glyphOrder []int, glyphMap map[uint16]uint16, locFormat int16) ([]byte, []byte, error) {
	newGlyf := make([]byte, 0, len(glyphOrder)*64)
	newOffsets := make([]uint32, len(glyphOrder)+1)
	for subsetGID, originalGID := range glyphOrder {
		newOffsets[subsetGID] = uint32(len(newGlyf))
		start := int(loca[originalGID])
		end := int(loca[originalGID+1])
		if start < 0 || end < start || end > len(glyf) {
			return nil, nil, fmt.Errorf("glyph %d has invalid loca offsets", originalGID)
		}
		glyphData := slices.Clone(glyf[start:end])
		if err := rewriteCompositeGlyphReferences(glyphData, glyphMap, originalGID); err != nil {
			return nil, nil, err
		}
		newGlyf = append(newGlyf, glyphData...)
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

func rewriteCompositeGlyphReferences(glyph []byte, glyphMap map[uint16]uint16, originalGID int) error {
	if len(glyph) == 0 {
		return nil
	}
	if len(glyph) < 10 {
		return fmt.Errorf("glyph %d data too short", originalGID)
	}
	contours := int16(binary.BigEndian.Uint16(glyph[0:2]))
	if contours >= 0 {
		return nil
	}
	offset := 10
	for {
		if offset+4 > len(glyph) {
			return fmt.Errorf("composite glyph %d component truncated", originalGID)
		}
		flags := binary.BigEndian.Uint16(glyph[offset : offset+2])
		componentGID := binary.BigEndian.Uint16(glyph[offset+2 : offset+4])
		mapped, ok := glyphMap[componentGID]
		if !ok {
			return fmt.Errorf("composite glyph %d component %d missing from subset", originalGID, componentGID)
		}
		binary.BigEndian.PutUint16(glyph[offset+2:offset+4], mapped)
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
			return fmt.Errorf("composite glyph %d component data truncated", originalGID)
		}
		if flags&0x0020 == 0 {
			break
		}
	}
	return nil
}

func buildCompactSubsetHmtx(hmtx []byte, glyphOrder []int, numberOfHMetrics int) ([]byte, error) {
	if numberOfHMetrics <= 0 || len(hmtx) < numberOfHMetrics*4 {
		return nil, fmt.Errorf("invalid hmtx table")
	}
	out := make([]byte, len(glyphOrder)*4)
	for subsetGID, originalGID := range glyphOrder {
		advance, lsb, err := hmtxMetric(hmtx, originalGID, numberOfHMetrics)
		if err != nil {
			return nil, err
		}
		binary.BigEndian.PutUint16(out[subsetGID*4:], advance)
		binary.BigEndian.PutUint16(out[subsetGID*4+2:], uint16(lsb))
	}
	return out, nil
}

func hmtxMetric(hmtx []byte, gid int, numberOfHMetrics int) (uint16, int16, error) {
	if gid < 0 {
		return 0, 0, fmt.Errorf("invalid glyph id %d", gid)
	}
	if gid < numberOfHMetrics {
		offset := gid * 4
		if offset+4 > len(hmtx) {
			return 0, 0, fmt.Errorf("hmtx metric for glyph %d truncated", gid)
		}
		return binary.BigEndian.Uint16(hmtx[offset:]), int16(binary.BigEndian.Uint16(hmtx[offset+2:])), nil
	}
	advanceOffset := (numberOfHMetrics - 1) * 4
	lsbOffset := numberOfHMetrics*4 + (gid-numberOfHMetrics)*2
	if advanceOffset+4 > len(hmtx) || lsbOffset+2 > len(hmtx) {
		return 0, 0, fmt.Errorf("hmtx left side bearing for glyph %d truncated", gid)
	}
	return binary.BigEndian.Uint16(hmtx[advanceOffset:]), int16(binary.BigEndian.Uint16(hmtx[lsbOffset:])), nil
}

func buildCompactSubsetCmap(used map[uint16]shapedGlyph, glyphMap map[uint16]uint16) []byte {
	entries := make([]ttfCmapEntry, 0, len(used))
	seen := make(map[uint16]bool, len(used))
	for originalGID, glyph := range used {
		if glyph.Rune == 0 || glyph.Rune > 0xFFFF || seen[uint16(glyph.Rune)] {
			continue
		}
		subsetGID, ok := glyphMap[originalGID]
		if !ok {
			continue
		}
		code := uint16(glyph.Rune)
		seen[code] = true
		entries = append(entries, ttfCmapEntry{Code: code, GID: subsetGID})
	}
	slices.SortFunc(entries, func(a, b ttfCmapEntry) int {
		return int(a.Code) - int(b.Code)
	})

	segCount := len(entries) + 1
	segCountX2 := uint16(segCount * 2)
	searchRange, entrySelector, rangeShift := ttfCmapSearchParams(segCount)
	subtableLen := 16 + segCount*8
	subtable := make([]byte, subtableLen)
	binary.BigEndian.PutUint16(subtable[0:], 4)
	binary.BigEndian.PutUint16(subtable[2:], uint16(subtableLen))
	binary.BigEndian.PutUint16(subtable[6:], segCountX2)
	binary.BigEndian.PutUint16(subtable[8:], searchRange)
	binary.BigEndian.PutUint16(subtable[10:], entrySelector)
	binary.BigEndian.PutUint16(subtable[12:], rangeShift)

	endCodeOffset := 14
	startCodeOffset := endCodeOffset + segCount*2 + 2
	idDeltaOffset := startCodeOffset + segCount*2
	idRangeOffsetOffset := idDeltaOffset + segCount*2
	for i, entry := range entries {
		binary.BigEndian.PutUint16(subtable[endCodeOffset+i*2:], entry.Code)
		binary.BigEndian.PutUint16(subtable[startCodeOffset+i*2:], entry.Code)
		binary.BigEndian.PutUint16(subtable[idDeltaOffset+i*2:], uint16(int(entry.GID)-int(entry.Code)))
	}
	sentinel := segCount - 1
	binary.BigEndian.PutUint16(subtable[endCodeOffset+sentinel*2:], 0xFFFF)
	binary.BigEndian.PutUint16(subtable[startCodeOffset+sentinel*2:], 0xFFFF)
	binary.BigEndian.PutUint16(subtable[idDeltaOffset+sentinel*2:], 1)
	// idRangeOffset remains zero for all segments.
	_ = idRangeOffsetOffset

	out := make([]byte, 12+len(subtable))
	binary.BigEndian.PutUint16(out[2:], 1)
	binary.BigEndian.PutUint16(out[4:], 3)
	binary.BigEndian.PutUint16(out[6:], 1)
	binary.BigEndian.PutUint32(out[8:], 12)
	copy(out[12:], subtable)
	return out
}

type ttfCmapEntry struct {
	Code uint16
	GID  uint16
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

func ttfCmapSearchParams(segCount int) (uint16, uint16, uint16) {
	maxPower := 1
	entrySelector := 0
	for maxPower*2 <= segCount {
		maxPower *= 2
		entrySelector++
	}
	searchRange := maxPower * 2
	return uint16(searchRange), uint16(entrySelector), uint16(segCount*2 - searchRange)
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
