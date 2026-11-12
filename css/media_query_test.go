package css_test

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

func TestSharedMediaEvaluator(t *testing.T) {
	for _, tt := range []struct {
		query       string
		matches     [3]bool
		resolved    [3]bool
		unsupported bool
	}{
		{"amzn-kf8 and not amzn-et", [3]bool{true, false, false}, [3]bool{true, true, true}, false},
		{"fbc-pdf and amzn-et", [3]bool{false, false, true}, [3]bool{true, true, true}, false},
		{"print, screen", [3]bool{true, true, true}, [3]bool{true, true, true}, false},
		{"only screen", [3]bool{true, true, true}, [3]bool{true, true, true}, false},
		{"only amzn-et", [3]bool{false, true, true}, [3]bool{true, true, true}, false},
		{"amzn-mobi, fbc-pdf", [3]bool{false, false, true}, [3]bool{true, true, true}, false},
		{"not amzn-kf8 and amzn-et", [3]bool{true, false, false}, [3]bool{true, true, true}, false},
		{"screen and not (amzn-et)", [3]bool{true, false, false}, [3]bool{true, true, true}, false},
		{"screen and (min-width: 600px)", [3]bool{false, false, false}, [3]bool{false, false, false}, true},
		{"not (min-width: 600px)", [3]bool{true, true, true}, [3]bool{false, false, false}, true},
		{"amzn-et and (color)", [3]bool{false, false, false}, [3]bool{true, false, false}, true},
		{"all, (color)", [3]bool{true, true, true}, [3]bool{true, true, true}, true},
		{"amzn-kf8 or amzn-et", [3]bool{false, false, false}, [3]bool{false, false, false}, true},
		{"screen and", [3]bool{false, false, false}, [3]bool{false, false, false}, true},
		{"(color", [3]bool{false, false, false}, [3]bool{false, false, false}, true},
		{"", [3]bool{false, false, false}, [3]bool{false, false, false}, true},
	} {
		for i, profile := range []struct {
			name    string
			context css.MediaContext
		}{
			{"KF8", css.MediaContext{KF8: true}},
			{"KFX", css.MediaContext{KF8: true, ET: true}},
			{"PDF", css.MediaContext{KF8: true, ET: true, FBCPDF: true}},
		} {
			t.Run(profile.name+"/"+tt.query, func(t *testing.T) {
				query := css.MediaQuery{Raw: tt.query}
				got := query.Evaluate(profile.context)
				if got.Matches != tt.matches[i] || got.Resolved != tt.resolved[i] || (len(got.Unsupported) > 0) != tt.unsupported {
					t.Fatalf("media result = %+v", got)
				}
				// AST parsing and direct/raw queries use exactly the same evaluator.
				if tt.query != "" && tt.query != "(color" {
					sheet := css.NewParser(nil).Parse([]byte("@media "+tt.query+" {p {color: red;}}"), "")
					if len(sheet.Items) != 1 || sheet.Items[0].MediaBlock == nil {
						t.Fatal("query was lost by stylesheet parsing")
					}
					parsed := sheet.Items[0].MediaBlock.Query.Evaluate(profile.context)
					if !reflect.DeepEqual(parsed, got) {
						t.Fatalf("parsed evaluator differs from direct query: %+v / %+v", parsed, got)
					}
				}
			})
		}
	}
}

func TestDefaultStylesheetMediaParity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "convert", "default.css"))
	if err != nil {
		t.Fatal(err)
	}
	queries := []string{
		"fbc-pdf", "amzn-mobi", "amzn-kf8 and not amzn-et", "amzn-kf8 and not amzn-et", "fbc-pdf", "not amzn-et", "amzn-et",
	}
	// Profile decisions, including the KF8-only drop-cap block, are pinned independently of the evaluator.
	for _, profile := range []struct {
		name     string
		context  css.MediaContext
		original [7]bool
	}{
		{"KF8", css.MediaContext{KF8: true}, [7]bool{false, false, true, true, false, true, false}},
		{"KFX", css.MediaContext{KF8: true, ET: true}, [7]bool{false, false, false, false, false, false, true}},
		{"PDF", css.MediaContext{KF8: true, ET: true, FBCPDF: true}, [7]bool{true, false, false, false, true, false, true}},
	} {
		t.Run(profile.name, func(t *testing.T) {
			sheet := css.NewParser(nil).Parse(data, "")
			var expected []css.Rule
			index := 0
			for _, item := range sheet.Items {
				if item.Rule != nil {
					expected = append(expected, *item.Rule)
				} else if item.MediaBlock != nil {
					if index >= len(queries) || item.MediaBlock.Query.Raw != queries[index] {
						t.Fatal("bundled media queries changed; update the explicit compatibility expectations")
					}
					result := item.MediaBlock.Query.Evaluate(profile.context)
					if !result.Resolved || len(result.Unsupported) != 0 || result.Matches != profile.original[index] {
						t.Fatalf("bundled query %q changed its original decision: %+v", queries[index], result)
					}
					if profile.original[index] {
						expected = append(expected, item.MediaBlock.Rules...)
					}
					index++
				}
			}
			if index != len(queries) {
				t.Fatal("bundled media queries disappeared")
			}
			selected, err := css.ResolveMedia(data, profile.context, nil)
			if err != nil {
				t.Fatal(err)
			}
			var actual []css.Rule
			for _, item := range css.NewParser(nil).Parse(selected, "").Items {
				if item.MediaBlock != nil {
					t.Fatal("known bundled condition was left unresolved")
				}
				if item.Rule != nil {
					actual = append(actual, *item.Rule)
				}
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatal("shared evaluation changed the original selected selectors, properties, or cascade order")
			}
		})
	}
}

func TestMediaDebugDecisions(t *testing.T) {
	query := css.MediaQuery{Raw: "screen and (min-width: 600px)"}
	profile := css.MediaContext{KF8: true, ET: true, FBCPDF: true}
	for _, preserve := range []bool{false, true} {
		name := "compiled fallback"
		if preserve {
			name = "reader preservation"
		}
		t.Run(name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			result := query.Evaluate(profile)
			result.Log(query.Raw, profile, preserve, zap.New(core))
			entries := logs.FilterMessage("Unsupported CSS media condition").All()
			if len(entries) != 1 || entries[0].Level != zap.DebugLevel {
				t.Fatal("unsupported conditions must have one debug diagnostic, not a warning")
			}
			fields := entries[0].ContextMap()
			action := "skipped"
			if preserve {
				action = "preserved for reader"
			}
			if fields["query"] != query.Raw || fields["action"] != action || fields["resolved"] != false ||
				fields["kf8"] != true || fields["et"] != true || fields["fbc_pdf"] != true ||
				fields["screen"] != true || fields["print"] != false || fields["matched"] != false {
				t.Fatal("debug output lost the query, profile, or actual fallback action", fields)
			}
			if len(result.Unsupported) != 1 || !strings.Contains(result.Unsupported[0], "min-width") {
				t.Fatal("unsupported feature was not identified", result.Unsupported)
			}
		})
	}
	core, logs := observer.New(zap.DebugLevel)
	_, err := css.ResolveMedia([]byte("@media "+query.Raw+" {p {color: red;}}"), profile, zap.New(core))
	if err != nil || logs.FilterMessage("Unsupported CSS media condition").Len() != 1 {
		t.Fatal("native CSS selection did not report the shared evaluator diagnostic", err)
	}
}

func TestNestedMediaParserDiagnostic(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	css.NewParser(zap.New(core)).Parse([]byte(`@media screen {
 @media (min-width: 600px) { p { color: red; } }
}`), "nested stylesheet")
	entries := logs.FilterMessage("Unsupported nested CSS at-rule in media block").All()
	if len(entries) != 1 || entries[0].Level != zap.DebugLevel {
		t.Fatal("nested stylesheet limitations must be visible at debug level")
	}
	fields := entries[0].ContextMap()
	if fields["rule"] != "@media" || !strings.Contains(fields["query"].(string), "min-width") {
		t.Fatal("nested at-rule diagnostic lost the source query", fields)
	}
	if logs.FilterLevelExact(zap.WarnLevel).Len() != 0 {
		t.Fatal("diagnostic-only reporting introduced warnings")
	}
}

func FuzzMediaQueryEvaluation(f *testing.F) {
	for _, query := range []string{"amzn-kf8 and not amzn-et", "print, screen", "only screen", "not (min-width: 600px)", ""} {
		f.Add(query)
	}
	f.Fuzz(func(t *testing.T, query string) {
		if len(query) > 4096 {
			t.Skip()
		}
		for _, profile := range []css.MediaContext{{KF8: true}, {KF8: true, ET: true}, {KF8: true, ET: true, FBCPDF: true}} {
			mq := css.MediaQuery{Raw: query}
			result := mq.Evaluate(profile)
			if !reflect.DeepEqual(result, mq.Evaluate(profile)) {
				t.Fatal("query evaluation is not deterministic")
			}
			if !result.Resolved && len(result.Unsupported) == 0 {
				t.Fatal("unresolved query has no diagnostic explanation")
			}
		}
	})
}
