package azw3

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beevik/etree"

	"fbc/convert/publication"
)

// assembleTextProbe supplies only fixture metadata and text-pass records. It is
// not the production container adapter: resources, navigation, guide and APNX
// must be implemented before an actual publication compiler can be exposed.
func assembleTextProbe(t *testing.T, text *textLayout) []byte {
	t.Helper()
	chunks, err := splitText(text.Raw)
	if err != nil {
		t.Fatal(err)
	}
	header := probeLayout()
	header.TextLength, header.TextRecords = uint32(len(text.Raw)), uint16(len(chunks))
	records := append([][]byte{nil}, chunks...)
	header.FirstNonText = uint32(len(records))
	header.Fragments = uint32(len(records))
	records = append(records, text.Fragments...)
	header.Skeletons = uint32(len(records))
	records = append(records, text.Skeletons...)
	header.FlowCount = text.FlowCount
	if text.FlowCount > 1 {
		header.FDST = uint32(len(records))
		records = append(records, text.FDST)
	}
	header.FirstResource = uint32(len(records))
	records = append(records, []byte{0xe9, 0x8e, 0x0d, 0x0a})
	records[0], err = encodeHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writePalmDB(context.Background(), &output, "Native_text_probe", records); err != nil {
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
	data := assembleTextProbe(t, compiled)
	writeProbeArtifacts(t, map[string][]byte{"publication-text.azw3": data})
	work := unpackProbe(t, "publication-text", data)
	assertUnpackedText(t, work, pub, compiled)
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
