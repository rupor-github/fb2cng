package css_test

import (
	"bytes"
	"reflect"
	"testing"

	"fbc/css"
)

func TestRewriteStylesheetURLs(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		urls              []string
	}{
		{"literal preservation", `/* url(no.png) */ #id > p[data-x="url(no.png)"] {
 content: "url(no.png)"; margin-left: 2em; margin: 0; color: red; color: blue !important;
 background: url(image.png); }`, `/* url(no.png) */ #id > p[data-x="url(no.png)"] {
 content: "url(no.png)"; margin-left: 2em; margin: 0; color: red; color: blue !important;
 background: url(new/image.png); }`, []string{"image.png"}},
		{"quoted", `p { /* comment */ background: URL( 'image.png' ); }`,
			`p { /* comment */ background: url("new/image.png"); }`, []string{"image.png"}},
		{"imports", `@import "base.css" screen; @import url(other.css);`,
			`@import "new/base.css" screen; @import url(new/other.css);`, []string{"base.css", "other.css"}},
		{"escaped", `p { background: url(im\61 ge.png); src: url("a\ b.ttf"); }`,
			`p { background: url(new/image.png); src: url("new/a b.ttf"); }`, []string{"image.png", "a b.ttf"}},
		{"at rules", `@supports (display:grid) { .a + .b { background:url(a.png); } }`,
			`@supports (display:grid) { .a + .b { background:url(new/a.png); } }`, []string{"a.png"}},
		{"incomplete URL", `p { background: url(asset.png`, `p { background: url(asset.png`, nil},
		{"incomplete import", `@import"`, `@import"`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := []byte(test.input)
			before := bytes.Clone(input)
			var urls []string
			got, err := css.RewriteStylesheetURLs(input, func(original string) string {
				urls = append(urls, original)
				return "new/" + original
			})
			if err != nil || string(got) != test.want || !reflect.DeepEqual(urls, test.urls) || !bytes.Equal(input, before) {
				t.Fatalf("rewrite=%s, URLs=%v, error=%v", got, urls, err)
			}
			unchanged, err := css.RewriteStylesheetURLs(input, func(original string) string { return original })
			if err != nil || !bytes.Equal(unchanged, before) {
				t.Fatal("identity rewrite changed the stylesheet", err)
			}
		})
	}
}

func FuzzRewriteStylesheetURLs(f *testing.F) {
	for _, input := range []string{
		`p{background:url("a.png")}`, `@import 'a.css';`, `/* url(fake) */ p{content:"url(fake)"}`,
		`p{background:url(im\61 ge.png)}`, `p{background:url("unterminated`, "", "\x00",
	} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}
		data := []byte(input)
		before := bytes.Clone(data)
		got, err := css.RewriteStylesheetURLs(data, func(original string) string { return original })
		if err == nil && !bytes.Equal(got, before) {
			t.Fatalf("identity rewrite changed CSS: %q -> %q", input, got)
		}
		if !bytes.Equal(data, before) {
			t.Fatal("rewriter mutated its source")
		}
	})
}
