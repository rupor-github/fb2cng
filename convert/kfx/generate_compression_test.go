package kfx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/state"
)

func TestGenerateCompressedPreservesInnerAndDiagnostics(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	ctx := state.ContextWithEnv(t.Context())
	env := state.EnvFromContext(ctx)
	env.Cfg, env.Log = cfg, zap.NewNop()
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Compression test</book-title><lang>en</lang></title-info>
<document-info><id>compression-test-fixed-id</id></document-info></description>
<body><section><title><p>Chapter</p></title><p>Unicode 世界 and opaque byte preservation.</p></section></body></FictionBook>`
	for _, format := range []common.OutputFmt{common.OutputFmtKfx, common.OutputFmtAzw8} {
		t.Run(format.String(), func(t *testing.T) {
			c, err := content.Prepare(ctx, strings.NewReader(xml), "test.fb2", format, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(c.WorkDir) })
			c.Debug = true
			dir := t.TempDir()
			raw := filepath.Join(dir, "raw"+format.Ext())
			cfg.Document.KFX.Compression = "none"
			if err := Generate(ctx, c, raw, &cfg.Document, env.Log); err != nil {
				t.Fatal(err)
			}
			baseline, err := os.ReadFile(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(baseline, []byte("CONT")) {
				t.Fatal("default output is not CONT")
			}
			if _, err := os.Stat(filepath.Join(c.WorkDir, "book.debug.txt")); err != nil {
				t.Fatal("missing debug output when no separate final path is supplied", err)
			}
			cfg.Document.KFX.Compression = "lzma"
			cfg.Document.KFX.CompressionSource = "cli"
			// The result name is valid on common filesystems, but appending diagnostic
			// suffixes to it would exceed their 255-byte filename limit.
			out := filepath.Join(dir, strings.Repeat("Книга", 24)+"-long"+format.Ext())
			temporary := filepath.Join(dir, ".compressed"+format.Ext()+".123.tmp")
			if err := Generate(ctx, c, temporary, &cfg.Document, env.Log, out); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(temporary)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ReadFile(t.Context(), data)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(parsed.InnerBytes, baseline) {
				t.Fatal("compression modified inner generation")
			}
			original, err := os.ReadFile(filepath.Join(c.WorkDir, "book.inner.kfx"))
			if err != nil || !bytes.Equal(original, baseline) {
				t.Fatal("missing or changed report baseline", err)
			}
			for _, suffix := range []string{".envelope.txt", ".debug.txt"} {
				text, err := os.ReadFile(filepath.Join(c.WorkDir, "book"+suffix))
				if err != nil || !strings.Contains(string(text), "compression=lzma") {
					t.Fatal("missing compression diagnostics", err)
				}
				if suffix == ".debug.txt" && !strings.Contains(string(text), "source: cli") {
					t.Fatal("missing effective choice source")
				}
			}
			for _, suffix := range []string{".debug.txt", ".envelope.txt", ".inner.kfx"} {
				if _, err := os.Stat(filepath.Join(c.WorkDir, filepath.Base(temporary)+suffix)); !os.IsNotExist(err) {
					t.Fatal("diagnostic artifact used temporary output basename", suffix, err)
				}
			}
		})
	}
}
