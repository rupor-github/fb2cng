package publication

import (
	"reflect"
	"strings"
	"testing"

	"fbc/fb2"
)

func TestAssembleStylesheet(t *testing.T) {
	font := fb2.StylesheetResource{OriginalURL: "#asset", Filename: "fonts/book.otf", Data: []byte("otf")}
	art := fb2.StylesheetResource{OriginalURL: "#asset", Filename: "other/background.png", Data: []byte("png")}
	styles := []fb2.Stylesheet{
		{Type: "ignored", Data: "ignored", Resources: []fb2.StylesheetResource{{Filename: "ignored"}}},
		{Type: "text/css", Data: "@font-face { src: url('#asset'); } body { color: red; }", Resources: []fb2.StylesheetResource{font}},
		{Type: "text/css", Data: "body { background: url('#asset'); color: blue; }", Resources: []fb2.StylesheetResource{art}},
		{Type: "text/css", Data: "p { background: url('unresolved.png'); }"},
	}
	assembled, err := AssembleStylesheet(styles, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(assembled.Resources, []fb2.StylesheetResource{font, art}) {
		t.Errorf("resources = %+v", assembled.Resources)
	}
	data := string(assembled.Data)
	for _, fragment := range []string{`url("fonts/book.otf")`, `url("other/background.png")`, `url("unresolved.png")`} {
		if !strings.Contains(data, fragment) {
			t.Errorf("missing CSS fragment %s in %s", fragment, data)
		}
	}
	if strings.Contains(data, "ignored") || strings.Index(data, "red") > strings.Index(data, "blue") {
		t.Error("stylesheet selection or cascade order changed")
	}
	if !strings.Contains(styles[1].Data, "#asset") || styles[1].Resources[0].OriginalURL != "#asset" {
		t.Error("assembly modified normalized source styles")
	}
}

func TestAssembleStylesheetEmpty(t *testing.T) {
	style, err := AssembleStylesheet(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(style.Data) != 0 || len(style.Resources) != 0 {
		t.Error("empty input produced stylesheet content")
	}
}
