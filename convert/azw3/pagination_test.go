package azw3

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beevik/etree"

	"fbc/convert/publication"
)

func TestCompileTextPagination(t *testing.T) {
	for _, test := range []struct {
		name, stylesheet, body string
		breaks                 int
		deferred               bool
	}{
		{"comment body title", `.body-title {page-break-before:always}`,
			`<p>Previous notes</p><div class="footnote-body"><div class="body-title" id="comments">Comments</div></div>`, 1, false},
		{"specific override", `.body-title {page-break-before:always} .footnote-body .body-title {page-break-before:auto}`,
			`<div class="body-title">Main</div><div class="footnote-body"><div class="body-title">Comments</div></div>`, 1, false},
		{"important declaration order", `.title {page-break-before:always!important;page-break-before:auto}`,
			`<div class="title">Title</div>`, 1, false},
		{"inline override", `.title {page-break-before:always}`,
			`<div class="title" style="page-break-before:auto">Title</div>`, 0, false},
		{"important beats inline", `.title {page-break-before:always!important}`,
			`<div class="title" style="page-break-before:auto">Title</div>`, 1, false},
		{"important inline override", `.title {page-break-before:always!important}`,
			`<div class="title" style="page-break-before:auto!important">Title</div>`, 0, false},
		{"child id and compound classes", `#main > div.a.b, .unused {page-break-before:always}`,
			`<div id="main"><div class="a b">Title</div><div><div class="a b">Not a child</div></div></div>`, 1, false},
		{"hidden ancestor", `.hidden {display:none} .title {page-break-before:always}`,
			`<div class="hidden"><div class="title">Hidden</div></div>`, 0, false},
		{"floating box", `.title {float:left;page-break-before:always}`,
			`<div class="title">Floated</div>`, 0, false},
		{"absolute ancestor", `.absolute {position:absolute} .title {page-break-before:always}`,
			`<div class="absolute"><div class="title">Absolute</div></div>`, 0, false},
		{"inline element", `.title {page-break-before:always}`,
			`<div><span class="title">Inline</span></div>`, 0, false},
		{"block display", `.title {page-break-before:always;display:block}`,
			`<div><span class="title">Block</span></div>`, 1, false},
		{"media profile", `@media amzn-kf8 and not amzn-et {.title{page-break-before:always}}
@media amzn-et {.title{page-break-before:auto}}`, `<div class="title">Title</div>`, 1, false},
		{"unknown media override", `.title{page-break-before:always}
@media (min-width:600px){.title{page-break-before:auto}}`, `<div class="title">Title</div>`, 0, true},
		{"unknown selector override", `.title{page-break-before:always}.title:first-child{page-break-before:auto}`,
			`<div class="title">Title</div>`, 0, true},
		{"unknown inline override", `.title{page-break-before:always}`,
			`<div class="title" style="page-break-before:var(--break)">Title</div>`, 0, true},
		{"after and adjacent before", `.one{page-break-after:always}.two{page-break-before:always}`,
			"<div class=\"one\">First</div>\n<div class=\"two\">Second</div>", 1, false},
		{"existing marker", `.title{page-break-before:always}`,
			`<div style="page-break-after:always"></div><div class="title">Title</div>`, 1, false},
		{"existing following marker", `.title{page-break-after:always}`,
			`<div class="title">Title</div><div style="page-break-after:always"></div>`, 1, false},
		{"CSS marker canceled by important rule", `.marker{page-break-after:auto!important}`,
			`<div class="marker" style="page-break-after:always"></div>`, 0, false},
		{"native marker", `.title{page-break-before:always}`,
			`<div data-AmznPageBreak="always"></div><div class="title">Title</div>`, 1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(`<html><head><title>Pagination</title></head><body>` + test.body + `</body></html>`); err != nil {
				t.Fatal(err)
			}
			before, err := doc.WriteToBytes()
			if err != nil {
				t.Fatal(err)
			}
			pub := &publication.Publication{
				Chapters:   []publication.Chapter{{Filename: "main.xhtml", Doc: doc}},
				Stylesheet: publication.Stylesheet{Data: []byte(test.stylesheet)},
			}
			layout, err := compileText(t.Context(), pub)
			if err != nil {
				t.Fatal(err)
			}
			data := layout.Documents[0].Data
			if got := bytes.Count(data, []byte(`data-AmznPageBreak="always"`)); got != test.breaks {
				t.Fatalf("got %d breaks, want %d:\n%s", got, test.breaks, data)
			}
			if bytes.Contains(data, []byte(`data-AmznPageBreak="always"/>`)) {
				t.Fatal("native break div must have an explicit HTML end tag")
			}
			deferred := false
			for _, change := range layout.Documents[0].Changes {
				deferred = deferred || strings.Contains(change.Action, "retained CSS pagination")
			}
			if deferred != test.deferred {
				t.Fatalf("unexpected pagination diagnostics: %+v", layout.Documents[0].Changes)
			}
			if after, err := doc.WriteToBytes(); err != nil || !bytes.Equal(before, after) {
				t.Fatal("pagination lowering mutated the shared publication", err)
			}
			if len(layout.Documents) != 1 {
				t.Fatal("native lowering changed converter-controlled document splitting")
			}
		})
	}
}

func TestPaginationKindleUnpack(t *testing.T) {
	for _, compression := range []string{"none", "standard", "huffdic"} {
		t.Run(compression, func(t *testing.T) {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(`<html><head><title>Pagination</title></head><body><p>Previous note</p>` +
				`<div id="comments"><div class="body-title">Comments</div><p>Comment text</p>` +
				`<div class="section-title-h2">Nested section</div><p>Following text</p></div></body></html>`); err != nil {
				t.Fatal(err)
			}
			pub := &publication.Publication{
				Chapters: []publication.Chapter{{Filename: "main.xhtml", Doc: doc}},
				Stylesheet: publication.Stylesheet{Data: []byte(
					`.body-title,.section-title-h2{page-break-before:always}`)},
			}
			compiled, err := compileText(t.Context(), pub)
			if err != nil {
				t.Fatal(err)
			}
			data := assembleTextProbe(t, compiled, nil, nil, compression)
			work := unpackProbe(t, "pagination-"+compression, data, nil)
			paths, err := filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "*.rawml"))
			if err != nil || len(paths) != 1 {
				t.Fatal("expected one independent raw stream", paths, err)
			}
			raw, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) < int(compiled.MainLength) || !bytes.Equal(raw[:compiled.MainLength], compiled.Raw[:compiled.MainLength]) ||
				bytes.Count(raw[:compiled.MainLength], []byte(`data-AmznPageBreak="always"`)) != 2 {
				t.Fatal("native break attributes changed during independent decompression")
			}
			paths, err = filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "OEBPS", "Text", "*.xhtml"))
			if err != nil || len(paths) != 1 {
				t.Fatal("expected one reconstructed document", paths, err)
			}
			xhtml, err := os.ReadFile(paths[0])
			if err != nil || bytes.Count(xhtml, []byte(`style="page-break-after:always"`)) != 2 {
				t.Fatal("decoder did not reconstruct both native breaks", err)
			}
		})
	}
}

func TestPaginationNativeMarkerPlacement(t *testing.T) {
	for _, test := range []struct{ name, stylesheet, source, want string }{
		{"comments", `.body-title{page-break-before:always}`,
			`<body><p>Previous note</p><div id="comments"><div class="body-title">Comments</div></div></body>`,
			`<body><p>Previous note</p><div id="comments"><div data-AmznPageBreak="always"></div>` +
				`<div class="body-title">Comments</div></div></body>`},
		{"nested sections", `.section-title-h2{page-break-before:always}`,
			`<body><div><p>Previous section</p><div class="section-title-h2">Nested section</div></div></body>`,
			`<body><div><p>Previous section</p><div data-AmznPageBreak="always"></div>` +
				`<div class="section-title-h2">Nested section</div></div></body>`},
		{"after", `p{page-break-after:always}`, `<body><p>End</p><div>Next</div></body>`,
			`<body><p>End</p><div data-AmznPageBreak="always"></div><div>Next</div></body>`},
		{"CSS marker keeps anchor", ``, `<body><div id="anchor" style="page-break-after:always"></div></body>`,
			`<body><div id="anchor" data-AmznPageBreak="always"/></body>`},
		{"coalesced CSS marker keeps anchor", ``,
			`<body><div data-AmznPageBreak="always"></div><div id="anchor" style="page-break-after:always"></div></body>`,
			`<body><div data-AmznPageBreak="always"/><div id="anchor"/></body>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(test.source); err != nil {
				t.Fatal(err)
			}
			pending := pendingDocument{Filename: "main.xhtml", Doc: doc}
			if err := compilePagination([]byte(test.stylesheet)).apply(t.Context(), &pending); err != nil {
				t.Fatal(err)
			}
			if got, err := doc.WriteToString(); err != nil || got != test.want {
				t.Fatalf("native break placement mismatch:\ngot: %s\nwant: %s\nerror: %v", got, test.want, err)
			}
		})
	}
}

func FuzzPaginationLowering(f *testing.F) {
	for _, stylesheet := range []string{
		`.title {page-break-before:always}`, `.title {page-break-before:always!important;page-break-before:auto}`,
		`@media (min-width:600px){.title{page-break-before:always}}`, `.hide{display:none}`,
		`#main > div.title {page-break-after:always}`, `@import"`, "",
	} {
		f.Add(stylesheet)
	}
	f.Fuzz(func(t *testing.T, stylesheet string) {
		if len(stylesheet) > 4096 {
			t.Skip()
		}
		const source = `<html><head><title>Pagination</title></head><body><div id="main">` +
			`<div class="title">Heading</div><div class="hide"><p>Hidden candidate</p></div><p>Following text</p></div></body></html>`
		var previous []byte
		for range 2 {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(source); err != nil {
				t.Fatal(err)
			}
			before := elementText(doc.Root())
			pending := pendingDocument{Filename: "main.xhtml", Doc: doc}
			if err := compilePagination([]byte(stylesheet)).apply(t.Context(), &pending); err != nil {
				t.Fatal(err)
			}
			data, err := doc.WriteToBytes()
			if err != nil {
				t.Fatal(err)
			}
			if elementText(doc.Root()) != before {
				t.Fatal("pagination changed readable text")
			}
			if previous != nil && !bytes.Equal(data, previous) {
				t.Fatal("pagination lowering is not deterministic")
			}
			previous = data
		}
	})
}
