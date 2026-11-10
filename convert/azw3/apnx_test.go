package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
)

type testAPNX struct {
	Content map[string]string
	Pages   map[string]string
	Offsets []uint32
}

func parseAPNX(t *testing.T, data []byte) testAPNX {
	t.Helper()
	if len(data) < 20 || binary.BigEndian.Uint16(data) != 1 || binary.BigEndian.Uint16(data[2:]) != 1 {
		t.Fatal("APNX version header is incorrect")
	}
	pageOffset, contentLength := int(binary.BigEndian.Uint32(data[4:])), int(binary.BigEndian.Uint32(data[8:]))
	if pageOffset != contentLength+12 || pageOffset+8 > len(data) {
		t.Fatal("APNX page-header pointer does not follow its content JSON")
	}
	result := testAPNX{}
	if err := json.Unmarshal(data[12:pageOffset], &result.Content); err != nil {
		t.Fatal(err)
	}
	headerLength, count := int(binary.BigEndian.Uint16(data[pageOffset+2:])), int(binary.BigEndian.Uint16(data[pageOffset+4:]))
	if binary.BigEndian.Uint16(data[pageOffset:]) != 1 || binary.BigEndian.Uint16(data[pageOffset+6:]) != 32 ||
		pageOffset+8+headerLength+4*count != len(data) {
		t.Fatal("APNX offset count, JSON byte length or offset width is incorrect")
	}
	if err := json.Unmarshal(data[pageOffset+8:pageOffset+8+headerLength], &result.Pages); err != nil {
		t.Fatal(err)
	}
	for i := range count {
		result.Offsets = append(result.Offsets, binary.BigEndian.Uint32(data[pageOffset+8+headerLength+4*i:]))
	}
	return result
}

func apnxTestPublication(t *testing.T, ebook bool) (*publication.Publication, *textLayout, *metadataLayout, *config.DocumentConfig) {
	t.Helper()
	c, cfg := metadataTestContent()
	if ebook {
		c.KindleEbook, c.ASIN = true, "b012345678"
	}
	pub := textTestPublication(t)
	resources, _ := resourceTestPublication(t)
	pub.Images, pub.CoverID = resources.Images, "a"
	pub.Pages = []content.PageMapEntry{
		{PageNum: 1, Filename: "cover.xhtml"},
		{PageNum: 2, Filename: "Text/first.xhtml"},
		{PageNum: 3, Filename: "Text/first.xhtml", SpanID: "first"},
		{PageNum: 7, Filename: "Text/second.xhtml", SpanID: "late"},
		{PageNum: 8, Filename: "Notes/notes.xhtml", SpanID: "n_é"},
	}
	pub, err := prepareCoverDocument(context.Background(), pub, &cfg.Images)
	if err != nil {
		t.Fatal(err)
	}
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	cfg.PageMap.Enable = true
	return pub, text, metadata, cfg
}

func TestCompileAPNX(t *testing.T) {
	pub, text, metadata, cfg := apnxTestPublication(t, false)
	pagesBefore, positionsBefore, rawBefore := slices.Clone(pub.Pages), maps.Clone(text.Positions), bytes.Clone(text.Raw)
	bookPath := filepath.Join(t.TempDir(), "Книга with spaces.azw3")
	var expected []byte
	for _, mode := range []string{"app", "eink"} {
		t.Run(mode, func(t *testing.T) {
			cfg.AZW3.APNX = mode
			metadata.UID = 1 // GUID is the same 32-bit book UID, including leading zeroes.
			result, err := compileAPNX(context.Background(), bookPath, pub, text, metadata, cfg, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			if result == nil {
				t.Fatal("requested APNX was not prepared")
			}
			decoded := parseAPNX(t, result.Data)
			want := map[string]string{
				"contentGuid": "00000001", "asin": metadata.ASIN, "cdeType": "PDOC",
				"format": "MOBI_8", "fileRevisionId": "1", "acr": metadata.Database,
			}
			if !reflect.DeepEqual(decoded.Content, want) || decoded.Pages["asin"] != metadata.ASIN ||
				decoded.Pages["pageMap"] != "(1,a,1),(4,a,7)" {
				t.Errorf("APNX identity or numeric page ranges are incorrect: %+v", decoded)
			}
			for i, page := range pub.Pages {
				want := text.Positions[linkTarget{Filename: page.Filename, ID: page.SpanID}].Position
				if decoded.Offsets[i] != want {
					t.Errorf("page %d offset = %d, want %d UTF-8 bytes", page.PageNum, decoded.Offsets[i], want)
				}
			}
			if expected == nil {
				expected = bytes.Clone(result.Data)
			} else if !bytes.Equal(expected, result.Data) {
				t.Error("app/eink placement must not change APNX data")
			}
			again, err := compileAPNX(context.Background(), bookPath, pub, text, metadata, cfg, zap.NewNop())
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Error("APNX preparation is not deterministic")
			}
			if _, err := os.Stat(result.Path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("APNX compilation must not publish files or create sidecar directories")
			}
			if mode == "eink" {
				if _, err := os.Stat(filepath.Dir(result.Path)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("APNX compilation created the device sidecar directory before staging")
				}
			}
		})
	}
	if !reflect.DeepEqual(pub.Pages, pagesBefore) || !maps.Equal(text.Positions, positionsBefore) || !bytes.Equal(text.Raw, rawBefore) {
		t.Error("APNX preparation changed source page tracking or frozen text positions")
	}
}

func TestAPNXSuppression(t *testing.T) {
	for _, tt := range []struct {
		mode     string
		enabled  bool
		empty    bool
		wantWarn bool
	}{
		{mode: "", enabled: true}, {mode: "none", enabled: true},
		{mode: "eink", enabled: false, wantWarn: true}, {mode: "app", enabled: false, wantWarn: true},
		{mode: "app", enabled: true, empty: true, wantWarn: true},
	} {
		t.Run(fmt.Sprintf("%s/enabled=%t/empty=%t", tt.mode, tt.enabled, tt.empty), func(t *testing.T) {
			pub, text, metadata, cfg := apnxTestPublication(t, false)
			cfg.AZW3.APNX, cfg.PageMap.Enable = tt.mode, tt.enabled
			if tt.empty {
				pub.Pages = nil
			}
			before := slices.Clone(pub.Pages)
			core, observed := observer.New(zap.WarnLevel)
			result, err := compileAPNX(context.Background(), "", pub, text, metadata, cfg, zap.New(core))
			if err != nil || result != nil {
				t.Fatalf("suppressed APNX = %+v, error = %v", result, err)
			}
			wantWarnings := 0
			if tt.wantWarn {
				wantWarnings = 1
			}
			if observed.Len() != wantWarnings || !reflect.DeepEqual(before, pub.Pages) {
				t.Error("suppression lost its warning or mutated the shared page map")
			}
		})
	}
}

func TestAPNXOutputPath(t *testing.T) {
	for _, tt := range []struct {
		book, mode, want string
		invalid          bool
	}{
		{"/books/My book.azw3", "app", "/books/My book.apnx", false},
		{"/books/My book.azw3", "eink", "/books/My book.sdr/My book.apnx", false},
		{"Книга.AZW3", "eink", "Книга.sdr/Книга.apnx", false},
		{"book", "app", "book.apnx", false},
		{"", "none", "", false}, {"", "", "", false},
		{"book.azw3", "auto", "", true}, {"", "app", "", true},
		{"/books/", "eink", "", true}, {".azw3", "app", "", true},
		{".", "app", "", true}, {"..", "eink", "", true}, {"book.apnx", "app", "", true},
	} {
		t.Run(tt.book+"/"+tt.mode, func(t *testing.T) {
			got, err := apnxOutputPath(filepath.FromSlash(tt.book), tt.mode)
			if tt.invalid {
				if err == nil {
					t.Fatal("invalid APNX output path was accepted")
				}
				return
			}
			if err != nil || got != filepath.FromSlash(tt.want) {
				t.Fatalf("APNX path = %q, error = %v, want %q", got, err, tt.want)
			}
		})
	}
}

func TestAPNXWireReference(t *testing.T) {
	identity := apnxIdentity{
		ContentGUID: "00000001", ASIN: "B012345678", ContentType: "PDOC",
		Format: "MOBI_8", FileRevisionID: "1", ACR: "APNX_probe",
	}
	data, err := encodeAPNX(identity, "(1,a,1)", []uint32{123, 65536})
	if err != nil {
		t.Fatal(err)
	}
	content := `{"contentGuid":"00000001","asin":"B012345678","cdeType":"PDOC",` +
		`"format":"MOBI_8","fileRevisionId":"1","acr":"APNX_probe"}`
	pages := `{"asin":"B012345678","pageMap":"(1,a,1)"}`
	// Pin complete wire bytes against the legacy PageMapProcessor's APNX layout.
	prefix := fmt.Sprintf("00010001%08x%08x", 12+len(content), len(content))
	middle := fmt.Sprintf("0001%04x00020020", len(pages))
	want, err := hex.DecodeString(prefix + hex.EncodeToString([]byte(content)) + middle + hex.EncodeToString([]byte(pages)) + "0000007b00010000")
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("APNX wire bytes differ from reference: %v", err)
	}
}

func TestAPNXWireBounds(t *testing.T) {
	identity := apnxIdentity{ASIN: "B012345678"}
	overhead := len(`{"asin":"B012345678","pageMap":""}`) + len("(1,c,)")
	pageMap := "(1,c," + strings.Repeat("x", math.MaxUint16-overhead) + ")"
	data, err := encodeAPNX(identity, pageMap, []uint32{math.MaxUint32})
	if err != nil {
		t.Fatal(err)
	}
	decoded := parseAPNX(t, data)
	if decoded.Pages["pageMap"] != pageMap || decoded.Offsets[0] != math.MaxUint32 {
		t.Error("maximum page-header length or 32-bit offset was truncated")
	}
	for _, tt := range []struct {
		name    string
		pageMap string
		offsets []uint32
	}{
		{"oversized JSON header", pageMap + "x", []uint32{0}},
		{"invalid UTF-8", string([]byte{0xff}), []uint32{0}},
		{"empty offsets", "(1,a,1)", nil},
		{"too many offsets", "(1,a,1)", make([]uint32, math.MaxUint16+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if data, err := encodeAPNX(identity, tt.pageMap, tt.offsets); err == nil || data != nil {
				t.Fatal("unrepresentable APNX data should fail without output")
			}
		})
	}
}

func TestAPNXRejectsInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*publication.Publication, *textLayout, *metadataLayout, *config.DocumentConfig)
	}{
		{"unknown mode", func(_ *publication.Publication, _ *textLayout, _ *metadataLayout, cfg *config.DocumentConfig) {
			cfg.AZW3.APNX = "auto"
		}},
		{"zero page number", func(pub *publication.Publication, _ *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			pub.Pages[0].PageNum = 0
		}},
		{"duplicate page number", func(pub *publication.Publication, _ *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			pub.Pages[1].PageNum = pub.Pages[0].PageNum
		}},
		{"missing page marker", func(pub *publication.Publication, _ *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			pub.Pages[1].SpanID = "missing"
		}},
		{"decreasing page offset", func(pub *publication.Publication, _ *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			pub.Pages[4].Filename, pub.Pages[4].SpanID = "cover.xhtml", ""
		}},
		{"position outside main flow", func(pub *publication.Publication, text *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			target := linkTarget{Filename: pub.Pages[0].Filename}
			position := text.Positions[target]
			position.Position = text.MainLength
			text.Positions[target] = position
		}},
		{"wrong fragment coordinates", func(pub *publication.Publication, text *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			target := linkTarget{Filename: pub.Pages[3].Filename, ID: pub.Pages[3].SpanID}
			position := text.Positions[target]
			position.Offset++
			text.Positions[target] = position
		}},
		{"position past its own document", func(pub *publication.Publication, text *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			target := linkTarget{Filename: pub.Pages[3].Filename, ID: pub.Pages[3].SpanID}
			position := text.Positions[target]
			next := text.Positions[linkTarget{Filename: "Notes/notes.xhtml"}]
			top := text.Positions[linkTarget{Filename: target.Filename}]
			position.Position, position.Offset = next.Position, next.Position-top.Position
			text.Positions[target] = position
		}},
		{"wrong document FID", func(pub *publication.Publication, text *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			target := linkTarget{Filename: pub.Pages[3].Filename, ID: pub.Pages[3].SpanID}
			position := text.Positions[target]
			position.FID = 0
			text.Positions[target] = position
		}},
		{"wrong main length", func(_ *publication.Publication, text *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			text.MainLength--
		}},
		{"empty content identity", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.ASIN = ""
		}},
		{"unnormalized ASIN", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.ASIN = "b012345678"
		}},
		{"inconsistent ASIN", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.ASIN = "B012345678"
		}},
		{"duplicate EXTH identity", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.EXTH = append(meta.EXTH, exthRecord{ID: 113, Data: []byte(meta.ASIN)})
		}},
		{"missing content key", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.EXTH = slices.DeleteFunc(meta.EXTH, func(field exthRecord) bool { return field.ID == 504 })
		}},
		{"invalid content type", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.ContentType = "EBSP"
		}},
		{"invalid database identity", func(_ *publication.Publication, _ *textLayout, meta *metadataLayout, _ *config.DocumentConfig) {
			meta.Database = "Книга"
		}},
		{"oversized page-name ranges", func(pub *publication.Publication, _ *textLayout, _ *metadataLayout, _ *config.DocumentConfig) {
			page := pub.Pages[0]
			pub.Pages = make([]content.PageMapEntry, 10000)
			for i := range pub.Pages {
				pub.Pages[i] = page
				pub.Pages[i].PageNum = 2*i + 1
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub, text, metadata, cfg := apnxTestPublication(t, false)
			cfg.AZW3.APNX = "app"
			tt.change(pub, text, metadata, cfg)
			if result, err := compileAPNX(context.Background(), "book.azw3", pub, text, metadata, cfg, zap.NewNop()); err == nil || result != nil {
				t.Fatal("invalid APNX inputs were accepted")
			}
		})
	}
	if _, err := compileAPNX(context.Background(), "", nil, nil, nil, nil, zap.NewNop()); err == nil {
		t.Error("missing APNX configuration was accepted")
	}
	cfg := &config.DocumentConfig{AZW3: config.AZW3Config{APNX: "app"}, PageMap: config.PageMapConfig{Enable: true}}
	if _, err := compileAPNX(context.Background(), "book.azw3", nil, nil, nil, cfg, zap.NewNop()); err == nil {
		t.Error("missing APNX compilation inputs were accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileAPNX(ctx, "", nil, nil, nil, nil, zap.NewNop()); !errors.Is(err, context.Canceled) {
		t.Error("APNX preparation did not preserve cancellation precedence")
	}
}

func TestAPNXPageCountLimit(t *testing.T) {
	pub, text, metadata, cfg := apnxTestPublication(t, false)
	cfg.AZW3.APNX = "app"
	page := pub.Pages[0]
	pub.Pages = make([]content.PageMapEntry, math.MaxUint16)
	for i := range pub.Pages {
		pub.Pages[i] = page
		pub.Pages[i].PageNum = i + 1
	}
	result, err := compileAPNX(context.Background(), "book.azw3", pub, text, metadata, cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	decoded := parseAPNX(t, result.Data)
	if len(decoded.Offsets) != math.MaxUint16 || decoded.Pages["pageMap"] != "(1,a,1)" {
		t.Fatal("maximum APNX page count was truncated")
	}
	pub.Pages = append(pub.Pages, content.PageMapEntry{PageNum: math.MaxUint16 + 1, Filename: page.Filename})
	if result, err := compileAPNX(context.Background(), "book.azw3", pub, text, metadata, cfg, zap.NewNop()); err == nil || result != nil {
		t.Fatal("65536-page APNX should fail without output")
	}
}

func TestAPNXJSONByteLengths(t *testing.T) {
	identity := apnxIdentity{ASIN: "B012345678", ACR: `A & "B"`}
	pageMap := "(1,c,Книга|🙂)"
	data, err := encodeAPNX(identity, pageMap, []uint32{0, 20})
	if err != nil {
		t.Fatal(err)
	}
	decoded := parseAPNX(t, data)
	if decoded.Content["acr"] != identity.ACR || decoded.Pages["pageMap"] != pageMap {
		t.Fatal("APNX JSON escaping or Unicode byte lengths changed metadata")
	}
}

func TestAPNXKindleUnpack(t *testing.T) {
	for _, ebook := range []bool{false, true} {
		t.Run(fmt.Sprintf("ebook=%t", ebook), func(t *testing.T) {
			pub, text, metadata, cfg := apnxTestPublication(t, ebook)
			for _, mode := range []string{"app", "eink"} {
				for _, compression := range []string{"none", "standard", "huffdic"} {
					t.Run(mode+"/"+compression, func(t *testing.T) {
						cfg.AZW3.APNX = mode
						name := fmt.Sprintf("apnx-ebook-%t-%s-%s", ebook, mode, compression)
						book, err := compileBook(context.Background(), text, nil, metadata, compression)
						if err != nil {
							t.Fatal(err)
						}
						apnx, err := compileAPNX(context.Background(), name+".azw3", pub, text, metadata, cfg, zap.NewNop())
						if err != nil {
							t.Fatal(err)
						}
						assertAPNXBookIdentity(t, apnx.Data, book)
						var output bytes.Buffer
						if err := writePalmDB(context.Background(), &output, book.Database, time.Time{}, book.Records); err != nil {
							t.Fatal(err)
						}
						writeProbeArtifacts(t, map[string][]byte{name + ".azw3": output.Bytes(), name + ".apnx": apnx.Data})
						work := unpackProbe(t, name, output.Bytes(), apnx.Data)
						assertUnpackedPages(t, work, pub, text)
					})
				}
			}
		})
	}
}

func assertAPNXBookIdentity(t *testing.T, data []byte, book *bookLayout) {
	t.Helper()
	decoded := parseAPNX(t, data)
	header := book.Records[0]
	if decoded.Content["acr"] != book.Database || decoded.Content["contentGuid"] != fmt.Sprintf("%08x", binary.BigEndian.Uint32(header[0x20:])) ||
		decoded.Content["format"] != "MOBI_8" || decoded.Content["fileRevisionId"] != "1" {
		t.Fatal("APNX does not identify the serialized native book")
	}
	exth := int(16 + binary.BigEndian.Uint32(header[0x14:]))
	count := int(binary.BigEndian.Uint32(header[exth+8:]))
	fields := make(map[uint32][]string)
	at := exth + 12
	for range count {
		if at+8 > len(header) {
			t.Fatal("truncated book EXTH header")
		}
		id, size := binary.BigEndian.Uint32(header[at:]), int(binary.BigEndian.Uint32(header[at+4:]))
		if size < 8 || at+size > len(header) {
			t.Fatal("invalid book EXTH field length")
		}
		fields[id] = append(fields[id], string(header[at+8:at+size]))
		at += size
	}
	for _, field := range []struct {
		id   uint32
		want string
	}{{113, decoded.Content["asin"]}, {504, decoded.Pages["asin"]}, {501, decoded.Content["cdeType"]}} {
		if len(fields[field.id]) != 1 || fields[field.id][0] != field.want {
			t.Fatalf("APNX identity does not match raw book EXTH %d", field.id)
		}
	}
	if decoded.Content["asin"] != decoded.Pages["asin"] {
		t.Fatal("APNX content/page header identifiers disagree")
	}
}

func TestAPNXEmptyPageMarker(t *testing.T) {
	c, cfg := metadataTestContent()
	cfg.PageMap.Enable, cfg.AZW3.APNX = true, "app"
	pub, _ := resourceTestPublication(t)
	marker := pub.Chapters[0].Doc.FindElement("//body").CreateElement("span")
	marker.CreateAttr("id", "page-é")
	marker.CreateAttr("class", "page-marker")
	pub.Pages = []content.PageMapEntry{{PageNum: 1, Filename: "chapter.xhtml", SpanID: "page-é"}}
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(text.Documents[0].Data, []byte(`<span id="page-é" class="page-marker"></span>`)) || len(marker.Child) != 0 {
		t.Fatal("native page-marker lowering failed or changed the shared source")
	}
	metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	apnx, err := compileAPNX(context.Background(), "page-marker.azw3", pub, text, metadata, cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	decoded := parseAPNX(t, apnx.Data)
	want := bytes.Index(text.Documents[0].Data, []byte(`<span id="page-é"`))
	if decoded.Offsets[0] != uint32(want) {
		t.Fatal("APNX must retain the marker's frozen tag-start position, not a decoder-specific offset adjustment")
	}
	data := assembleTextProbe(t, text, nil, metadata, "standard")
	work := unpackProbe(t, "apnx-empty-marker", data, apnx.Data)
	assertUnpackedPages(t, work, pub, text)
}

func assertUnpackedPages(t *testing.T, work string, pub *publication.Publication, text *textLayout) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(work, "unpacked", "mobi8", "OEBPS", "page-map.xml"))
	if err != nil {
		t.Fatal(err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		t.Fatal(err)
	}
	pages := doc.Root().SelectElements("page")
	if len(pages) != len(pub.Pages) {
		t.Fatalf("independent page-map has %d pages, want %d", len(pages), len(pub.Pages))
	}
	for i, page := range pub.Pages {
		position := text.Positions[linkTarget{Filename: page.Filename, ID: page.SpanID}]
		href := pages[i].SelectAttrValue("href", "")
		uri, err := url.Parse(href)
		if err != nil || path.Base(uri.Path) != fmt.Sprintf("part%04d.xhtml", position.FID) ||
			page.SpanID != "" && uri.Fragment != page.SpanID || pages[i].SelectAttrValue("name", "") != strconv.Itoa(page.PageNum) {
			t.Errorf("page %d reconstructed as %s (%s)", page.PageNum, href, pages[i].SelectAttrValue("name", ""))
		}
	}
}

func FuzzAPNXSerialization(f *testing.F) {
	for _, seed := range []string{"(1,a,1)", "(1,c,Книга|🙂)", `quotes " & < >`, string([]byte{0xff})} {
		f.Add(seed, uint8(3))
	}
	f.Fuzz(func(t *testing.T, pageMap string, count uint8) {
		if len(pageMap) > 4096 {
			t.Skip()
		}
		identity := apnxIdentity{ASIN: "B012345678", ContentGUID: "00000001", Format: "MOBI_8", ContentType: "PDOC"}
		offsets := make([]uint32, 1+int(count)%32)
		for i := range offsets {
			offsets[i] = uint32(i) * 4096
		}
		data, err := encodeAPNX(identity, pageMap, offsets)
		if !utf8.ValidString(pageMap) {
			if err == nil || data != nil {
				t.Fatal("invalid UTF-8 page names were accepted")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		decoded := parseAPNX(t, data)
		if decoded.Pages["pageMap"] != pageMap || !slices.Equal(decoded.Offsets, offsets) {
			t.Fatal("APNX JSON/binary round trip changed page names or offsets")
		}
	})
}
