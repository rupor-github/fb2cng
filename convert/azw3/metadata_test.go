package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
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
	c := &content.Content{SrcName: "metadata.fb2", OutputFormat: common.OutputFmtEpub3, Book: &fb2.FictionBook{
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
			kind := "PDOC"
			if ebook {
				kind = "EBOK"
			}
			if metadata.ContentType != kind || metadata.Title != "epub3: Книга & café" || metadata.Language != 0x809 || !metadata.HasFonts {
				t.Errorf("incorrect book metadata: %+v", metadata)
			}
			if ebook && metadata.ASIN != "B012345678" || !ebook && metadata.ASIN != "J1G60Q4JFU3IH9JQTLNDNU8G4HCVW39P" {
				t.Errorf("incorrect ASIN/content ID %q", metadata.ASIN)
			}
			for id, want := range map[uint32][]string{
				100: {"Иванов, Иван [epub3]", "Second [epub3]"}, 101: {"Publisher & Co"},
				103: {"Описание & café"}, 104: {"9780306406157"}, 105: {"sf", "Keyword", "Другое"}, 106: {"2020-02-29"},
				108: {"Переводчик, Пётр [epub3]"},
				113: {metadata.ASIN}, 501: {kind}, 503: {metadata.Title}, 504: {metadata.ASIN}, 524: {"en-GB"},
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
			if metadataNumber(t, metadata, 116) != start.Position || metadataNumber(t, metadata, 125) != 5 ||
				metadataNumber(t, metadata, 131) != 3 || metadataNumber(t, metadata, 201) != 1 || metadataNumber(t, metadata, 202) != 4 {
				t.Error("metadata, cover or thumbnail namespace offsets are incorrect")
			}
			xml := metadataXML(t, metadata.Resources[3])
			if xml.FindElement("//dc:identifier").Text() != c.Book.Description.DocumentInfo.ID ||
				xml.FindElement("//dc:contributor").Text() != "Переводчик, Пётр [epub3]" ||
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
	for _, id := range []uint32{104, 106, 116, 201, 202, 203} {
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
	if metadata.Title != fb2.Transliterate("epub3: Книга & café") ||
		string(metadataValues(metadata, 100)[0]) != fb2.Transliterate("Иванов, Иван [epub3]") {
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
		{"Title / café", "Title _ caf_"},
		{strings.Repeat("К", 40), strings.Repeat("_", 31)},
		{"The Book 12-3", "The Book 12-3"},
	} {
		t.Run(tt.title, func(t *testing.T) {
			if got := databaseName(tt.title); got != tt.want {
				t.Errorf("PalmDB name = %q, want %q", got, tt.want)
			}
		})
	}
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
			pub, err := prepareCoverDocument(context.Background(), pub, &cfg.Images)
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
			work := unpackProbe(t, name, data)
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
			if len(spine) != 2 || spine[0].SelectAttrValue("linear", "") != "no" {
				t.Error("RESC did not preserve the adapter cover's non-linear spine entry")
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
	work := unpackProbe(t, "publication-metadata-coverless", data)
	assertUnpackedText(t, work, pub, text)
}
