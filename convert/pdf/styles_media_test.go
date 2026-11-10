package pdf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/css"
	"fbc/fb2"
)

func TestPDFDefaultMediaStyleParity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "default.css"))
	if err != nil {
		t.Fatal(err)
	}
	baseline := css.NewParser(nil).Parse(data, "")
	var original []css.StylesheetItem
	for _, item := range baseline.Items {
		if item.MediaBlock == nil {
			original = append(original, item)
			continue
		}
		switch item.MediaBlock.Query.Raw {
		case "fbc-pdf", "amzn-et": // Original PDF profile retains ET compatibility.
			for i := range item.MediaBlock.Rules {
				original = append(original, css.StylesheetItem{Rule: &item.MediaBlock.Rules[i]})
			}
		case "amzn-mobi", "amzn-kf8 and not amzn-et", "not amzn-et":
		default:
			t.Fatal("bundled query changed; update the explicit PDF compatibility baseline", item.MediaBlock.Query.Raw)
		}
	}
	baseline.Items = original
	want := newPDFStyleResolverFromParsed([]pdfParsedStylesheet{{parsed: baseline}}, nil, zap.NewNop())
	got := newPDFStyleResolverFromParsed([]pdfParsedStylesheet{{parsed: css.NewParser(nil).Parse(data, "")}}, nil, zap.NewNop())
	if !reflect.DeepEqual(got.styles, want.styles) || !reflect.DeepEqual(got.dropcaps, want.dropcaps) ||
		!reflect.DeepEqual(got.pseudoContent, want.pseudoContent) {
		t.Fatal("shared media evaluation changed original PDF styles, dropcaps or pseudo content")
	}
}

func TestPDFSharedMediaDiagnostics(t *testing.T) {
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `
p { text-align: left; }
@media only screen { p { text-align: right; } }
@media print { p { text-align: center; } }
@media screen and (min-width: 600px) { p { text-align: center; } }
@media print, screen { h1 { text-align: left; } }
@media not (min-width: 600px) { code { text-align: right; } }
`}}}
	core, logs := observer.New(zap.DebugLevel)
	tracer := newPDFStyleTracer(t.TempDir())
	resolver := newPDFStyleResolver(book, tracer, zap.New(core))
	paragraph := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph})
	if paragraph.Paragraph.Align != textAlignRight {
		t.Fatal("PDF no longer uses screen styling, or an unsupported condition changed its false fallback")
	}
	heading := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockHeading, Depth: 1})
	if heading.Paragraph.Align != textAlignLeft {
		t.Fatal("PDF did not use the shared comma-alternative evaluator")
	}
	if resolver.styles[pdfStyleCode].Paragraph.Align != textAlignRight {
		t.Fatal("negated unsupported condition lost its existing fallback behavior")
	}
	unsupported := logs.FilterMessage("Unsupported CSS media condition").All()
	if len(unsupported) != 2 || logs.FilterLevelExact(zap.WarnLevel).Len() != 0 {
		t.Fatal("unsupported media conditions must be reported once at debug level")
	}
	for _, entry := range unsupported {
		fields := entry.ContextMap()
		if fields["kf8"] != true || fields["et"] != true || fields["fbc_pdf"] != true ||
			fields["screen"] != true || fields["print"] != false || fields["resolved"] != false {
			t.Fatal("PDF diagnostic lost its existing profile or unresolved state", fields)
		}
	}
	traced := 0
	for _, entry := range tracer.entries {
		if entry.Operation == "MEDIA" && strings.Contains(entry.Details, "unsupported atoms treated as false") {
			traced++
		}
	}
	if traced != 2 {
		t.Fatal("unsupported media decisions were not retained in the PDF debug trace")
	}
}
