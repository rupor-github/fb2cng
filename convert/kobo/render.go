// Package kobo supplies reader-specific decoration for shared publication rendering.
package kobo

import (
	"fmt"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/content/text"
)

// Renderer owns one build's sentence tokenizer and document-local span numbering.
// Its zero value wraps each text chunk without sentence tokenization.
type Renderer struct {
	splitter   *text.Splitter
	paragraphs int
	sentences  int
}

func NewRenderer(lang language.Tag, log *zap.Logger) *Renderer {
	return &Renderer{splitter: text.NewSplitter(lang, log)}
}

func (r *Renderer) DocumentRoot(body *etree.Element) *etree.Element {
	r.paragraphs, r.sentences = 0, 0
	columns := body.CreateElement("div")
	columns.CreateAttr("id", "book-columns")
	inner := columns.CreateElement("div")
	inner.CreateAttr("id", "book-inner")
	return inner
}

func (r *Renderer) NextParagraph() {
	r.paragraphs++
	r.sentences = 0
}

func (r *Renderer) SavePosition() func() {
	paragraphs, sentences := r.paragraphs, r.sentences
	return func() { r.paragraphs, r.sentences = paragraphs, sentences }
}

func (r *Renderer) InlineParent(parent *etree.Element) *etree.Element {
	r.sentences++
	span := parent.CreateElement("span")
	span.CreateAttr("class", "koboSpan")
	span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", r.paragraphs, r.sentences))
	return span
}

func (r *Renderer) AppendText(parent *etree.Element, chunk string) {
	for sentence := range r.splitter.Sentences(chunk) {
		r.InlineParent(parent).SetText(sentence)
	}
}

func (*Renderer) IsWrapper(element *etree.Element) bool {
	return element.Tag == "span" && element.SelectAttrValue("class", "") == "koboSpan"
}
