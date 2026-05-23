package kfx

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestKFXOutputLogFieldsIncludeTemporaryAndFinalPaths(t *testing.T) {
	fields := kfxOutputLogFields("/tmp/.book.kfx.123.tmp", "/tmp/book.kfx")
	got := stringLogFields(fields)
	if got["output"] != "/tmp/book.kfx" {
		t.Fatalf("output field = %q, want final path", got["output"])
	}
	if got["temporary_file"] != "/tmp/.book.kfx.123.tmp" {
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
