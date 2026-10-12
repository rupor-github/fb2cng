package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestPublicationLongBasenames(t *testing.T) {
	for _, test := range []struct {
		name     string
		basename string
	}{
		{"245 bytes", strings.Repeat("a", 240) + ".azw8"},
		{"255 bytes", strings.Repeat("a", 250) + ".azw8"},
		{"254 UTF-8 bytes", strings.Repeat("世", 83) + ".azw8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, test.basename)
			original, replacement := []byte("original"), []byte("replacement")
			if err := writeAtomic(t.Context(), path, original, false); err != nil {
				t.Fatal(err)
			}
			if err := writeAtomic(t.Context(), path, replacement, false); !errors.Is(err, os.ErrExist) {
				t.Fatalf("expected existing destination error, got %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatal("changed existing output", err)
			}
			if err := writeAtomic(t.Context(), path, replacement, true); err != nil {
				t.Fatal(err)
			}
			got, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(got, replacement) {
				t.Fatal("replacement output differs", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatal("temporary file leaked", err)
			}
		})
	}
}

func TestPublicationRefusesExistingDestination(t *testing.T) {
	for _, kind := range []string{"file", "directory", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "output.kfx")
			original := []byte("existing output")
			switch kind {
			case "file":
				if err := os.WriteFile(path, original, 0644); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "dangling symlink":
				if err := os.Symlink("missing", path); err != nil {
					t.Skipf("symbolic links unavailable: %v", err)
				}
			}
			if err := writeAtomic(t.Context(), path, []byte("replacement"), false); err == nil {
				t.Fatal("published over an existing destination")
			}
			switch kind {
			case "file":
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatal("changed existing output", err)
				}
			case "directory":
				info, err := os.Stat(path)
				if err != nil || !info.IsDir() {
					t.Fatal("changed existing directory", err)
				}
			case "dangling symlink":
				target, err := os.Readlink(path)
				if err != nil || target != "missing" {
					t.Fatal("changed existing symlink", err)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatal("temporary file leaked", err)
			}
		})
	}
}

func TestConcurrentNoOverwritePublication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.kfx")
	payloads := [][]byte{bytes.Repeat([]byte("first"), 1024), bytes.Repeat([]byte("second"), 1024)}
	start := make(chan struct{})
	results := make(chan error, len(payloads))
	var wg sync.WaitGroup
	for _, data := range payloads {
		wg.Go(func() {
			<-start
			results <- writeAtomic(t.Context(), path, data, false)
		})
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, os.ErrExist) {
			t.Fatalf("unexpected publication error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful publication, got %d", successes)
	}
	got, err := os.ReadFile(path)
	if err != nil || (!bytes.Equal(got, payloads[0]) && !bytes.Equal(got, payloads[1])) {
		t.Fatal("output is not a complete winning payload", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary file leaked", err)
	}
}
