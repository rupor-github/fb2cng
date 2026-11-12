package azw3

import (
	"strings"
	"testing"

	"fbc/convert/publication"
	"fbc/fb2"
)

func TestAssembleAZW3StylesheetPreservesCascade(t *testing.T) {
	data := `/* native CSS */ #id > p + p { margin-left: 2em; margin: 0; color: red; color: blue !important; }
@supports (display: grid) { .a[data-x] { background: url('#asset'); } }`
	style, err := publication.AssembleStylesheet([]fb2.Stylesheet{{Type: "text/css", Data: data,
		Resources: []fb2.StylesheetResource{{OriginalURL: "#asset", Filename: "other/art.png"}},
	}}, publicationOptions().RewriteStylesheet)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(style.Data), strings.ReplaceAll(data, "url('#asset')", `url("other/art.png")`)+"\n"; got != want {
		t.Fatalf("native stylesheet changed declarations, selectors or at-rules:\n%s", got)
	}
}
