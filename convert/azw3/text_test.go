package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/beevik/etree"
	"golang.org/x/net/html"

	"fbc/convert/publication"
)

func textTestPublication(t *testing.T) *publication.Publication {
	t.Helper()
	document := func(filename, body string) publication.Chapter {
		doc := etree.NewDocument()
		data := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html>` +
			`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">` +
			`<head><link rel="stylesheet" href="../stylesheet.css"/><title>Test</title></head><body>` + body + `</body></html>`
		if err := doc.ReadFromString(data); err != nil {
			t.Fatal(err)
		}
		return publication.Chapter{Filename: filename, Doc: doc}
	}
	return &publication.Publication{
		Chapters: []publication.Chapter{
			document("Text/first.xhtml", `<p id="first">Кириллица 🙂 &amp; café.</p>`+
				`<a id="ref1" epub:type="noteref" href="../Notes/notes.xhtml#n_%C3%A9">Note</a>`+
				`<a id="same" href="#first">Same file</a><a id="next" href="second.xhtml">Next file</a>`+
				`<a id="external" href="https://example.org/?x=1&amp;y=2#section">External</a>`),
			document("Text/second.xhtml", strings.Repeat(`<p>Long Unicode text: Ж🙂 &amp; café.</p>`, 200)+
				`<p id="late">Late target</p><a href="../Text/first.xhtml#first">Previous</a>`+
				`<a name="named"/><a id="named-ref" href="#named">Named anchor</a>`),
			document("Notes/notes.xhtml", `<aside id="n_é" epub:type="footnote"><p><b>Rich</b> note.</p>`+
				`<a id="back" epub:type="backlink" href="../Text/first.xhtml#ref1">Back</a>`+
				`<a id="forward" href="../Text/second.xhtml#late">Late target</a></aside>`),
			{Filename: "Notes/notes.xhtml", AnchorID: "n_é"},
		},
		Stylesheet: publication.Stylesheet{Data: []byte("p { color: navy; }\naside { font-style: italic; }\n")},
	}
}

func TestCompileTextHTMLNonVoidElements(t *testing.T) {
	pub := textTestPublication(t)
	doc := etree.NewDocument()
	input := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Empty tags</title></head><body>` +
		`<div id="empty" class="emptyline"/><a id="anchor"/><span id="marker" class="page-marker"/>` +
		`<p id="following">Following text</p><br/><hr/>` +
		`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0L1 1"/></svg></body></html>`
	if err := doc.ReadFromString(input); err != nil {
		t.Fatal(err)
	}
	pub.Chapters = []publication.Chapter{{Filename: "main.xhtml", Doc: doc}}
	layout, err := compileText(t.Context(), pub)
	if err != nil {
		t.Fatal(err)
	}
	data := layout.Documents[0].Data
	withoutAIDs := regexp.MustCompile(` aid="[0-9A-V]+"`).ReplaceAll(data, nil)
	for _, fragment := range []string{
		`class="emptyline"></div>`, `id="anchor"></a>`, `class="page-marker"></span>`, `<br/>`, `<hr/>`, `d="M0 0L1 1"/>`,
	} {
		if !bytes.Contains(withoutAIDs, []byte(fragment)) {
			t.Errorf("missing HTML/foreign-element serialization %s", fragment)
		}
	}
	parsed, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		for _, attr := range node.Attr {
			if attr.Key == "id" && attr.Val == "following" && (node.Parent == nil || node.Parent.Data != "body") {
				t.Error("HTML reader nested following text inside an empty marker")
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(parsed)
	expansions := 0
	for _, change := range layout.Documents[0].Changes {
		if change.Action == "expanded non-void HTML element" {
			expansions++
		}
	}
	if expansions != 3 {
		t.Fatalf("expected three traced HTML expansions, got %+v", layout.Documents[0].Changes)
	}
	if got, err := doc.WriteToString(); err != nil || got != input {
		t.Fatal("native lowering mutated the shared publication", err)
	}
}

func snapshotPublication(t *testing.T, pub *publication.Publication) [][]byte {
	t.Helper()
	var result [][]byte
	for chapter := range pub.Documents() {
		data, err := chapter.Doc.WriteToBytes()
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, data)
	}
	return result
}

func TestCompileText(t *testing.T) {
	pub := textTestPublication(t)
	before := snapshotPublication(t, pub)
	cssBefore := bytes.Clone(pub.Stylesheet.Data)
	layout, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Documents) != 3 || layout.FlowCount != 2 {
		t.Fatal("virtual navigation entries must not add documents or flows")
	}
	if !reflect.DeepEqual(before, snapshotPublication(t, pub)) || !bytes.Equal(cssBefore, pub.Stylesheet.Data) {
		t.Error("native compilation mutated the shared publication")
	}
	again, err := compileText(context.Background(), pub)
	if err != nil || !reflect.DeepEqual(layout, again) {
		t.Error("compiling the same publication is not deterministic")
	}
	mainLength := binary.BigEndian.Uint32(layout.FDST[16:])
	if !bytes.Equal(layout.Raw[mainLength:], pub.Stylesheet.Data) {
		t.Error("CSS flow changed")
	}
	var rebuilt []byte
	for _, doc := range layout.Documents {
		rebuilt = append(rebuilt, doc.Data...)
	}
	if uint32(len(rebuilt)) != mainLength || bytes.Equal(rebuilt, layout.Raw[:mainLength]) {
		t.Error("expected skeleton/fragment rearrangement with unchanged total text length")
	}
	for _, tt := range []struct {
		doc int
		id  string
		to  linkTarget
	}{
		{0, "ref1", linkTarget{"Notes/notes.xhtml", "n_é"}},
		{0, "same", linkTarget{"Text/first.xhtml", "first"}},
		{0, "next", linkTarget{Filename: "Text/second.xhtml"}},
		{1, "named-ref", linkTarget{"Text/second.xhtml", "named"}},
		{2, "back", linkTarget{"Text/first.xhtml", "ref1"}},
		{2, "forward", linkTarget{"Text/second.xhtml", "late"}},
	} {
		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(layout.Documents[tt.doc].Data); err != nil {
			t.Fatal(err)
		}
		link := doc.FindElement("//a[@id='" + tt.id + "']")
		got := link.SelectAttrValue("href", "")
		want := positionURI(layout.Positions[tt.to])
		if got != want || len(got) != len(positionURI(textPosition{})) {
			t.Errorf("link %s = %s, want fixed-width %s", tt.id, got, want)
		}
	}
	if !bytes.Contains(layout.Documents[0].Data, []byte(`href="https://example.org/?x=1&amp;y=2#section"`)) {
		t.Error("external URI changed")
	}
	if !bytes.Contains(layout.Documents[0].Data, []byte(`href="kindle:flow:0001?mime=text/css"`)) {
		t.Error("stylesheet URI was not lowered")
	}
	late := layout.Positions[linkTarget{"Text/second.xhtml", "late"}]
	second := layout.Documents[1].Data
	scan, err := scanDocument(second)
	if err != nil {
		t.Fatal(err)
	}
	offset := bytes.Index(second, []byte(`<p id="late"`))
	if late.Offset != uint32(offset)-scan.BodyStart || late.Position != uint32(len(layout.Documents[0].Data)+offset) ||
		int(late.Offset) == utf8.RuneCount(second[scan.BodyStart:offset]) {
		t.Error("fragment links must use encoded-byte offsets in reassembled text")
	}
	if late.Offset <= textRecordSize {
		t.Error("test must exercise a target beyond the first text record")
	}
	var skeletons, fragments []indexEntry
	var start uint32
	var stringsLength uint32
	for i, doc := range layout.Documents {
		scan, err := scanDocument(doc.Data)
		if err != nil {
			t.Fatal(err)
		}
		bodyLength := scan.BodyEnd - scan.BodyStart
		skeletons = append(skeletons, indexEntry{
			Key: fmt.Sprintf("SKEL%010d", i), Values: map[byte][]uint32{1: {1}, 6: {start, uint32(len(doc.Data)) - bodyLength}},
		})
		fragments = append(fragments, indexEntry{
			Key:    fmt.Sprintf("%010d", start+scan.BodyStart),
			Values: map[byte][]uint32{2: {stringsLength}, 3: {uint32(i)}, 4: {uint32(i)}, 6: {0, bodyLength}},
		})
		label := "P-//*[@aid='" + base32(uint32(i), 4) + "']"
		stringsLength += uint32(1 + len(label))
		start += uint32(len(doc.Data))
	}
	assertIndexEntries(t, layout.Skeletons, skeletons)
	leafCount := binary.BigEndian.Uint32(layout.Fragments[0][24:])
	assertIndexEntries(t, layout.Fragments[:1+leafCount], fragments)
}

func TestResolveTextTarget(t *testing.T) {
	for _, tt := range []struct {
		href  string
		want  linkTarget
		local bool
	}{
		{"#n_%C3%A9", linkTarget{"Text/main.xhtml", "n_é"}, true},
		{"../Notes/note%20one.xhtml#x", linkTarget{"Notes/note one.xhtml", "x"}, true},
		{"chapter%25.xhtml#n_%C3%A9", linkTarget{"Text/chapter%.xhtml", "n_é"}, true},
		{"", linkTarget{Filename: "Text/main.xhtml"}, true},
		{"https://example.org/?q=x#y", linkTarget{}, false},
		{"//example.org/chapter.xhtml", linkTarget{}, false},
		{"mailto:author@example.org", linkTarget{}, false},
	} {
		t.Run(tt.href, func(t *testing.T) {
			got, local, err := resolveTextTarget("Text/main.xhtml", tt.href)
			if err != nil || got != tt.want || local != tt.local {
				t.Errorf("resolved = %+v/%v/%v, want %+v/%v", got, local, err, tt.want, tt.local)
			}
		})
	}
	for _, href := range []string{"%xy", "../../outside.xhtml", "/root.xhtml", "a.xhtml?q=1", "a.xhtml?", `a\b.xhtml`, "kindle:flow:0001"} {
		t.Run(href, func(t *testing.T) {
			if _, _, err := resolveTextTarget("Text/main.xhtml", href); err == nil {
				t.Error("unsupported or unsafe local URI accepted")
			}
		})
	}
}

func TestCompileTextRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*publication.Publication)
	}{
		{"no documents", func(p *publication.Publication) { p.Chapters = nil }},
		{"invalid CSS", func(p *publication.Publication) { p.Stylesheet.Data = []byte{0xff} }},
		{"invalid filename", func(p *publication.Publication) { p.Chapters[0].Filename = "../outside.xhtml" }},
		{"missing root", func(p *publication.Publication) { p.Chapters[0].Doc = etree.NewDocument() }},
		{"wrong root", func(p *publication.Publication) { p.Chapters[0].Doc.Root().Tag = "section" }},
		{"no body", func(p *publication.Publication) {
			r := p.Chapters[0].Doc.Root()
			r.RemoveChild(r.SelectElement("body"))
		}},
		{"duplicate body", func(p *publication.Publication) { p.Chapters[0].Doc.Root().CreateElement("body") }},
		{"duplicate ID", func(p *publication.Publication) {
			p.Chapters[0].Doc.FindElement("//body").CreateElement("p").CreateAttr("id", "first")
		}},
		{"missing document", func(p *publication.Publication) {
			p.Chapters[0].Doc.FindElement("//a").CreateAttr("href", "missing.xhtml")
		}},
		{"missing anchor", func(p *publication.Publication) { p.Chapters[0].Doc.FindElement("//a").CreateAttr("href", "#missing") }},
		{"head anchor", func(p *publication.Publication) {
			p.Chapters[0].Doc.FindElement("//head").CreateAttr("id", "head")
			p.Chapters[0].Doc.FindElement("//a").CreateAttr("href", "#head")
		}},
		{"unsupported stylesheet", func(p *publication.Publication) {
			p.Chapters[0].Doc.FindElement("//link").CreateAttr("href", "other.css")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub := textTestPublication(t)
			tt.change(pub)
			if result, err := compileText(context.Background(), pub); err == nil || result != nil {
				t.Error("invalid publication accepted")
			}
		})
	}
	if _, err := compileText(context.Background(), nil); err == nil {
		t.Error("nil publication accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileText(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Error("cancellation should precede input validation")
	}
}

func TestCompileEmptyBody(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<html><head/><body/></html>`); err != nil {
		t.Fatal(err)
	}
	pub := &publication.Publication{Chapters: []publication.Chapter{{Filename: "empty.xhtml", Doc: doc}}}
	layout, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if layout.FlowCount != 1 || !bytes.Contains(layout.Documents[0].Data, []byte(`></body>`)) {
		t.Error("empty body should produce an explicit, zero-length fragment")
	}
}

func TestCompileEmptyStylesheet(t *testing.T) {
	pub := textTestPublication(t)
	pub.Stylesheet.Data = nil
	layout, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	cssStart := binary.BigEndian.Uint32(layout.FDST[20:])
	if layout.FlowCount != 2 || string(layout.Raw[cssStart:]) != "\n" {
		t.Error("a stylesheet link still needs a nonempty CSS flow")
	}
}

func TestScanDocument(t *testing.T) {
	for _, tt := range []struct {
		name string
		xml  string
	}{
		{"malformed XML", `<html><body><p></body></html>`},
		{"no body", `<html><head/></html>`},
		{"self-closing body", `<html><body/></html>`},
		{"duplicate body", `<html><body></body><body></body></html>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := scanDocument([]byte(tt.xml)); err == nil {
				t.Error("invalid serialized document accepted")
			}
		})
	}
	scan, err := scanDocument([]byte(`<html><body id="body"><a id="same" name="same"/><p id=""/></body></html>`))
	if err != nil || len(scan.IDs) != 2 {
		t.Error("empty IDs or equivalent ID/name attributes changed anchor collection")
	}
}

func TestPositionURI(t *testing.T) {
	position := textPosition{FID: 0xfffff, Offset: 0xffffffff}
	uri := positionURI(position)
	parts := strings.Split(uri, ":")
	fid, err := strconv.ParseUint(parts[3], 32, 32)
	if err != nil {
		t.Fatal(err)
	}
	offset, err := strconv.ParseUint(parts[5], 32, 32)
	if err != nil || uint32(fid) != position.FID || uint32(offset) != position.Offset || len(parts[3]) != 4 || len(parts[5]) != 10 {
		t.Error("fixed-width base32 position did not round trip")
	}
}
