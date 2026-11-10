package common

import (
	"slices"
	"testing"
)

func TestAZW3FormatIdentity(t *testing.T) {
	for _, spelling := range []string{"azw3", "AZW3", "Azw3"} {
		t.Run(spelling, func(t *testing.T) {
			format, err := ParseOutputFmt(spelling)
			if err != nil || format != OutputFmtAzw3 || format.String() != "azw3" || !format.IsValid() {
				t.Fatalf("AZW3 identity = %v, %v", format, err)
			}
			if format.Ext() != ".azw3" || !format.IsKindle() || format.IsEPUB3() || format.IsKobo() {
				t.Fatal("AZW3 must be Kindle output, not EPUB or Kobo output")
			}
			data, err := format.MarshalText()
			if err != nil || string(data) != "azw3" {
				t.Fatal("AZW3 text encoding failed", err)
			}
			var decoded OutputFmt
			if err := decoded.UnmarshalText(data); err != nil || decoded != format {
				t.Fatal("AZW3 text decoding failed", err)
			}
		})
	}
	if !slices.Contains(OutputFmtNames(), "azw3") {
		t.Fatal("AZW3 is missing from the supported format names")
	}
	// Appending the new format must not renumber existing formats used by clients.
	for i, format := range []OutputFmt{
		OutputFmtEpub2, OutputFmtEpub3, OutputFmtKepub, OutputFmtKepub3, OutputFmtKfx,
		OutputFmtAzw8, OutputFmtPdf, OutputFmtTxt, OutputFmtMd,
	} {
		if int(format) != i {
			t.Errorf("existing format %s was renumbered: %d, want %d", format, format, i)
		}
	}
}
