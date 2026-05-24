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
			if face.TextFace == nil {
				t.Error("parsed OpenType text face is nil")
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

func TestFontEmbeddingFSTypeReadsOS2EmbeddingFlags(t *testing.T) {
	fontData, err := gunzipFont(notoSerifRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
	if got := fontEmbeddingFSType(fontData); got != 0 {
		t.Fatalf("bundled font fsType = %#04x, want installable embedding", got)
	}
	patched := append([]byte(nil), fontData...)
	if !patchFontEmbeddingFSType(patched, 0x0102) {
		t.Fatal("patchFontEmbeddingFSType() = false")
	}
	if got := fontEmbeddingFSType(patched); got != 0x0102 {
		t.Fatalf("patched fsType = %#04x, want 0x0102", got)
	}
}

func patchFontEmbeddingFSType(data []byte, fsType uint16) bool {
	os2Table, ok := rawTTFTable(data, "OS/2")
	if !ok || len(os2Table) < 10 {
		return false
	}
	os2Table[8] = byte(fsType >> 8)
	os2Table[9] = byte(fsType)
	return true
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
