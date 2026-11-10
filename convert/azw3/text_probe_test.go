package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/convert/publication"
	"fbc/fb2"
)

// assembleTextProbe supplies fixture metadata when needed and exercises the
// selected record assembly mode. APNX and staged publication remain CLI gates.
func assembleTextProbe(
	t *testing.T, text *textLayout, navigation *navigationLayout, metadata *metadataLayout, compression string,
) []byte {
	t.Helper()
	if metadata == nil {
		header := probeLayout()
		metadata = &metadataLayout{
			Title: header.Title, UID: header.UID, Language: header.Language, Database: "Native_text_probe",
			EXTH: header.Metadata, Resources: text.Resources,
		}
		for _, reference := range text.References {
			metadata.HasFonts = metadata.HasFonts || reference.Kind == resourceFont
		}
		if navigation != nil && navigation.Start != nil {
			metadata.EXTH = append(metadata.EXTH, exthRecord{ID: 116, Data: binary.BigEndian.AppendUint32(nil, navigation.Start.Position)})
		}
	}
	book, err := compileBook(context.Background(), text, navigation, metadata, compression)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writePalmDB(context.Background(), &output, book.Database, time.Time{}, book.Records); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestPublicationTextKindleUnpack(t *testing.T) {
	pub := textTestPublication(t)
	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	data := assembleTextProbe(t, compiled, nil, nil, "standard")
	writeProbeArtifacts(t, map[string][]byte{"publication-text.azw3": data})
	work := unpackProbe(t, "publication-text", data, nil)
	assertUnpackedText(t, work, pub, compiled)
}

func TestPublicationResourcesKindleUnpack(t *testing.T) {
	pub, font := resourceTestPublication(t)
	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	data := assembleTextProbe(t, compiled, nil, nil, "standard")
	writeProbeArtifacts(t, map[string][]byte{"publication-resources.azw3": data})
	work := unpackProbe(t, "publication-resources", data, nil)

	wantPayloads := [][]byte{font, pub.Images["a"].Data, pub.Stylesheet.Resources[0].Data}
	found := make([]bool, len(wantPayloads))
	err = filepath.WalkDir(filepath.Join(work, "unpacked", "mobi8", "OEBPS"), func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		payload, readErr := os.ReadFile(filename)
		if readErr != nil {
			return readErr
		}
		for i, want := range wantPayloads {
			if bytes.Equal(payload, want) {
				found[i] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, ok := range found {
		if !ok {
			t.Errorf("resource payload %d was not reconstructed exactly", i)
		}
	}

	xhtmlFiles, err := filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "OEBPS", "Text", "*.xhtml"))
	if err != nil || len(xhtmlFiles) != 1 {
		t.Fatalf("expected one reconstructed XHTML document: %v", err)
	}
	xhtml, err := os.ReadFile(xhtmlFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(xhtml, []byte(`src="../Images/`)) || !bytes.Contains(xhtml, []byte(`url(../Images/`)) {
		t.Errorf("resource links were not reconstructed in XHTML: %s", xhtml)
	}
	styles, err := filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "OEBPS", "Styles", "*.css"))
	if err != nil || len(styles) != 1 {
		t.Fatalf("expected one reconstructed stylesheet: %v", err)
	}
	stylesheet, err := os.ReadFile(styles[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stylesheet, []byte(`url("../Fonts/`)) || !bytes.Contains(stylesheet, []byte(`url("../Images/`)) {
		t.Errorf("resource links were not reconstructed in CSS: %s", stylesheet)
	}
}

func TestPublicationImageFallbackKindleUnpack(t *testing.T) {
	pub, font := resourceTestPublication(t)
	pub.Images["a"] = &fb2.BookImage{
		Filename: "images/art.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG), BuiltinVignette: true,
	}
	pub.Chapters[0].Doc.FindElement("//img").CreateAttr("src", "images/art.svg")
	before := snapshotPublication(t, pub)
	cfg := imageTestConfig()
	cfg.RasterizeSVG = true
	prepared, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compileText(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	data := assembleTextProbe(t, compiled, nil, nil, "standard")
	writeProbeArtifacts(t, map[string][]byte{"publication-image-fallback.azw3": data})
	work := unpackProbe(t, "publication-image-fallback", data, nil)
	base := filepath.Join(work, "unpacked", "mobi8", "OEBPS")
	xhtml, err := os.ReadFile(filepath.Join(base, "Text", "part0000.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	rebuilt := etree.NewDocument()
	if err := rebuilt.ReadFromBytes(xhtml); err != nil {
		t.Fatal(err)
	}
	src := rebuilt.FindElement("//img").SelectAttrValue("src", "")
	if !strings.HasPrefix(src, "../Images/") {
		t.Fatalf("unresolved fallback image URI %q", src)
	}
	image, err := os.ReadFile(filepath.Join(base, "Text", filepath.FromSlash(src)))
	if err != nil || !bytes.Equal(image, prepared.Images["a"].Data) {
		t.Fatalf("SVG fallback did not reconstruct exact prepared JPEG bytes: %v", err)
	}
	styles, err := os.ReadFile(filepath.Join(base, "Styles", "style0001.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(styles, []byte(`url("../Fonts/`)) || !bytes.Contains(styles, []byte(`url("../Images/`)) {
		t.Errorf("CSS raster/font URLs were not reconstructed: %s", styles)
	}
	fonts, err := filepath.Glob(filepath.Join(base, "Fonts", "*.ttf"))
	if err != nil || len(fonts) != 1 {
		t.Fatalf("expected one font: %v", err)
	}
	fontData, err := os.ReadFile(fonts[0])
	if err != nil || !bytes.Equal(fontData, font) {
		t.Fatal("image preparation changed the font payload")
	}
	if !bytes.Equal(bytes.Join(before, nil), bytes.Join(snapshotPublication(t, pub), nil)) {
		t.Error("SVG fallback/lowering changed source markup")
	}
}

func assertUnpackedText(t *testing.T, work string, pub *publication.Publication, compiled *textLayout) {
	t.Helper()
	expectedFiles := make(map[string]string)
	var sources []publication.Chapter
	for source := range pub.Documents() {
		expectedFiles[source.Filename] = fmt.Sprintf("part%04d.xhtml", len(sources))
		sources = append(sources, source)
	}
	for i, source := range sources {
		file := filepath.Join(work, "unpacked", "mobi8", "OEBPS", "Text", expectedFiles[source.Filename])
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt := etree.NewDocument()
		if err := rebuilt.ReadFromBytes(data); err != nil {
			t.Fatal(err)
		}
		if elementText(source.Doc.Root().SelectElement("body")) != elementText(rebuilt.Root().SelectElement("body")) {
			t.Errorf("body text changed in %s", source.Filename)
		}
		// Compare element structure and every attribute, allowing only the native
		// adapter's resource paths, IDs it adds, and equivalent top-of-file links.
		original := source.Doc.Copy()
		original.FindElement("//body").RemoveAttr("aid")
		for _, marker := range original.FindElements("//span") {
			if len(marker.Child) == 0 && slices.Contains(strings.Fields(marker.SelectAttrValue("class", "")), "page-marker") {
				marker.CreateText("") // Native KF8 lowers non-void page markers to explicit end tags.
			}
		}
		sourceLinks := original.FindElements("//a[@href]")
		rebuiltLinks := rebuilt.FindElements("//a[@href]")
		if len(sourceLinks) != len(rebuiltLinks) {
			t.Fatalf("link count changed in %s", source.Filename)
		}
		for j, link := range sourceLinks {
			target, local, err := resolveTextTarget(source.Filename, link.SelectAttrValue("href", ""))
			if err != nil {
				t.Fatal(err)
			}
			got := rebuiltLinks[j].SelectAttrValue("href", "")
			if !local {
				if got != link.SelectAttrValue("href", "") {
					t.Error("external link changed")
				}
				continue
			}
			u, err := url.Parse(got)
			if err != nil || path.Base(u.Path) != expectedFiles[target.Filename] || (target.ID != "" && u.Fragment != target.ID) {
				t.Errorf("link %s in %s reconstructed as %s", link.SelectAttrValue("href", ""), source.Filename, got)
			}
			link.CreateAttr("href", got)
		}
		for _, link := range original.FindElements("//link[@rel='stylesheet']") {
			link.CreateAttr("href", "../Styles/style0001.css")
		}
		want, err := original.WriteToBytes()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, data) {
			at := 0
			for at < min(len(want), len(data)) && want[at] == data[at] {
				at++
			}
			left := max(0, at-40)
			t.Errorf("markup or attributes changed in document %d (%s) at byte %d\nwant: %q\ngot: %q",
				i, source.Filename, at, want[left:min(len(want), at+80)], data[left:min(len(data), at+80)])
		}
	}
	css, err := os.ReadFile(filepath.Join(work, "unpacked", "mobi8", "OEBPS", "Styles", "style0001.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(css, pub.Stylesheet.Data) {
		t.Error("independent reconstruction changed stylesheet bytes")
	}
	rawFiles, err := filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "*.rawml"))
	if err != nil || len(rawFiles) != 1 {
		t.Fatal("expected one decoded rawML flow stream")
	}
	raw, err := os.ReadFile(rawFiles[0])
	if err != nil || !bytes.Equal(raw, compiled.Raw) {
		t.Error("independent decoder changed rawML bytes")
	}
}

func elementText(element *etree.Element) string {
	var result strings.Builder
	for _, token := range element.Child {
		switch token := token.(type) {
		case *etree.CharData:
			result.WriteString(token.Data)
		case *etree.Element:
			result.WriteString(elementText(token))
		}
	}
	return result.String()
}
