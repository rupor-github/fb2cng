package fb2

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
)

func TestNormalizeStylesheetsKF8Media(t *testing.T) {
	defaultCSS := []byte(`.section-title-h2 { page-break-before: auto; }
@media amzn-et { p { background: url("never-load-et.png"); } }`)
	before := bytes.Clone(defaultCSS)
	book := &FictionBook{
		Binaries: []BinaryObject{{ID: "kf8.ttf", ContentType: "font/ttf", Data: []byte("font bytes")}},
		Stylesheets: []Stylesheet{{Type: "text/css", Data: `
@media amzn-kf8 and not amzn-et {
 @font-face { font-family: KF8; src: url("#kf8.ttf"); }
 .section-title-h2 { page-break-before: always; }
 .body-title { page-break-before: always; }
 p { color: navy; }
}
@media amzn-et {
 @font-face { font-family: ET; src: url("#never-load-et.woff2"); }
 .section-title-h2 { page-break-before: auto; }
 .body-title { page-break-before: auto; }
 p { color: red; }
}
@media amzn-mobi { p { background: url("never-load-mobi.png"); } }
@media fbc-pdf { p { background: url("never-load-pdf.png"); } }
`}},
	}
	core, logs := observer.New(zap.DebugLevel)
	book.NormalizeStylesheets("book.fb2", defaultCSS, common.OutputFmtAzw3, zap.New(core))
	if !bytes.Equal(defaultCSS, before) {
		t.Fatal("media selection changed the shared default stylesheet bytes")
	}
	if !book.SectionNeedsBreak(2) || !book.BodyTitleNeedsBreak() {
		t.Fatal("nonmatching ET rules overrode KF8 pagination during normalization")
	}
	for _, sheet := range book.Stylesheets {
		if strings.Contains(sheet.Data, "@media") || strings.Contains(sheet.Data, "never-load") || strings.Contains(sheet.Data, "color: red") {
			t.Fatal("non-KF8 conditional content survived normalization", sheet.Data)
		}
	}
	resources := book.Stylesheets[1].Resources
	if len(resources) != 1 || resources[0].OriginalURL != "#kf8.ttf" || resources[0].ResolvedID != "kf8.ttf" {
		t.Fatalf("conditional KF8 font was not discovered after selection: %+v", resources)
	}
	if logs.FilterLevelExact(zap.WarnLevel).Len() != 0 {
		t.Fatal("inactive stylesheet resources were resolved and warned about", logs.FilterLevelExact(zap.WarnLevel).All())
	}
}

func TestNormalizeStylesheetsPreservesOtherMediaPolicies(t *testing.T) {
	input := `@media amzn-et { p { color: red; } }
@media amzn-kf8 and not amzn-et { p { color: navy; } }
@media fbc-pdf { p { color: green; } }`
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
		common.OutputFmtKfx, common.OutputFmtAzw8, common.OutputFmtPdf, common.OutputFmtTxt, common.OutputFmtMd,
	} {
		t.Run(format.String(), func(t *testing.T) {
			book := &FictionBook{Stylesheets: []Stylesheet{{Type: "text/css", Data: input}}}
			book.NormalizeStylesheets("book.fb2", nil, format, zap.NewNop())
			if book.Stylesheets[0].Data != input {
				t.Fatal("AZW3 media selection changed another output adapter's CSS policy")
			}
		})
	}
}

func TestNormalizeStylesheetsMalformedKF8Media(t *testing.T) {
	input := `@media amzn-kf8 { p { color: navy; }`
	book := &FictionBook{Stylesheets: []Stylesheet{{Type: "text/css", Data: input}}}
	core, logs := observer.New(zap.WarnLevel)
	book.NormalizeStylesheets("book.fb2", nil, common.OutputFmtAzw3, zap.New(core))
	if book.Stylesheets[0].Data != input ||
		logs.FilterMessage("Unable to select KF8 stylesheet media; retaining original CSS").Len() != 1 {
		t.Fatal("malformed CSS recovery changed the source or lost its warning")
	}
}

func TestNormalizeStylesheetsKF8ComplexSelectorResources(t *testing.T) {
	input := `#chapter > p + p[data-x] { background: url("#art.png"); }
@supports (display: grid) { .a:hover { background: url("#art.png"); } }
p::before { content: "url(#not-a-resource.png)"; }`
	book := &FictionBook{
		Binaries:    []BinaryObject{{ID: "art.png", ContentType: "image/png", Data: []byte("image bytes")}},
		Stylesheets: []Stylesheet{{Type: "text/css", Data: input}},
	}
	core, logs := observer.New(zap.DebugLevel)
	book.NormalizeStylesheets("book.fb2", nil, common.OutputFmtAzw3, zap.New(core))
	resources := book.Stylesheets[0].Resources
	if len(resources) != 1 || resources[0].OriginalURL != "#art.png" || book.Stylesheets[0].Data != input {
		t.Fatalf("native resource discovery lost complex-selector CSS or its resource: %+v", resources)
	}
	if logs.FilterLevelExact(zap.WarnLevel).Len() != 0 {
		t.Fatal("literal URL text was incorrectly treated as a resource")
	}
}
