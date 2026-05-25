package pdf

import (
	"testing"

	"fbc/css"
)

func TestPDFCSSColor(t *testing.T) {
	tests := []struct {
		name  string
		value css.Value
		want  string
	}{
		{name: "hex six", value: css.Value{Raw: "#336699", Keyword: "#336699"}, want: "#336699"},
		{name: "hex three", value: css.Value{Raw: "#369", Keyword: "#369"}, want: "#336699"},
		{name: "named", value: css.Value{Keyword: "red"}, want: "#ff0000"},
		{name: "rgb bytes", value: css.Value{Raw: "rgb(51, 102, 153)", Keyword: "rgb(51, 102, 153)"}, want: "#336699"},
		{name: "rgb percent", value: css.Value{Raw: "rgb(20%, 40%, 60%)", Keyword: "rgb(20%, 40%, 60%)"}, want: "#336699"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pdfCSSColor(tt.value)
			if !ok {
				t.Fatal("pdfCSSColor() did not parse value")
			}
			if got.String() != tt.want {
				t.Fatalf("pdfCSSColor() = %s, want %s", got, tt.want)
			}
		})
	}
}
