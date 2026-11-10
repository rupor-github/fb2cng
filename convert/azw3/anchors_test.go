package azw3

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func TestCompileTextNativeAnchors(t *testing.T) {
	pub := textTestPublication(t)
	before := snapshotPublication(t, pub)
	layout, err := compileText(t.Context(), pub)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for i, document := range layout.Documents {
		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(document.Data); err != nil {
			t.Fatal(err)
		}
		body := doc.FindElement("//body")
		if got := body.SelectAttrValue("aid", ""); got != base32(uint32(i), 4) {
			t.Fatalf("fragment selector's body identity changed: %q", got)
		}
		for _, element := range body.FindElements(".//*") {
			if aid := element.SelectAttrValue("aid", ""); aid == "" {
				t.Errorf("reader-facing identity missing on <%s> in %s", element.Tag, document.Filename)
			}
		}
		for _, element := range doc.FindElements("//*[@aid]") {
			aid := element.SelectAttrValue("aid", "")
			if seen[aid] {
				t.Fatalf("duplicate native element identity %q", aid)
			}
			seen[aid] = true
		}
	}
	if !reflect.DeepEqual(before, snapshotPublication(t, pub)) {
		t.Fatal("native anchor allocation changed the shared publication")
	}
	again, err := compileText(t.Context(), pub)
	if err != nil || !bytes.Equal(layout.Raw, again.Raw) {
		t.Fatal("native element identities are not deterministic", err)
	}
}

func TestNativeAnchorApplicability(t *testing.T) {
	for _, test := range []struct{ name, source, anchored, excluded string }{
		{"notes", `<a id="ref" href="#note" epub:type="noteref">1</a><aside id="note" epub:type="footnote">` +
			`<p>Note</p><a href="#ref" epub:type="backlink">Back</a></aside>`, ".//a|aside|aside/p", ""},
		{"compiler-owned identities", `<p aid="duplicate"><a aid="duplicate">Link</a></p>`, "p|p/a", ""},
		{"table", `<p>Outside</p><table><tr><td><p>Cell</p><a id="cell" href="#cell">Link</a></td></tr></table>`,
			"p[1]", "table|table//*"},
		{"foreign markup", `<p>Outside</p><svg xmlns="http://www.w3.org/2000/svg"><a id="foreign">SVG link</a></svg>`,
			"p", "svg|svg//*"},
		{"native break", `<div data-AmznPageBreak="always"></div><div>Title</div>`, "div[2]", "div[1]"},
		{"void element", `<p>Text<img src="external.png"/><br/></p>`, "p", "p/img|p/br"},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(`<body xmlns:epub="http://www.idpf.org/2007/ops" aid="duplicate">` +
				test.source + `</body>`); err != nil {
				t.Fatal(err)
			}
			pending := pendingDocument{Filename: "main.xhtml", Doc: doc}
			next := uint32(1)
			if err := assignTextAnchors(t.Context(), &pending, doc.Root(), 0, &next); err != nil {
				t.Fatal(err)
			}
			for selector := range strings.SplitSeq(test.anchored, "|") {
				elements := doc.Root().FindElements(selector)
				if len(elements) == 0 {
					t.Fatal("missing anchor test fixture", selector)
				}
				for _, element := range elements {
					if element.SelectAttrValue("aid", "") == "" {
						t.Errorf("missing native identity on %s", selector)
					}
				}
			}
			if test.excluded != "" {
				for selector := range strings.SplitSeq(test.excluded, "|") {
					elements := doc.Root().FindElements(selector)
					if len(elements) == 0 {
						t.Fatal("missing exclusion test fixture", selector)
					}
					for _, element := range elements {
						if element.SelectAttr("aid") != nil {
							t.Errorf("native identity added in excluded context %s", selector)
						}
					}
				}
			}
			seen := make(map[string]bool)
			for _, element := range doc.FindElements("//*[@aid]") {
				aid := element.SelectAttrValue("aid", "")
				if aid == "duplicate" || seen[aid] {
					t.Fatal("compiler-owned native identity was not replaced uniquely", aid)
				}
				seen[aid] = true
			}
		})
	}
}

func TestNativeAnchorsCanceledAndOverflow(t *testing.T) {
	for _, test := range []struct {
		name string
		next uint32
	}{
		{name: "canceled", next: 1},
		{name: "overflow", next: math.MaxUint32},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(`<body><p>Text</p></body>`); err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			if test.name == "canceled" {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			pending := pendingDocument{Filename: "main.xhtml", Doc: doc}
			err := assignTextAnchors(ctx, &pending, doc.Root(), 0, &test.next)
			if err == nil || (test.name == "canceled" && !errors.Is(err, context.Canceled)) {
				t.Fatal("expected bounded native anchor allocation failure", err)
			}
		})
	}
}
