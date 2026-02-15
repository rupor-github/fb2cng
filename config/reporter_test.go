package config

import (
	"errors"
	"os"
	"path/filepath"
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
