package azw3

import (
	"reflect"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func TestCompileFootnoteMoreVisibility(t *testing.T) {
	for _, tt := range []struct {
		name, css string
		visible   bool
	}{
		{name: "visible by default", visible: true},
		{name: "hidden", css: ".footnote-more { display: none }"},
		{name: "KF8 media", css: "@media amzn-kf8 and not amzn-et { .footnote-more { display: none } }"},
		{name: "ET media excluded", css: "@media amzn-et { .footnote-more { display: none } }", visible: true},
		{name: "override", css: ".footnote-more { display: none } .footnote-more { display: inline }", visible: true},
		{name: "specificity", css: "aside .footnote-more { display: none } .footnote-more { display: inline }"},
		{name: "important", css: ".footnote-more { display: none !important } aside .footnote-more { display: inline }"},
		{name: "hidden ancestor", css: "aside { display: none }", visible: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub := textTestPublication(t)
			pub.Stylesheet.Data = []byte(tt.css)
			p := pub.Chapters[2].Doc.FindElement("//aside/p")
			p.CreateText(" first\n")
			marker := p.CreateElement("span")
			marker.CreateAttr("class", "footnote-more")
			marker.CreateAttr("data-fbc-footnote-more", "")
			marker.SetText("MORE<&>🙂 ")
			p.CreateText("second")
			before := snapshotPublication(t, pub)
			compiled, err := compileText(t.Context(), pub)
			if err != nil {
				t.Fatal(err)
			}
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(compiled.Documents[2].Data); err != nil {
				t.Fatal(err)
			}
			if len(doc.FindElements("//span")) != 0 || strings.Contains(string(compiled.Documents[2].Data), "data-fbc-footnote-more") {
				t.Fatal("temporary marker survived native compilation")
			}
			if strings.Contains(doc.FindElement("//aside/p/b").Tail(), "MORE<&>🙂 ") != tt.visible {
				t.Fatal("indicator text did not follow CSS visibility")
			}
			if !reflect.DeepEqual(before, snapshotPublication(t, pub)) {
				t.Fatal("indicator lowering changed the source publication")
			}
		})
	}
}
