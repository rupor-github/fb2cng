package pdf

import (
	"strings"
	"testing"
)

func TestPageContentDrawsImages(t *testing.T) {
	content := string(pageContent(pdfPage{Images: []pdfPageImage{{
		Name:   "Im1",
		X:      10,
		Y:      20,
		Width:  30,
		Height: 40,
	}}}))
	for _, want := range []string{"30 0 0 40 10 20 cm", "/Im1 Do"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}
