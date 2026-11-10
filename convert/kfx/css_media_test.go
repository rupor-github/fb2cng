package kfx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/css"
)

func TestKFXDefaultMediaStyleParity(t *testing.T) {
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
		case "amzn-et": // Original KFX profile: only this bundled media block matches.
			for i := range item.MediaBlock.Rules {
				original = append(original, css.StylesheetItem{Rule: &item.MediaBlock.Rules[i]})
			}
		case "fbc-pdf", "amzn-mobi", "amzn-kf8 and not amzn-et", "not amzn-et":
		default:
			t.Fatal("bundled query changed; update the explicit KFX compatibility baseline", item.MediaBlock.Query.Raw)
		}
	}
	baseline.Items = original
	want, wantWarnings := NewStyleRegistryFromCSS(baseline, nil, zap.NewNop())
	got, gotWarnings := NewStyleRegistryFromCSS(css.NewParser(nil).Parse(data, ""), nil, zap.NewNop())
	if !reflect.DeepEqual(got.styles, want.styles) || !reflect.DeepEqual(got.pseudoContent, want.pseudoContent) ||
		!reflect.DeepEqual(gotWarnings, wantWarnings) {
		t.Fatal("shared media evaluation changed original KFX styles or pseudo content")
	}
}

func TestKFXSharedMediaDiagnostics(t *testing.T) {
	input := []byte(`
@media print, screen { .alternative { text-align: right; } }
@media only screen { .qualified { text-align: center; } }
@media screen and (min-width: 600px) { .unsupported { font-weight: bold; } }
@media not (min-width: 600px) { .negated { font-weight: bold; } }
`)
	core, logs := observer.New(zap.DebugLevel)
	tracer := NewStyleTracer(t.TempDir())
	registry, _ := NewStyleRegistryFromCSS(css.NewParser(nil).Parse(input, ""), tracer, zap.New(core))
	for name, alignment := range map[string]KFXSymbol{"alternative": SymRight, "qualified": SymCenter} {
		style, found := registry.Get(name)
		if !found || !isSymbol(style.Properties[SymTextAlignment], alignment) {
			t.Errorf("shared evaluator did not apply media style %q", name)
		}
	}
	if _, found := registry.Get("unsupported"); found {
		t.Fatal("unsupported device condition changed its false fallback")
	}
	if _, found := registry.Get("negated"); !found {
		t.Fatal("negated unsupported condition lost its existing fallback behavior")
	}
	unsupported := logs.FilterMessage("Unsupported CSS media condition").All()
	if len(unsupported) != 2 || logs.FilterLevelExact(zap.WarnLevel).Len() != 0 {
		t.Fatal("unsupported media conditions must be reported once at debug level")
	}
	for _, entry := range unsupported {
		fields := entry.ContextMap()
		if fields["kf8"] != true || fields["et"] != true || fields["fbc_pdf"] != false || fields["resolved"] != false {
			t.Fatal("KFX diagnostic lost its existing profile or unresolved state", fields)
		}
	}
	traced := 0
	for _, entry := range tracer.entries {
		if entry.operation == "MEDIA" && strings.Contains(entry.details, "unsupported atoms treated as false") {
			traced++
		}
	}
	if traced != 2 {
		t.Fatal("unsupported media decisions were not retained in the KFX debug trace")
	}
}
