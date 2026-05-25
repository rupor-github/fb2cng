package pdf

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"fbc/fb2"
)

func TestPDFParsedStylesheetsSharedForStylesAndFonts(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	log := zap.New(core)
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `
		@font-face { font-family: "Demo"; src: url("demo.ttf"); }
		p { color: #112233; }
	`}}}

	parsed := parsePDFStylesheets(book, log)
	_ = newPDFStyleResolverFromParsed(parsed, log)
	_ = newPDFFontRegistryFromParsed(parsed, log)

	entries := logs.FilterMessage("Parsing CSS").All()
	if len(entries) != 1 {
		t.Fatalf("Parsing CSS log entries = %d, want 1", len(entries))
	}
}

func TestPDFParsedStylesheetsPreserveFontResourceScope(t *testing.T) {
	fontData, err := gunzipFont(notoMonoRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{
		{
			Type:      "text/css",
			Data:      `@font-face { font-family: ScopedOne; src: url("#shared"); }`,
			Resources: []fb2.StylesheetResource{{OriginalURL: "#shared", MimeType: "font/ttf", Data: fontData}},
		},
		{
			Type: "text/css",
			Data: `@font-face { font-family: ScopedTwo; src: url("#shared"); }`,
		},
	}}

	parsed := parsePDFStylesheets(book, nil)
	registry := newPDFFontRegistryFromParsed(parsed, nil)

	if _, ok := registry.families["scopedone"]; !ok {
		t.Fatal("font family scopedone was not loaded from its stylesheet resource")
	}
	if _, ok := registry.families["scopedtwo"]; ok {
		t.Fatal("font family scopedtwo loaded resource from a different stylesheet")
	}
}
