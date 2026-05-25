package target

import (
	"testing"

	"fbc/common"
)

func TestSupported(t *testing.T) {
	for _, name := range []string{EPUB, MOBI, PDF} {
		if !Supported(name) {
			t.Fatalf("target %s should be supported", name)
		}
	}
	if Supported("fbc") {
		t.Fatal("unexpected support for direct fbc target")
	}
}

func TestDefaultOutputFormat(t *testing.T) {
	tests := []struct {
		name string
		want common.OutputFmt
	}{
		{EPUB, common.OutputFmtEpub2},
		{MOBI, common.OutputFmtKfx},
		{PDF, common.OutputFmtPdf},
	}
	for _, tt := range tests {
		if got := DefaultOutputFormat(tt.name); got != tt.want {
			t.Fatalf("default format for %s = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestSupportsOutputFormat(t *testing.T) {
	tests := []struct {
		name   string
		target string
		format common.OutputFmt
		want   bool
	}{
		{"epub2 for epub", EPUB, common.OutputFmtEpub2, true},
		{"epub3 for epub", EPUB, common.OutputFmtEpub3, true},
		{"kepub for epub", EPUB, common.OutputFmtKepub, true},
		{"pdf rejected for epub", EPUB, common.OutputFmtPdf, false},
		{"kfx rejected for epub", EPUB, common.OutputFmtKfx, false},
		{"kfx for mobi", MOBI, common.OutputFmtKfx, true},
		{"azw8 for mobi", MOBI, common.OutputFmtAzw8, true},
		{"epub rejected for mobi", MOBI, common.OutputFmtEpub2, false},
		{"pdf rejected for mobi", MOBI, common.OutputFmtPdf, false},
		{"pdf for pdf", PDF, common.OutputFmtPdf, true},
		{"epub rejected for pdf", PDF, common.OutputFmtEpub2, false},
		{"kfx rejected for pdf", PDF, common.OutputFmtKfx, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportsOutputFormat(tt.target, tt.format); got != tt.want {
				t.Fatalf("supports(%s, %s) = %t, want %t", tt.target, tt.format, got, tt.want)
			}
		})
	}
}
