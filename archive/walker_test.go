package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWalk(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a test zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)

	// Add files with different prefixes
	files := []struct {
		name    string
		content string
	}{
		{"docs/readme.txt", "readme content"},
		{"docs/guide.txt", "guide content"},
		{"src/main.go", "main content"},
		{"src/test.go", "test content"},
		{"config.yml", "config content"},
	}

	for _, f := range files {
		fw, err := w.Create(f.name)
		if err != nil {
			t.Fatalf("Failed to create file %s in zip: %v", f.name, err)
		}
		if _, err := fw.Write([]byte(f.content)); err != nil {
			t.Fatalf("Failed to write content for %s: %v", f.name, err)
		}
	}

	w.Close()
	zipFile.Close()

	t.Run("walk with docs prefix", func(t *testing.T) {
		var visited []string
		err := Walk(zipPath, "docs/", func(archive string, file *zip.File) error {
			if archive != zipPath {
				t.Errorf("archive = %s, want %s", archive, zipPath)
			}
			visited = append(visited, file.Name)
			return nil
		})

		if err != nil {
			t.Errorf("Walk() error = %v", err)
		}

		if len(visited) != 2 {
			t.Errorf("visited %d files, want 2", len(visited))
		}

		expected := map[string]bool{
			"docs/readme.txt": true,
			"docs/guide.txt":  true,
		}
		for _, name := range visited {
			if !expected[name] {
				t.Errorf("unexpected file visited: %s", name)
			}
		}
	})

	t.Run("walk with src prefix", func(t *testing.T) {
		var visited []string
		err := Walk(zipPath, "src/", func(archive string, file *zip.File) error {
			visited = append(visited, file.Name)
			return nil
		})

		if err != nil {
			t.Errorf("Walk() error = %v", err)
		}

		if len(visited) != 2 {
			t.Errorf("visited %d files, want 2", len(visited))
		}
	})

	t.Run("walk with no matching prefix", func(t *testing.T) {
		var visited []string
		err := Walk(zipPath, "nonexistent/", func(archive string, file *zip.File) error {
			visited = append(visited, file.Name)
			return nil
		})

		if err != nil {
			t.Errorf("Walk() error = %v", err)
		}

		if len(visited) != 0 {
			t.Errorf("visited %d files, want 0", len(visited))
		}
	})

	t.Run("walk with empty prefix", func(t *testing.T) {
		var visited []string
		err := Walk(zipPath, "", func(archive string, file *zip.File) error {
			visited = append(visited, file.Name)
			return nil
		})

		if err != nil {
			t.Errorf("Walk() error = %v", err)
		}

		if len(visited) != 5 {
			t.Errorf("visited %d files, want 5", len(visited))
		}
	})

	t.Run("walkFn returns error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		err := Walk(zipPath, "docs/", func(archive string, file *zip.File) error {
			return expectedErr
		})

		if err != expectedErr {
			t.Errorf("Walk() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestWalk_InvalidArchive(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		err := Walk("/nonexistent/file.zip", "", func(archive string, file *zip.File) error {
			return nil
		})

		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("invalid zip file", func(t *testing.T) {
		tmpDir := t.TempDir()
		invalidZip := filepath.Join(tmpDir, "invalid.zip")

		if err := os.WriteFile(invalidZip, []byte("not a zip file"), 0644); err != nil {
			t.Fatalf("Failed to create invalid zip: %v", err)
		}

		err := Walk(invalidZip, "", func(archive string, file *zip.File) error {
			return nil
		})

		if err == nil {
			t.Error("Expected error for invalid zip file")
		}
	})
}

func TestWalk_WithDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip with directories
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)

	// Add directory entries (usually created by zip utilities)
	dirHeader := &zip.FileHeader{
		Name: "mydir/",
	}
	dirHeader.SetMode(os.ModeDir | 0755)
	if _, err := w.CreateHeader(dirHeader); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Add a file
	fw, err := w.Create("mydir/file.txt")
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	fw.Write([]byte("content"))

	w.Close()
	zipFile.Close()

	// Walk should not call walkFn for directories
	var visited []string
	err = Walk(zipPath, "mydir/", func(archive string, file *zip.File) error {
		visited = append(visited, file.Name)
		return nil
	})

	if err != nil {
		t.Errorf("Walk() error = %v", err)
	}

	// Should only visit the file, not the directory
	if len(visited) != 1 {
		t.Errorf("visited %d entries, want 1 (file only, not directory)", len(visited))
	}

	if len(visited) > 0 && visited[0] != "mydir/file.txt" {
		t.Errorf("visited %s, want mydir/file.txt", visited[0])
	}
}

func TestWalk_EarlyTermination(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip with multiple files
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)
	for i := 0; i < 5; i++ {
		fw, err := w.Create(filepath.Join("files", "file"+string(rune('0'+i))+".txt"))
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		fw.Write([]byte("content"))
	}
	w.Close()
	zipFile.Close()

	// Walk should stop when walkFn returns error
	var visited int
	stopErr := errors.New("stop walking")
	err = Walk(zipPath, "files/", func(archive string, file *zip.File) error {
		visited++
		if visited == 2 {
			return stopErr
		}
		return nil
	})

	if err != stopErr {
		t.Errorf("Walk() error = %v, want %v", err, stopErr)
	}

	if visited != 2 {
		t.Errorf("visited %d files, want 2 (early termination)", visited)
	}
}

func TestWalk_FileContent(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)
	content := []byte("test content")
	fw, err := w.Create("test.txt")
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}
	fw.Write(content)
	w.Close()
	zipFile.Close()

	// Walk and read file content
	err = Walk(zipPath, "", func(archive string, file *zip.File) error {
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			return err
		}

		if !bytes.Equal(buf.Bytes(), content) {
			t.Errorf("content = %s, want %s", buf.Bytes(), content)
		}

		return nil
	})

	if err != nil {
		t.Errorf("Walk() error = %v", err)
	}
}

func TestWalkFunc(t *testing.T) {
	// Test that WalkFunc has the correct signature
	var fn WalkFunc = func(archive string, file *zip.File) error {
		return nil
	}

	// Should be able to call it
	err := fn("test.zip", nil)
	if err != nil {
		t.Error("WalkFunc should work with nil file")
	}
}

func TestWalk_CaseSensitivity(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)
	fw, err := w.Create("Docs/README.txt")
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	fw.Write([]byte("content"))
	w.Close()
	zipFile.Close()

	// Prefix matching is case-sensitive
	t.Run("case sensitive match", func(t *testing.T) {
		var visited int
		err := Walk(zipPath, "Docs/", func(archive string, file *zip.File) error {
			visited++
			return nil
		})
		if err != nil {
			t.Errorf("Walk() error = %v", err)
		}
		if visited != 1 {
			t.Errorf("visited %d files with 'Docs/', want 1", visited)
		}
	})

	t.Run("case sensitive no match", func(t *testing.T) {
		var visited int
		err := Walk(zipPath, "docs/", func(archive string, file *zip.File) error {
			visited++
			return nil
		})
		if err != nil {
			t.Errorf("Walk() error = %v", err)
		}
		if visited != 0 {
			t.Errorf("visited %d files with 'docs/', want 0", visited)
		}
	})
}
