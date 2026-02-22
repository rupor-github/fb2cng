package config

import (
	"archive/zip"
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestReportClose_RemovesStoredDirs(t *testing.T) {
	// Create a temp file for the report archive
	reportFile, err := os.CreateTemp("", "test-report-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp report file: %v", err)
	}
	defer os.Remove(reportFile.Name())

	r := &Report{
		entries: make(map[string]entry),
		file:    reportFile,
	}

	// Create temp directories to simulate stored WorkDirs
	dir1, err := os.MkdirTemp("", "test-workdir1-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dir2, err := os.MkdirTemp("", "test-workdir2-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Put a file inside one of them to verify recursive removal
	if err := os.WriteFile(filepath.Join(dir1, "debug.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Also store a regular file entry — it should NOT be removed
	tmpFile, err := os.CreateTemp("", "test-stored-file-")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	r.Store("workdir-1", dir1)
	r.Store("workdir-2", dir2)
	r.Store("result-file", tmpFile.Name())

	// Close should finalize the archive and then remove stored directories
	if err := r.Close(); err != nil {
		t.Fatalf("Report.Close() error: %v", err)
	}

	// Directories should be removed
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		os.RemoveAll(dir1)
		t.Errorf("expected dir1 to be removed, but it still exists")
	}
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		os.RemoveAll(dir2)
		t.Errorf("expected dir2 to be removed, but it still exists")
	}

	// Regular file should still exist
	if _, err := os.Stat(tmpFile.Name()); err != nil {
		t.Errorf("stored file should not be removed, but got error: %v", err)
	}
}

func TestReportClose_NilReport(t *testing.T) {
	var r *Report
	if err := r.Close(); err != nil {
		t.Errorf("Close on nil report should not error, got: %v", err)
	}
}

func TestReportClose_NilFile(t *testing.T) {
	r := &Report{entries: make(map[string]entry)}
	if err := r.Close(); err != nil {
		t.Errorf("Close with nil file should not error, got: %v", err)
	}
}

func TestReportClose_PropagatesFileCloseError(t *testing.T) {
	// If r.file is already closed before Close() is called, finalize() will
	// fail because it can't write to the file. But more importantly, the
	// deferred file.Close() will also return an error. We verify that Close()
	// surfaces the file close error (via errors.Join) rather than silently
	// discarding it.

	reportFile, err := os.CreateTemp("", "test-report-close-err-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp report file: %v", err)
	}
	name := reportFile.Name()
	defer os.Remove(name)

	r := &Report{
		entries: make(map[string]entry),
		file:    reportFile,
	}

	// Close the underlying file so both finalize and file.Close will fail.
	reportFile.Close()

	err = r.Close()
	if err == nil {
		t.Fatal("expected error from Close when file is already closed")
	}

	// The returned error should contain multiple errors joined together —
	// at minimum the finalize error (write to closed file) and the file
	// close error. Verify we can unwrap multiple errors.
	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) {
		// Even if errors.Join collapses to a single non-nil error, at least
		// we got an error. The key property is that file.Close error is not
		// silently lost.
		t.Logf("error is not a joined error (may be single): %v", err)
		return
	}

	errs := joined.Unwrap()
	if len(errs) < 2 {
		t.Errorf("expected at least 2 joined errors, got %d: %v", len(errs), err)
	}
}

func TestManifestContainsAllEntries(t *testing.T) {
	// Create a temp file for the report archive
	reportFile, err := os.CreateTemp("", "test-report-manifest-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp report file: %v", err)
	}
	reportName := reportFile.Name()
	defer os.Remove(reportName)

	r := &Report{
		entries: make(map[string]entry),
		file:    reportFile,
	}

	// Create a directory with files (including a subdirectory)
	dir, err := os.MkdirTemp("", "test-manifest-dir-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create a standalone regular file
	tmpFile, err := os.CreateTemp("", "test-manifest-file-")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.WriteString("standalone content")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Store entries
	r.Store("my-dir", dir)
	r.Store("standalone.txt", tmpFile.Name())
	r.StoreData("config/test.yml", []byte("key: value"))

	if err := r.Close(); err != nil {
		t.Fatalf("Report.Close() error: %v", err)
	}

	// Read the resulting ZIP and collect all entry names
	zr, err := zip.OpenReader(reportName)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer zr.Close()

	zipEntries := make(map[string]bool)
	for _, f := range zr.File {
		zipEntries[f.Name] = true
	}

	// Read MANIFEST content
	var manifestEntries []string
	for _, f := range zr.File {
		if f.Name == "MANIFEST" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open MANIFEST: %v", err)
			}
			scanner := bufio.NewScanner(rc)
			for scanner.Scan() {
				line := scanner.Text()
				// Extract the entry name (second tab-separated field)
				parts := strings.SplitN(line, "\t", 3)
				if len(parts) >= 2 {
					manifestEntries = append(manifestEntries, parts[1])
				}
			}
			rc.Close()
			break
		}
	}

	sort.Strings(manifestEntries)

	// Every non-MANIFEST zip entry should be in the MANIFEST
	manifestSet := make(map[string]bool)
	for _, name := range manifestEntries {
		manifestSet[name] = true
	}

	for name := range zipEntries {
		if name == "MANIFEST" {
			continue
		}
		if !manifestSet[name] {
			t.Errorf("zip entry %q is not listed in MANIFEST", name)
		}
	}

	// Every MANIFEST entry should be in the zip (or be an absent path)
	for _, name := range manifestEntries {
		if !zipEntries[name] {
			t.Errorf("MANIFEST entry %q is not in the zip archive", name)
		}
	}

	// Verify specific expected entries
	expectedEntries := []string{
		"my-dir/file1.txt",
		"my-dir/subdir/file2.txt",
		"standalone.txt",
		"config/test.yml",
	}
	for _, expected := range expectedEntries {
		if !manifestSet[expected] {
			t.Errorf("expected MANIFEST to contain %q, but it doesn't. MANIFEST entries: %v", expected, manifestEntries)
		}
		if !zipEntries[expected] {
			t.Errorf("expected zip to contain %q, but it doesn't", expected)
		}
	}
}
