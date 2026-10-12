package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fbc/convert/kfx"
	"fbc/convert/kfx/envelope"
)

func TestCompressedDumpsAndExtraction(t *testing.T) {
	c := kfx.NewContainer()
	if err := c.Fragments.Add(kfx.NewRootFragment(kfx.SymMetadata, kfx.NewStruct())); err != nil {
		t.Fatal(err)
	}
	inner, err := c.WriteContainer()
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := envelope.Encode(t.Context(), inner, envelope.LZMA)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "compressed.azw8")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	if err := run(t.Context(), []string{"--all", path}, &console, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-dump.txt", "-resources.zip", "-styles.txt", "-storyline.txt", "-margins.txt", "-envelope.txt"} {
		if _, err := os.Stat(filepath.Join(dir, "compressed"+suffix)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "compressed-inner.kfx")); !os.IsNotExist(err) {
		t.Fatal("--all extracted binary implicitly")
	}
	if !strings.Contains(console.String(), "compression=lzma") || !strings.Contains(console.String(), "Container compression: 0") {
		t.Fatal("compression layers missing", console.String())
	}
	if err := run(t.Context(), []string{"--extract-inner", path}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "compressed-inner.kfx"))
	if err != nil || !bytes.Equal(got, inner) {
		t.Fatal("extraction changed bytes", err)
	}
	// An opaque stream is inspectable without parsing it as CONT.
	opaque, _, err := envelope.Encode(t.Context(), []byte("opaque"), envelope.Plaintext)
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, "opaque.kfx")
	if err := os.WriteFile(path, opaque, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"--envelope", path}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
