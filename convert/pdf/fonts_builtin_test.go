package pdf

import (
	"strings"
	"testing"
)

func TestBuiltinFontSelectionAndMetrics(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		bold       bool
		italic     bool
		wantPrefix string
	}{
		{name: "default serif", family: "", wantPrefix: "NotoSerif"},
		{name: "serif bold italic", family: "serif", bold: true, italic: true, wantPrefix: "NotoSerif"},
		{name: "sans", family: "Noto Sans, sans-serif", wantPrefix: "NotoSans"},
		{name: "mono", family: "monospace", bold: true, wantPrefix: "NotoSansMono"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			face, err := builtinFont(tt.family, tt.bold, tt.italic)
			if err != nil {
				t.Fatalf("builtinFont() error = %v", err)
			}
			if face == nil {
				t.Fatal("builtinFont() returned nil")
			}
			if !strings.HasPrefix(face.PostScriptName, tt.wantPrefix) {
				t.Errorf("PostScriptName = %q, want prefix %q", face.PostScriptName, tt.wantPrefix)
			}
			if len(face.Data) == 0 {
				t.Error("font data is empty")
			}
			if face.Font == nil {
				t.Error("parsed sfnt font is nil")
			}
			if face.UnitsPerEm <= 0 {
				t.Errorf("UnitsPerEm = %d, want positive", face.UnitsPerEm)
			}
			if face.Ascent <= 0 {
				t.Errorf("Ascent = %d, want positive", face.Ascent)
			}
			if face.Descent >= 0 {
				t.Errorf("Descent = %d, want negative PDF descent", face.Descent)
			}
			if face.BBox[0] >= face.BBox[2] || face.BBox[1] >= face.BBox[3] {
				t.Errorf("invalid bounding box: %v", face.BBox)
			}
		})
	}
}

func TestBuiltinMonoItalicFallback(t *testing.T) {
	regular, err := builtinFont("monospace", false, false)
	if err != nil {
		t.Fatalf("builtinFont regular error = %v", err)
	}
	italic, err := builtinFont("monospace", false, true)
	if err != nil {
		t.Fatalf("builtinFont italic error = %v", err)
	}
	if regular != italic {
		t.Error("monospace italic should reuse regular because no bundled italic variant exists")
	}

	bold, err := builtinFont("monospace", true, false)
	if err != nil {
		t.Fatalf("builtinFont bold error = %v", err)
	}
	boldItalic, err := builtinFont("monospace", true, true)
	if err != nil {
		t.Fatalf("builtinFont bold italic error = %v", err)
	}
	if bold != boldItalic {
		t.Error("monospace bold italic should reuse bold because no bundled italic variant exists")
	}
}

func TestSanitizePDFName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Noto Serif 12pt Regular", want: "Noto-Serif-12pt-Regular"},
		{in: "NotoSans-Regular", want: "NotoSans-Regular"},
		{in: "bad/name#with spaces", want: "badnamewith-spaces"},
		{in: "", want: "FBCFont"},
		{in: "///", want: "FBCFont"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := sanitizePDFName(tt.in); got != tt.want {
				t.Errorf("sanitizePDFName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
