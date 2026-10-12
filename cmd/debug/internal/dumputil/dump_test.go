package dumputil

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fbc/convert/kfx"
)

func resourceContainer(t *testing.T, name string) *kfx.Container {
	t.Helper()
	c := kfx.NewContainer()
	frag := kfx.NewRootFragment(kfx.SymRawMedia, kfx.RawValue([]byte("resource data")))
	frag.FIDName = name
	if err := c.Fragments.Add(frag); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDumpOutputsProtectExistingDestinations(t *testing.T) {
	c := resourceContainer(t, "media")
	for _, writer := range []struct {
		name, suffix string
		write        func(string, bool) error
	}{
		{"text", "-dump.txt", func(dir string, overwrite bool) error {
			return WriteOutput("book.kfx", dir, "-dump.txt", []byte("text output"), overwrite)
		}},
		{"resources", "-resources.zip", func(dir string, overwrite bool) error {
			return DumpResources(c, "book.kfx", dir, overwrite)
		}},
	} {
		for _, kind := range []string{"dangling symlink", "file", "symlink", "directory"} {
			t.Run(writer.name+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "book"+writer.suffix)
				target := filepath.Join(dir, "target")
				original := []byte("existing output")
				entriesWant := 1
				switch kind {
				case "file":
					if err := os.WriteFile(path, original, 0644); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(path, 0755); err != nil {
						t.Fatal(err)
					}
				default:
					if kind == "symlink" {
						if err := os.WriteFile(target, original, 0644); err != nil {
							t.Fatal(err)
						}
						entriesWant++
					}
					if err := os.Symlink("target", path); err != nil {
						t.Skipf("symbolic links unavailable: %v", err)
					}
				}
				if err := writer.write(dir, false); !errors.Is(err, os.ErrExist) {
					t.Fatalf("expected existing destination error, got %v", err)
				}
				if kind == "file" {
					data, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(data, original) {
						t.Fatalf("existing output changed: %q, err=%v", data, err)
					}
				}
				if kind == "dangling symlink" || kind == "symlink" {
					link, err := os.Readlink(path)
					if err != nil || link != "target" {
						t.Fatalf("existing symlink changed: %q, err=%v", link, err)
					}
				}
				if kind != "directory" {
					if err := writer.write(dir, true); err != nil {
						t.Fatal(err)
					}
					info, err := os.Lstat(path)
					if err != nil || !info.Mode().IsRegular() {
						t.Fatalf("replacement is not a regular file: %v", err)
					}
					checkDumpOutput(t, path, writer.name == "resources")
				}
				if kind == "dangling symlink" {
					if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("created dangling symlink target: %v", err)
					}
				} else if kind == "symlink" {
					data, err := os.ReadFile(target)
					if err != nil || !bytes.Equal(data, original) {
						t.Fatalf("symlink target changed: %q, err=%v", data, err)
					}
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != entriesWant {
					t.Fatalf("temporary output leaked: entries=%v, err=%v", entries, err)
				}
			})
		}
	}
}

func checkDumpOutput(t *testing.T, path string, resources bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !resources {
		if !bytes.Equal(data, []byte("text output")) {
			t.Fatalf("unexpected text output: %q", data)
		}
		return
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(r.File) != 1 {
		t.Fatalf("invalid resource ZIP: %v", err)
	}
	f, err := r.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	blob, readErr := io.ReadAll(f)
	if err := errors.Join(readErr, f.Close()); err != nil || !bytes.Equal(blob, []byte("resource data")) {
		t.Fatalf("invalid resource contents: %q, err=%v", blob, err)
	}
}

func TestDumpResourcesFailurePreservesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book-resources.zip")
	original := []byte("existing archive")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	// ZIP entry names cannot exceed 65535 bytes, so archive generation fails.
	c := resourceContainer(t, strings.Repeat("a", 65536))
	if err := DumpResources(c, "book.kfx", dir, true); err == nil {
		t.Fatal("expected ZIP generation error")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("failed overwrite changed existing archive: size=%d, err=%v", len(data), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary output leaked: entries=%v, err=%v", entries, err)
	}
}

func TestOutputPublicationDestinationCreatedDuringWrite(t *testing.T) {
	for _, test := range []struct {
		name      string
		overwrite bool
	}{
		{"no overwrite", false}, {"overwrite", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "output.txt")
			original, replacement := []byte("another writer's output"), []byte("completed output")
			err := writeOutputFile(path, test.overwrite, func(out io.Writer) error {
				if _, err := out.Write(replacement); err != nil {
					return err
				}
				return os.WriteFile(path, original, 0644)
			})
			want := original
			if test.overwrite {
				if err != nil {
					t.Fatalf("overwrite failed: %v", err)
				}
				want = replacement
			} else if !errors.Is(err, os.ErrExist) {
				t.Fatalf("expected existing destination error, got %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, want) {
				t.Fatalf("unexpected output: %q, want %q, err=%v", data, want, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary output leaked: entries=%v, err=%v", entries, err)
			}
		})
	}
}
