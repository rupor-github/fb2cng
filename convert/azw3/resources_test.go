package azw3

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/beevik/etree"

	"fbc/convert/publication"
	"fbc/fb2"
)

func resourceTestPublication(t *testing.T) (*publication.Publication, []byte) {
	t.Helper()
	font, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fonts", "bookerly-regular_9_5.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<html><head><link rel="stylesheet" href="stylesheet.css"/></head><body>` +
		`<img id="body-image" src="images/a.jpg"/>` +
		`<div id="styled" style="background-image: url('other/background.png')">Styled</div>` +
		`</body></html>`); err != nil {
		t.Fatal(err)
	}
	pixel := image.NewRGBA(image.Rect(0, 0, 2, 2))
	pixel.Set(0, 0, color.RGBA{R: 255, A: 255})
	var imageBuffer bytes.Buffer
	if err := jpeg.Encode(&imageBuffer, pixel, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	imageData := bytes.Clone(imageBuffer.Bytes())
	imageBuffer.Reset()
	if err := png.Encode(&imageBuffer, pixel); err != nil {
		t.Fatal(err)
	}
	background := bytes.Clone(imageBuffer.Bytes())
	return &publication.Publication{
		Chapters: []publication.Chapter{{Filename: "chapter.xhtml", Doc: doc}},
		Stylesheet: publication.Stylesheet{
			Data: []byte(`@font-face { font-family: "Bookerly"; src: url("fonts/bookerly.ttf"); }
.hero { background-image: url("other/background.png"); }
`),
			Resources: []fb2.StylesheetResource{
				{Filename: "other/background.png", MimeType: "image/png", Data: background},
				{Filename: "fonts/bookerly.ttf", MimeType: "font/ttf", Data: font},
			},
		},
		Images: fb2.BookImages{
			"z": {Filename: "images/z.jpg", MimeType: "image/jpeg", Data: imageData},
			"a": {Filename: "images/a.jpg", MimeType: "image/jpeg", Data: imageData},
		},
	}, font
}

func TestCompileResourcesAndLowerURIs(t *testing.T) {
	pub, font := resourceTestPublication(t)
	before := snapshotPublication(t, pub)
	cssBefore := bytes.Clone(pub.Stylesheet.Data)

	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Resources) != 3 {
		t.Fatalf("got %d physical resources, want 3", len(compiled.Resources))
	}
	fontRef := compiled.References["fonts/bookerly.ttf"]
	imageRef := compiled.References["images/a.jpg"]
	if fontRef.Number != 1 || fontRef.Kind != resourceFont || imageRef.Number != 2 ||
		compiled.References["images/z.jpg"] != imageRef || compiled.References["other/background.png"].Number != 3 {
		t.Errorf("unexpected deterministic resource inventory: %+v", compiled.References)
	}
	assertCompressedFontRecord(t, compiled.Resources[0], font)
	if !bytes.Equal(compiled.Resources[1], pub.Images["a"].Data) ||
		!bytes.Equal(compiled.Resources[2], pub.Stylesheet.Resources[0].Data) {
		t.Error("raster resource bytes changed")
	}
	document := compiled.Documents[0].Data
	if !bytes.Contains(document, []byte(`src="kindle:embed:0002?mime=image/jpeg"`)) ||
		!bytes.Contains(document, []byte(`url(kindle:embed:0003?mime=image/png)`)) {
		t.Errorf("XHTML resource URIs were not lowered: %s", document)
	}
	cssStart := bytes.Index(compiled.Raw, []byte("@font-face"))
	if cssStart < 0 {
		t.Fatal("lowered CSS flow is missing")
	}
	loweredCSS := compiled.Raw[cssStart:]
	if !bytes.Contains(loweredCSS, []byte(`url("kindle:embed:0001")`)) ||
		!bytes.Contains(loweredCSS, []byte(`url("kindle:embed:0003?mime=image/png")`)) {
		t.Errorf("CSS resource URIs were not lowered: %s", loweredCSS)
	}
	if !reflect.DeepEqual(before, snapshotPublication(t, pub)) || !bytes.Equal(cssBefore, pub.Stylesheet.Data) {
		t.Error("resource lowering mutated the shared publication")
	}

	again, err := compileText(context.Background(), pub)
	if err != nil || !reflect.DeepEqual(compiled, again) {
		t.Error("resource allocation is not deterministic")
	}
}

func assertCompressedFontRecord(t *testing.T, record, want []byte) {
	t.Helper()
	if len(record) < 24 || string(record[:4]) != "FONT" {
		t.Fatal("missing FONT record header")
	}
	if binary.BigEndian.Uint32(record[4:]) != uint32(len(want)) || binary.BigEndian.Uint32(record[8:]) != 1 ||
		binary.BigEndian.Uint32(record[12:]) != 24 || binary.BigEndian.Uint32(record[16:]) != 0 ||
		binary.BigEndian.Uint32(record[20:]) != 0 {
		t.Fatalf("incorrect compressed FONT header: %x", record[:24])
	}
	reader, err := zlib.NewReader(bytes.NewReader(record[24:]))
	if err != nil {
		t.Fatalf("FONT payload is not zlib-compressed: %v", err)
	}
	got, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("decompress FONT: read=%v, close=%v", readErr, closeErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("FONT compression changed the original font bytes")
	}
}

func TestEncodeFontResources(t *testing.T) {
	_, font := resourceTestPublication(t)
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "TrueType", data: font},
		{name: "OpenType payload", data: append([]byte("OTTO"), bytes.Repeat([]byte("font data"), 100)...)},
		{name: "tiny payload still compressed", data: []byte{0, 1, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := bytes.Clone(tt.data)
			candidate := resourceCandidate{Kind: resourceFont, Data: tt.data}
			record, err := encodeResource(candidate)
			if err != nil {
				t.Fatal(err)
			}
			assertCompressedFontRecord(t, record, tt.data)
			if !bytes.Equal(tt.data, before) {
				t.Fatal("FONT encoding modified the input buffer")
			}
			again, err := encodeResource(candidate)
			if err != nil || !bytes.Equal(record, again) {
				t.Fatal("FONT compression is not deterministic")
			}
		})
	}
}

func TestCompileResourcesRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name     string
		resource fb2.StylesheetResource
		contains string
	}{
		{"WOFF2 font", fb2.StylesheetResource{Filename: "fonts/a.woff2", MimeType: "font/woff2", Data: []byte("wOF2")}, "use TTF or OTF"},
		{"invalid TTF", fb2.StylesheetResource{Filename: "fonts/a.ttf", MimeType: "font/ttf", Data: []byte("bad")}, "TTF or OTF"},
		{"invalid SVG encoding", fb2.StylesheetResource{Filename: "other/a.svg", MimeType: "image/svg+xml", Data: []byte{0xff}}, "UTF-8"},
		{"unsupported raster", fb2.StylesheetResource{Filename: "other/a.webp", MimeType: "image/webp", Data: []byte("RIFF")}, "unsupported KF8 raster"},
		{
			"unsafe filename",
			fb2.StylesheetResource{Filename: "../a.png", MimeType: "image/png", Data: []byte("png")},
			"invalid publication resource filename",
		},
		{"empty", fb2.StylesheetResource{Filename: "other/a.png", MimeType: "image/png"}, "empty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub := &publication.Publication{Stylesheet: publication.Stylesheet{Resources: []fb2.StylesheetResource{tt.resource}}}
			_, err := compileResources(context.Background(), pub)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("got %v, want error containing %q", err, tt.contains)
			}
		})
	}
}

func TestLowerStylesheetResourcesPreservesCascade(t *testing.T) {
	input := []byte(`/* source order */ #id > p + p {
 margin-left: 2em; margin: 0; color: red; color: blue !important;
 background: url("art.png"); content: "url(not-a-resource.png)";
}`)
	resources := &resourceLayout{References: map[string]resourceReference{
		"art.png": {Number: 1, Kind: resourceImage, MimeType: "image/png"},
	}}
	output, err := lowerStylesheetResources(input, resources)
	want := bytes.ReplaceAll(input, []byte(`url("art.png")`), []byte(`url("kindle:embed:0001?mime=image/png")`))
	if err != nil || !bytes.Equal(output, want) {
		t.Fatalf("native resource lowering changed CSS beyond its URL: %s, %v", output, err)
	}
}

func TestClassifySFNTResources(t *testing.T) {
	for _, tt := range []struct {
		name     string
		mimeType string
		data     []byte
		wantMIME string
	}{
		{"TrueType", "font/ttf", []byte{0, 1, 0, 0}, "font/ttf"},
		{"OpenType CFF", "font/otf", []byte("OTTO"), "font/otf"},
		{"sfnt MIME", "application/font-sfnt", []byte("OTTO"), "font/otf"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			kind, mimeType, err := classifyResource(tt.mimeType, tt.data)
			if err != nil || kind != resourceFont || mimeType != tt.wantMIME {
				t.Errorf("classified as %d/%q/%v, want font/%q", kind, mimeType, err, tt.wantMIME)
			}
		})
	}
}

func TestCompileResourcesRejectsConflictsAndMissingReferences(t *testing.T) {
	pub := &publication.Publication{
		Images: fb2.BookImages{"a": {Filename: "images/a.jpg", MimeType: "image/jpeg", Data: []byte("one")}},
		Stylesheet: publication.Stylesheet{Resources: []fb2.StylesheetResource{
			{Filename: "images/a.jpg", MimeType: "image/jpeg", Data: []byte("two")},
		}},
	}
	if _, err := compileResources(context.Background(), pub); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Errorf("got %v, want conflicting filename error", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<html><body><img src="images/missing.jpg"/></body></html>`); err != nil {
		t.Fatal(err)
	}
	pub = &publication.Publication{Chapters: []publication.Chapter{{Filename: "chapter.xhtml", Doc: doc}}}
	if _, err := compileText(context.Background(), pub); err == nil || !strings.Contains(err.Error(), "unresolved publication resource") {
		t.Errorf("got %v, want unresolved image error", err)
	}

	pub = &publication.Publication{
		Chapters:   []publication.Chapter{{Filename: "chapter.xhtml", Doc: etree.NewDocument()}},
		Stylesheet: publication.Stylesheet{Data: []byte(`p { background: url("images/missing.png"); }`)},
	}
	if err := pub.Chapters[0].Doc.ReadFromString(`<html><body><p>Missing CSS image</p></body></html>`); err != nil {
		t.Fatal(err)
	}
	if _, err := compileText(context.Background(), pub); err == nil || !strings.Contains(err.Error(), "unresolved publication resource") {
		t.Errorf("got %v, want unresolved CSS resource error", err)
	}
}
