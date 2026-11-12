package kobo

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"golang.org/x/text/language"
)

func TestRendererPosition(t *testing.T) {
	for _, tt := range []struct {
		name string
		step func(*Renderer)
		want string
	}{
		{name: "next sentence", step: func(*Renderer) {}, want: "kobo.5.11"},
		{name: "next paragraph", step: func(r *Renderer) { r.NextParagraph() }, want: "kobo.6.1"},
		{name: "restore position", step: func(r *Renderer) {
			restore := r.SavePosition()
			r.NextParagraph()
			r.InlineParent(etree.NewElement("p"))
			restore()
		}, want: "kobo.5.11"},
		{name: "new document", step: func(r *Renderer) { r.DocumentRoot(etree.NewElement("body")) }, want: "kobo.0.1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &Renderer{paragraphs: 5, sentences: 10}
			tt.step(r)
			span := r.InlineParent(etree.NewElement("p"))
			if got := span.SelectAttrValue("id", ""); got != tt.want || !r.IsWrapper(span) {
				t.Fatalf("span = %v, want wrapper with ID %s", span, tt.want)
			}
		})
	}
}

func TestRendererSentences(t *testing.T) {
	for _, tt := range []struct {
		name  string
		fresh func() *Renderer
		want  int
	}{
		{name: "tokenized", fresh: func() *Renderer { return NewRenderer(language.English, zap.NewNop()) }, want: 2},
		{name: "zero value", fresh: func() *Renderer { return &Renderer{} }, want: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.fresh()
			body := etree.NewElement("body")
			root := r.DocumentRoot(body)
			if root.SelectAttrValue("id", "") != "book-inner" || root.Parent().SelectAttrValue("id", "") != "book-columns" {
				t.Fatal("missing reader document wrappers")
			}
			r.NextParagraph()
			const input = "Sentence one. Sentence two."
			r.AppendText(root, input)
			if len(root.ChildElements()) != tt.want {
				t.Fatalf("sentence spans = %d, want %d", len(root.ChildElements()), tt.want)
			}
			var text strings.Builder
			for _, span := range root.ChildElements() {
				text.WriteString(span.Text())
			}
			if text.String() != input {
				t.Fatalf("sentence wrapping changed text: %q", text.String())
			}
			other := tt.fresh()
			if span := other.InlineParent(etree.NewElement("p")); span.SelectAttrValue("id", "") != "kobo.0.1" {
				t.Fatal("renderer state leaked between builds")
			}
			if r.IsWrapper(etree.NewElement("span")) {
				t.Fatal("authored span mistaken for reader decoration")
			}
		})
	}
}
