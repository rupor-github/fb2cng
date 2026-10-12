package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"fbc/convert/kfx"
)

func TestUtilityStrictTransforms(t *testing.T) {
	inner, err := kfx.NewContainer().WriteContainer()
	if err != nil {
		t.Fatal(err)
	}
	compressed, _, err := transform(t.Context(), inner, "compress", false)
	if err != nil {
		t.Fatal(err)
	}
	plain, _, err := transform(t.Context(), inner, "compress", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range [][]byte{compressed, plain} {
		got, _, err := transform(t.Context(), input, "uncompress", false)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(inner, got) {
			t.Fatal("changed original CONT")
		}
	}
	if _, _, err := transform(t.Context(), plain, "compress", false); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		input     []byte
		operation string
		plaintext bool
	}{
		{"already compressed", compressed, "compress", false},
		{"already raw", inner, "uncompress", false},
		{"already control", plain, "compress", true},
		{"invalid", []byte("not CONT"), "compress", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := transform(t.Context(), test.input, test.operation, test.plaintext); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUtilityPublication(t *testing.T) {
	inner, err := kfx.NewContainer().WriteContainer()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "book.kfx")
	if err := os.WriteFile(path, inner, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"compress", path, path}, &bytes.Buffer{}); err == nil {
		t.Fatal("overwrote without flag")
	}
	for _, operation := range []string{"compress", "uncompress"} {
		if err := run(t.Context(), []string{operation, "--overwrite", path, path}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, inner) {
		t.Fatal("same-path round-trip changed input")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := writeAtomic(ctx, path, []byte("bad"), true); err == nil {
		t.Fatal("published after cancellation")
	}
	if err := writeAtomic(t.Context(), path, []byte("bad"), false); err == nil {
		t.Fatal("overwrote existing output")
	}
	got, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(got, inner) {
		t.Fatal("failed publication damaged destination", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary file leaked", err)
	}
}
