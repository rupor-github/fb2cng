package css_test

import (
	"bytes"
	"strings"
	"testing"

	"fbc/css"
)

func TestResolveKF8Media(t *testing.T) {
	for _, tt := range []struct {
		query string
		want  bool
	}{
		{"amzn-kf8", true}, {"amzn-et", false}, {"amzn-mobi", false}, {"fbc-pdf", false},
		{"amzn-kf8 and not amzn-et", true}, {"amzn-kf8 and amzn-et", false},
		{"not amzn-et", true}, {"not amzn-kf8", false}, {"not amzn-mobi", true}, {"not fbc-pdf", true},
		{"all", true}, {"screen", true}, {"print", false}, {"not print", true}, {"only amzn-kf8", true},
		{"AMZN-KF8 AND NOT AMZN-ET", true},
		{"amzn-kf8 and (amzn-et)", false}, {"amzn-kf8 and not (amzn-et)", true},
		{"amzn-et, amzn-kf8", true}, {"amzn-mobi, fbc-pdf", false},
		{"not amzn-kf8 and amzn-et", true}, // Leading not negates the full query.
	} {
		t.Run(tt.query, func(t *testing.T) {
			input := []byte("p {color: red;} @media " + tt.query + " {p {color: blue;}} p {color: green;}")
			before := bytes.Clone(input)
			got, err := css.ResolveMedia(input, css.MediaContext{KF8: true})
			if err != nil || strings.Contains(string(got), "@media") || bytes.Contains(got, []byte("blue")) != tt.want {
				t.Fatalf("selected CSS = %s, error = %v", got, err)
			}
			if !bytes.Equal(input, before) || bytes.Index(got, []byte("red")) > bytes.Index(got, []byte("green")) {
				t.Fatal("media selection changed its source or reversed the cascade")
			}
		})
	}
}

func TestResolveMediaPreservesContents(t *testing.T) {
	input := `/* @media amzn-et { ignored } */
p + p[data-text="{ @media amzn-et }"] { --raw: "}"; margin: 0; margin: 1em; }
@media amzn-kf8 and not amzn-et {
 @font-face { font-family: "KF8"; src: url("fonts/kf8.ttf"); font-display: swap; }
 p::before { content: "literal @media amzn-et { } café"; }
 @media amzn-et { .wrong { background: url("wrong.png"); } }
 @media not amzn-et { .right { color: blue; } }
}
@supports (display: grid) {
 @MEDIA AMZN-KF8 { .grid { display: grid; } }
 @media amzn-et { .wrong-grid { display: none; } }
}
@supports (--probe: {value}) { @media amzn-kf8 { .custom-probe { color: navy; } } }
@media amzn-et { @media amzn-kf8 { .wrong-nested { color: green; } } }
@media screen and (min-width: 600px) { .wide { color: purple; } }
`
	got, err := css.ResolveMedia([]byte(input), css.MediaContext{KF8: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, preserved := range []string{
		`p + p[data-text="{ @media amzn-et }"] { --raw: "}"; margin: 0; margin: 1em; }`,
		`@font-face { font-family: "KF8"; src: url("fonts/kf8.ttf"); font-display: swap; }`,
		`content: "literal @media amzn-et { } café"`, `.right { color: blue; }`,
		`@supports (display: grid)`, `.grid { display: grid; }`,
		`@supports (--probe: {value})`, `.custom-probe { color: navy; }`,
		`@media screen and (min-width: 600px)`, `.wide { color: purple; }`,
	} {
		if !bytes.Contains(got, []byte(preserved)) {
			t.Errorf("CSS contents were lost: %s\n%s", preserved, got)
		}
	}
	if bytes.Contains(got, []byte(".wrong")) {
		t.Fatalf("nonmatching nested content survived: %s", got)
	}
	again, err := css.ResolveMedia(got, css.MediaContext{KF8: true})
	if err != nil || !bytes.Equal(again, got) {
		t.Fatal("media selection is not idempotent", err)
	}
}

func TestResolveMediaReaderConditions(t *testing.T) {
	for _, query := range []string{
		"screen and (min-width: 600px)", "(orientation: landscape)", "amzn-kf8 and (min-width: 600px)",
		"screen and (color), amzn-et", "amzn-kf8 or amzn-et", "unknown-device", "not unknown-device",
		"amzn-kf8 and (not-amazon-feature)",
	} {
		t.Run(query, func(t *testing.T) {
			input := []byte("@media " + query + " {p {color: red;}}")
			got, err := css.ResolveMedia(input, css.MediaContext{KF8: true})
			if err != nil || !bytes.Equal(got, input) {
				t.Fatalf("reader-dependent condition changed: %s, %v", got, err)
			}
		})
	}
	for _, query := range []string{"amzn-et and (min-width: 600px)", "fbc-pdf and (color)"} {
		t.Run(query, func(t *testing.T) {
			got, err := css.ResolveMedia([]byte("@media "+query+" {p {color: red;}}"), css.MediaContext{KF8: true})
			if err != nil || len(got) != 0 {
				t.Fatal("known false output condition must exclude reader-dependent content", string(got), err)
			}
		})
	}
}

func TestResolveMediaContextIsExplicit(t *testing.T) {
	input := []byte(`@media amzn-et { .enhanced { color: red; } } @media fbc-pdf { .pdf { color: blue; } }`)
	for _, tt := range []struct {
		name    string
		context css.MediaContext
		et, pdf bool
	}{
		{"KF8", css.MediaContext{KF8: true}, false, false},
		{"KFX", css.MediaContext{KF8: true, ET: true}, true, false},
		{"PDF", css.MediaContext{KF8: true, ET: true, FBCPDF: true}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := css.ResolveMedia(input, tt.context)
			if err != nil || bytes.Contains(got, []byte(".enhanced")) != tt.et || bytes.Contains(got, []byte(".pdf")) != tt.pdf {
				t.Fatal("media context was ignored", string(got), err)
			}
		})
	}
}

func TestResolveMediaRejectsMalformedBlocks(t *testing.T) {
	for _, input := range []string{
		"@media amzn-kf8 { p {color: red;}", "p {color: red;}}", strings.Repeat("@media screen {", 129),
	} {
		t.Run(input[:min(len(input), 30)], func(t *testing.T) {
			if got, err := css.ResolveMedia([]byte(input), css.MediaContext{KF8: true}); err == nil || got != nil {
				t.Fatal("malformed CSS selection returned partial output")
			}
		})
	}
}

func FuzzResolveKF8Media(f *testing.F) {
	for _, input := range []string{
		`@media amzn-kf8 and not amzn-et {p {margin: 1em;}}`, `@media amzn-et {p {content: "}";}}`,
		`@font-face {src: url("font.ttf");}`, `@media screen and (min-width: 600px) {p {color: red;}}`,
	} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 16*1024 {
			t.Skip()
		}
		got, err := css.ResolveMedia([]byte(input), css.MediaContext{KF8: true})
		if err != nil {
			if got != nil {
				t.Fatal("failed media selection returned partial output")
			}
			return
		}
		again, err := css.ResolveMedia(got, css.MediaContext{KF8: true})
		if err != nil || !bytes.Equal(again, got) {
			t.Fatal("media selection is not idempotent", err)
		}
	})
}
