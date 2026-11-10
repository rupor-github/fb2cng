package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/convert/publication"
	"fbc/convert/tocnav"
)

func TestCompileBook(t *testing.T) {
	for _, name := range []string{"text only", "single flow", "images and font", "native SVG", "large CSS flow"} {
		t.Run(name, func(t *testing.T) {
			c, cfg := metadataTestContent()
			pub := textTestPublication(t)
			switch name {
			case "single flow":
				pub.Stylesheet = publication.Stylesheet{}
				for chapter := range pub.Documents() {
					for _, link := range chapter.Doc.FindElements("//link") {
						link.Parent().RemoveChild(link)
					}
				}
			case "images and font":
				pub, _ = resourceTestPublication(t)
				pub.CoverID = "a"
			case "native SVG":
				pub = svgTestPublication(t)
				pub.CoverID = "z"
			case "large CSS flow":
				pub, _ = resourceTestPublication(t)
				pub.Stylesheet.Data = append(pub.Stylesheet.Data, []byte(strings.Repeat("/* Кириллица, café, 中文, 🙂 */\n", 500))...)
			}
			pub.Navigation = []tocnav.Item{{Title: "Chapter", Href: pub.Chapters[0].Filename, Level: 1}}
			pub.Landmarks = []publication.Landmark{{Kind: "bodymatter", Title: "Start", Href: pub.Chapters[0].Filename}}
			pub, err := prepareCoverPublication(context.Background(), pub, &cfg.Images)
			if err != nil {
				t.Fatal(err)
			}
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			navigation, err := compileNavigation(context.Background(), pub, text, common.TOCTypeNormal)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, navigation, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(text.Raw)
			for _, mode := range []string{"none", "standard", "huffdic", ""} {
				t.Run("compression="+mode, func(t *testing.T) {
					book, err := compileBook(context.Background(), text, navigation, metadata, mode)
					if err != nil {
						t.Fatal(err)
					}
					assertBookRecords(t, book, text, navigation, metadata)
					again, err := compileBook(context.Background(), text, navigation, metadata, mode)
					if err != nil || !reflect.DeepEqual(book, again) || !bytes.Equal(before, text.Raw) {
						t.Error("assembly changed frozen bytes or is not deterministic")
					}
					var output bytes.Buffer
					created := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
					if err := writePalmDB(context.Background(), &output, book.Database, created, book.Records); err != nil {
						t.Fatal(err)
					}
					artifact := "assembled-" + strings.ReplaceAll(name, " ", "-") + "-" + book.Compression
					writeProbeArtifacts(t, map[string][]byte{artifact + ".azw3": output.Bytes()})
					if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
						work := unpackProbe(t, artifact, output.Bytes(), nil)
						assertUnpackedRaw(t, work, text.Raw)
						assertUnpackedNavigation(t, work, pub, text, common.TOCTypeNormal)
					}
				})
			}
		})
	}
}

func assertBookRecords(t *testing.T, book *bookLayout, text *textLayout, navigation *navigationLayout, metadata *metadataLayout) {
	t.Helper()
	header := book.Records[0]
	compression := binary.BigEndian.Uint16(header)
	expected := map[string]uint16{"none": 1, "standard": 2, "huffdic": 17480}[book.Compression]
	if expected == 0 || compression != expected || binary.BigEndian.Uint16(header[0xf2:]) != 1 {
		t.Fatal("KF8 output must advertise the selected compression and only UTF-8 overlap trailers")
	}
	if binary.BigEndian.Uint32(header[4:]) != text.MainLength {
		t.Error("PalmDOC header text length must exclude CSS/SVG flows")
	}
	count := int(binary.BigEndian.Uint16(header[8:]))
	var huff *testHuffDecoder
	firstDictionary, dictionaryCount := binary.BigEndian.Uint32(header[0x70:]), binary.BigEndian.Uint32(header[0x74:])
	if compression == 17480 {
		if firstDictionary < uint32(count+1) || dictionaryCount < 2 ||
			uint64(firstDictionary)+uint64(dictionaryCount) > uint64(len(book.Records)) {
			t.Fatal("HUFF/CDIC header dictionary references are invalid")
		}
		huff = newTestHuffDecoder(t, book.Records[firstDictionary:firstDictionary+dictionaryCount])
	} else if firstDictionary != 0 || dictionaryCount != 0 {
		t.Fatal("non-HUFF mode contains a dictionary reference")
	}
	var decoded []byte
	for i, record := range book.Records[1 : 1+count] {
		overlap := int(record[len(record)-1])
		if overlap > 3 || overlap >= len(record) {
			t.Fatal("invalid UTF-8 overlap trailer")
		}
		plain := record[:len(record)-overlap-1]
		if compression == 2 {
			plain = decodePalmDOC(t, plain)
		} else if huff != nil {
			plain = huff.unpack(t, plain)
		}
		if i < count-1 && len(plain) != textRecordSize {
			t.Error("non-final text block does not cover exactly 4096 source bytes")
		}
		decoded = append(decoded, plain...)
		if len(decoded)+overlap > len(text.Raw) ||
			!bytes.Equal(record[len(record)-overlap-1:len(record)-1], text.Raw[len(decoded):len(decoded)+overlap]) {
			t.Fatal("overlap does not match the next source bytes")
		}
	}
	if !bytes.Equal(decoded, text.Raw) || binary.BigEndian.Uint32(header[0x50:]) != uint32(count+1) {
		t.Error("compressed text changed frozen bytes or first-nontext pointer")
	}
	var toc, guide [][]byte
	if navigation != nil {
		toc, guide = navigation.TOC, navigation.Guide
	}
	for _, group := range []struct {
		offset  int
		records [][]byte
	}{
		{0xf8, text.Fragments}, {0xfc, text.Skeletons}, {0xf4, toc}, {0x104, guide}, {0x6c, metadata.Resources},
	} {
		start := binary.BigEndian.Uint32(header[group.offset:])
		if len(group.records) == 0 && group.offset != 0x6c {
			if start != missingRecord {
				t.Errorf("absent group %#x has a non-sentinel pointer", group.offset)
			}
			continue
		}
		if start < uint32(count+1) || uint64(start)+uint64(len(group.records)) > uint64(len(book.Records)) {
			t.Fatalf("group %#x points outside non-text records", group.offset)
		}
		for i, expected := range group.records {
			if !bytes.Equal(book.Records[int(start)+i], expected) {
				t.Errorf("group %#x changed record %d", group.offset, i)
			}
		}
	}
	for _, offset := range []int{0xc8, 0xd0} {
		start := binary.BigEndian.Uint32(header[offset:])
		if start >= uint32(len(book.Records)) || binary.BigEndian.Uint32(header[offset+4:]) != 1 {
			t.Fatalf("ancillary header pointer/count at %#x is invalid", offset)
		}
		if offset == 0xc8 {
			if !bytes.Equal(book.Records[start], encodeFCIS(text.MainLength)) {
				t.Error("FCIS record does not describe main-flow bytes")
			}
		} else if !bytes.Equal(book.Records[start], encodeFLIS()) {
			t.Error("FLIS record differs from the conventional wire format")
		}
	}
	fdst := binary.BigEndian.Uint32(header[0xc0:])
	if text.FlowCount > 1 {
		if fdst >= uint32(len(book.Records)) || !bytes.Equal(book.Records[fdst], text.FDST) {
			t.Error("FDST record moved without updating its header reference")
		}
	} else if fdst != missingRecord {
		t.Error("single-flow book unexpectedly references FDST")
	}
	if binary.BigEndian.Uint32(header[0x100:]) != missingRecord || !bytes.Equal(book.Records[len(book.Records)-1], []byte{0xe9, 0x8e, 13, 10}) {
		t.Error("absent DATP or final EOF record is encoded incorrectly")
	}
}

func assertUnpackedRaw(t *testing.T, work string, expected []byte) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "*.rawml"))
	if err != nil || len(files) != 1 {
		t.Fatal("expected exactly one independently decoded rawML stream")
	}
	got, err := os.ReadFile(files[0])
	if err != nil || !bytes.Equal(got, expected) {
		t.Fatalf("independent decoder changed frozen text/CSS/SVG bytes: %v", err)
	}
}

func TestCompileBookRejectsInvalidLayout(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*textLayout, *metadataLayout)
	}{
		{"empty main flow", func(text *textLayout, _ *metadataLayout) { text.MainLength = 0 }},
		{"oversized main flow", func(text *textLayout, _ *metadataLayout) { text.MainLength = uint32(len(text.Raw)) + 1 }},
		{"invalid UTF-8", func(text *textLayout, _ *metadataLayout) { text.Raw[0] = 0xff }},
		{"missing skeletons", func(text *textLayout, _ *metadataLayout) { text.Skeletons = nil }},
		{"empty index record", func(text *textLayout, _ *metadataLayout) { text.Fragments[0] = nil }},
		{"no flows", func(text *textLayout, _ *metadataLayout) { text.FlowCount = 0 }},
		{"unindexed flow", func(text *textLayout, _ *metadataLayout) { text.FlowCount = 1 }},
		{"truncated FDST", func(text *textLayout, _ *metadataLayout) { text.FDST = text.FDST[:3] }},
		{"wrong FDST signature", func(text *textLayout, _ *metadataLayout) { text.FDST[0] = 'X' }},
		{"wrong main flow length", func(text *textLayout, _ *metadataLayout) { binary.BigEndian.PutUint32(text.FDST[16:], 1) }},
		{"discontinuous flow", func(text *textLayout, _ *metadataLayout) { binary.BigEndian.PutUint32(text.FDST[20:], 1) }},
		{"oversized flow", func(text *textLayout, _ *metadataLayout) {
			binary.BigEndian.PutUint32(text.FDST[24:], uint32(len(text.Raw))+1)
		}},
		{"short final flow", func(text *textLayout, _ *metadataLayout) {
			binary.BigEndian.PutUint32(text.FDST[24:], uint32(len(text.Raw))-1)
		}},
		{"missing resources", func(_ *textLayout, meta *metadataLayout) { meta.Resources = nil }},
		{"renumbered resources", func(_ *textLayout, meta *metadataLayout) { meta.Resources[0] = []byte("replaced") }},
		{"empty resource", func(_ *textLayout, meta *metadataLayout) { meta.Resources = append(meta.Resources, nil) }},
		{"too many records", func(_ *textLayout, meta *metadataLayout) {
			meta.Resources = append(meta.Resources, make([][]byte, math.MaxUint16)...)
		}},
		{"empty title", func(_ *textLayout, meta *metadataLayout) { meta.Title = "" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub, _ := resourceTestPublication(t)
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			c, cfg := metadataTestContent()
			metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			tt.change(text, metadata)
			if book, err := compileBook(context.Background(), text, nil, metadata, "standard"); err == nil || book != nil {
				t.Error("invalid frozen layout was accepted")
			}
		})
	}
	if _, err := compileBook(context.Background(), nil, nil, nil, "standard"); err == nil {
		t.Error("missing compilation passes were accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileBook(ctx, nil, nil, nil, "standard"); !errors.Is(err, context.Canceled) {
		t.Error("cancellation must precede layout validation")
	}
}

func TestAncillaryRecords(t *testing.T) {
	// Exact bytes from the independent small KindleGen KF8 reference, not values
	// regenerated by the same writer under test. FCIS describes 224 main bytes.
	for _, tt := range []struct {
		name string
		data []byte
		wire string
	}{
		{"FLIS", encodeFLIS(), "464c4953000000080041000000000000ffffffff000100030000000300000001ffffffff"},
		{"FCIS", encodeFCIS(224), "4643495300000014000000100000000100000000000000e00000000000000020000000080001000100000000"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want, err := hex.DecodeString(tt.wire)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(tt.data, want) {
				t.Errorf("ancillary bytes = %x, want %x", tt.data, want)
			}
		})
	}
}

func TestCompileBookRecordLimit(t *testing.T) {
	for _, compression := range []string{"none", "standard", "huffdic"} {
		t.Run(compression, func(t *testing.T) {
			pub := textTestPublication(t)
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			c, cfg := metadataTestContent()
			metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			book, err := compileBook(context.Background(), text, nil, metadata, compression)
			if err != nil {
				t.Fatal(err)
			}
			for range math.MaxUint16 - len(book.Records) {
				metadata.Resources = append(metadata.Resources, []byte{1})
			}
			book, err = compileBook(context.Background(), text, nil, metadata, compression)
			if err != nil || len(book.Records) != math.MaxUint16 {
				t.Fatalf("PalmDB record-count boundary rejected: %v", err)
			}
			var output bytes.Buffer
			if err := writePalmDB(context.Background(), &output, book.Database, time.Time{}, book.Records); err != nil {
				t.Fatal(err)
			}
			if binary.BigEndian.Uint16(output.Bytes()[76:]) != math.MaxUint16 {
				t.Error("PalmDB record count was truncated at its 16-bit boundary")
			}
			metadata.Resources = append(metadata.Resources, []byte{1})
			if _, err := compileBook(context.Background(), text, nil, metadata, compression); err == nil {
				t.Error("65536-record book should fail before serialization")
			}
		})
	}
}
