package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
	"fbc/convert/tocnav"
	"fbc/fb2"
)

func metadataTestContent() (*content.Content, *config.DocumentConfig) {
	number := 7
	c := &content.Content{SrcName: "metadata.fb2", OutputFormat: common.OutputFmtAzw3, Book: &fb2.FictionBook{
		Description: fb2.Description{
			DocumentInfo: fb2.DocumentInfo{ID: "01353247-b826-4fac-8129-ce00e9af2154"},
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Книга & café"}, Lang: language.BritishEnglish,
				Authors:     []fb2.Author{{FirstName: "Иван", LastName: "Иванов"}, {Nickname: "Second"}},
				Translators: []fb2.Author{{FirstName: "Пётр", LastName: "Переводчик"}},
				Genres:      []fb2.GenreRef{{Value: "sf"}}, Keywords: &fb2.TextField{Value: "SF; Keyword, Другое"},
				Date: &fb2.Date{Value: time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC)},
				Annotation: &fb2.Flow{Items: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
					Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Описание & café"}},
				}}}},
				Sequences: []fb2.Sequence{{Name: "Серия & One", Number: &number, Children: []fb2.Sequence{{Name: "Subset"}}}},
			},
			PublishInfo: &fb2.PublishInfo{Publisher: &fb2.TextField{Value: " Publisher & Co "}, ISBN: &fb2.TextField{Value: "978-0-306-40615-7"}},
		},
	}}
	cfg := &config.DocumentConfig{Metainformation: config.MetainformationConfig{
		TitleTemplate:       `{{.Format}}: {{.Title}}`,
		CreatorNameTemplate: `{{if .LastName}}{{.LastName}}, {{.FirstName}}{{else}}{{.Nickname}}{{end}} [{{.Format}}]`,
	}}
	return c, cfg
}

func metadataValues(metadata *metadataLayout, id uint32) [][]byte {
	var values [][]byte
	for _, field := range metadata.EXTH {
		if field.ID == id {
			values = append(values, field.Data)
		}
	}
	return values
}

func metadataNumber(t *testing.T, metadata *metadataLayout, id uint32) uint32 {
	t.Helper()
	values := metadataValues(metadata, id)
	if len(values) != 1 || len(values[0]) != 4 {
		t.Fatalf("EXTH %d must contain one big-endian integer", id)
	}
	return binary.BigEndian.Uint32(values[0])
}

func metadataXML(t *testing.T, data []byte) *etree.Document {
	t.Helper()
	if string(data[:4]) != "RESC" || binary.BigEndian.Uint32(data[4:]) != 16 || binary.BigEndian.Uint32(data[8:]) != 1 {
		t.Fatal("invalid RESC binary header")
	}
	headerSize := binary.BigEndian.Uint32(data[12:])
	header := string(data[16 : 16+headerSize])
	size, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(header, "size="), "&version=1&type=1"), 32, 32)
	if err != nil || size != uint64(len(data))-16-uint64(headerSize) {
		t.Fatalf("RESC declares an incorrect XML byte size: %s", header)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data[16+headerSize:]); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCompileMetadata(t *testing.T) {
	for _, ebook := range []bool{false, true} {
		t.Run(fmt.Sprintf("ebook=%t", ebook), func(t *testing.T) {
			c, cfg := metadataTestContent()
			c.KindleEbook = ebook
			if ebook {
				c.ASIN = "b012345678"
			}
			pub, _ := resourceTestPublication(t)
			pub.CoverID = "a"
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(text.Raw)
			resourcesBefore := append([][]byte(nil), text.Resources...)
			start := text.Positions[linkTarget{Filename: "chapter.xhtml"}]
			metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, &navigationLayout{Start: &start}, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			kind := ""
			if ebook {
				kind = "EBOK"
			}
			if metadata.ContentType != kind || metadata.Title != "azw3: Книга & café" || metadata.Language != 0x809 || !metadata.HasFonts {
				t.Errorf("incorrect book metadata: %+v", metadata)
			}
			if ebook && metadata.ASIN != "B012345678" || !ebook && metadata.ASIN != "" {
				t.Errorf("incorrect ASIN/content ID %q", metadata.ASIN)
			}
			for id, want := range map[uint32][]string{
				100: {"Иванов, Иван [azw3]", "Second [azw3]"}, 101: {"Publisher & Co"},
				103: {"Описание & café"}, 104: {"9780306406157"}, 105: {"sf", "Keyword", "Другое"}, 106: {"2020-02-29"},
				108: {"Переводчик, Пётр [azw3]"},
				503: {metadata.Title}, 524: {"en-GB"}, 528: {"true"},
				129: {"kindle:embed:0004"},
			} {
				var got []string
				for _, value := range metadataValues(metadata, id) {
					got = append(got, string(value))
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("EXTH %d = %q, want %q", id, got, want)
				}
			}
			for id, want := range map[uint32]string{113: metadata.ASIN, 501: kind, 504: metadata.ASIN} {
				values := metadataValues(metadata, id)
				if want == "" {
					if len(values) != 0 {
						t.Errorf("local output must omit EXTH %d", id)
					}
				} else if len(values) != 1 || string(values[0]) != want {
					t.Errorf("explicit catalog output lost EXTH %d", id)
				}
			}
			if metadataNumber(t, metadata, 116) != start.Position || metadataNumber(t, metadata, 125) != 5 ||
				metadataNumber(t, metadata, 131) != 3 || metadataNumber(t, metadata, 201) != 1 || metadataNumber(t, metadata, 202) != 4 {
				t.Error("metadata, cover or thumbnail namespace offsets are incorrect")
			}
			xml := metadataXML(t, metadata.Resources[3])
			if xml.FindElement("//dc:identifier").Text() != c.Book.Description.DocumentInfo.ID ||
				xml.FindElement("//dc:contributor").Text() != "Переводчик, Пётр [azw3]" ||
				len(xml.FindElements("//meta[@property='belongs-to-collection']")) != 2 {
				t.Error("source UUID, translator or nested collections were lost")
			}
			if xml.FindElement("//meta[@property='role']").Text() != "trl" ||
				xml.FindElement("//meta[@refines='#collection2'][@property='collection-type']").Text() != "set" ||
				xml.FindElement("//meta[@property='group-position']").Text() != "7" {
				t.Error("translator role, nested collection type or series number was lost")
			}
			thumb := metadata.Resources[4]
			decoded, err := jpeg.DecodeConfig(bytes.NewReader(thumb))
			if err != nil || decoded.Width != 330 || decoded.Height != 470 {
				t.Fatalf("incorrect thumbnail dimensions: %+v/%v", decoded, err)
			}
			if !bytes.Equal(thumb[6:11], []byte("JFIF\x00")) || thumb[13] != 1 ||
				binary.BigEndian.Uint16(thumb[14:]) != 300 || binary.BigEndian.Uint16(thumb[16:]) != 300 {
				t.Error("thumbnail must carry 300-DPI JFIF metadata")
			}
			if !bytes.Equal(before, text.Raw) || !reflect.DeepEqual(text.Resources, resourcesBefore) {
				t.Error("metadata preparation changed frozen text or resources")
			}
			again, err := compileMetadata(context.Background(), c, cfg, pub, text, &navigationLayout{Start: &start}, zap.NewNop())
			if err != nil || !reflect.DeepEqual(again, metadata) {
				t.Error("metadata compilation is not deterministic")
			}
			data := assembleTextProbe(t, text, nil, metadata, "standard")
			header := binary.BigEndian.Uint32(data[78:])
			flags := binary.BigEndian.Uint32(data[header+0x80:])
			if flags != 0x1050 {
				t.Errorf("standalone book with fonts has wrong EXTH flags: 0x%x", flags)
			}
		})
	}
}

func TestMetadataIdentityPolicy(t *testing.T) {
	for _, test := range []struct {
		name, asin, wantASIN, wantType string
		ebook                          bool
	}{
		{name: "local"},
		{name: "explicit ASIN", asin: "b012345678", wantASIN: "B012345678", wantType: "PDOC"},
		{name: "ebook", ebook: true, wantASIN: "J1G60Q4JFU3IH9JQTLNDNU8G4HCVW39P", wantType: "EBOK"},
		{name: "ebook and ASIN", ebook: true, asin: "b012345678", wantASIN: "B012345678", wantType: "EBOK"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, cfg := metadataTestContent()
			c.ASIN, c.KindleEbook = test.asin, test.ebook
			pub := textTestPublication(t)
			text, err := compileText(t.Context(), pub)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := compileMetadata(t.Context(), c, cfg, pub, text, nil, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			if metadata.ASIN != test.wantASIN || metadata.ContentType != test.wantType {
				t.Fatal("incorrect effective catalog identity", metadata.ASIN, metadata.ContentType)
			}
			for id, want := range map[uint32]string{113: test.wantASIN, 501: test.wantType, 504: test.wantASIN} {
				values := metadataValues(metadata, id)
				if want == "" {
					if len(values) != 0 {
						t.Errorf("local metadata acquired EXTH %d", id)
					}
				} else if len(values) != 1 || string(values[0]) != want {
					t.Errorf("catalog identity EXTH %d = %q, want %q", id, values, want)
				}
			}
		})
	}
}

func TestMetadataCoverReadingOrder(t *testing.T) {
	for _, ebook := range []bool{false, true} {
		t.Run(fmt.Sprintf("ebook=%t", ebook), func(t *testing.T) {
			c, cfg := metadataTestContent()
			c.KindleEbook = ebook
			pub, _ := resourceTestPublication(t)
			pub.CoverID = "a"
			pub.Landmarks = []publication.Landmark{{Kind: "bodymatter", Title: "Start", Href: "chapter.xhtml"}}
			pub, err := prepareCoverPublication(t.Context(), pub, &cfg.Images)
			if err != nil {
				t.Fatal(err)
			}
			text, err := compileText(t.Context(), pub)
			if err != nil {
				t.Fatal(err)
			}
			navigation, err := compileNavigation(t.Context(), pub, text, common.TOCTypeNormal)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := compileMetadata(t.Context(), c, cfg, pub, text, navigation, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			doc := metadataXML(t, metadata.Resources[metadataNumber(t, metadata, 131)])
			spine := doc.FindElements("//spine/itemref")
			if len(spine) != len(text.Documents)+1 || len(spine) != 2 || text.Documents[0].Filename == "cover.xhtml" {
				t.Fatal("virtual cover must not introduce a main-text skeleton")
			}
			if spine[0].SelectAttrValue("linear", "yes") != "no" || spine[0].SelectAttr("skelid") != nil {
				t.Fatal("virtual cover entry must remain outside the main text")
			}
			for i, item := range spine[1:] {
				if item.SelectAttrValue("linear", "yes") != "yes" || item.SelectAttrValue("skelid", "") != fmt.Sprint(i) {
					t.Fatal("native reading-order entry must be linear and map to its real skeleton")
				}
			}
			body := text.Positions[linkTarget{Filename: "chapter.xhtml"}]
			if navigation.Start == nil || *navigation.Start != body || metadataNumber(t, metadata, 116) != body.Position {
				t.Fatal("making the cover reachable must not redirect initial reading away from the body")
			}
		})
	}
}

func TestMetadataNoCoverAndInvalidOptionalValues(t *testing.T) {
	c, cfg := metadataTestContent()
	c.Book.Description.PublishInfo.ISBN.Value = "invalid"
	c.Book.Description.TitleInfo.Date = &fb2.Date{Display: "invalid"}
	cfg.Metainformation.TitleTemplate, cfg.Metainformation.CreatorNameTemplate = "{{", "{{"
	pub := textTestPublication(t)
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Title != c.Book.Description.TitleInfo.BookTitle.Value || metadata.HasFonts || len(metadata.Resources) != 1 {
		t.Error("metadata fallback or coverless/fontless resource handling changed")
	}
	for _, id := range []uint32{104, 106, 116, 201, 202, 203, 528} {
		if len(metadataValues(metadata, id)) != 0 {
			t.Errorf("unexpected absent/invalid optional EXTH field %d", id)
		}
	}
}

func TestMetadataRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*content.Content, *config.DocumentConfig, *publication.Publication, *textLayout)
	}{
		{"invalid book ID", func(c *content.Content, _ *config.DocumentConfig, _ *publication.Publication, _ *textLayout) {
			c.Book.Description.DocumentInfo.ID = "invalid"
		}},
		{"invalid ASIN", func(c *content.Content, _ *config.DocumentConfig, _ *publication.Publication, _ *textLayout) {
			c.ASIN = "invalid"
		}},
		{"empty title", func(c *content.Content, cfg *config.DocumentConfig, _ *publication.Publication, _ *textLayout) {
			cfg.Metainformation.TitleTemplate = ""
			c.Book.Description.TitleInfo.BookTitle.Value = ""
		}},
		{"invalid publisher encoding", func(c *content.Content, _ *config.DocumentConfig, _ *publication.Publication, _ *textLayout) {
			c.Book.Description.PublishInfo.Publisher.Value = string([]byte{0xff})
		}},
		{"missing cover", func(_ *content.Content, _ *config.DocumentConfig, pub *publication.Publication, _ *textLayout) {
			pub.CoverID = "missing"
		}},
		{"changed frozen cover", func(_ *content.Content, _ *config.DocumentConfig, pub *publication.Publication, _ *textLayout) {
			pub.CoverID = "a"
			pub.Images["a"].Data = []byte("changed after text compilation")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, cfg := metadataTestContent()
			pub, _ := resourceTestPublication(t)
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			tt.change(c, cfg, pub, text)
			if result, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop()); err == nil || result != nil {
				t.Error("invalid metadata input accepted")
			}
		})
	}
	if _, err := compileMetadata(context.Background(), nil, nil, nil, nil, nil, zap.NewNop()); err == nil {
		t.Error("missing metadata inputs accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileMetadata(ctx, nil, nil, nil, nil, nil, zap.NewNop()); !errors.Is(err, context.Canceled) {
		t.Error("cancellation should precede metadata validation")
	}
}

func TestMetadataTransliterationAndBrokenCover(t *testing.T) {
	c, cfg := metadataTestContent()
	cfg.Metainformation.Transliterate = true
	pub, _ := resourceTestPublication(t)
	pub.CoverID = "a"
	pub.Images["a"].Data = []byte{0xff, 0xd8, 0xff, 0xe0}
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Title != fb2.Transliterate("azw3: Книга & café") ||
		string(metadataValues(metadata, 100)[0]) != fb2.Transliterate("Иванов, Иван [azw3]") {
		t.Error("metadata transliteration differs from the shared Kindle policy")
	}
	cover := metadataNumber(t, metadata, 201)
	if metadataNumber(t, metadata, 202) != cover || !bytes.Equal(metadata.Resources[cover], pub.Images["a"].Data) {
		t.Error("thumbnail failure must retain and reference the original broken cover")
	}
}

func TestMetadataRetainedSVGCover(t *testing.T) {
	c, cfg := metadataTestContent()
	cfg.Images.UseBroken = true
	pub, _ := resourceTestPublication(t)
	pub.CoverID = "a"
	pub.Images["a"].Data = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path></svg>`)
	pub.Images["a"].MimeType = "image/svg+xml"
	pub, err := prepareCoverPublication(context.Background(), pub, &cfg.Images)
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
	if len(metadataValues(metadata, 201)) != 0 || len(metadataValues(metadata, 202)) != 0 {
		t.Error("retained SVG must not be misrepresented as a raster cover/thumbnail offset")
	}
	ref := text.References[pub.Images["a"].Filename]
	if !bytes.Equal(compiledFlow(t, text, ref.Number), pub.Images["a"].Data) {
		t.Error("original malformed SVG cover was not preserved")
	}
}

func TestMobiLanguage(t *testing.T) {
	for _, tt := range []struct {
		tag  string
		want uint32
	}{
		{"und", 0}, {"ru", 25}, {"en", 9}, {"en-GB", 0x809}, {"en-US", 0x409},
		{"pt-BR", 0x416}, {"zh-Hant", 0x404}, {"zh-Hans", 0x804}, {"es-MX", 0x80a},
		{"uk", 34}, {"sr", 0xc1a}, {"sr-Latn", 0x81a}, {"eo", 0},
	} {
		t.Run(tt.tag, func(t *testing.T) {
			if got := mobiLanguage(language.MustParse(tt.tag)); got != tt.want {
				t.Errorf("language header = 0x%x, want 0x%x", got, tt.want)
			}
		})
	}
}

func TestDatabaseName(t *testing.T) {
	for _, tt := range []struct {
		title string
		want  string
	}{
		{"(Т - 01) Тестовая книга", "S_-_01_SLXYUIGL_QTOJG"},
		{"Title / café", "Title__cafe"},
		{strings.Repeat("К", 40), strings.Repeat("K", 12) + "-" + strings.Repeat("K", 14)},
		{"The Book 12-3", "The_Book_12-3"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ0", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ01", "ABCDEFGHIJKL-OPQRSTUVWXYZ01"},
		{strings.Repeat("Æ", 28), strings.Repeat("Æ", 12) + "-" + strings.Repeat("Æ", 14)},
		{"éèêë ñöüß ÆŒ", "eeee_nouß_ÆA"},
		{"e\u0301 a\u0308", "eP_aW"},
		{"Ελληνικά", "HTTPVRSE"},
		{"中文 日本語", "TR_HWW"},
		{"العربية", "PSHZQYR"},
		{"עברית", "YHEPG"},
		{"🙂😀𝄞", "SEA"},
		{"A\U00010000B", "AB"},
		{"A\U00010020B", "A_B"},
		{"A\U00010041B", "AAB"},
		{"A\U000100e9B", "AeB"},
		{"A\U00010100B", "AAB"},
		{"AĀāB", "AAXB"},
		{"  A   B\tC\nD\rE  ", "A___BCDE"},
		{"A\u00a0B", "AB"},
		{"\u2003Book\u2003", "FBookF"},
		{"-_- A--B__C", "-_-_A--B__C"},
		{"()!?/.,", "."},
		{"!!!", ""},
	} {
		t.Run(tt.title, func(t *testing.T) {
			if got := databaseName(tt.title); got != tt.want {
				t.Errorf("PalmDB name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDatabaseNameKindleGenSingleByteProbes(t *testing.T) {
	// Measured KindleGen output for A<U+0080..U+00FF>B. Zero means
	// the middle character was removed; nonzero values are raw name bytes.
	want, err := hex.DecodeString(
		"0000008300000000000053008c005a000000000000000000000073009c007a59" +
			"000000000000000000006100000000000000000000bc000000006f0000000000" +
			"414141414141c6434545454549494949d04e4f4f4f4f4f00d85555555559dedf" +
			"616161616161e6636565656569696969f06e6f6f6f6f6f00f87575757579fe79")
	if err != nil || len(want) != 128 {
		t.Fatal("invalid measured name fixture", err)
	}
	for i, middle := range want {
		t.Run(fmt.Sprintf("U+%04X", 0x80+i), func(t *testing.T) {
			got, err := encodePalmDBName(databaseName("A" + string(rune(0x80+i)) + "B"))
			expected := []byte{'A'}
			if middle != 0 {
				expected = append(expected, middle)
			}
			expected = append(expected, 'B')
			if err != nil || !bytes.Equal(got, expected) {
				t.Fatalf("name bytes = %x, want %x: %v", got, expected, err)
			}
		})
	}
}

func TestDatabaseNameExternalProbes(t *testing.T) {
	filename := os.Getenv("AZW3_DATABASE_NAME_PROBES")
	if filename == "" {
		t.Skip("set AZW3_DATABASE_NAME_PROBES to a KindleGen name-probe JSON inventory")
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Cases []struct {
			Label          string `json:"label"`
			Title          string `json:"title"`
			HeaderFieldHex string `json:"header_field_hex"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	tested := 0
	for _, probe := range inventory.Cases {
		if probe.HeaderFieldHex == "" {
			continue // KindleGen rejected the source title and produced no book.
		}
		tested++
		t.Run(probe.Label, func(t *testing.T) {
			want, err := hex.DecodeString(probe.HeaderFieldHex)
			if err != nil || len(want) != 32 {
				t.Fatal("invalid raw PalmDB name fixture", err)
			}
			encoded, err := encodePalmDBName(databaseName(probe.Title))
			got := make([]byte, 32)
			copy(got, encoded)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("name for %q = %x, KindleGen produced %x: %v", probe.Title, got, want, err)
			}
		})
	}
	if tested == 0 {
		t.Fatal("inventory contains no successful KindleGen name probes")
	}
}

func TestDatabaseNameHeaderAndAPNX(t *testing.T) {
	for _, test := range []struct{ title, name, raw string }{
		{"(Т - 01) Тестовая книга", "S_-_01_SLXYUIGL_QTOJG", "S_-_01_SLXYUIGL_QTOJG"},
		{"éßÆŒ", "eßÆA", "e\xdf\xc6A"},
		{"!!!", "", ""},
	} {
		t.Run(test.title, func(t *testing.T) {
			c, cfg := metadataTestContent()
			c.Book.Description.TitleInfo.BookTitle.Value = test.title
			cfg.Metainformation.TitleTemplate = ""
			cfg.PageMap.Enable, cfg.AZW3.APNX = true, "app"
			pub := textTestPublication(t)
			pub.Pages = []content.PageMapEntry{{PageNum: 1, Filename: "Text/first.xhtml"}}
			text, err := compileText(t.Context(), pub)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := compileMetadata(t.Context(), c, cfg, pub, text, nil, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			apnx, err := compileAPNX(t.Context(), "named.azw3", pub, text, metadata, cfg, zap.NewNop())
			if err != nil || apnx == nil {
				t.Fatal("name mapping prevented APNX generation", err)
			}
			if got := parseAPNX(t, apnx.Data).Content["acr"]; got != test.name || metadata.Database != got {
				t.Fatal("APNX did not retain the mapped Unicode name", got)
			}
			data := assembleTextProbe(t, text, nil, metadata, "standard")
			if got := string(bytes.TrimRight(data[:32], "\x00")); got != test.raw {
				t.Fatalf("PalmDB name bytes = %x, want %x", got, test.raw)
			}
			if metadata.Title != test.title || string(metadataValues(metadata, 503)[0]) != test.title {
				t.Fatal("database-name mapping changed the full Unicode display title")
			}
			if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
				work := unpackProbe(t, "database-name", data, apnx.Data)
				assertUnpackedRaw(t, work, text.Raw)
				assertUnpackedPages(t, work, pub, text)
			}
		})
	}
}

func FuzzDatabaseName(f *testing.F) {
	for _, title := range []string{"(Т - 01) Тестовая книга", "éßÆŒ", "🙂😀𝄞", "!!!", strings.Repeat("Æ", 40), ""} {
		f.Add(title)
	}
	f.Fuzz(func(t *testing.T, title string) {
		if len(title) > 4096 {
			t.Skip()
		}
		name := databaseName(title)
		encoded, err := encodePalmDBName(name)
		if err != nil || len(encoded) > 27 || !utf8.ValidString(name) || bytes.IndexByte(encoded, 0) >= 0 {
			t.Fatalf("unrepresentable mapped name %q (%x): %v", name, encoded, err)
		}
		if databaseName(title) != name {
			t.Fatal("name mapping is not deterministic")
		}
	})
}

func TestMetadataKindleUnpack(t *testing.T) {
	for _, ebook := range []bool{false, true} {
		t.Run(fmt.Sprintf("ebook=%t", ebook), func(t *testing.T) {
			c, cfg := metadataTestContent()
			c.KindleEbook = ebook
			if ebook {
				c.ASIN = "b012345678"
			}
			pub, _ := resourceTestPublication(t)
			pub.CoverID = "a"
			pub.Navigation = []tocnav.Item{{Title: "Metadata chapter", Href: "chapter.xhtml", Level: 1}}
			pub.Landmarks = []publication.Landmark{{Kind: "bodymatter", Title: "Start", Href: "chapter.xhtml"}}
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
			data := assembleTextProbe(t, text, navigation, metadata, "standard")
			name := fmt.Sprintf("publication-metadata-ebook-%t", ebook)
			writeProbeArtifacts(t, map[string][]byte{name + ".azw3": data})
			work := unpackProbe(t, name, data, nil)
			base := filepath.Join(work, "unpacked", "mobi8", "OEBPS")
			files, err := filepath.Glob(filepath.Join(base, "*.opf"))
			if err != nil || len(files) != 1 {
				t.Fatal("expected one metadata OPF")
			}
			payload, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatal(err)
			}
			writeProbeArtifacts(t, map[string][]byte{name + ".opf": payload})
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(payload); err != nil {
				t.Fatal(err)
			}
			if doc.FindElement("//dc:title").Text() != metadata.Title || len(doc.FindElements("//dc:creator")) != 2 ||
				len(doc.FindElements("//dc:contributor")) != 1 || !bytes.Contains(payload, []byte("Серия &amp; One")) ||
				!bytes.Contains(payload, []byte("Subset")) {
				t.Errorf("independent decoding lost metadata: %s", payload)
			}
			if doc.FindElement("//reference[@type='text']") == nil || doc.FindElement("//reference[@type='cover']") == nil {
				t.Error("real book metadata did not retain start-reading and cover guide entries")
			}
			spine := doc.FindElement("//spine").SelectElements("itemref")
			if len(spine) != 2 || spine[0].SelectAttrValue("linear", "yes") != "no" {
				t.Error("decoder must reconstruct the virtual cover outside the linear text")
			}
			foundCover, foundThumbnail, foundRESC := false, false, false
			err = filepath.WalkDir(filepath.Join(work, "unpacked"), func(file string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				payload, readErr := os.ReadFile(file)
				if readErr != nil {
					return readErr
				}
				foundCover = foundCover || bytes.Equal(payload, pub.Images["a"].Data)
				foundThumbnail = foundThumbnail || bytes.Equal(payload, metadata.Resources[metadataNumber(t, metadata, 202)])
				foundRESC = foundRESC || bytes.Equal(payload, metadata.Resources[metadataNumber(t, metadata, 131)])
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if !foundCover || !foundThumbnail || !foundRESC {
				t.Error("independent decoder did not recover exact cover, thumbnail and RESC bytes")
			}
		})
	}
}

func TestMetadataCoverlessKindleUnpack(t *testing.T) {
	c, cfg := metadataTestContent()
	pub := textTestPublication(t)
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := compileMetadata(context.Background(), c, cfg, pub, text, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	data := assembleTextProbe(t, text, nil, metadata, "standard")
	writeProbeArtifacts(t, map[string][]byte{"publication-metadata-coverless.azw3": data})
	work := unpackProbe(t, "publication-metadata-coverless", data, nil)
	assertUnpackedText(t, work, pub, text)
}
