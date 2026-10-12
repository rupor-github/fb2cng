package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/state"
)

func TestPrepareUsesFixedBookArtifactNames(t *testing.T) {
	_, ctx := setupTestContent(t)
	env := state.EnvFromContext(ctx)
	env.Cfg.Document.Images.Cover.Generate = false
	report, err := (&config.ReporterConfig{DestinationTemplate: filepath.Join(t.TempDir(), "report.zip")}).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	env.Rpt = report
	t.Cleanup(func() {
		if err := report.Close(); err != nil {
			t.Error(err)
		}
	})
	sourceName := strings.Repeat("Длинное имя книги ", 20) + ".fb2"
	xml := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Book</book-title><lang>en</lang></title-info>
<document-info><id>12345678-1234-1234-1234-123456789abc</id></document-info></description>
<body><section><p>Content</p></section></body></FictionBook>`
	c, err := Prepare(ctx, strings.NewReader(xml), sourceName, common.OutputFmtTxt, env.Log)
	if err != nil {
		t.Fatal(err)
	}
	if c.SrcName != sourceName {
		t.Fatal("debug naming changed the source name")
	}
	for _, name := range []string{"book.fb2", "book.pristine.txt", "book.prepared.txt"} {
		data, err := os.ReadFile(filepath.Join(c.WorkDir, name))
		if err != nil || len(data) == 0 {
			t.Fatal("missing book artifact", name, err)
		}
	}
	entries, err := os.ReadDir(c.WorkDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "book.") {
			t.Fatal("nonuniform artifact", entry.Name())
		}
	}
}
