package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type bookLayout struct {
	Database    string
	Records     [][]byte
	Compression string
}

// compileBook resolves every physical record reference in a single final layout
// pass. Text positions and resource ordinals are already frozen; compression must
// not change them. This is not a public conversion or file-publication entry point.
func compileBook(
	ctx context.Context, text *textLayout, navigation *navigationLayout, metadata *metadataLayout, compression string,
) (*bookLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if text == nil || metadata == nil {
		return nil, errors.New("compiled text and metadata are required")
	}
	if text.MainLength == 0 || uint64(text.MainLength) > uint64(len(text.Raw)) || len(text.Skeletons) == 0 {
		return nil, errors.New("invalid frozen KF8 text layout")
	}
	if err := validateBookFlows(text); err != nil {
		return nil, err
	}
	if len(metadata.Resources) < len(text.Resources) {
		return nil, errors.New("metadata dropped frozen KF8 resources")
	}
	for i, resource := range text.Resources {
		if !bytes.Equal(resource, metadata.Resources[i]) {
			return nil, errors.New("metadata changed frozen KF8 resource ordinals")
		}
	}
	chunks, err := splitText(text.Raw)
	if err != nil {
		return nil, err
	}
	compressed, err := compileTextRecords(ctx, chunks, compression)
	if err != nil {
		return nil, err
	}
	header := headerLayout{
		Title: metadata.Title, UID: metadata.UID, Language: metadata.Language,
		Compression: compressed.Type, TrailingFlags: 1, TextLength: text.MainLength, TextRecords: uint16(len(chunks)),
		FirstNonText: uint32(len(chunks) + 1), FirstResource: missingRecord,
		FDST: missingRecord, FlowCount: text.FlowCount, Navigation: missingRecord,
		Fragments: missingRecord, Skeletons: missingRecord, Guide: missingRecord,
		FCIS: missingRecord, FLIS: missingRecord, Metadata: metadata.EXTH, HasFonts: metadata.HasFonts,
		HuffmanCount: uint32(len(compressed.Dictionary)),
	}
	var toc, guide, fdst [][]byte
	if navigation != nil {
		toc, guide = navigation.TOC, navigation.Guide
	}
	if text.FlowCount > 1 {
		fdst = [][]byte{text.FDST}
	}
	// Resource records remain contiguous and precede ancillary records, as in a
	// standalone KindleGen reference. DATP and navigation TBS trailers are optional
	// and not emitted: no unknown accelerator tables or trailer flags are invented.
	groups := []struct {
		reference *uint32
		records   [][]byte
	}{
		{&header.Huffman, compressed.Dictionary},
		{&header.Fragments, text.Fragments}, {&header.Skeletons, text.Skeletons},
		{&header.Navigation, toc}, {&header.Guide, guide},
		{&header.FirstResource, metadata.Resources},
		{&header.FDST, fdst}, {&header.FLIS, [][]byte{encodeFLIS()}},
		{&header.FCIS, [][]byte{encodeFCIS(text.MainLength)}},
		{nil, [][]byte{{0xe9, 0x8e, 0x0d, 0x0a}}},
	}
	count := uint64(1 + len(chunks))
	for _, group := range groups {
		count += uint64(len(group.records))
	}
	if count > math.MaxUint16 {
		return nil, errors.New("assembled KF8 book exceeds PalmDB's 65535-record limit")
	}
	records := make([][]byte, 1, int(count))
	records = append(records, compressed.Records...)
	for _, group := range groups {
		// Even an empty resource inventory needs a real scan boundary. Some
		// decoders interpret an absent FirstResource as "immediately after text"
		// and then misidentify structural INDX records as image resources.
		if len(group.records) == 0 && group.reference != &header.FirstResource {
			continue
		}
		if group.reference != nil {
			*group.reference = uint32(len(records))
		}
		for _, record := range group.records {
			if len(record) == 0 {
				return nil, fmt.Errorf("empty KF8 record %d", len(records))
			}
			records = append(records, record)
		}
	}
	records[0], err = encodeHeader(header)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &bookLayout{Database: metadata.Database, Records: records, Compression: compressed.Mode}, nil
}

func validateBookFlows(text *textLayout) error {
	if text.FlowCount == 1 {
		if uint64(text.MainLength) != uint64(len(text.Raw)) {
			return errors.New("KF8 single-flow text has unindexed trailing bytes")
		}
		return nil
	}
	if text.FlowCount == 0 || uint64(len(text.FDST)) != 12+8*uint64(text.FlowCount) || string(text.FDST[:4]) != "FDST" ||
		binary.BigEndian.Uint32(text.FDST[4:]) != 12 || binary.BigEndian.Uint32(text.FDST[8:]) != text.FlowCount {
		return errors.New("invalid frozen KF8 flow directory")
	}
	var previous uint32
	for i := range text.FlowCount {
		entry := text.FDST[12+8*int(i):]
		start, end := binary.BigEndian.Uint32(entry), binary.BigEndian.Uint32(entry[4:])
		if start != previous || end <= start || uint64(end) > uint64(len(text.Raw)) || i == 0 && end != text.MainLength {
			return errors.New("KF8 flow directory disagrees with frozen text")
		}
		previous = end
	}
	if uint64(previous) != uint64(len(text.Raw)) {
		return errors.New("KF8 flow directory omits frozen text bytes")
	}
	return nil
}

// These conventional FLIS/FCIS layouts are confirmed by the small KindleGen KF8
// reference. FCIS's text length describes main-flow bytes, just like PalmDOC's.
func encodeFLIS() []byte {
	data := []byte("FLIS")
	for _, word := range []uint32{8, 0x00410000, 0, missingRecord, 0x00010003, 3, 1, missingRecord} {
		data = binary.BigEndian.AppendUint32(data, word)
	}
	return data
}

func encodeFCIS(textLength uint32) []byte {
	data := []byte("FCIS")
	for _, word := range []uint32{20, 16, 1, 0, textLength, 0, 32, 8, 0x00010001, 0} {
		data = binary.BigEndian.AppendUint32(data, word)
	}
	return data
}
