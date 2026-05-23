package epub

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestEPUBOutputLogFieldsIncludeTemporaryAndFinalPaths(t *testing.T) {
	fields := epubOutputLogFields("/tmp/.book.epub.123.tmp", "/tmp/book.epub")
	got := stringLogFields(fields)
	if got["output"] != "/tmp/book.epub" {
		t.Fatalf("output field = %q, want final path", got["output"])
	}
	if got["temporary_file"] != "/tmp/.book.epub.123.tmp" {
		t.Fatalf("temporary_file field = %q, want temporary path", got["temporary_file"])
	}
}

func stringLogFields(fields []zapcore.Field) map[string]string {
	out := make(map[string]string, len(fields))
	for _, field := range fields {
		if field.Type == zapcore.StringType {
			out[field.Key] = field.String
		}
	}
	return out
}
